import { defineStore } from 'pinia'
import { teamsService, type Team, type TeamMember } from '@/services/api'
import { createCrudStore } from './createCrudStore'
import { getErrorMessage } from '@/lib/api-utils'

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

// Create the base CRUD store
const useBaseCrudStore = createCrudStore<Team, CreateTeamData, UpdateTeamData>(
  'teams-crud',
  {
    list: () => teamsService.list(),
    create: (data) => teamsService.create(data),
    update: (id, data) => teamsService.update(id, data),
    delete: (id) => teamsService.delete(id),
  },
  {
    unwrapList: (response: any) => (response.data as any).data?.teams || response.data?.teams || [],
    unwrapSingle: (response: any) => (response.data as any).data?.team || response.data?.team,
    entityName: 'Team',
  }
)

// Extend with team-specific methods
export const useTeamsStore = defineStore('teams', () => {
  const baseCrud = useBaseCrudStore()

  // Re-export base CRUD functionality
  const teams = baseCrud.items
  const loading = baseCrud.loading
  const error = baseCrud.error
  const fetchTeams = baseCrud.fetchItems
  const createTeam = baseCrud.createItem
  const updateTeam = baseCrud.updateItem
  const deleteTeam = baseCrud.deleteItem

  // Team member management methods
  async function fetchTeamMembers(teamId: string): Promise<TeamMember[]> {
    try {
      const response = await teamsService.listMembers(teamId)
      return (response.data as any).data?.members || response.data?.members || []
    } catch (err: any) {
      error.value = getErrorMessage(err, 'Failed to fetch team members')
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
      error.value = getErrorMessage(err, 'Failed to add team member')
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
      error.value = getErrorMessage(err, 'Failed to remove team member')
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
