<script setup lang="ts">
import { ref } from 'vue'
import { Badge } from '@/components/ui/badge'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { ChevronDown } from 'lucide-vue-next'
import { formatLabel } from '@/lib/utils'

const props = defineProps<{
  label: string
  data: any
}>()

const isOpen = ref(true)

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
  <Collapsible v-model:open="isOpen" class="border-t pt-3">
    <CollapsibleTrigger class="flex items-center justify-between w-full py-1 text-sm font-medium hover:text-primary transition-colors">
      <span>
        {{ label }}
        <span v-if="isArrayOfObjects(data)" class="text-muted-foreground font-normal">({{ data.length }})</span>
      </span>
      <ChevronDown
        :class="[
          'h-4 w-4 text-muted-foreground transition-transform',
          isOpen ? '' : '-rotate-90'
        ]"
      />
    </CollapsibleTrigger>
    <CollapsibleContent>
      <!-- Object: key-value rows -->
      <div v-if="isObject(data)" class="mt-2 rounded-md border">
        <div
          v-for="(val, key) in data"
          :key="key"
          class="flex justify-between items-start px-3 py-1.5 border-b border-muted/50 last:border-0"
        >
          <span class="text-xs text-muted-foreground shrink-0">{{ formatLabel(String(key)) }}</span>
          <Badge
            v-if="typeof val === 'boolean'"
            :variant="val ? 'default' : 'secondary'"
            class="text-[10px] ml-2"
          >
            {{ val ? 'Yes' : 'No' }}
          </Badge>
          <span v-else class="text-xs font-medium text-right max-w-[60%] break-words ml-2">
            {{ formatValue(val) }}
          </span>
        </div>
      </div>

      <!-- Array of objects: table -->
      <div v-else-if="isArrayOfObjects(data)" class="mt-2 rounded-md border overflow-x-auto">
        <table class="w-full text-xs">
          <thead>
            <tr class="border-b bg-muted/50">
              <th
                v-for="col in getTableColumns(data)"
                :key="col"
                class="px-3 py-1.5 text-left font-medium text-muted-foreground"
              >
                {{ formatLabel(col) }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(row, idx) in data"
              :key="idx"
              class="border-b last:border-0"
            >
              <td
                v-for="col in getTableColumns(data)"
                :key="col"
                class="px-3 py-1.5"
              >
                {{ formatValue(row[col]) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Array of primitives: inline badges -->
      <div v-else-if="isArrayOfPrimitives(data)" class="mt-2 flex flex-wrap gap-1.5">
        <Badge v-for="(item, idx) in data" :key="idx" variant="secondary" class="text-xs">
          {{ String(item) }}
        </Badge>
      </div>

      <!-- Single primitive (fallback) -->
      <div v-else class="mt-2 rounded-md border">
        <div class="flex justify-between items-start px-3 py-1.5">
          <span class="text-xs text-muted-foreground">{{ label }}</span>
          <Badge
            v-if="typeof data === 'boolean'"
            :variant="data ? 'default' : 'secondary'"
            class="text-[10px] ml-2"
          >
            {{ data ? 'Yes' : 'No' }}
          </Badge>
          <span v-else class="text-xs font-medium text-right max-w-[60%] break-words ml-2">
            {{ formatValue(data) }}
          </span>
        </div>
      </div>
    </CollapsibleContent>
  </Collapsible>
</template>
