<script setup lang="ts" generic="T extends Record<string, any>">
import { computed } from 'vue'
import { ArrowUpDown, ArrowUp, ArrowDown } from '@/lib/icons'
import PaginationControls from './PaginationControls.vue'
import type { Component } from 'vue'
import type { Column } from './types'

const props = withDefaults(defineProps<{
  items: T[]
  columns: Column<T>[]
  isLoading?: boolean
  emptyIcon?: Component
  emptyTitle?: string
  emptyDescription?: string
  // Sorting
  sortKey?: string
  sortDirection?: 'asc' | 'desc'
  // Row key - defaults to 'id' but can be customized
  rowKey?: string
  // Server-side pagination (recommended)
  // When enabled, parent handles pagination and passes page/totalItems
  serverPagination?: boolean
  currentPage?: number
  totalItems?: number
  pageSize?: number
  itemName?: string
  // Optional max height for the table area (e.g., 'calc(100vh - 320px)')
  // When set, the table body becomes scrollable while header and pagination stay fixed
  maxHeight?: string
}>(), {
  rowKey: 'id',
  serverPagination: false,
  currentPage: 1,
  totalItems: 0,
  pageSize: 10
})

const emit = defineEmits<{
  'update:sortKey': [key: string]
  'update:sortDirection': [direction: 'asc' | 'desc']
  'sort': [key: string, direction: 'asc' | 'desc']
  'update:currentPage': [page: number]
  'page-change': [page: number]
}>()

defineSlots<{
  [key: `cell-${string}`]: (props: { item: T; index: number }) => any
  empty: () => any
  'empty-action': () => any
}>()

const hasSortableColumns = computed(() => props.columns.some(col => col.sortable))

function handleSort(column: Column<T>) {
  if (!column.sortable) return

  const sortKey = column.sortKey || column.key
  let newDirection: 'asc' | 'desc' = 'desc'

  if (props.sortKey === sortKey) {
    newDirection = props.sortDirection === 'asc' ? 'desc' : 'asc'
  }

  emit('update:sortKey', sortKey)
  emit('update:sortDirection', newDirection)
  emit('sort', sortKey, newDirection)
}

// Helper to get nested property value (e.g., 'role.name' -> item.role.name)
function getNestedValue(obj: Record<string, any>, path: string): any {
  return path.split('.').reduce((acc, key) => acc?.[key], obj)
}

// For client-side sorting (when server doesn't handle sorting)
const sortedItems = computed(() => {
  if (!props.sortKey || !hasSortableColumns.value) {
    return props.items
  }

  return [...props.items].sort((a, b) => {
    const aVal = getNestedValue(a, props.sortKey!)
    const bVal = getNestedValue(b, props.sortKey!)

    // Handle null/undefined
    if (aVal == null && bVal == null) return 0
    if (aVal == null) return props.sortDirection === 'asc' ? -1 : 1
    if (bVal == null) return props.sortDirection === 'asc' ? 1 : -1

    // Boolean comparison
    if (typeof aVal === 'boolean' && typeof bVal === 'boolean') {
      if (aVal === bVal) return 0
      return props.sortDirection === 'asc' ? (aVal ? 1 : -1) : (aVal ? -1 : 1)
    }

    // String comparison
    if (typeof aVal === 'string' && typeof bVal === 'string') {
      const comparison = aVal.localeCompare(bVal, undefined, { sensitivity: 'base' })
      return props.sortDirection === 'asc' ? comparison : -comparison
    }

    // Numeric comparison
    if (aVal < bVal) return props.sortDirection === 'asc' ? -1 : 1
    if (aVal > bVal) return props.sortDirection === 'asc' ? 1 : -1
    return 0
  })
})

// Pagination computed properties
// Use totalItems from props if server pagination, otherwise use items length
const effectiveTotalItems = computed(() => {
  if (props.serverPagination) {
    // If server returns total, use it; otherwise fallback to items length
    return props.totalItems > 0 ? props.totalItems : sortedItems.value.length
  }
  return sortedItems.value.length
})

