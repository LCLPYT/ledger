// https://nuxt.com/docs/api/configuration/nuxt-config
import tailwindcss from "@tailwindcss/vite";

export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },

  modules: ['@pinia/nuxt', '@vueuse/nuxt', 'shadcn-nuxt'],

  runtimeConfig: {
    public: {
      apiBase: 'http://localhost:8080',
    },
  },

  shadcn: {
    prefix: '',
    componentDir: '@/components/ui'
  },

  css: [
      '~/assets/css/main.css'
  ],

  vite: {
    plugins: [
      tailwindcss(),
    ],
  },
})