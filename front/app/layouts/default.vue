<template>
  <div class="flex h-screen bg-background">
    <!-- Desktop sidebar (md+) -->
    <aside class="hidden md:flex w-60 border-r border-border bg-sidebar flex-col">
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

    <!-- Content column (mobile: full width with top bar; desktop: flex-1) -->
    <div class="flex flex-col flex-1 min-w-0">
      <!-- Mobile top bar (< md) -->
      <header class="flex md:hidden items-center gap-3 px-4 h-14 border-b border-border bg-sidebar shrink-0">
        <Button variant="ghost" size="icon" @click="sidebarOpen = true">
          <Menu class="h-5 w-5" />
        </Button>
        <span class="font-semibold text-sidebar-foreground">Ledger</span>
      </header>

      <main class="flex-1 overflow-auto">
        <slot />
      </main>
    </div>

    <!-- Mobile drawer -->
    <Sheet v-model:open="sidebarOpen">
      <SheetContent side="left" class="w-60 p-0 bg-sidebar flex flex-col border-r border-border">
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
            @click="sidebarOpen = false"
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
      </SheetContent>
    </Sheet>
  </div>
</template>

<script setup lang="ts">
import { LayoutDashboard, Users, Shield, KeyRound, Settings, Menu } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent } from '@/components/ui/sheet'

const { user, logout, fetchUser, fetchPermissions, hasPermission } = useAuth()
const route = useRoute()

const sidebarOpen = ref(false)

watch(route, () => {
  sidebarOpen.value = false
})

const ALL_NAV_ITEMS = [
  { to: '/',         label: 'Dashboard', icon: LayoutDashboard, permission: null },
  { to: '/users',    label: 'Users',     icon: Users,           permission: 'users.read' },
  { to: '/roles',    label: 'Roles',     icon: Shield,          permission: 'roles.read' },
  { to: '/tokens',   label: 'Tokens',    icon: KeyRound,        permission: 'user.create_token' },
  { to: '/settings', label: 'Settings',  icon: Settings,        permission: null },
]

const navItems = computed(() =>
  ALL_NAV_ITEMS.filter(i => i.permission === null || hasPermission(i.permission))
)

await fetchUser()
await fetchPermissions()
</script>
