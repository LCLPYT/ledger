<template>
  <div class="flex h-screen bg-background">
    <!-- Sidebar -->
    <aside class="w-60 border-r border-border bg-sidebar flex flex-col">
      <div class="p-4 border-b border-sidebar-border">
        <h1 class="text-lg font-semibold text-sidebar-foreground">Ledger</h1>
      </div>

      <nav class="flex-1 p-2 space-y-1">
        <NuxtLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="flex items-center gap-2 px-3 py-2 rounded-md text-sm text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground transition-colors"
          active-class="bg-sidebar-accent text-sidebar-accent-foreground font-medium"
        >
          <component :is="item.icon" class="h-4 w-4" />
          {{ item.label }}
        </NuxtLink>
      </nav>

      <div class="p-3 border-t border-sidebar-border">
        <div class="flex items-center justify-between">
          <span class="text-xs text-sidebar-foreground truncate">{{ user?.username ?? '—' }}</span>
          <Button variant="ghost" size="sm" @click="logout">Sign out</Button>
        </div>
      </div>
    </aside>

    <!-- Main content -->
    <main class="flex-1 overflow-auto">
      <slot />
    </main>
  </div>
</template>

<script setup lang="ts">
import { LayoutDashboard, Users } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

const { user, logout, fetchUser } = useAuth()

const navItems = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/users', label: 'Users', icon: Users },
]

await fetchUser()
</script>