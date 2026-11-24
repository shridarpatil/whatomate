<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Plus,
  Trash2,
  GripVertical,
  Type,
  TextCursorInput,
  CircleDot,
  CheckSquare,
  Calendar,
  ChevronDown,
  Image,
  ArrowRight,
  ArrowUp,
  ArrowDown,
  Settings2,
  Layers
} from '@/lib/icons'

const { t } = useI18n()

// Component types available in WhatsApp Flows
const componentTypes = [
  { type: 'TextHeading', labelKey: 'waFlowBuilder.types.heading', icon: Type },
  { type: 'TextSubheading', labelKey: 'waFlowBuilder.types.subheading', icon: Type },
  { type: 'TextBody', labelKey: 'waFlowBuilder.types.text', icon: Type },
  { type: 'TextInput', labelKey: 'waFlowBuilder.types.textInput', icon: TextCursorInput },
  { type: 'TextArea', labelKey: 'waFlowBuilder.types.textArea', icon: TextCursorInput },
  { type: 'Dropdown', labelKey: 'waFlowBuilder.types.dropdown', icon: ChevronDown },
  { type: 'RadioButtonsGroup', labelKey: 'waFlowBuilder.types.radioButtons', icon: CircleDot },
  { type: 'CheckboxGroup', labelKey: 'waFlowBuilder.types.checkboxes', icon: CheckSquare },
  { type: 'DatePicker', labelKey: 'waFlowBuilder.types.datePicker', icon: Calendar },
  { type: 'Image', labelKey: 'waFlowBuilder.types.image', icon: Image },
  { type: 'Footer', labelKey: 'waFlowBuilder.types.footerButton', icon: ArrowRight },
]

interface FlowComponent {
  id: string
  type: string
  name?: string
  label?: string
  text?: string
  required?: boolean
  'data-source'?: any[]
  'on-click-action'?: any
  [key: string]: any
}

interface FlowScreen {
  id: string
  title: string
  data: Record<string, any>
  layout: {
    type: string
    children: FlowComponent[]
  }
}

interface Props {
  modelValue?: { screens: FlowScreen[] }
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: () => ({ screens: [] })
})

const emit = defineEmits<{
  'update:modelValue': [value: { screens: FlowScreen[] }]
}>()

const screens = ref<FlowScreen[]>(props.modelValue?.screens || [])
const selectedScreenIndex = ref<number>(0)
const selectedComponentIndex = ref<number | null>(null)

// Watch for external changes
watch(() => props.modelValue, (newVal) => {
  if (newVal?.screens) {
    screens.value = newVal.screens
  }
}, { deep: true })

// Emit changes
watch(screens, (newScreens) => {
  emit('update:modelValue', { screens: newScreens })
}, { deep: true })

const selectedScreen = computed(() => screens.value[selectedScreenIndex.value])
const selectedComponent = computed(() => {
  if (selectedComponentIndex.value === null || !selectedScreen.value) return null
  return selectedScreen.value.layout.children[selectedComponentIndex.value]
})

