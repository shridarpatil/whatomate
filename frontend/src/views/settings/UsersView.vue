<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { useUsersStore, type User } from '@/stores/users'
import { useAuthStore } from '@/stores/auth'
import { useRolesStore } from '@/stores/roles'
import { useOrganizationsStore } from '@/stores/organizations'
import { toast } from 'vue-sonner'
import {
  Plus,
  Pencil,
  Trash2,
  User as UserIcon,
  Shield,
  ShieldCheck,
  UserCog,
  Loader2,
  Search,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  ArrowLeft,
  Users,
} from 'lucide-vue-next'

const usersStore = useUsersStore()
const authStore = useAuthStore()
const rolesStore = useRolesStore()
const organizationsStore = useOrganizationsStore()

const isLoading = ref(true)
const isDialogOpen = ref(false)
const isSubmitting = ref(false)
const editingUser = ref<User | null>(null)
const deleteDialogOpen = ref(false)
const userToDelete = ref<User | null>(null)

// Pagination and search
const searchQuery = ref('')
const currentPage = ref(1)
const pageSize = ref(20)

const formData = ref({
  email: '',
  password: '',
  full_name: '',
  role_id: '',
  is_active: true,
  is_super_admin: false
})

// Get the default role ID (agent role)
const getDefaultRoleId = () => {
  const agentRole = rolesStore.roles.find(r => r.name === 'agent' && r.is_system)
  return agentRole?.id || ''
}

const currentUserId = computed(() => authStore.user?.id)
const isSuperAdmin = computed(() => authStore.user?.is_super_admin || false)

// Filtered and paginated users
const filteredUsers = computed(() => {
  if (!searchQuery.value.trim()) {
    return usersStore.users
  }
  const query = searchQuery.value.toLowerCase()
  return usersStore.users.filter(user =>
    user.full_name.toLowerCase().includes(query) ||
    user.email.toLowerCase().includes(query) ||
    (user.role?.name || '').toLowerCase().includes(query)
  )
})

const totalPages = computed(() => Math.ceil(filteredUsers.value.length / pageSize.value))

const paginatedUsers = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return filteredUsers.value.slice(start, end)
})

const paginationInfo = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value + 1
  const end = Math.min(currentPage.value * pageSize.value, filteredUsers.value.length)
  return { start, end, total: filteredUsers.value.length }
})

// Reset to page 1 when search changes
watch(searchQuery, () => {
  currentPage.value = 1
})

// Refetch data when organization changes
watch(() => organizationsStore.selectedOrgId, () => {
  fetchData()
})

onMounted(async () => {
  await fetchData()
})

async function fetchData() {
  isLoading.value = true
  try {
    await Promise.all([
      usersStore.fetchUsers(),
      rolesStore.fetchRoles()
    ])
  } catch (error: any) {
    toast.error('Failed to load data')
  } finally {
    isLoading.value = false
  }
}

function openCreateDialog() {
  editingUser.value = null
  formData.value = {
    email: '',
    password: '',
    full_name: '',
    role_id: getDefaultRoleId(),
    is_active: true,
    is_super_admin: false
  }
  isDialogOpen.value = true
}

function openEditDialog(user: User) {
  editingUser.value = user
  formData.value = {
    email: user.email,
    password: '',
    full_name: user.full_name,
    role_id: user.role_id || '',
    is_active: user.is_active,
    is_super_admin: user.is_super_admin || false
  }
  isDialogOpen.value = true
}

