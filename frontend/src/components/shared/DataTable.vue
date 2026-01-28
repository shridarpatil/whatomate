<script setup lang="ts" generic="T extends { id: string }">
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Loader2 } from 'lucide-vue-next'
import type { Component } from 'vue'

export interface Column<T> {
  key: string
  label: string
  width?: string
  align?: 'left' | 'center' | 'right'
}

defineProps<{
  items: T[]
  columns: Column<T>[]
  isLoading?: boolean
  emptyIcon?: Component
  emptyTitle?: string
  emptyDescription?: string
}>()

defineSlots<{
  [key: `cell-${string}`]: (props: { item: T; index: number }) => any
  empty: () => any
  'empty-action': () => any
}>()
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
          ]"
        >
          {{ col.label }}
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
      <TableRow v-else-if="items.length === 0">
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
      <TableRow v-else v-for="(item, index) in items" :key="item.id">
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
</template>
