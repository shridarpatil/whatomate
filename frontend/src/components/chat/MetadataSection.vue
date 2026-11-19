<script setup lang="ts">
import { formatLabel } from '@/lib/utils'

defineProps<{
  label: string
  data: any
}>()

function isObject(val: any): val is Record<string, any> {
  return val !== null && typeof val === 'object' && !Array.isArray(val)
}

function isArrayOfObjects(val: any): val is Record<string, any>[] {
  return Array.isArray(val) && val.length > 0 && isObject(val[0])
}

function isArrayOfPrimitives(val: any): boolean {
  return Array.isArray(val) && val.length > 0 && !isObject(val[0])
}

function formatValue(val: any): string {
  if (val === null || val === undefined) return '-'
  if (typeof val === 'object') return JSON.stringify(val)
  return String(val)
}

function getTableColumns(arr: Record<string, any>[]): string[] {
  const keys = new Set<string>()
  for (const row of arr) {
    for (const key of Object.keys(row)) {
      keys.add(key)
    }
  }
  return Array.from(keys)
}
</script>

<template>
  <details open>
    <summary>
      <span>
        {{ label }}
        <span v-if="isArrayOfObjects(data)" class="text-light">({{ data.length }})</span>
      </span>
    </summary>

    <!-- Object: key-value rows -->
    <div v-if="isObject(data)" class="vstack gap-1">
      <div v-for="(val, key) in data" :key="key" class="flex justify-between gap-2">
        <small class="text-light shrink-0">{{ formatLabel(String(key)) }}</small>
        <span v-if="typeof val === 'boolean'" class="badge" :data-variant="val ? undefined : 'secondary'">
          {{ val ? $t('common.yes') : $t('common.no') }}
        </span>
        <p v-else class="min-w-0 align-right"><small>{{ formatValue(val) }}</small></p>
      </div>
    </div>

    <!-- Array of objects: table -->
    <div v-else-if="isArrayOfObjects(data)" class="table">
      <table>
        <thead>
          <tr>
            <th v-for="col in getTableColumns(data)" :key="col">{{ formatLabel(col) }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, idx) in data" :key="idx">
            <td v-for="col in getTableColumns(data)" :key="col">{{ formatValue(row[col]) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Array of primitives: inline badges -->
    <div v-else-if="isArrayOfPrimitives(data)" class="hstack gap-1">
      <span v-for="(item, idx) in data" :key="idx" class="badge" data-variant="secondary">{{ String(item) }}</span>
    </div>

    <!-- Single primitive (fallback) -->
    <div v-else class="flex justify-between gap-2">
      <small class="text-light">{{ label }}</small>
      <span v-if="typeof data === 'boolean'" class="badge" :data-variant="data ? undefined : 'secondary'">
        {{ data ? $t('common.yes') : $t('common.no') }}
      </span>
      <p v-else class="min-w-0 align-right"><small>{{ formatValue(data) }}</small></p>
    </div>
  </details>
</template>
