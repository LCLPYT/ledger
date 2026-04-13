<template>
  <div class="p-6 max-w-md">
    <h2 class="text-xl font-semibold mb-6">Settings</h2>

    <Card class="mb-4">
      <CardHeader>
        <CardTitle>Change username</CardTitle>
      </CardHeader>
      <CardContent>
        <p v-if="usernameSuccess" class="text-sm text-green-600">
          Username updated.
        </p>

        <form v-else class="space-y-4" @submit.prevent="handleUsernameSubmit">
          <div class="space-y-1">
            <Label for="new-username">New username</Label>
            <Input
              id="new-username"
              v-model="usernameForm.username"
              type="text"
              autocomplete="username"
              required
            />
          </div>

          <div class="space-y-1">
            <Label for="username-password">Current password</Label>
            <Input
              id="username-password"
              v-model="usernameForm.password"
              type="password"
              autocomplete="current-password"
              required
            />
          </div>

          <p v-if="usernameError" class="text-sm text-destructive">{{ usernameError }}</p>

          <Button type="submit" :disabled="usernameLoading" class="w-full">
            {{ usernameLoading ? 'Saving…' : 'Change username' }}
          </Button>
        </form>
      </CardContent>
    </Card>

    <Card class="mb-4">
      <CardHeader>
        <CardTitle>Change email</CardTitle>
      </CardHeader>
      <CardContent>
        <p v-if="emailSuccess" class="text-sm text-green-600">
          Email updated.
        </p>

        <form v-else class="space-y-4" @submit.prevent="handleEmailSubmit">
          <div class="space-y-1">
            <Label for="new-email">New email</Label>
            <Input
              id="new-email"
              v-model="emailForm.email"
              type="email"
              autocomplete="email"
              required
            />
          </div>

          <div class="space-y-1">
            <Label for="email-password">Current password</Label>
            <Input
              id="email-password"
              v-model="emailForm.password"
              type="password"
              autocomplete="current-password"
              required
            />
          </div>

          <p v-if="emailError" class="text-sm text-destructive">{{ emailError }}</p>

          <Button type="submit" :disabled="emailLoading" class="w-full">
            {{ emailLoading ? 'Saving…' : 'Change email' }}
          </Button>
        </form>
      </CardContent>
    </Card>

    <Card>
      <CardHeader>
        <CardTitle>Change password</CardTitle>
        <CardDescription>You will be signed out after changing your password.</CardDescription>
      </CardHeader>
      <CardContent>
        <p v-if="success" class="text-sm text-green-600">
          Password changed. Signing out…
        </p>

        <form v-else class="space-y-4" @submit.prevent="handleSubmit">
          <div class="space-y-1">
            <Label for="current">Current password</Label>
            <Input
              id="current"
              v-model="form.current"
              type="password"
              autocomplete="current-password"
              required
            />
          </div>

          <div class="space-y-1">
            <Label for="new">New password</Label>
            <Input
              id="new"
              v-model="form.newPassword"
              type="password"
              autocomplete="new-password"
              required
            />
          </div>

          <div class="space-y-1">
            <Label for="confirm">Confirm new password</Label>
            <Input
              id="confirm"
              v-model="form.confirm"
              type="password"
              autocomplete="new-password"
              required
            />
          </div>

          <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

          <Button type="submit" :disabled="loading" class="w-full">
            {{ loading ? 'Saving…' : 'Change password' }}
          </Button>
        </form>
      </CardContent>
    </Card>
  </div>
</template>

<script setup lang="ts">
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'

definePageMeta({
  middleware: ['auth'],
})

const { changePassword, logout, updateUsername, updateEmail } = useAuth()
const { validatePassword } = usePasswordPolicy()

const usernameForm = reactive({ username: '', password: '' })
const usernameError = ref('')
const usernameLoading = ref(false)
const usernameSuccess = ref(false)

const emailForm = reactive({ email: '', password: '' })
const emailError = ref('')
const emailLoading = ref(false)
const emailSuccess = ref(false)

const form = reactive({ current: '', newPassword: '', confirm: '' })
const error = ref('')
const loading = ref(false)
const success = ref(false)

async function handleUsernameSubmit() {
  usernameError.value = ''
  usernameLoading.value = true
  try {
    await updateUsername(usernameForm.username, usernameForm.password)
    usernameSuccess.value = true
    usernameForm.username = ''
    usernameForm.password = ''
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error ?? 'Failed to update username.'
    usernameError.value = msg
  } finally {
    usernameLoading.value = false
  }
}

async function handleEmailSubmit() {
  emailError.value = ''
  emailLoading.value = true
  try {
    await updateEmail(emailForm.email, emailForm.password)
    emailSuccess.value = true
    emailForm.email = ''
    emailForm.password = ''
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error ?? 'Failed to update email.'
    emailError.value = msg
  } finally {
    emailLoading.value = false
  }
}

async function handleSubmit() {
  error.value = ''

  if (form.newPassword !== form.confirm) {
    error.value = 'Passwords do not match.'
    return
  }
  const policyError = validatePassword(form.newPassword)
  if (policyError) {
    error.value = policyError
    return
  }

  loading.value = true
  try {
    await changePassword(form.current, form.newPassword)
    success.value = true
    await new Promise(r => setTimeout(r, 1200))
    await logout()
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error ?? 'Failed to change password.'
    error.value = msg
  } finally {
    loading.value = false
  }
}
</script>
