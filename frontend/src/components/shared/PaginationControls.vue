<script setup lang="ts">
import { computed } from 'vue'
import {
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
} from '@/lib/icons'
import { getPageNumbers } from '@/composables/usePagination'

const props = defineProps<{
  currentPage: number
  totalPages: number
  totalItems: number
  pageSize: number
  itemName?: string
}>()

const emit = defineEmits<{
  'update:currentPage': [page: number]
}>()

const paginationInfo = computed(() => {
  const start = (props.currentPage - 1) * props.pageSize + 1
  const end = Math.min(props.currentPage * props.pageSize, props.totalItems)
  return { start, end }
})

const pageNumbers = computed(() => getPageNumbers(props.currentPage, props.totalPages))

function goToPage(page: number | '...') {
  if (page === '...') return
  emit('update:currentPage', page)
}
</script>

<template>
  <div class="hstack justify-between flex-wrap gap-2">
    <small class="text-light">
      Showing {{ paginationInfo.start }} to {{ paginationInfo.end }} of {{ totalItems }} {{ itemName || 'items' }}
    </small>
    <nav :aria-label="$t('common.pagination')">
      <menu class="buttons">
        <li>
          <button
            type="button"
            class="outline small icon"
            :title="$t('common.firstPage')"
            :aria-label="$t('common.firstPage')"
            :disabled="currentPage === 1"
            @click="goToPage(1)"
          >
            <ChevronsLeft />
          </button>
        </li>
        <li>
          <button
            type="button"
            class="outline small icon"
            :title="$t('common.previous')"
            :aria-label="$t('common.previous')"
            :disabled="currentPage === 1"
            @click="goToPage(currentPage - 1)"
          >
            <ChevronLeft />
          </button>
        </li>
        <template v-for="(page, index) in pageNumbers" :key="index">
          <li v-if="page !== '...'">
            <button
              type="button"
              class="small"
              :class="{ outline: page !== currentPage }"
              :aria-current="page === currentPage ? 'page' : undefined"
              @click="goToPage(page)"
            >
              {{ page }}
            </button>
          </li>
          <li v-else aria-hidden="true">...</li>
        </template>
        <li>
          <button
            type="button"
            class="outline small icon"
            :title="$t('common.next')"
            :aria-label="$t('common.next')"
            :disabled="currentPage === totalPages"
            @click="goToPage(currentPage + 1)"
          >
            <ChevronRight />
          </button>
        </li>
        <li>
          <button
            type="button"
            class="outline small icon"
            :title="$t('common.lastPage')"
            :aria-label="$t('common.lastPage')"
            :disabled="currentPage === totalPages"
            @click="goToPage(totalPages)"
          >
            <ChevronsRight />
          </button>
        </li>
      </menu>
    </nav>
  </div>
</template>
