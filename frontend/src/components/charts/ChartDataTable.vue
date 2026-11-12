<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{ rowLabels: string[]; columns: string[]; cells: string[][] }>()
const { t } = useI18n()
</script>

<template>
  <details>
    <summary>{{ t('common.showData') }}</summary>
    <div class="table">
      <table>
        <thead>
          <tr>
            <td></td>
            <th v-for="(c, i) in columns" :key="i" scope="col">{{ c }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(r, i) in rowLabels" :key="i">
            <th scope="row">{{ r }}</th>
            <td v-for="(c, j) in cells[i]" :key="j">{{ c }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </details>
</template>