async function saveUser() {
  if (!formData.value.email.trim() || !formData.value.full_name.trim()) {
    toast.error('Please fill in email and name')
    return
  }

  if (!editingUser.value && !formData.value.password.trim()) {
    toast.error('Password is required for new users')
    return
  }

  if (!formData.value.role_id) {
    toast.error('Please select a role')
    return
  }

  isSubmitting.value = true
  try {
    if (editingUser.value) {
      const updateData: any = {
        email: formData.value.email,
        full_name: formData.value.full_name,
        role_id: formData.value.role_id,
        is_active: formData.value.is_active
      }
      if (formData.value.password) {
        updateData.password = formData.value.password
      }
      // Only include is_super_admin if current user is a super admin
      if (isSuperAdmin.value) {
        updateData.is_super_admin = formData.value.is_super_admin
      }
      await usersStore.updateUser(editingUser.value.id, updateData)
      toast.success('User updated successfully')
    } else {
      const createData: any = {
        email: formData.value.email,
        password: formData.value.password,
        full_name: formData.value.full_name,
        role_id: formData.value.role_id
      }
      // Only include is_super_admin if current user is a super admin
      if (isSuperAdmin.value && formData.value.is_super_admin) {
        createData.is_super_admin = true
      }
      await usersStore.createUser(createData)
      toast.success('User created successfully')
    }
    isDialogOpen.value = false
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to save user'
    toast.error(message)
  } finally {
    isSubmitting.value = false
  }
}

function openDeleteDialog(user: User) {
  userToDelete.value = user
  deleteDialogOpen.value = true
}

async function confirmDelete() {
  if (!userToDelete.value) return

  try {
    await usersStore.deleteUser(userToDelete.value.id)
    toast.success('User deleted')
    deleteDialogOpen.value = false
    userToDelete.value = null
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to delete user'
    toast.error(message)
  }
}

function getRoleBadgeVariant(roleName: string): 'default' | 'secondary' | 'outline' {
  switch (roleName.toLowerCase()) {
    case 'admin':
      return 'default'
    case 'manager':
      return 'secondary'
    default:
      return 'outline'
  }
}

function getRoleIcon(roleName: string) {
  switch (roleName.toLowerCase()) {
    case 'admin':
      return ShieldCheck
    case 'manager':
      return Shield
    default:
      return UserCog
  }
}

function getRoleName(user: User): string {
  return user.role?.name || 'No role'
}

function formatDate(dateString: string) {
  return new Date(dateString).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  })
}

function goToPage(page: number) {
  if (page >= 1 && page <= totalPages.value) {
    currentPage.value = page
  }
}
</script>

