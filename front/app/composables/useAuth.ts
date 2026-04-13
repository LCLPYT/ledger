interface User {
  id: number
  username: string
  email: string
  created: string
}

// Session tokens last 1 hour - keep cookie in sync
const SESSION_MAX_AGE = 3600
// Refresh when less than 30 minutes remain
const REFRESH_THRESHOLD = 1800

function tokenExpiresAt(jwt: string): number {
  try {
    let tok = jwt.split('.')[1] ?? '';
    const payload = JSON.parse(atob(tok))
    return payload.exp as number // Unix seconds
  } catch {
    return 0
  }
}

export const useAuth = () => {
  const config = useRuntimeConfig()
  const token = useCookie<string | null>('token', {
    default: () => null,
    maxAge: SESSION_MAX_AGE,
  })
  const user = useState<User | null>('auth:user', () => null)
  const permissions = useState<string[] | null>('auth:permissions', () => null)

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
    await fetchPermissions()
  }

  async function fetchUser() {
    if (!token.value) return
    try {
      user.value = await apiFetch<User>('/api/v1/user')
    } catch {
      user.value = null
    }
  }

  async function fetchPermissions() {
    if (!token.value) return
    try {
      const res = await apiFetch<{ permissions: string[] }>('/api/v1/user/permissions')
      permissions.value = res.permissions
    } catch {
      permissions.value = []
    }
  }

  function hasPermission(perm: string): boolean {
    if (permissions.value === null) return false
    return permissions.value.includes(perm)
  }

  async function logout() {
    token.value = null
    user.value = null
    permissions.value = null
    await navigateTo('/login')
  }

  // Returns 'refreshed' | 'ok' | 'cleared'
  async function refreshIfNeeded(): Promise<'refreshed' | 'ok' | 'cleared'> {
    if (!token.value) return 'cleared'

    const exp = tokenExpiresAt(token.value)
    const secsRemaining = exp - Date.now() / 1000
    if (secsRemaining >= REFRESH_THRESHOLD) return 'ok'

    try {
      const res = await apiFetch<{ token: string }>('/api/v1/user/session/refresh', {
        method: 'POST',
      })
      token.value = res.token
      return 'refreshed'
    } catch (e: unknown) {
      const status = (e as { status?: number })?.status
      if (status === 401 || status === 403) {
        token.value = null
        user.value = null
        return 'cleared'
      }
      return 'ok'
    }
  }

  async function changePassword(currentPassword: string, newPassword: string) {
    await apiFetch('/api/v1/user/password', {
      method: 'PUT',
      body: { current_password: currentPassword, new_password: newPassword },
    })
  }

  return { token, user, permissions, login, logout, fetchUser, fetchPermissions, hasPermission, apiFetch, refreshIfNeeded, changePassword }
}
