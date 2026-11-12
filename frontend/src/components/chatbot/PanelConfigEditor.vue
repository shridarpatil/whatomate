<script setup lang="ts">
import { computed } from 'vue'
import { Plus, Trash2 } from '@/lib/icons'

export interface PanelField {
  key: string
  label: string
  order: number
  display_type?: 'text' | 'badge' | 'tag'
  color?: 'default' | 'success' | 'warning' | 'error' | 'info'
}

export interface PanelSection {
  id: string
  label: string
  columns: 1 | 2
  collapsible: boolean
  default_collapsed: boolean
  order: number
  fields: PanelField[]
}

export interface PanelConfig {
  sections: PanelSection[]
}

export interface AvailableVariable {
  key: string
  source: string
  stepName?: string
}

const props = defineProps<{
  panelConfig: PanelConfig
  availableVariables: AvailableVariable[]
}>()

const emit = defineEmits<{
  'update:panelConfig': [config: PanelConfig]
}>()

function update(config: PanelConfig) {
  emit('update:panelConfig', config)
}

const assignedKeys = computed(() => {
  const s = new Set<string>()
  for (const section of props.panelConfig.sections) {
    for (const f of section.fields) s.add(f.key)
  }
  return s
})

const unassignedVariables = computed(() =>
  props.availableVariables.filter((v) => !assignedKeys.value.has(v.key)),
)

function addSection() {
  const sections = [...props.panelConfig.sections, {
    id: `section_${Date.now()}`,
    label: 'New section',
    columns: 1 as const,
    collapsible: true,
    default_collapsed: false,
    order: props.panelConfig.sections.length + 1,
    fields: [] as PanelField[],
  }]
  update({ sections })
}

function removeSection(index: number) {
  const sections = props.panelConfig.sections
    .filter((_, i) => i !== index)
    .map((s, i) => ({ ...s, order: i + 1 }))
  update({ sections })
}

function setSection(index: number, patch: Partial<PanelSection>) {
  const sections = props.panelConfig.sections.map((s, i) => (i === index ? { ...s, ...patch } : s))
  update({ sections })
}

function addField(sectionIndex: number, variableKey: string | number | bigint | Record<string, any> | null | undefined) {
  if (typeof variableKey !== 'string' || !variableKey) return
  if (assignedKeys.value.has(variableKey)) return
  const sections = props.panelConfig.sections.map((s, i) => {
    if (i !== sectionIndex) return s
    return {
      ...s,
      fields: [
        ...s.fields,
        {
          key: variableKey,
          label: variableKey.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase()),
          order: s.fields.length + 1,
        },
      ],
    }
  })
  update({ sections })
}

// The "Add field…" select is an action, not a value: add, then reset it.
function onAddFieldChange(sectionIndex: number, e: Event) {
  const select = e.target as HTMLSelectElement
  addField(sectionIndex, select.value)
  select.value = ''
}

function val(e: Event): string {
  return (e.target as HTMLInputElement).value
}

function checked(e: Event): boolean {
  return (e.target as HTMLInputElement).checked
}

function removeField(sectionIndex: number, fieldIndex: number) {
  const sections = props.panelConfig.sections.map((s, i) => {
    if (i !== sectionIndex) return s
    return {
      ...s,
      fields: s.fields.filter((_, j) => j !== fieldIndex).map((f, k) => ({ ...f, order: k + 1 })),
    }
  })
  update({ sections })
}

function setField(sectionIndex: number, fieldIndex: number, patch: Partial<PanelField>) {
  const sections = props.panelConfig.sections.map((s, i) => {
    if (i !== sectionIndex) return s
    return {
      ...s,
      fields: s.fields.map((f, j) => (j === fieldIndex ? { ...f, ...patch } : f)),
    }
  })
  update({ sections })
}
</script>

