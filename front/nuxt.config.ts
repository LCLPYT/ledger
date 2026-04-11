// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },

  modules: ['@nuxtjs/tailwindcss', '@pinia/nuxt', '@vueuse/nuxt'],

  runtimeConfig: {
    public: {
      apiBase: 'http://localhost:8080',
    },
  },

  css: ['~/assets/css/main.css'],
})