<template>
  <div class="min-h-screen flex items-center justify-center bg-background">
    <Card class="w-full max-w-sm">
      <CardHeader>
        <CardTitle>Set your password</CardTitle>
        <CardDescription>Choose a password to activate your account.</CardDescription>
      </CardHeader>
      <CardContent>
        <p v-if="!token" class="text-sm text-destructive">
          Invalid or missing invitation token. Please use the link from your email.
        </p>

        <p v-else-if="success" class="text-sm text-green-600">
          Account activated! Redirecting to login…
        </p>

        <form v-else class="space-y-4" @submit.prevent="handleSubmit">
          <div class="space-y-1">
            <Label for="password">Password</Label>
            <Input
              id="password"
              v-model="form.password"
              type="password"
              autocomplete="new-password"
              required
              minlength="8"
            />
          </div>

          <div class="space-y-1">
            <Label for="confirm">Confirm password</Label>
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
            {{ loading ? 'Activating…' : 'Activate account' }}
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
  layout: false,
  middleware: [],
})

const config = useRuntimeConfig()
const route = useRoute()

const token = computed(() => route.query.token as string | undefined)

const form = reactive({ password: '', confirm: '' })
const error = ref('')
const loading = ref(false)
const success = ref(false)

async function handleSubmit() {
  error.value = ''

  if (form.password !== form.confirm) {
    error.value = 'Passwords do not match.'
    return
  }
  if (form.password.length < 8) {
    error.value = 'Password must be at least 8 characters.'
    return
  }

  loading.value = true
  try {
    await $fetch('/api/v1/auth/set-password', {
      baseURL: config.public.apiBase,
      method: 'POST',
      body: { token: token.value, password: form.password },
    })
    success.value = true
    await new Promise(r => setTimeout(r, 1200))
    await navigateTo('/login')
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error ?? 'Activation failed. The link may have expired.'
    error.value = msg
  } finally {
    loading.value = false
  }
}
</script>
