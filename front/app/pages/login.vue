<template>
  <div class="min-h-screen flex items-center justify-center bg-background">
    <div class="w-full max-w-sm space-y-6 p-8 border border-border rounded-lg bg-card shadow-sm">
      <div class="space-y-1">
        <h1 class="text-2xl font-semibold text-card-foreground">Sign in</h1>
        <p class="text-sm text-muted-foreground">Enter your username or email</p>
      </div>

      <form class="space-y-4" @submit.prevent="handleLogin">
        <div class="space-y-1">
          <label class="text-sm font-medium text-foreground" for="identifier">Username or email</label>
          <input
            id="identifier"
            v-model="form.identifier"
            type="text"
            autocomplete="username"
            required
            class="w-full px-3 py-2 border border-input rounded-md bg-background text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
            placeholder="admin"
          />
        </div>

        <div class="space-y-1">
          <label class="text-sm font-medium text-foreground" for="password">Password</label>
          <input
            id="password"
            v-model="form.password"
            type="password"
            autocomplete="current-password"
            required
            class="w-full px-3 py-2 border border-input rounded-md bg-background text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          />
        </div>

        <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

        <button
          type="submit"
          :disabled="loading"
          class="w-full py-2 px-4 bg-primary text-primary-foreground rounded-md text-sm font-medium hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {{ loading ? 'Signing in…' : 'Sign in' }}
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
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