const totalPages = computed(() => {
  return Math.ceil(effectiveTotalItems.value / props.pageSize) || 1
})

const needsPagination = computed(() => props.serverPagination && totalPages.value > 1)

// Display items - relies on server to handle pagination when serverPagination is enabled
const displayItems = computed(() => {
  return sortedItems.value
})

function handlePageChange(page: number) {
  emit('update:currentPage', page)
  emit('page-change', page)
}

function sortState(col: Column<T>): 'ascending' | 'descending' | 'none' | undefined {
  if (!col.sortable) return undefined
  if (props.sortKey !== (col.sortKey || col.key)) return 'none'
  return props.sortDirection === 'asc' ? 'ascending' : 'descending'
}

function alignClass(col: Column<T>) {
  return col.align === 'right' ? 'align-right' : col.align === 'center' ? 'align-center' : undefined
}

function justifyClass(col: Column<T>) {
  return col.align === 'right' ? 'justify-end' : col.align === 'center' ? 'justify-center' : undefined
}

function getRowKey(item: T, index: number): string {
  return item[props.rowKey] ?? `row-${index}`
}
</script>

<template>
  <div
    class="table"
    :class="{ 'scroll-y': maxHeight }"
    :style="maxHeight ? { maxHeight } : undefined"
  >
    <table>
      <thead>
        <tr>
          <th
            v-for="col in columns"
            :key="col.key"
            :class="[alignClass(col), maxHeight && 'sticky-top']"
            :aria-sort="sortState(col)"
            :tabindex="col.sortable ? 0 : undefined"
            @click="handleSort(col)"
            @keydown.enter="handleSort(col)"
          >
            <span class="hstack gap-1" :class="justifyClass(col)">
              {{ col.label }}
              <template v-if="col.sortable">
                <ArrowUp
                  v-if="sortKey === (col.sortKey || col.key) && sortDirection === 'asc'"
                  :size="12"
                />
                <ArrowDown
                  v-else-if="sortKey === (col.sortKey || col.key) && sortDirection === 'desc'"
                  :size="12"
                />
                <ArrowUpDown v-else :size="12" />
              </template>
            </span>
          </th>
        </tr>
      </thead>
      <tbody>
        <!-- Loading State - Skeleton Rows -->
        <template v-if="isLoading">
          <tr v-for="row in 5" :key="`skeleton-${row}`">
            <td v-for="col in columns" :key="`skeleton-${row}-${col.key}`">
              <div role="status" class="skeleton line"></div>
            </td>
          </tr>
        </template>

        <!-- Empty State -->
        <tr v-else-if="sortedItems.length === 0">
          <td :colspan="columns.length" class="align-center text-light">
            <slot name="empty">
              <component :is="emptyIcon" v-if="emptyIcon" :size="32" />
              <p v-if="emptyTitle">{{ emptyTitle }}</p>
              <p v-if="emptyDescription"><small>{{ emptyDescription }}</small></p>
              <div class="mt-4">
                <slot name="empty-action" />
              </div>
            </slot>
          </td>
        </tr>

        <!-- Data Rows -->
        <tr v-else v-for="(item, index) in displayItems" :key="getRowKey(item, index)">
          <td v-for="col in columns" :key="col.key" :class="alignClass(col)">
            <slot :name="`cell-${col.key}`" :item="item" :index="index">
              {{ (item as any)[col.key] }}
            </slot>
          </td>
        </tr>
      </tbody>
    </table>
  </div>

  <!-- Server-side Pagination -->
  <div v-if="needsPagination && !isLoading" class="p-4">
    <PaginationControls
      :current-page="currentPage"
      :total-pages="totalPages"
      :total-items="totalItems"
      :page-size="pageSize"
      :item-name="itemName"
      @update:current-page="handlePageChange"
    />
  </div>
</template>
