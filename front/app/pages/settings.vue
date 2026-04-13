<template>
  <div class="p-6 max-w-md">
    <h2 class="text-xl font-semibold mb-6">Settings</h2>

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

const { changePassword, logout } = useAuth()

const form = reactive({ current: '', newPassword: '', confirm: '' })
const error = ref('')
const loading = ref(false)
const success = ref(false)

function validatePassword(pw: string): string {
  if (pw.length < 8) return 'Password must be at least 8 characters.'
  if (!/[A-Z]/.test(pw)) return 'Password must contain at least one uppercase letter.'
  if (!/[a-z]/.test(pw)) return 'Password must contain at least one lowercase letter.'
  if (!/[0-9]/.test(pw)) return 'Password must contain at least one digit.'
  if (!/[^A-Za-z0-9]/.test(pw)) return 'Password must contain at least one special character.'
  return ''
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
