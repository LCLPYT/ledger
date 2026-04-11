<template>
  <div class="p-8 space-y-6">
    <h2 class="text-2xl font-semibold text-foreground">Users</h2>

    <!-- Error state -->
    <p v-if="fetchError" class="text-sm text-destructive">{{ fetchError }}</p>

    <!-- Users table -->
    <div v-else class="border border-border rounded-lg overflow-hidden">
      <table class="w-full text-sm">
        <thead class="bg-muted/50">
          <tr>
            <th class="text-left px-4 py-3 font-medium text-muted-foreground">Username</th>
            <th class="text-left px-4 py-3 font-medium text-muted-foreground">Email</th>
            <th class="text-left px-4 py-3 font-medium text-muted-foreground">Created</th>
            <th class="text-left px-4 py-3 font-medium text-muted-foreground">Roles</th>
            <th class="px-4 py-3"></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="u in users"
            :key="u.id"
            class="border-t border-border hover:bg-muted/30 transition-colors"
          >
            <td class="px-4 py-3 font-medium text-foreground">{{ u.username }}</td>
            <td class="px-4 py-3 text-muted-foreground">{{ u.email }}</td>
            <td class="px-4 py-3 text-muted-foreground">{{ formatDate(u.created) }}</td>
            <td class="px-4 py-3">
              <div class="flex flex-wrap gap-1">
                <span
                  v-for="role in userRoles(u.id)"
                  :key="role"
                  class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-primary/10 text-primary"
                >
                  {{ role }}
                  <button
                    class="hover:text-destructive transition-colors"
                    title="Remove role"
                    @click="removeRole(u.id, role)"
                  >
                    ×
                  </button>
                </span>
                <span
                  v-if="userRoles(u.id).length === 0"
                  class="text-xs text-muted-foreground"
                >
                  —
                </span>
              </div>
            </td>
            <td class="px-4 py-3 text-right">
              <button
                class="text-xs text-primary hover:underline"
                @click="openAssignDialog(u)"
              >
                Assign role
              </button>
            </td>
          </tr>
          <tr v-if="users.length === 0 && !fetchError">
            <td colspan="5" class="px-4 py-8 text-center text-muted-foreground">No users found.</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Assign role dialog -->
    <div
      v-if="dialog.open"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      @click.self="dialog.open = false"
    >
      <div class="bg-card border border-border rounded-lg p-6 w-80 space-y-4 shadow-lg">
        <h3 class="font-semibold text-card-foreground">Assign role to {{ dialog.user?.username }}</h3>

        <div class="space-y-1">
          <label class="text-sm font-medium text-foreground">Role</label>
          <select
            v-model="dialog.selectedRole"
            class="w-full px-3 py-2 border border-input rounded-md bg-background text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-ring"
          >
            <option value="" disabled>Select a role…</option>
            <option
              v-for="r in availableRoles(dialog.user?.id ?? 0)"
              :key="r.name"
              :value="r.name"
            >
              {{ r.name }}
            </option>
          </select>
        </div>

        <p v-if="dialog.error" class="text-sm text-destructive">{{ dialog.error }}</p>

        <div class="flex gap-2 justify-end">
          <button
            class="px-3 py-1.5 text-sm rounded-md border border-input hover:bg-muted transition-colors"
            @click="dialog.open = false"
          >
            Cancel
          </button>
          <button
            :disabled="!dialog.selectedRole || dialog.loading"
            class="px-3 py-1.5 text-sm rounded-md bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            @click="assignRole"
          >
            {{ dialog.loading ? 'Assigning…' : 'Assign' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
definePageMeta({
  middleware: ['auth'],
})

const config = useRuntimeConfig()
const { apiFetch } = useAuth()

interface User {
  id: number
  username: string
  email: string
  created: string
}

interface Role {
  id: number
  name: string
  created_at: string
  members: string[]
}

const users = ref<User[]>([])
const roles = ref<Role[]>([])
const fetchError = ref('')

async function load() {
  try {
    const [u, r] = await Promise.all([
      apiFetch<User[]>('/api/v1/users'),
      apiFetch<Role[]>('/api/v1/roles'),
    ])
    users.value = u
    roles.value = r
  } catch {
    fetchError.value = 'Failed to load data. Check your permissions.'
  }
}

await load()

function userRoles(userId: number): string[] {
  const id = String(userId)
  return roles.value
    .filter(r => r.members.includes(id))
    .map(r => r.name)
}

function availableRoles(userId: number) {
  const current = userRoles(userId)
  return roles.value.filter(r => !current.includes(r.name))
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString()
}

const dialog = reactive({
  open: false,
  user: null as User | null,
  selectedRole: '',
  loading: false,
  error: '',
})

function openAssignDialog(u: User) {
  dialog.user = u
  dialog.selectedRole = ''
  dialog.error = ''
  dialog.open = true
}

async function assignRole() {
  if (!dialog.user || !dialog.selectedRole) return
  dialog.loading = true
  dialog.error = ''
  try {
    await apiFetch(`/api/v1/roles/${dialog.selectedRole}/users`, {
      method: 'POST',
      body: { user_id: String(dialog.user.id) },
    })
    await load()
    dialog.open = false
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error
    dialog.error = msg ?? 'Failed to assign role'
  } finally {
    dialog.loading = false
  }
}

async function removeRole(userId: number, roleName: string) {
  try {
    await apiFetch(`/api/v1/roles/${roleName}/users`, {
      method: 'DELETE',
      body: { user_id: String(userId) },
    })
    await load()
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error
    alert(msg ?? 'Failed to remove role')
  }
}
</script>
