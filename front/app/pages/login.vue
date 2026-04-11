<template>
  <div class="min-h-screen flex items-center justify-center bg-background">
    <Card class="w-full max-w-sm">
      <CardHeader>
        <CardTitle>Sign in</CardTitle>
        <CardDescription>Enter your username or email</CardDescription>
      </CardHeader>
      <CardContent>
        <form class="space-y-4" @submit.prevent="handleLogin">
          <div class="space-y-1">
            <Label for="identifier">Username or email</Label>
            <Input
              id="identifier"
              v-model="form.identifier"
              type="text"
              autocomplete="username"
              required
              placeholder="admin"
            />
          </div>

          <div class="space-y-1">
            <Label for="password">Password</Label>
            <Input
              id="password"
              v-model="form.password"
              type="password"
              autocomplete="current-password"
              required
            />
          </div>

          <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

          <Button type="submit" :disabled="loading" class="w-full">
            {{ loading ? 'Signing in...' : 'Sign in' }}
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

const { login } = useAuth()
const router = useRouter()

const form = reactive({ identifier: '', password: '' })
const error = ref('')
const loading = ref(false)

async function handleLogin() {
  error.value = ''
  loading.value = true
  try {
    await login(form.identifier, form.password)
    await router.push('/')
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error
    error.value = msg ?? 'Login failed'
  } finally {
    loading.value = false
  }
}
</script>
