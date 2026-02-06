import { ref, type Ref } from 'vue'

export interface CrudState<T, F> {
  items: Ref<T[]>
  isLoading: Ref<boolean>
  isSubmitting: Ref<boolean>
  isDialogOpen: Ref<boolean>
  editingItem: Ref<T | null>
  deleteDialogOpen: Ref<boolean>
  itemToDelete: Ref<T | null>
  formData: Ref<F>
  searchQuery: Ref<string>
  openCreateDialog: () => void
  openEditDialog: (item: T, mapToForm: (item: T) => F) => void
  openDeleteDialog: (item: T) => void
  closeDialog: () => void
  closeDeleteDialog: () => void
  resetForm: () => void
}

/**
 * Composable for managing common CRUD state variables.
 * Provides state and methods for create/edit dialogs, delete confirmations, and forms.
 *
 * @param defaultFormData - The default values for the form when creating a new item
 * @returns Object containing all state refs and dialog management methods
 *
 * @example
 * ```ts
 * const defaultForm = { name: '', email: '', is_active: true }
 * const {
 *   items, isLoading, isDialogOpen, editingItem,
 *   openCreateDialog, openEditDialog, closeDialog
 * } = useCrudState<User, typeof defaultForm>(defaultForm)
 * ```
 */
export function useCrudState<T, F extends Record<string, any>>(
  defaultFormData: F
): CrudState<T, F> {
  // Core state
  const items = ref<T[]>([]) as Ref<T[]>
  const isLoading = ref(true)
  const isSubmitting = ref(false)

  // Dialog state
  const isDialogOpen = ref(false)
  const editingItem = ref<T | null>(null) as Ref<T | null>

  // Delete dialog state
  const deleteDialogOpen = ref(false)
  const itemToDelete = ref<T | null>(null) as Ref<T | null>

  // Form state
  const formData = ref<F>({ ...defaultFormData }) as Ref<F>

  // Search state
  const searchQuery = ref('')

  /**
   * Opens the create dialog with default form values
   */
  function openCreateDialog(): void {
    editingItem.value = null
    formData.value = { ...defaultFormData } as F
    isDialogOpen.value = true
  }

  /**
   * Opens the edit dialog with form values mapped from the item
   * @param item - The item to edit
   * @param mapToForm - Function to map item properties to form data
   */
  function openEditDialog(item: T, mapToForm: (item: T) => F): void {
    editingItem.value = item
    formData.value = { ...mapToForm(item) } as F
    isDialogOpen.value = true
  }

  /**
   * Opens the delete confirmation dialog
   * @param item - The item to potentially delete
   */
  function openDeleteDialog(item: T): void {
    itemToDelete.value = item
    deleteDialogOpen.value = true
  }

  /**
   * Closes the create/edit dialog and resets the editing item
   */
  function closeDialog(): void {
    isDialogOpen.value = false
    editingItem.value = null
  }

  /**
   * Closes the delete confirmation dialog
   */
  function closeDeleteDialog(): void {
    deleteDialogOpen.value = false
    itemToDelete.value = null
  }

  /**
   * Resets the form to default values
   */
  function resetForm(): void {
    formData.value = { ...defaultFormData } as F
  }

  return {
    // State
    items,
    isLoading,
    isSubmitting,
    isDialogOpen,
    editingItem,
    deleteDialogOpen,
    itemToDelete,
    formData,
    searchQuery,
    // Methods
    openCreateDialog,
    openEditDialog,
    openDeleteDialog,
    closeDialog,
    closeDeleteDialog,
    resetForm,
  }
}