// Generate a unique ID using only alphabets and underscores (Meta requirement)
function generateId() {
  const chars = 'abcdefghijklmnopqrstuvwxyz'
  let result = 'id_'
  for (let i = 0; i < 12; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  return result
}

// Generate a user-friendly field name based on component type
function generateFieldName(type: string): string {
  // Count existing fields of this type across all screens
  let count = 1
  const prefix = type === 'TextInput' ? 'text_input' :
                 type === 'TextArea' ? 'text_area' :
                 type === 'Dropdown' ? 'dropdown' :
                 type === 'RadioButtonsGroup' ? 'radio' :
                 type === 'CheckboxGroup' ? 'checkbox' :
                 type === 'DatePicker' ? 'date' : 'field'

  for (const screen of screens.value) {
    for (const comp of screen.layout.children) {
      if (comp.name?.startsWith(prefix)) {
        count++
      }
    }
  }

  return `${prefix}_${count}`
}

// Convert number to letter sequence (1=A, 2=B, ..., 27=AA, etc.)
function numberToLetters(num: number): string {
  let result = ''
  while (num > 0) {
    num--
    result = String.fromCharCode(65 + (num % 26)) + result
    num = Math.floor(num / 26)
  }
  return result
}

function addScreen() {
  const screenNum = screens.value.length + 1
  screens.value.push({
    id: `SCREEN_${numberToLetters(screenNum)}`,
    title: `Screen ${screenNum}`,
    data: {},
    layout: {
      type: 'SingleColumnLayout',
      children: []
    }
  })
  selectedScreenIndex.value = screens.value.length - 1
  selectedComponentIndex.value = null
}

function removeScreen(index: number) {
  screens.value.splice(index, 1)
  if (selectedScreenIndex.value >= screens.value.length) {
    selectedScreenIndex.value = Math.max(0, screens.value.length - 1)
  }
  selectedComponentIndex.value = null
}

function selectScreen(index: number) {
  selectedScreenIndex.value = index
  selectedComponentIndex.value = null
}

function addComponent(type: string) {
  if (!selectedScreen.value) return

  const component: FlowComponent = {
    id: generateId(),
    type
  }

  // Set default properties based on type
  switch (type) {
    case 'TextHeading':
    case 'TextSubheading':
    case 'TextBody':
      component.text = 'Enter text here'
      break
    case 'TextInput':
      component.name = generateFieldName(type)
      component.label = 'Label'
      component.required = false
      component['input-type'] = 'text'
      break
    case 'TextArea':
      component.name = generateFieldName(type)
      component.label = 'Label'
      component.required = false
      break
    case 'Dropdown':
      component.name = generateFieldName(type)
      component.label = 'Select an option'
      component.required = false
      component['data-source'] = [
        { id: 'option_a', title: 'Option 1' },
        { id: 'option_b', title: 'Option 2' }
      ]
      break
    case 'RadioButtonsGroup':
      component.name = generateFieldName(type)
      component.label = 'Choose one'
      component.required = false
      component['data-source'] = [
        { id: 'option_a', title: 'Option 1' },
        { id: 'option_b', title: 'Option 2' }
      ]
      break
    case 'CheckboxGroup':
      component.name = generateFieldName(type)
      component.label = 'Select options'
      component.required = false
      component['data-source'] = [
        { id: 'option_a', title: 'Option 1' },
        { id: 'option_b', title: 'Option 2' }
      ]
      break
    case 'DatePicker':
      component.name = generateFieldName(type)
      component.label = 'Select date'
      component.required = false
      break
    case 'Image':
      component.src = ''
      component['aspect-ratio'] = 1
      break
    case 'Footer':
      component.label = 'Continue'
      component['on-click-action'] = {
        name: 'complete',
        payload: {}
      }
      break
  }

  selectedScreen.value.layout.children.push(component)
  selectedComponentIndex.value = selectedScreen.value.layout.children.length - 1
}

function removeComponent(index: number) {
  if (!selectedScreen.value) return
  selectedScreen.value.layout.children.splice(index, 1)
  selectedComponentIndex.value = null
}

function selectComponent(index: number) {
  selectedComponentIndex.value = index
}

function moveComponent(index: number, direction: 'up' | 'down') {
  if (!selectedScreen.value) return
  const children = selectedScreen.value.layout.children
  const newIndex = direction === 'up' ? index - 1 : index + 1

  if (newIndex < 0 || newIndex >= children.length) return

  const temp = children[index]
  children[index] = children[newIndex]
  children[newIndex] = temp

  selectedComponentIndex.value = newIndex
}

function updateComponentProperty(key: string, value: any) {
  if (selectedComponentIndex.value === null || !selectedScreen.value) return
  selectedScreen.value.layout.children[selectedComponentIndex.value][key] = value
}

function addOption() {
  if (!selectedComponent.value || !selectedComponent.value['data-source']) return
  selectedComponent.value['data-source'].push({
    id: generateId(),
    title: 'New Option'
  })
}

function removeOption(index: number) {
  if (!selectedComponent.value || !selectedComponent.value['data-source']) return
  selectedComponent.value['data-source'].splice(index, 1)
}

function updateOption(index: number, key: string, value: string) {
  if (!selectedComponent.value || !selectedComponent.value['data-source']) return
  selectedComponent.value['data-source'][index][key] = value
}

function getComponentLabel(comp: FlowComponent): string {
  const typeInfo = componentTypes.find(t => t.type === comp.type)
  return typeInfo ? t(typeInfo.labelKey) : comp.type
}

function getComponentIcon(type: string) {
  return componentTypes.find(t => t.type === type)?.icon || Type
}

// Components that should NOT have an 'id' property when sent to Meta API
const componentsWithoutId = [
  'TextHeading',
  'TextSubheading',
  'TextBody',
  'TextInput',
  'TextArea',
  'Dropdown',
  'RadioButtonsGroup',
  'CheckboxGroup',
  'DatePicker',
  'Image',
  'Footer'
]

// Sanitize flow JSON for Meta API by removing 'id' from components that don't support it
function sanitizeFlowForMeta(flowData: { screens: FlowScreen[] }): { screens: any[] } {
  return {
    screens: flowData.screens.map(screen => ({
      id: screen.id,
      title: screen.title,
      data: screen.data,
      layout: {
        type: screen.layout.type,
        children: screen.layout.children.map(comp => {
          // Create a copy without the 'id' if component type doesn't support it
          const { id, ...rest } = comp
          if (componentsWithoutId.includes(comp.type)) {
            return rest
          }
          return comp
        })
      }
    }))
  }
}

// Expose sanitize function for parent components
defineExpose({
  sanitizeFlowForMeta
})
</script>

<template>
  <div class="row h-full">
    <!-- Screens Panel -->
    <article class="card col-3 flex flex-col overflow-hidden min-h-0">
      <header class="hstack justify-between">
        <h3 class="hstack gap-2">
          <Layers :size="16" />
          {{ t('waFlowBuilder.screens') }}
        </h3>
        <button type="button" class="ghost small icon" :title="t('waFlowBuilder.addScreen')" :aria-label="t('waFlowBuilder.addScreen')" @click="addScreen">
          <Plus :size="16" />
        </button>
      </header>
      <div class="scroll-y flex-1 vstack gap-1">
        <div v-for="(screen, index) in screens" :key="screen.id" class="hstack gap-1">
          <button
            type="button"
            class="flex-1 truncate"
            :class="{ ghost: selectedScreenIndex !== index }"
            :aria-current="selectedScreenIndex === index ? 'true' : undefined"
            @click="selectScreen(index)"
          >
            <GripVertical :size="16" />
            {{ screen.title }}
          </button>
          <button
            v-if="screens.length > 1"
            type="button"
            class="ghost small icon"
            :title="t('waFlowBuilder.removeScreen')"
            :aria-label="t('waFlowBuilder.removeScreen')"
            @click="removeScreen(index)"
          >
            <Trash2 :size="14" />
          </button>
        </div>
        <p v-if="screens.length === 0" class="align-center text-light">
          {{ t('waFlowBuilder.noScreens') }}
        </p>
      </div>
    </article>

    <!-- Screen Editor -->
    <article class="card col-6 flex flex-col overflow-hidden min-h-0">
      <header>
        <div v-if="selectedScreen" class="hstack gap-2">
          <input
            v-model="selectedScreen.title"
            :aria-label="t('waFlowBuilder.screenTitle')"
            :placeholder="t('waFlowBuilder.screenTitle')"
          />
          <span class="badge outline">{{ selectedScreen.id }}</span>
        </div>
        <h3 v-else>{{ t('waFlowBuilder.selectScreen') }}</h3>
      </header>

      <div v-if="selectedScreen" class="row flex-1 min-h-0">
        <!-- Component Palette -->
        <div class="col-4 scroll-y">
          <p class="text-light mb-2"><small>{{ t('waFlowBuilder.addComponents') }}</small></p>
          <div class="row">
            <button
              v-for="comp in componentTypes"
              :key="comp.type"
              type="button"
              class="outline small col-6 vstack"
              @click="addComponent(comp.type)"
            >
              <component :is="comp.icon" :size="16" />
              <small>{{ t(comp.labelKey) }}</small>
            </button>
          </div>
        </div>

        <!-- Screen Preview -->
        <div class="col-8 scroll-y">
          <h3 class="mb-4">{{ selectedScreen.title }}</h3>
          <div class="vstack gap-2">
            <div
              v-for="(comp, index) in selectedScreen.layout.children"
              :key="comp.id"
              tabindex="0"
              :class="selectedComponentIndex === index ? 'card' : 'p-4'"
              :aria-current="selectedComponentIndex === index ? 'true' : undefined"
              @click="selectComponent(index)"
              @keydown.enter.self="selectComponent(index)"
            >
              <!-- Text Components -->
              <template v-if="comp.type === 'TextHeading'">
                <h2>{{ comp.text }}</h2>
              </template>
              <template v-else-if="comp.type === 'TextSubheading'">
                <h3>{{ comp.text }}</h3>
              </template>
              <template v-else-if="comp.type === 'TextBody'">
                <p>{{ comp.text }}</p>
              </template>

              <!-- Input Components -->
              <template v-else-if="comp.type === 'TextInput' || comp.type === 'TextArea'">
                <label data-field>
                  <span>{{ comp.label }}<span v-if="comp.required"> *</span></span>
                  <input v-if="comp.type === 'TextInput'" disabled :placeholder="comp.label" />
                  <textarea v-else disabled :placeholder="comp.label" rows="3" />
                </label>
              </template>

              <!-- Dropdown -->
              <template v-else-if="comp.type === 'Dropdown'">
                <label data-field>
                  <span>{{ comp.label }}<span v-if="comp.required"> *</span></span>
                  <select disabled>
                    <option>{{ t('waFlowBuilder.selectPlaceholder') }}</option>
                  </select>
                </label>
              </template>

              <!-- Radio Buttons -->
              <template v-else-if="comp.type === 'RadioButtonsGroup'">
                <fieldset>
                  <legend>{{ comp.label }}<span v-if="comp.required"> *</span></legend>
                  <label v-for="opt in comp['data-source']" :key="opt.id">
                    <input type="radio" disabled />
                    {{ opt.title }}
                  </label>
                </fieldset>
              </template>

              <!-- Checkboxes -->
              <template v-else-if="comp.type === 'CheckboxGroup'">
                <fieldset>
                  <legend>{{ comp.label }}<span v-if="comp.required"> *</span></legend>
                  <label v-for="opt in comp['data-source']" :key="opt.id">
                    <input type="checkbox" disabled />
                    {{ opt.title }}
                  </label>
                </fieldset>
              </template>

              <!-- Date Picker -->
              <template v-else-if="comp.type === 'DatePicker'">
                <label data-field>
                  <span>{{ comp.label }}<span v-if="comp.required"> *</span></span>
                  <input type="date" disabled />
                </label>
              </template>

              <!-- Image -->
              <template v-else-if="comp.type === 'Image'">
                <div class="vstack items-center p-4">
                  <Image :size="32" />
                </div>
              </template>

              <!-- Footer -->
              <template v-else-if="comp.type === 'Footer'">
                <button type="button" class="w-100">{{ comp.label }}</button>
              </template>

              <!-- Generic fallback -->
              <template v-else>
                <div class="hstack gap-2 text-light">
                  <component :is="getComponentIcon(comp.type)" :size="16" />
                  {{ getComponentLabel(comp) }}
                </div>
              </template>
            </div>

            <p v-if="selectedScreen.layout.children.length === 0" class="align-center text-light p-4">
              {{ t('waFlowBuilder.addComponentsHint') }}
            </p>
          </div>
        </div>
      </div>

      <div v-else class="flex-1 flex items-center justify-center">
        <div class="vstack items-center">
          <Layers :size="48" />
          <p class="text-light">{{ t('waFlowBuilder.addScreenToStart') }}</p>
          <button type="button" @click="addScreen">
            <Plus :size="16" />
            {{ t('waFlowBuilder.addScreen') }}
          </button>
        </div>
      </div>
    </article>

    <!-- Properties Panel -->
    <article class="card col-3 flex flex-col overflow-hidden min-h-0">
      <header>
        <h3 class="hstack gap-2">
          <Settings2 :size="16" />
          {{ t('waFlowBuilder.properties') }}
        </h3>
      </header>
      <div class="scroll-y flex-1">
        <div v-if="selectedComponent" class="vstack gap-4">
          <div class="hstack justify-between">
            <span class="badge">{{ getComponentLabel(selectedComponent) }}</span>
            <div class="hstack gap-1">
              <button
                type="button"
                class="ghost small icon"
                :title="t('waFlowBuilder.moveUp')"
                :aria-label="t('waFlowBuilder.moveUp')"
                :disabled="selectedComponentIndex === 0"
                @click="moveComponent(selectedComponentIndex!, 'up')"
              >
                <ArrowUp :size="14" />
              </button>
              <button
                type="button"
                class="ghost small icon"
                :title="t('waFlowBuilder.moveDown')"
                :aria-label="t('waFlowBuilder.moveDown')"
                :disabled="selectedComponentIndex === selectedScreen!.layout.children.length - 1"
                @click="moveComponent(selectedComponentIndex!, 'down')"
              >
                <ArrowDown :size="14" />
              </button>
              <button
                type="button"
                class="ghost small icon"
                data-variant="danger"
                :title="t('waFlowBuilder.removeComponent')"
                :aria-label="t('waFlowBuilder.removeComponent')"
                @click="removeComponent(selectedComponentIndex!)"
              >
                <Trash2 :size="14" />
              </button>
            </div>
          </div>

          <!-- Text property -->
          <label v-if="'text' in selectedComponent" data-field>
            {{ t('waFlowBuilder.text') }}
            <input
              :value="selectedComponent.text"
              @input="updateComponentProperty('text', ($event.target as HTMLInputElement).value)"
            />
          </label>

          <!-- Label property -->
          <label v-if="'label' in selectedComponent && selectedComponent.type !== 'Footer'" data-field>
            {{ t('waFlowBuilder.label') }}
            <input
              :value="selectedComponent.label"
              @input="updateComponentProperty('label', ($event.target as HTMLInputElement).value)"
            />
          </label>

          <!-- Name property -->
          <label v-if="'name' in selectedComponent" data-field>
            {{ t('waFlowBuilder.fieldName') }}
            <input
              :value="selectedComponent.name"
              :placeholder="t('waFlowBuilder.fieldNamePlaceholder')"
              @input="updateComponentProperty('name', ($event.target as HTMLInputElement).value)"
            />
            <small data-hint>{{ t('waFlowBuilder.fieldNameHint') }}</small>
          </label>

          <!-- Required property -->
          <label v-if="'required' in selectedComponent" class="hstack justify-between">
            {{ t('waFlowBuilder.required') }}
            <input
              type="checkbox"
              role="switch"
              :checked="selectedComponent.required"
              @change="updateComponentProperty('required', ($event.target as HTMLInputElement).checked)"
            />
          </label>

          <!-- Options for Dropdown, Radio, Checkbox -->
          <div v-if="selectedComponent['data-source']" class="vstack gap-2">
            <div class="hstack justify-between">
              <strong>{{ t('waFlowBuilder.options') }}</strong>
              <button type="button" class="ghost small" @click="addOption">
                <Plus :size="14" />
                {{ t('waFlowBuilder.add') }}
              </button>
            </div>
            <div v-for="(opt, index) in selectedComponent['data-source']" :key="opt.id" class="hstack gap-2">
              <input
                class="flex-1"
                :value="opt.title"
                :placeholder="t('waFlowBuilder.optionPlaceholder')"
                :aria-label="t('waFlowBuilder.optionPlaceholder')"
                @input="updateOption(index, 'title', ($event.target as HTMLInputElement).value)"
              />
              <button
                type="button"
                class="ghost small icon"
                :title="t('waFlowBuilder.removeOption')"
                :aria-label="t('waFlowBuilder.removeOption')"
                @click="removeOption(index)"
              >
                <Trash2 :size="14" />
              </button>
            </div>
          </div>

          <!-- Footer specific -->
          <div v-if="selectedComponent.type === 'Footer'" class="vstack gap-4">
            <label data-field>
              {{ t('waFlowBuilder.buttonText') }}
              <input
                :value="selectedComponent.label"
                @input="updateComponentProperty('label', ($event.target as HTMLInputElement).value)"
              />
            </label>
            <label data-field>
              {{ t('waFlowBuilder.action') }}
              <select
                :value="selectedComponent['on-click-action']?.name || 'complete'"
                @change="updateComponentProperty('on-click-action', { name: ($event.target as HTMLSelectElement).value, payload: {} })"
              >
                <option value="complete">{{ t('waFlowBuilder.completeFlow') }}</option>
                <option value="navigate">{{ t('waFlowBuilder.navigateToScreen') }}</option>
              </select>
            </label>
            <label v-if="selectedComponent['on-click-action']?.name === 'navigate'" data-field>
              {{ t('waFlowBuilder.targetScreen') }}
              <select
                :value="selectedComponent['on-click-action']?.next?.name || ''"
                @change="updateComponentProperty('on-click-action', {
                  name: 'navigate',
                  next: { type: 'screen', name: ($event.target as HTMLSelectElement).value }
                })"
              >
                <option value="" disabled>{{ t('waFlowBuilder.selectTargetScreen') }}</option>
                <option
                  v-for="screen in screens.filter(s => s.id !== selectedScreen?.id)"
                  :key="screen.id"
                  :value="screen.id"
                >
                  {{ screen.title }}
                </option>
              </select>
            </label>
          </div>
        </div>
        <p v-else class="align-center text-light">
          {{ t('waFlowBuilder.selectComponentHint') }}
        </p>
      </div>
    </article>
  </div>
</template>
