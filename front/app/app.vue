<script setup lang="ts">
import { Sonner } from '@/components/ui/sonner'
const colorMode = useColorMode()
const theme = computed(() => colorMode.state.value)
</script>

<template>
  <NuxtRouteAnnouncer />
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
  <Sonner rich-colors :theme="theme" position="bottom-right" />
</template>
