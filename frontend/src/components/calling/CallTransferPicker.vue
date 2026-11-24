<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCallingStore } from '@/stores/calling'
import { teamsService, type Team, type TeamMember } from '@/services/api'
import Modal from '@/components/oat/Modal.vue'
import { ArrowLeft, Users } from '@/lib/icons'
import { toast } from '@/lib/toast'

const emit = defineEmits<{ close: [] }>()

const { t } = useI18n()
const store = useCallingStore()

const step = ref<'teams' | 'members'>('teams')
const teams = ref<Team[]>([])
const members = ref<TeamMember[]>([])
const selectedTeam = ref<Team | null>(null)
const loading = ref(false)
const membersLoading = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    const response = await teamsService.list()
    const data = response.data as any
    teams.value = data.data?.teams ?? data.teams ?? []
  } catch {
    // ignore
  } finally {
    loading.value = false
  }
})

async function selectTeam(team: Team) {
  selectedTeam.value = team
  step.value = 'members'
  membersLoading.value = true
  try {
    const response = await teamsService.listMembers(team.id)
    const data = response.data as any
    members.value = data.data?.members ?? data.members ?? []
  } catch {
    // ignore
  } finally {
    membersLoading.value = false
  }
}

function goBack() {
  step.value = 'teams'
  selectedTeam.value = null
  members.value = []
}

async function doTransfer(agentId?: string) {
  if (!selectedTeam.value) return
  try {
    await store.initiateTransfer(selectedTeam.value.id, agentId)
    emit('close')
  } catch (err: any) {
    toast.error(t('callTransfers.transferFailed'), {
      description: err.message || '',
    })
  }
}
</script>

<template>
  <Modal :open="true" :title="t('callTransfers.transferCall')" @update:open="emit('close')">
    <!-- Step 1: Team list -->
    <div v-if="step === 'teams'">
      <p class="text-light mb-4">{{ t('callTransfers.selectTeam') }}</p>
      <div v-if="loading" class="align-center p-4" role="status" aria-busy="true" data-spinner="large" />
      <p v-else-if="teams.length === 0" class="align-center text-light p-4">
        {{ t('callTransfers.noTeams') }}
      </p>
      <div v-else class="vstack gap-1 scroll-y picker-list">
        <button
          v-for="team in teams"
          :key="team.id"
          type="button"
          class="ghost hstack gap-2"
          @click="selectTeam(team)"
        >
          <Users :size="20" />
          <span class="min-w-0">
            <strong class="truncate">{{ team.name }}</strong>
            <small class="text-light">{{ team.member_count }} members</small>
          </span>
        </button>
      </div>
    </div>

    <!-- Step 2: Team members -->
    <div v-else>
      <div class="hstack items-center gap-2 mb-4">
        <button type="button" class="ghost icon small" :title="t('callTransfers.back')" :aria-label="t('callTransfers.back')" @click="goBack">
          <ArrowLeft />
        </button>
        <span class="text-light">{{ selectedTeam?.name }}</span>
      </div>

      <button
        type="button"
        class="outline w-100 mb-2"
        :disabled="store.isTransferring"
        :aria-busy="store.isTransferring ? 'true' : undefined"
        data-spinner="small"
        @click="doTransfer()"
      >
        {{ t('callTransfers.transferToTeam') }}
      </button>

      <p class="text-light mb-2"><small>{{ t('callTransfers.selectAgent') }}</small></p>

      <div v-if="membersLoading" class="align-center p-4" role="status" aria-busy="true" data-spinner="large" />
      <div v-else class="vstack gap-1 scroll-y picker-list">
        <button
          v-for="member in members"
          :key="member.user_id"
          type="button"
          class="ghost hstack gap-2"
          :disabled="store.isTransferring"
          @click="doTransfer(member.user_id)"
        >
          <span class="badge" :data-variant="member.is_available ? 'success' : 'secondary'">
            {{ member.is_available ? t('common.active') : t('common.inactive') }}
          </span>
          <span class="min-w-0">
            <strong class="truncate">{{ member.full_name }}</strong>
            <small class="text-light truncate">{{ member.email }}</small>
          </span>
        </button>
      </div>
    </div>
  </Modal>
</template>
