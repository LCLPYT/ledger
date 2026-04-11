<script setup lang="ts">
import { Sonner } from '@/components/ui/sonner'
useColorMode()
</script>

<template>
  <NuxtRouteAnnouncer />
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
  <Sonner position="bottom-right" />
</template>
