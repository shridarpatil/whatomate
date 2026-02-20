<script setup lang="ts" generic="T extends Record<string, any>">
import { computed } from 'vue'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Loader2, ArrowUpDown, ArrowUp, ArrowDown } from 'lucide-vue-next'
import PaginationControls from './PaginationControls.vue'
import type { Component } from 'vue'

export interface Column<_T = unknown> {
  key: string
  label: string
  width?: string
  align?: 'left' | 'center' | 'right'
  sortable?: boolean
  sortKey?: string // Custom sort key if different from display key
}

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

function getRowKey(item: T, index: number): string {
  return item[props.rowKey] ?? `row-${index}`
}
</script>

<template>
  <Table>
    <TableHeader>
      <TableRow>
        <TableHead
          v-for="col in columns"
          :key="col.key"
          :class="[
            col.width,
            col.align === 'right' && 'text-right',
            col.align === 'center' && 'text-center',
            col.sortable && 'cursor-pointer select-none hover:text-foreground transition-colors',
          ]"
          @click="handleSort(col)"
        >
          <div
            :class="[
              'flex items-center gap-1',
              col.align === 'right' && 'justify-end',
              col.align === 'center' && 'justify-center',
            ]"
          >
            {{ col.label }}
            <template v-if="col.sortable">
              <ArrowUp
                v-if="sortKey === (col.sortKey || col.key) && sortDirection === 'asc'"
                class="h-3 w-3"
              />
              <ArrowDown
                v-else-if="sortKey === (col.sortKey || col.key) && sortDirection === 'desc'"
                class="h-3 w-3"
              />
              <ArrowUpDown v-else class="h-3 w-3 opacity-30" />
            </template>
          </div>
        </TableHead>
      </TableRow>
    </TableHeader>
    <TableBody>
      <!-- Loading State -->
      <TableRow v-if="isLoading">
        <TableCell :colspan="columns.length" class="h-24 text-center">
          <Loader2 class="h-6 w-6 animate-spin mx-auto" />
        </TableCell>
      </TableRow>

      <!-- Empty State -->
      <TableRow v-else-if="sortedItems.length === 0">
        <TableCell :colspan="columns.length" class="h-24 text-center text-muted-foreground">
          <slot name="empty">
            <component v-if="emptyIcon" :is="emptyIcon" class="h-8 w-8 mx-auto mb-2 opacity-50" />
            <p v-if="emptyTitle">{{ emptyTitle }}</p>
            <p v-if="emptyDescription" class="text-sm">{{ emptyDescription }}</p>
            <div class="mt-3">
              <slot name="empty-action" />
            </div>
          </slot>
        </TableCell>
      </TableRow>

      <!-- Data Rows -->
      <TableRow v-else v-for="(item, index) in displayItems" :key="getRowKey(item, index)">
        <TableCell
          v-for="col in columns"
          :key="col.key"
          :class="[
            col.align === 'right' && 'text-right',
            col.align === 'center' && 'text-center',
          ]"
        >
          <slot :name="`cell-${col.key}`" :item="item" :index="index">
            {{ (item as any)[col.key] }}
          </slot>
        </TableCell>
      </TableRow>
    </TableBody>
  </Table>

  <!-- Server-side Pagination -->
  <div v-if="needsPagination && !isLoading" class="mt-4">
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
