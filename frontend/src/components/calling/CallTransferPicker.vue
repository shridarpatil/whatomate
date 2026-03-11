<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useCallingStore } from '@/stores/calling'
import { teamsService, type Team, type TeamMember } from '@/services/api'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { ArrowLeft, Users, Loader2 } from 'lucide-vue-next'
import { toast } from 'vue-sonner'

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
  <Dialog :open="true" @update:open="emit('close')">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ t('callTransfers.transferCall') }}</DialogTitle>
      </DialogHeader>

      <!-- Step 1: Team list -->
      <div v-if="step === 'teams'">
        <p class="text-sm text-muted-foreground mb-3">{{ t('callTransfers.selectTeam') }}</p>
        <div v-if="loading" class="flex justify-center py-8">
          <Loader2 class="h-5 w-5 animate-spin text-muted-foreground" />
        </div>
        <div v-else-if="teams.length === 0" class="text-center py-8 text-sm text-muted-foreground">
          {{ t('callTransfers.noTeams') }}
        </div>
        <div v-else class="space-y-1 max-h-64 overflow-y-auto">
          <button
            v-for="team in teams"
            :key="team.id"
            class="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg hover:bg-accent text-left transition-colors"
            @click="selectTeam(team)"
          >
            <div class="w-8 h-8 rounded-full bg-primary/10 flex items-center justify-center shrink-0">
              <Users class="h-4 w-4 text-primary" />
            </div>
            <div class="min-w-0">
              <p class="text-sm font-medium truncate">{{ team.name }}</p>
              <p class="text-xs text-muted-foreground">{{ team.member_count }} members</p>
            </div>
          </button>
        </div>
      </div>

      <!-- Step 2: Team members -->
      <div v-else>
        <div class="flex items-center gap-2 mb-3">
          <Button variant="ghost" size="sm" class="h-7 w-7 p-0" @click="goBack">
            <ArrowLeft class="h-4 w-4" />
          </Button>
          <p class="text-sm text-muted-foreground">{{ selectedTeam?.name }}</p>
        </div>

        <Button
          variant="outline"
          class="w-full mb-2"
          :disabled="store.isTransferring"
          @click="doTransfer()"
        >
          <Loader2 v-if="store.isTransferring" class="h-4 w-4 animate-spin mr-2" />
          {{ t('callTransfers.transferToTeam') }}
        </Button>

        <p class="text-xs text-muted-foreground mb-2">{{ t('callTransfers.selectAgent') }}</p>

        <div v-if="membersLoading" class="flex justify-center py-6">
          <Loader2 class="h-5 w-5 animate-spin text-muted-foreground" />
        </div>
        <div v-else class="space-y-1 max-h-48 overflow-y-auto">
          <button
            v-for="member in members"
            :key="member.user_id"
            class="w-full flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-accent text-left transition-colors"
            :disabled="store.isTransferring"
            @click="doTransfer(member.user_id)"
          >
            <span
              class="w-2 h-2 rounded-full shrink-0"
              :class="member.is_available ? 'bg-green-500' : 'bg-zinc-400'"
            />
            <div class="min-w-0">
              <p class="text-sm font-medium truncate">{{ member.full_name }}</p>
              <p class="text-xs text-muted-foreground truncate">{{ member.email }}</p>
            </div>
          </button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