<template>
  <div class="flex flex-col h-full bg-[#0a0a0b] light:bg-gray-50">
    <!-- Header -->
    <header class="border-b border-white/[0.08] light:border-gray-200 bg-[#0a0a0b]/95 light:bg-white/95 backdrop-blur">
      <div class="flex h-16 items-center px-6">
        <RouterLink to="/settings">
          <Button variant="ghost" size="icon" class="mr-3">
            <ArrowLeft class="h-5 w-5" />
          </Button>
        </RouterLink>
        <div class="h-8 w-8 rounded-lg bg-gradient-to-br from-blue-500 to-indigo-600 flex items-center justify-center mr-3 shadow-lg shadow-blue-500/20">
          <Users class="h-4 w-4 text-white" />
        </div>
        <div class="flex-1">
          <h1 class="text-xl font-semibold text-white light:text-gray-900">User Management</h1>
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>
                <BreadcrumbLink href="/settings">Settings</BreadcrumbLink>
              </BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbPage>Users</BreadcrumbPage>
              </BreadcrumbItem>
            </BreadcrumbList>
          </Breadcrumb>
        </div>
        <Button variant="outline" size="sm" @click="openCreateDialog">
          <Plus class="h-4 w-4 mr-2" />
          Add User
        </Button>
      </div>
    </header>

    <ScrollArea class="flex-1">
      <div class="p-6">
        <div class="max-w-6xl mx-auto space-y-4">
          <!-- Search and filters -->
          <div class="flex items-center gap-4">
            <div class="relative flex-1 max-w-sm">
              <Search class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                v-model="searchQuery"
                placeholder="Search by name, email, or role..."
                class="pl-9"
              />
            </div>
            <div class="text-sm text-muted-foreground">
              {{ filteredUsers.length }} user{{ filteredUsers.length !== 1 ? 's' : '' }}
            </div>
          </div>

          <!-- Users Table -->
          <Card>
            <CardHeader>
              <CardTitle>Your Users</CardTitle>
              <CardDescription>
                Manage team members and their roles. <RouterLink to="/settings/roles" class="text-primary hover:underline">Manage roles</RouterLink>
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
              <TableHeader>
                <TableRow>
                  <TableHead class="w-[300px]">User</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead class="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-if="isLoading">
                  <TableCell colspan="5" class="h-24 text-center">
                    <Loader2 class="h-6 w-6 animate-spin mx-auto" />
                  </TableCell>
                </TableRow>
                <TableRow v-else-if="paginatedUsers.length === 0">
                  <TableCell colspan="5" class="h-24 text-center text-muted-foreground">
                    <UserIcon class="h-8 w-8 mx-auto mb-2 opacity-50" />
                    <p>{{ searchQuery ? 'No users found matching your search' : 'No users found' }}</p>
                  </TableCell>
                </TableRow>
                <TableRow v-else v-for="user in paginatedUsers" :key="user.id">
                  <TableCell>
                    <div class="flex items-center gap-3">
                      <div class="h-9 w-9 rounded-full bg-primary/10 flex items-center justify-center flex-shrink-0">
                        <component :is="getRoleIcon(getRoleName(user))" class="h-4 w-4 text-primary" />
                      </div>
                      <div class="min-w-0">
                        <div class="flex items-center gap-2">
                          <p class="font-medium truncate">{{ user.full_name }}</p>
                          <Badge v-if="user.id === currentUserId" variant="outline" class="text-xs">You</Badge>
                          <Badge v-if="user.is_super_admin" variant="default" class="text-xs">Super Admin</Badge>
                        </div>
                        <p class="text-sm text-muted-foreground truncate">{{ user.email }}</p>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge :variant="getRoleBadgeVariant(getRoleName(user))" class="capitalize">
                      {{ getRoleName(user) }}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Badge
                      variant="outline"
                      :class="user.is_active ? 'border-green-600 text-green-600' : ''"
                    >
                      {{ user.is_active ? 'Active' : 'Inactive' }}
                    </Badge>
                  </TableCell>
                  <TableCell class="text-muted-foreground">
                    {{ formatDate(user.created_at) }}
                  </TableCell>
                  <TableCell class="text-right">
                    <div class="flex items-center justify-end gap-1">
                      <Tooltip>
                        <TooltipTrigger as-child>
                          <Button variant="ghost" size="icon" class="h-8 w-8" @click="openEditDialog(user)">
                            <Pencil class="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>Edit user</TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger as-child>
                          <Button
                            variant="ghost"
                            size="icon"
                            class="h-8 w-8"
                            @click="openDeleteDialog(user)"
                            :disabled="user.id === currentUserId"
                          >
                            <Trash2 class="h-4 w-4 text-destructive" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>
                          {{ user.id === currentUserId ? "Can't delete yourself" : 'Delete user' }}
                        </TooltipContent>
                      </Tooltip>
                    </div>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
            </CardContent>
          </Card>

          <!-- Pagination -->
          <div v-if="totalPages > 1" class="flex items-center justify-between">
            <p class="text-sm text-muted-foreground">
              Showing {{ paginationInfo.start }} to {{ paginationInfo.end }} of {{ paginationInfo.total }} users
            </p>
            <div class="flex items-center gap-1">
              <Button
                variant="outline"
                size="icon"
                class="h-8 w-8"
                :disabled="currentPage === 1"
                @click="goToPage(1)"
              >
                <ChevronsLeft class="h-4 w-4" />
              </Button>
              <Button
                variant="outline"
                size="icon"
                class="h-8 w-8"
                :disabled="currentPage === 1"
                @click="goToPage(currentPage - 1)"
              >
                <ChevronLeft class="h-4 w-4" />
              </Button>
              <div class="flex items-center gap-1 mx-2">
                <template v-for="page in totalPages" :key="page">
                  <Button
                    v-if="page === 1 || page === totalPages || (page >= currentPage - 1 && page <= currentPage + 1)"
                    :variant="page === currentPage ? 'default' : 'outline'"
                    size="icon"
                    class="h-8 w-8"
                    @click="goToPage(page)"
                  >
                    {{ page }}
                  </Button>
                  <span
                    v-else-if="page === currentPage - 2 || page === currentPage + 2"
                    class="px-1 text-muted-foreground"
                  >
                    ...
                  </span>
                </template>
              </div>
              <Button
                variant="outline"
                size="icon"
                class="h-8 w-8"
                :disabled="currentPage === totalPages"
                @click="goToPage(currentPage + 1)"
              >
                <ChevronRight class="h-4 w-4" />
              </Button>
              <Button
                variant="outline"
                size="icon"
                class="h-8 w-8"
                :disabled="currentPage === totalPages"
                @click="goToPage(totalPages)"
              >
                <ChevronsRight class="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
      </div>
    </ScrollArea>

    <!-- Add/Edit Dialog -->
    <Dialog v-model:open="isDialogOpen">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>{{ editingUser ? 'Edit' : 'Add' }} User</DialogTitle>
          <DialogDescription>
            {{ editingUser ? 'Update user details and permissions.' : 'Create a new team member account.' }}
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-4">
          <div class="space-y-2">
            <Label for="full_name">Full Name <span class="text-destructive">*</span></Label>
            <Input
              id="full_name"
              v-model="formData.full_name"
              placeholder="John Doe"
            />
          </div>

          <div class="space-y-2">
            <Label for="email">Email <span class="text-destructive">*</span></Label>
            <Input
              id="email"
              v-model="formData.email"
              type="email"
              placeholder="john@example.com"
            />
          </div>

          <div class="space-y-2">
            <Label for="password">
              Password
              <span v-if="!editingUser" class="text-destructive">*</span>
              <span v-else class="text-muted-foreground">(leave blank to keep existing)</span>
            </Label>
            <Input
              id="password"
              v-model="formData.password"
              type="password"
              placeholder="Enter password"
            />
          </div>

          <div class="space-y-2">
            <Label for="role">Role <span class="text-destructive">*</span></Label>
            <Select v-model="formData.role_id">
              <SelectTrigger>
                <SelectValue placeholder="Select role" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem
                  v-for="role in rolesStore.roles"
                  :key="role.id"
                  :value="role.id"
                >
                  <div class="flex items-center gap-2">
                    <span class="capitalize">{{ role.name }}</span>
                    <Badge v-if="role.is_system" variant="secondary" class="text-xs">System</Badge>
                  </div>
                </SelectItem>
              </SelectContent>
            </Select>
            <p class="text-xs text-muted-foreground">
              <RouterLink to="/settings/roles" class="text-primary hover:underline">
                Manage roles and permissions
              </RouterLink>
            </p>
          </div>

          <div v-if="editingUser" class="flex items-center justify-between">
            <Label for="is_active" class="font-normal cursor-pointer">
              Account Active
            </Label>
            <Switch
              id="is_active"
              :checked="formData.is_active"
              @update:checked="formData.is_active = $event"
              :disabled="editingUser?.id === currentUserId"
            />
          </div>

          <!-- Super Admin toggle - only visible to super admins -->
          <div v-if="isSuperAdmin" class="flex items-center justify-between border-t pt-4">
            <div>
              <Label for="is_super_admin" class="font-normal cursor-pointer">
                Super Admin
              </Label>
              <p class="text-xs text-muted-foreground">
                Super admins can access all organizations and manage other super admins
              </p>
            </div>
            <Switch
              id="is_super_admin"
              :checked="formData.is_super_admin"
              @update:checked="formData.is_super_admin = $event"
              :disabled="editingUser?.id === currentUserId && editingUser?.is_super_admin"
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" @click="isDialogOpen = false">Cancel</Button>
          <Button size="sm" @click="saveUser" :disabled="isSubmitting">
            <Loader2 v-if="isSubmitting" class="h-4 w-4 mr-2 animate-spin" />
            {{ editingUser ? 'Update' : 'Create' }} User
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete Confirmation Dialog -->
    <AlertDialog v-model:open="deleteDialogOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete User</AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to delete "{{ userToDelete?.full_name }}"? This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction @click="confirmDelete" class="bg-destructive text-destructive-foreground hover:bg-destructive/90">
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
