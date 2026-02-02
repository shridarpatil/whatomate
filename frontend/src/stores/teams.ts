import { defineStore } from 'pinia'
import { ref } from 'vue'
import { teamsService, type Team, type TeamMember } from '@/services/api'

export interface CreateTeamData {
  name: string
  description?: string
  assignment_strategy?: 'round_robin' | 'load_balanced' | 'manual'
}

export interface UpdateTeamData {
  name?: string
  description?: string
  assignment_strategy?: 'round_robin' | 'load_balanced' | 'manual'
  is_active?: boolean
}

export interface FetchTeamsParams {
  search?: string
  page?: number
  limit?: number
}

export interface FetchTeamsResponse {
  teams: Team[]
  total: number
  page: number
  limit: number
}

export const useTeamsStore = defineStore('teams', () => {
  const teams = ref<Team[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchTeams(params?: FetchTeamsParams): Promise<FetchTeamsResponse> {
    loading.value = true
    error.value = null
    try {
      const response = await teamsService.list(params)
      const data = (response.data as any).data || response.data
      teams.value = data.teams || []
      return {
        teams: data.teams || [],
        total: data.total ?? teams.value.length,
        page: data.page ?? 1,
        limit: data.limit ?? 50
      }
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to fetch teams'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function createTeam(data: CreateTeamData): Promise<Team> {
    loading.value = true
    error.value = null
    try {
      const response = await teamsService.create(data)
      const newTeam = (response.data as any).data?.team || response.data?.team
      teams.value.unshift(newTeam)
      return newTeam
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to create team'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function updateTeam(id: string, data: UpdateTeamData): Promise<Team> {
    loading.value = true
    error.value = null
    try {
      const response = await teamsService.update(id, data)
      const updatedTeam = (response.data as any).data?.team || response.data?.team
      const index = teams.value.findIndex(t => t.id === id)
      if (index !== -1) {
        teams.value[index] = updatedTeam
      }
      return updatedTeam
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to update team'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function deleteTeam(id: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      await teamsService.delete(id)
      teams.value = teams.value.filter(t => t.id !== id)
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to delete team'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function fetchTeamMembers(teamId: string): Promise<TeamMember[]> {
    try {
      const response = await teamsService.listMembers(teamId)
      return (response.data as any).data?.members || response.data?.members || []
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to fetch team members'
      throw err
    }
  }

  async function addTeamMember(teamId: string, userId: string, role: 'manager' | 'agent' = 'agent'): Promise<TeamMember> {
    try {
      const response = await teamsService.addMember(teamId, { user_id: userId, role })
      // Update member count
      const team = teams.value.find(t => t.id === teamId)
      if (team) {
        team.member_count = (team.member_count || 0) + 1
      }
      return (response.data as any).data?.member || response.data?.member
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to add team member'
      throw err
    }
  }

  async function removeTeamMember(teamId: string, userId: string): Promise<void> {
    try {
      await teamsService.removeMember(teamId, userId)
      // Update member count
      const team = teams.value.find(t => t.id === teamId)
      if (team && team.member_count > 0) {
        team.member_count = team.member_count - 1
      }
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to remove team member'
      throw err
    }
  }

  function getTeamById(id: string): Team | undefined {
    return teams.value.find(t => t.id === id)
  }

  return {
    teams,
    loading,
    error,
    fetchTeams,
    createTeam,
    updateTeam,
    deleteTeam,
    fetchTeamMembers,
    addTeamMember,
    removeTeamMember,
    getTeamById
  }
})
