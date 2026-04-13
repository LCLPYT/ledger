export default defineNuxtRouteMiddleware(async (to) => {
    if (to.path !== '/verify') return

    const { token } = useAuth()

    if (token.value) {
        // cannot verify when already logged in
        return navigateTo('/')
    }
})