<template>
  <div class="vstack gap-4 min-w-0">
    <div v-if="availableVariables.length > 0" class="vstack gap-1">
      <small><strong>Available variables</strong></small>
      <div class="hstack gap-1 flex-wrap">
        <code v-for="(v, i) in availableVariables" :key="v.key + i">{{ v.key }}</code>
      </div>
    </div>
    <p v-else class="text-light">
      <small>No variables captured yet. Add a Prompt node with a "Store response as" value, or an API node with response mapping.</small>
    </p>

    <div class="vstack gap-2">
      <div class="hstack justify-between">
        <span>Sections</span>
        <button type="button" class="outline small" @click="addSection">
          <Plus /> Add Section
        </button>
      </div>

      <p v-if="panelConfig.sections.length === 0" class="text-light align-center">
        <small>No sections configured.</small>
      </p>

      <article
        v-for="(section, sectionIdx) in panelConfig.sections"
        :key="section.id"
        class="card p-4 vstack gap-2 min-w-0"
      >
        <div class="hstack gap-2">
          <input
            class="flex-1 min-w-0"
            :value="section.label"
            placeholder="Section label"
            aria-label="Section label"
            @input="setSection(sectionIdx, { label: val($event) })"
          >
          <button
            type="button"
            class="ghost small icon"
            data-variant="danger"
            :aria-label="$t('common.delete')"
            :title="$t('common.delete')"
            @click="removeSection(sectionIdx)"
          >
            <Trash2 />
          </button>
        </div>

        <div class="hstack gap-4 flex-wrap">
          <label class="hstack gap-2">
            <small>Columns:</small>
            <select
              :value="String(section.columns)"
              @change="setSection(sectionIdx, { columns: (Number(val($event)) === 2 ? 2 : 1) })"
            >
              <option value="1">1</option>
              <option value="2">2</option>
            </select>
          </label>
          <label class="hstack gap-2">
            <input
              type="checkbox"
              role="switch"
              :checked="section.collapsible"
              @change="setSection(sectionIdx, { collapsible: checked($event) })"
            >
            <small>Collapsible</small>
          </label>
          <label v-if="section.collapsible" class="hstack gap-2">
            <input
              type="checkbox"
              role="switch"
              :checked="section.default_collapsed"
              @change="setSection(sectionIdx, { default_collapsed: checked($event) })"
            >
            <small>Collapsed by default</small>
          </label>
        </div>

        <div class="vstack gap-2">
          <div class="hstack justify-between gap-2">
            <small class="text-light">Fields:</small>
            <select value="" aria-label="Add field…" @change="onAddFieldChange(sectionIdx, $event)">
              <option value="" disabled>Add field…</option>
              <option v-for="v in unassignedVariables" :key="v.key" :value="v.key">
                {{ v.key }}
              </option>
              <option v-if="unassignedVariables.length === 0" value="" disabled>No unused variables left.</option>
            </select>
          </div>

          <p v-if="section.fields.length === 0" class="text-light align-center">
            <small>No fields added.</small>
          </p>

          <div
            v-for="(field, fieldIdx) in section.fields"
            :key="field.key"
            class="vstack gap-2"
          >
            <hr>
            <div class="hstack gap-1">
              <span class="badge" data-variant="secondary">{{ field.key }}</span>
              <input
                class="flex-1 min-w-0"
                :value="field.label"
                placeholder="Display label"
                aria-label="Display label"
                @input="setField(sectionIdx, fieldIdx, { label: val($event) })"
              >
              <button
                type="button"
                class="ghost small icon"
                data-variant="danger"
                :aria-label="$t('common.delete')"
                :title="$t('common.delete')"
                @click="removeField(sectionIdx, fieldIdx)"
              >
                <Trash2 />
              </button>
            </div>
            <div class="hstack gap-2">
              <select
                class="flex-1 min-w-0"
                :value="field.display_type || 'text'"
                :aria-label="$t('flowBuilder.displayType')"
                @change="setField(sectionIdx, fieldIdx, { display_type: val($event) as PanelField['display_type'] })"
              >
                <option value="text">Text</option>
                <option value="badge">Badge</option>
                <option value="tag">Tag</option>
              </select>
              <select
                class="flex-1 min-w-0"
                :value="field.color || 'default'"
                :disabled="(field.display_type || 'text') === 'text'"
                :aria-label="$t('flowBuilder.colorLabel')"
                @change="setField(sectionIdx, fieldIdx, { color: val($event) as PanelField['color'] })"
              >
                <option value="default">Default</option>
                <option value="success">Success</option>
                <option value="warning">Warning</option>
                <option value="error">Error</option>
                <option value="info">Info</option>
              </select>
            </div>
          </div>
        </div>
      </article>
    </div>
  </div>
</template>
