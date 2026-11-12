<script setup lang="ts">
import { computed, ref, watch } from 'vue'
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

// Keep openGroups in sync with the native <details> open state.
function onToggle(resource: string, e: Event) {
  const isOpen = (e.target as HTMLDetailsElement).open
  const has = openGroups.value.includes(resource)
  if (isOpen && !has) openGroups.value = [...openGroups.value, resource]
  else if (!isOpen && has) openGroups.value = openGroups.value.filter(g => g !== resource)
}

// Use a reactive value for the accordion - only expand groups with selections
const openGroups = ref<string[]>([])

// Compute groups that have selected permissions
const groupsWithSelections = computed(() => {
  return props.permissionGroups
    .filter(g => g.permissions.some(p => props.selectedPermissions.includes(p.key)))
    .map(g => g.resource)
})

// Sort groups: selected groups first, then alphabetically
const sortedPermissionGroups = computed(() => {
  return [...props.permissionGroups].sort((a, b) => {
    const aHasSelection = a.permissions.some(p => props.selectedPermissions.includes(p.key))
    const bHasSelection = b.permissions.some(p => props.selectedPermissions.includes(p.key))

    // Groups with selections come first
    if (aHasSelection && !bHasSelection) return -1
    if (!aHasSelection && bHasSelection) return 1

    // Within same category, sort alphabetically
    return a.label.localeCompare(b.label)
  })
})

// Set initial state when component mounts or permission groups load
watch(() => props.permissionGroups, (groups) => {
  if (groups.length > 0 && openGroups.value.length === 0) {
    // On initial load, only expand groups with selections
    openGroups.value = [...groupsWithSelections.value]
  }
}, { immediate: true })

// Auto-expand groups when permissions are selected
watch(() => props.selectedPermissions, (newPerms, oldPerms) => {
  if (!oldPerms) return

  // Find newly selected permissions
  const oldSet = new Set(oldPerms)
  const newlySelected = newPerms.filter(p => !oldSet.has(p))

  if (newlySelected.length > 0) {
    // Find which groups contain the newly selected permissions and expand them
    const groupsToExpand = props.permissionGroups
      .filter(g => g.permissions.some(p => newlySelected.includes(p.key)))
      .map(g => g.resource)

    // Add to openGroups if not already open
    const currentOpen = new Set(openGroups.value)
    groupsToExpand.forEach(g => currentOpen.add(g))
    openGroups.value = [...currentOpen]
  }
})
</script>

<template>
  <div class="vstack gap-2">
    <!-- Empty state -->
    <div v-if="permissionGroups.length === 0" class="align-center text-light p-4">
      <p>No permissions available. Please check if permissions are seeded in the database.</p>
    </div>

    <!-- Permission groups -->
    <template v-else>
      <details
        v-for="group in sortedPermissionGroups"
        :key="group.resource"
        :open="openGroups.includes(group.resource)"
        @toggle="onToggle(group.resource, $event)"
      >
        <summary>
          <span class="hstack gap-2">
            <input
              type="checkbox"
              :aria-label="group.label"
              :checked="isGroupFullySelected(group)"
              :indeterminate="isGroupPartiallySelected(group)"
              :disabled="disabled"
              @click.stop
              @change="toggleGroup(group)"
            >
            <strong>{{ group.label }}</strong>
            <span class="badge" data-variant="secondary">
              {{ getSelectedCountForGroup(group) }}/{{ group.permissions.length }}
            </span>
          </span>
        </summary>
        <div class="row p-4">
          <div
            v-for="permission in group.permissions"
            :key="permission.key"
            class="col-4"
          >
            <label :for="permission.key" :class="{ 'text-light': disabled }">
              <input
                :id="permission.key"
                type="checkbox"
                :checked="isSelected(permission.key)"
                :disabled="disabled"
                @change="togglePermission(permission.key)"
              >
              {{ getActionLabel(permission.action) }}
              <small v-if="permission.description" data-hint>{{ permission.description }}</small>
            </label>
          </div>
        </div>
      </details>
    </template>
  </div>
</template>
