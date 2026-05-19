<script setup lang="ts">
import { computed } from 'vue'
import { Plus, Trash2 } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

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
  <div class="space-y-3 min-w-0 overflow-hidden">
    <div v-if="availableVariables.length > 0" class="text-[10px] text-muted-foreground space-y-1">
      <div class="font-medium">Available variables</div>
      <div class="flex flex-wrap gap-1">
        <code
          v-for="(v, i) in availableVariables"
          :key="v.key + i"
          class="bg-muted px-1.5 py-0.5 rounded"
        >{{ v.key }}</code>
      </div>
    </div>
    <div v-else class="text-[10px] text-muted-foreground p-2 border rounded bg-muted/30">
      No variables captured yet. Add a Prompt node with a "Store response as" value, or an API node with response mapping.
    </div>

    <div class="space-y-2">
      <div class="flex items-center justify-between">
        <Label class="text-xs">Sections</Label>
        <Button variant="outline" size="sm" class="h-7 px-2 text-xs gap-1" @click="addSection">
          <Plus class="h-3.5 w-3.5" /> Add Section
        </Button>
      </div>

      <div
        v-if="panelConfig.sections.length === 0"
        class="text-[10px] text-muted-foreground p-2 border rounded bg-muted/30 text-center"
      >
        No sections configured.
      </div>

      <div
        v-for="(section, sectionIdx) in panelConfig.sections"
        :key="section.id"
        class="border rounded-md p-2 space-y-2 bg-muted/20 min-w-0"
      >
        <div class="flex items-center gap-2">
          <Input
            :model-value="section.label"
            @update:model-value="(v) => setSection(sectionIdx, { label: String(v ?? '') })"
            placeholder="Section label"
            class="h-7 text-xs flex-1"
          />
          <Button variant="ghost" size="icon" class="h-7 w-7" @click="removeSection(sectionIdx)">
            <Trash2 class="h-3 w-3 text-destructive" />
          </Button>
        </div>

        <div class="flex items-center gap-3 text-[10px]">
          <div class="flex items-center gap-1">
            <span class="text-muted-foreground">Columns:</span>
            <Select
              :model-value="String(section.columns)"
              @update:model-value="(v) => setSection(sectionIdx, { columns: (Number(v) === 2 ? 2 : 1) })"
            >
              <SelectTrigger class="h-6 w-14 text-[10px]"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="1">1</SelectItem>
                <SelectItem value="2">2</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="flex items-center gap-1">
            <Switch
              :checked="section.collapsible"
              @update:checked="(v) => setSection(sectionIdx, { collapsible: v })"
              class="scale-75"
            />
            <span class="text-muted-foreground">Collapsible</span>
          </div>
          <div v-if="section.collapsible" class="flex items-center gap-1">
            <Switch
              :checked="section.default_collapsed"
              @update:checked="(v) => setSection(sectionIdx, { default_collapsed: v })"
              class="scale-75"
            />
            <span class="text-muted-foreground">Collapsed by default</span>
          </div>
        </div>

        <div class="space-y-1">
          <div class="flex items-center justify-between">
            <span class="text-[10px] text-muted-foreground">Fields:</span>
            <Select @update:model-value="(v: any) => addField(sectionIdx, v)">
              <SelectTrigger class="h-6 w-32 text-[10px]">
                <SelectValue placeholder="Add field…" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem
                  v-for="v in unassignedVariables"
                  :key="v.key"
                  :value="v.key"
                >
                  {{ v.key }}
                </SelectItem>
                <div
                  v-if="unassignedVariables.length === 0"
                  class="p-2 text-[10px] text-muted-foreground"
                >
                  No unused variables left.
                </div>
              </SelectContent>
            </Select>
          </div>

          <div
            v-if="section.fields.length === 0"
            class="text-[10px] text-muted-foreground text-center py-1"
          >
            No fields added.
          </div>

          <div
            v-for="(field, fieldIdx) in section.fields"
            :key="field.key"
            class="bg-background rounded p-2 space-y-2"
          >
            <div class="flex items-center gap-1">
              <Badge variant="secondary" class="text-[10px] font-mono">{{ field.key }}</Badge>
              <Input
                :model-value="field.label"
                @update:model-value="(v) => setField(sectionIdx, fieldIdx, { label: String(v ?? '') })"
                placeholder="Display label"
                class="h-6 text-[10px] flex-1"
              />
              <Button variant="ghost" size="icon" class="h-6 w-6" @click="removeField(sectionIdx, fieldIdx)">
                <Trash2 class="h-3 w-3 text-destructive" />
              </Button>
            </div>
            <div class="flex items-center gap-2">
              <Select
                :model-value="field.display_type || 'text'"
                @update:model-value="(v: any) => setField(sectionIdx, fieldIdx, { display_type: v })"
              >
                <SelectTrigger class="h-6 text-[10px] w-20"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="text">Text</SelectItem>
                  <SelectItem value="badge">Badge</SelectItem>
                  <SelectItem value="tag">Tag</SelectItem>
                </SelectContent>
              </Select>
              <Select
                :model-value="field.color || 'default'"
                :disabled="(field.display_type || 'text') === 'text'"
                @update:model-value="(v: any) => setField(sectionIdx, fieldIdx, { color: v })"
              >
                <SelectTrigger class="h-6 text-[10px] flex-1"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="default">Default</SelectItem>
                  <SelectItem value="success">Success</SelectItem>
                  <SelectItem value="warning">Warning</SelectItem>
                  <SelectItem value="error">Error</SelectItem>
                  <SelectItem value="info">Info</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
