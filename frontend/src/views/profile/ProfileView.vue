<script setup lang="ts">
import { ref } from 'vue'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { toast } from 'vue-sonner'
import { User, Eye, EyeOff, Loader2 } from 'lucide-vue-next'
import { usersService } from '@/services/api'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const isChangingPassword = ref(false)
const showCurrentPassword = ref(false)
const showNewPassword = ref(false)
const showConfirmPassword = ref(false)

const passwordForm = ref({
  current_password: '',
  new_password: '',
  confirm_password: ''
})

async function changePassword() {
  // Validate passwords match
  if (passwordForm.value.new_password !== passwordForm.value.confirm_password) {
    toast.error('New passwords do not match')
    return
  }

  // Validate password length
  if (passwordForm.value.new_password.length < 6) {
    toast.error('New password must be at least 6 characters')
    return
  }

  isChangingPassword.value = true
  try {
    await usersService.changePassword({
      current_password: passwordForm.value.current_password,
      new_password: passwordForm.value.new_password
    })
    toast.success('Password changed successfully')
    // Clear the form
    passwordForm.value = {
      current_password: '',
      new_password: '',
      confirm_password: ''
    }
  } catch (error: any) {
    const message = error.response?.data?.message || 'Failed to change password'
    toast.error(message)
  } finally {
    isChangingPassword.value = false
  }
}
</script>

<template>
  <div class="flex flex-col h-full">
    <!-- Header -->
    <header class="border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div class="flex h-16 items-center px-6">
        <User class="h-5 w-5 mr-3" />
        <div class="flex-1">
          <h1 class="text-xl font-semibold">Profile</h1>
          <p class="text-sm text-muted-foreground">Manage your account settings</p>
        </div>
      </div>
    </header>

    <!-- Content -->
    <ScrollArea class="flex-1">
      <div class="p-6 space-y-6 max-w-2xl mx-auto">
        <!-- User Info -->
        <Card>
          <CardHeader>
            <CardTitle>Account Information</CardTitle>
            <CardDescription>Your account details</CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="grid grid-cols-2 gap-4">
              <div>
                <Label class="text-muted-foreground">Name</Label>
                <p class="font-medium">{{ authStore.user?.full_name }}</p>
              </div>
              <div>
                <Label class="text-muted-foreground">Email</Label>
                <p class="font-medium">{{ authStore.user?.email }}</p>
              </div>
              <div>
                <Label class="text-muted-foreground">Role</Label>
                <p class="font-medium capitalize">{{ authStore.user?.role?.name }}</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <!-- Change Password -->
        <Card>
          <CardHeader>
            <CardTitle>Change Password</CardTitle>
            <CardDescription>Update your account password</CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="space-y-2">
              <Label for="current_password">Current Password</Label>
              <div class="relative">
                <Input
                  id="current_password"
                  v-model="passwordForm.current_password"
                  :type="showCurrentPassword ? 'text' : 'password'"
                  placeholder="Enter current password"
                />
                <button
                  type="button"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  @click="showCurrentPassword = !showCurrentPassword"
                >
                  <Eye v-if="!showCurrentPassword" class="h-4 w-4" />
                  <EyeOff v-else class="h-4 w-4" />
                </button>
              </div>
            </div>
            <div class="space-y-2">
              <Label for="new_password">New Password</Label>
              <div class="relative">
                <Input
                  id="new_password"
                  v-model="passwordForm.new_password"
                  :type="showNewPassword ? 'text' : 'password'"
                  placeholder="Enter new password"
                />
                <button
                  type="button"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  @click="showNewPassword = !showNewPassword"
                >
                  <Eye v-if="!showNewPassword" class="h-4 w-4" />
                  <EyeOff v-else class="h-4 w-4" />
                </button>
              </div>
              <p class="text-xs text-muted-foreground">Must be at least 6 characters</p>
            </div>
            <div class="space-y-2">
              <Label for="confirm_password">Confirm New Password</Label>
              <div class="relative">
                <Input
                  id="confirm_password"
                  v-model="passwordForm.confirm_password"
                  :type="showConfirmPassword ? 'text' : 'password'"
                  placeholder="Confirm new password"
                />
                <button
                  type="button"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  @click="showConfirmPassword = !showConfirmPassword"
                >
                  <Eye v-if="!showConfirmPassword" class="h-4 w-4" />
                  <EyeOff v-else class="h-4 w-4" />
                </button>
              </div>
            </div>
            <div class="flex justify-end">
              <Button variant="outline" size="sm" @click="changePassword" :disabled="isChangingPassword">
                <Loader2 v-if="isChangingPassword" class="mr-2 h-4 w-4 animate-spin" />
                Change Password
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </ScrollArea>
  </div>
</template>
