<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger
} from '@/components/ui/accordion'
import type { PermissionGroup } from '@/stores/roles'

const props = defineProps<{
  permissionGroups: PermissionGroup[]
  selectedPermissions: string[]
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:selectedPermissions', value: string[]): void
}>()

// Action labels for better display
const actionLabels: Record<string, string> = {
  read: 'View',
  write: 'Create/Edit',
  delete: 'Delete',
  manage: 'Manage',
  export: 'Export',
  assign: 'Assign'
}

function getActionLabel(action: string): string {
  return actionLabels[action] || action.charAt(0).toUpperCase() + action.slice(1)
}

function isSelected(key: string): boolean {
  return props.selectedPermissions.includes(key)
}

function togglePermission(key: string) {
  if (props.disabled) return

  const newPermissions = isSelected(key)
    ? props.selectedPermissions.filter(p => p !== key)
    : [...props.selectedPermissions, key]

  emit('update:selectedPermissions', newPermissions)
}

function toggleGroup(group: PermissionGroup) {
  if (props.disabled) return

  const groupKeys = group.permissions.map(p => p.key)
  const allSelected = groupKeys.every(key => props.selectedPermissions.includes(key))

  let newPermissions: string[]
  if (allSelected) {
    // Deselect all in group
    newPermissions = props.selectedPermissions.filter(p => !groupKeys.includes(p))
  } else {
    // Select all in group
    const existing = new Set(props.selectedPermissions)
    groupKeys.forEach(key => existing.add(key))
    newPermissions = Array.from(existing)
  }

  emit('update:selectedPermissions', newPermissions)
}

function isGroupFullySelected(group: PermissionGroup): boolean {
  return group.permissions.every(p => props.selectedPermissions.includes(p.key))
}

function isGroupPartiallySelected(group: PermissionGroup): boolean {
  const selectedCount = group.permissions.filter(p =>
    props.selectedPermissions.includes(p.key)
  ).length
  return selectedCount > 0 && selectedCount < group.permissions.length
}

function getSelectedCountForGroup(group: PermissionGroup): number {
  return group.permissions.filter(p => props.selectedPermissions.includes(p.key)).length
}

// Default open all groups
const defaultOpenGroups = computed(() => props.permissionGroups.map(g => g.resource))

// Use a reactive value for the accordion
const openGroups = ref<string[]>([])

// When permissionGroups change, open all groups
watch(() => props.permissionGroups, (groups) => {
  if (groups.length > 0) {
    openGroups.value = groups.map(g => g.resource)
  }
}, { immediate: true })
</script>

<template>
  <div class="space-y-2">
    <!-- Empty state -->
    <div v-if="permissionGroups.length === 0" class="text-center py-8 text-muted-foreground border rounded-lg">
      <p>No permissions available. Please check if permissions are seeded in the database.</p>
    </div>

    <!-- Permission groups -->
    <Accordion v-else type="multiple" v-model="openGroups" class="w-full">
      <AccordionItem
        v-for="group in permissionGroups"
        :key="group.resource"
        :value="group.resource"
        class="border rounded-lg px-4"
      >
        <AccordionTrigger class="hover:no-underline py-3">
          <div class="flex items-center gap-3">
            <Checkbox
              :checked="isGroupFullySelected(group)"
              :indeterminate="isGroupPartiallySelected(group)"
              :disabled="disabled"
              @click.stop
              @update:checked="toggleGroup(group)"
            />
            <span class="font-medium">{{ group.label }}</span>
            <Badge variant="secondary" class="ml-2">
              {{ getSelectedCountForGroup(group) }}/{{ group.permissions.length }}
            </Badge>
          </div>
        </AccordionTrigger>
        <AccordionContent>
          <div class="grid grid-cols-2 md:grid-cols-3 gap-3 pb-4 pt-2">
            <div
              v-for="permission in group.permissions"
              :key="permission.key"
              class="flex items-start space-x-2"
            >
              <Checkbox
                :id="permission.key"
                :checked="isSelected(permission.key)"
                :disabled="disabled"
                @update:checked="togglePermission(permission.key)"
              />
              <div class="grid gap-0.5 leading-none">
                <Label
                  :for="permission.key"
                  class="text-sm font-medium cursor-pointer"
                  :class="{ 'text-muted-foreground': disabled }"
                >
                  {{ getActionLabel(permission.action) }}
                </Label>
                <p v-if="permission.description" class="text-xs text-muted-foreground">
                  {{ permission.description }}
                </p>
              </div>
            </div>
          </div>
        </AccordionContent>
      </AccordionItem>
    </Accordion>
  </div>
</template>
