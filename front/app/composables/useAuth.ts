interface User {
  id: number
  username: string
  email: string
  created: string
}

export const useAuth = () => {
  const config = useRuntimeConfig()
  const token = useCookie<string | null>('token', { default: () => null })
  const user = useState<User | null>('auth:user', () => null)

  const apiFetch = $fetch.create({
    baseURL: config.public.apiBase,
    onRequest({ options }) {
      if (token.value) {
        const headers = new Headers(options.headers as HeadersInit)
        headers.set('Authorization', `Bearer ${token.value}`)
        options.headers = headers
      }
    },
  })

  async function login(identifier: string, password: string) {
    const res = await $fetch<{ token: string }>('/api/v1/user/login', {
      baseURL: config.public.apiBase,
      method: 'POST',
      body: { identifier, password },
    })
    token.value = res.token
    await fetchUser()
  }

  async function fetchUser() {
    if (!token.value) return
    try {
      user.value = await apiFetch<User>('/api/v1/user')
    } catch {
      user.value = null
    }
  }

  async function logout() {
    token.value = null
    user.value = null
    await navigateTo('/login')
  }

  return { token, user, login, logout, fetchUser, apiFetch }
}