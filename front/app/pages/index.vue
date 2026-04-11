<template>
  <div class="p-8">
    <div class="max-w-2xl space-y-2">
      <h2 class="text-2xl font-semibold text-foreground">
        Welcome{{ user ? `, ${user.username}` : '' }}
      </h2>
      <p class="text-muted-foreground">
        Use the sidebar to navigate. More widgets coming soon.
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
definePageMeta({
  middleware: ['auth'],
})

const { user } = useAuth()
</script>
