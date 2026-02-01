import { defineStore } from 'pinia'
import { ref } from 'vue'
import { tagsService, type Tag } from '@/services/api'

export interface CreateTagData {
  name: string
  color?: string
}

export interface UpdateTagData {
  name?: string
  color?: string
}

export const useTagsStore = defineStore('tags', () => {
  const tags = ref<Tag[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchTags(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const response = await tagsService.list()
      tags.value = (response.data as any).data?.tags || response.data?.tags || []
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to fetch tags'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function createTag(data: CreateTagData): Promise<Tag> {
    loading.value = true
    error.value = null
    try {
      const response = await tagsService.create(data)
      const newTag = (response.data as any).data || response.data
      tags.value.push(newTag)
      // Sort by name
      tags.value.sort((a, b) => a.name.localeCompare(b.name))
      return newTag
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to create tag'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function updateTag(name: string, data: UpdateTagData): Promise<Tag> {
    loading.value = true
    error.value = null
    try {
      const response = await tagsService.update(name, data)
      const updatedTag = (response.data as any).data || response.data
      const index = tags.value.findIndex(t => t.name === name)
      if (index !== -1) {
        tags.value[index] = updatedTag
      }
      // Re-sort if name changed
      if (data.name && data.name !== name) {
        tags.value.sort((a, b) => a.name.localeCompare(b.name))
      }
      return updatedTag
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to update tag'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function deleteTag(name: string): Promise<void> {
    loading.value = true
    error.value = null
    try {
      await tagsService.delete(name)
      tags.value = tags.value.filter(t => t.name !== name)
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Failed to delete tag'
      throw err
    } finally {
      loading.value = false
    }
  }

  function getTagByName(name: string): Tag | undefined {
    return tags.value.find(t => t.name === name)
  }

  return {
    tags,
    loading,
    error,
    fetchTags,
    createTag,
    updateTag,
    deleteTag,
    getTagByName
  }
})
