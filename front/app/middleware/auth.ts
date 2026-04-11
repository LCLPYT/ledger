export default defineNuxtRouteMiddleware(async (to) => {
  const { token, refreshIfNeeded } = useAuth()

  if (!token.value) {
    if (to.path !== '/login') return navigateTo('/login')
    return
  }

  if (to.path === '/login') {
    return navigateTo('/')
  }

  // Slide the session window: re-issue token if < 30 min remain
  const result = await refreshIfNeeded()
  if (result === 'cleared') return navigateTo('/login')
})
