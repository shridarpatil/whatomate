<script setup lang="ts">
import { formatDateTime } from '@/lib/utils'
import { Clock, UserCircle } from '@/lib/icons'

defineProps<{
  createdAt?: string
  updatedAt?: string
  createdByName?: string
  updatedByName?: string
}>()
</script>

<template>
  <article class="card">
    <header>
      <h3>{{ $t('common.metadata', 'Metadata') }}</h3>
    </header>
    <div class="vstack gap-2">
      <div v-if="createdByName" class="hstack gap-2">
        <UserCircle :size="14" />
        <div class="min-w-0">
          <small class="text-light">{{ $t('common.createdBy', 'Created by') }}</small>
          <p class="truncate"><strong>{{ createdByName }}</strong></p>
        </div>
      </div>
      <div v-if="createdAt" class="hstack gap-2">
        <Clock :size="14" />
        <div class="min-w-0">
          <small class="text-light">{{ $t('common.createdAt', 'Created') }}</small>
          <p class="truncate">{{ formatDateTime(createdAt) }}</p>
        </div>
      </div>

      <hr v-if="(createdByName || createdAt) && (updatedByName || updatedAt)">

      <div v-if="updatedByName" class="hstack gap-2">
        <UserCircle :size="14" />
        <div class="min-w-0">
          <small class="text-light">{{ $t('common.updatedBy', 'Modified by') }}</small>
          <p class="truncate"><strong>{{ updatedByName }}</strong></p>
        </div>
      </div>
      <div v-if="updatedAt" class="hstack gap-2">
        <Clock :size="14" />
        <div class="min-w-0">
          <small class="text-light">{{ $t('common.lastUpdated', 'Last updated') }}</small>
          <p class="truncate">{{ formatDateTime(updatedAt) }}</p>
        </div>
      </div>
    </div>
  </article>
</template>
