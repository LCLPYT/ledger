<template>
  <div class="p-6 max-w-md">
    <h2 class="text-xl font-semibold mb-6">Settings</h2>

    <Card>
      <CardContent class="pt-6 space-y-4">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-muted-foreground">Username</p>
            <p class="text-sm font-medium">{{ user?.username }}</p>
          </div>
          <Button variant="ghost" size="icon" aria-label="Edit username" @click="openEdit('username')">
            <Pencil class="h-4 w-4" />
          </Button>
        </div>

        <Separator />

        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-muted-foreground">Email</p>
            <p class="text-sm font-medium">{{ user?.email }}</p>
          </div>
          <Button variant="ghost" size="icon" aria-label="Edit email" @click="openEdit('email')">
            <Pencil class="h-4 w-4" />
          </Button>
        </div>

        <Separator />

        <Button variant="link" class="h-auto p-0 text-sm" @click="passwordOpen = true">
          Change password
        </Button>
      </CardContent>
    </Card>

    <!-- Username / Email edit dialog -->
    <Dialog :open="editTarget !== null" @update:open="val => { if (!val) closeEdit() }">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{{ editTarget === 'username' ? 'Change username' : 'Change email' }}</DialogTitle>
        </DialogHeader>

        <form class="space-y-4" @submit.prevent="handleEditSubmit">
          <div class="space-y-1">
            <Label :for="editTarget === 'username' ? 'edit-username' : 'edit-email'">
              {{ editTarget === 'username' ? 'New username' : 'New email' }}
            </Label>
            <Input
              v-if="editTarget === 'username'"
              id="edit-username"
              v-model="editForm.value"
              type="text"
              autocomplete="username"
              required
            />
            <Input
              v-else
              id="edit-email"
              v-model="editForm.value"
              type="email"
              autocomplete="email"
              required
            />
          </div>

          <div class="space-y-1">
            <Label for="edit-password">Current password</Label>
            <Input
              id="edit-password"
              v-model="editForm.password"
              type="password"
              autocomplete="current-password"
              required
            />
          </div>

          <p v-if="editError" class="text-sm text-destructive">{{ editError }}</p>

          <DialogFooter>
            <Button type="button" variant="outline" @click="closeEdit">Cancel</Button>
            <Button type="submit" :disabled="editLoading">
              {{ editLoading ? 'Saving…' : 'Save' }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>

    <!-- Password change dialog -->
    <Dialog :open="passwordOpen" @update:open="val => { if (!val) closePassword() }">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Change password</DialogTitle>
          <DialogDescription>You will be signed out after changing your password.</DialogDescription>
        </DialogHeader>

        <div v-if="passwordSuccess" class="text-sm text-green-600">
          Password changed. Signing out…
        </div>

        <form v-else class="space-y-4" @submit.prevent="handlePasswordSubmit">
          <div class="space-y-1">
            <Label for="pw-current">Current password</Label>
            <Input
              id="pw-current"
              v-model="passwordForm.current"
              type="password"
              autocomplete="current-password"
              required
            />
          </div>

          <div class="space-y-1">
            <Label for="pw-new">New password</Label>
            <Input
              id="pw-new"
              v-model="passwordForm.newPassword"
              type="password"
              autocomplete="new-password"
              required
            />
          </div>

          <div class="space-y-1">
            <Label for="pw-confirm">Confirm new password</Label>
            <Input
              id="pw-confirm"
              v-model="passwordForm.confirm"
              type="password"
              autocomplete="new-password"
              required
            />
          </div>

          <p v-if="passwordError" class="text-sm text-destructive">{{ passwordError }}</p>

          <DialogFooter>
            <Button type="button" variant="outline" @click="closePassword">Cancel</Button>
            <Button type="submit" :disabled="passwordLoading">
              {{ passwordLoading ? 'Saving…' : 'Change password' }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { Pencil } from 'lucide-vue-next'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog'

definePageMeta({
  middleware: ['auth'],
})

const { user, updateUsername, updateEmail, changePassword, logout } = useAuth()
const { validatePassword } = usePasswordPolicy()

// --- Profile field edit dialog ---
type EditTarget = 'username' | 'email'
const editTarget = ref<EditTarget | null>(null)
const editForm = reactive({ value: '', password: '' })
const editError = ref('')
const editLoading = ref(false)

function openEdit(target: EditTarget) {
  editTarget.value = target
  editForm.value = ''
  editForm.password = ''
  editError.value = ''
}

function closeEdit() {
  editTarget.value = null
}

async function handleEditSubmit() {
  editError.value = ''
  editLoading.value = true
  try {
    if (editTarget.value === 'username') {
      await updateUsername(editForm.value, editForm.password)
    } else {
      await updateEmail(editForm.value, editForm.password)
    }
    closeEdit()
  } catch (e: unknown) {
    const field = editTarget.value === 'username' ? 'username' : 'email'
    editError.value = (e as { data?: { error?: string } })?.data?.error ?? `Failed to update ${field}.`
  } finally {
    editLoading.value = false
  }
}

// --- Password change dialog ---
const passwordOpen = ref(false)
const passwordForm = reactive({ current: '', newPassword: '', confirm: '' })
const passwordError = ref('')
const passwordLoading = ref(false)
const passwordSuccess = ref(false)

function closePassword() {
  if (passwordSuccess.value) return
  passwordOpen.value = false
}

async function handlePasswordSubmit() {
  passwordError.value = ''

  if (passwordForm.newPassword !== passwordForm.confirm) {
    passwordError.value = 'Passwords do not match.'
    return
  }
  const policyError = validatePassword(passwordForm.newPassword)
  if (policyError) {
    passwordError.value = policyError
    return
  }

  passwordLoading.value = true
  try {
    await changePassword(passwordForm.current, passwordForm.newPassword)
    passwordSuccess.value = true
    await new Promise(r => setTimeout(r, 1200))
    await logout()
  } catch (e: unknown) {
    passwordError.value = (e as { data?: { error?: string } })?.data?.error ?? 'Failed to change password.'
  } finally {
    passwordLoading.value = false
  }
}
</script>
