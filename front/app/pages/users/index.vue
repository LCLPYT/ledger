<template>
  <div class="p-4 md:p-8 space-y-6">
    <h2 class="text-2xl font-semibold text-foreground">Users</h2>

    <!-- Error state -->
    <p v-if="fetchError" class="text-sm text-destructive">{{ fetchError }}</p>

    <!-- Users table -->
    <template v-else>
      <div class="rounded-lg border overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Username</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Created</TableHead>
              <TableHead>Roles</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="u in users" :key="u.id">
              <TableCell class="font-medium">{{ u.username }}</TableCell>
              <TableCell>{{ u.email }}</TableCell>
              <TableCell>{{ formatDate(u.created) }}</TableCell>
              <TableCell>
                <div class="flex flex-wrap gap-1">
                  <Badge
                    v-for="role in userRoles(u.id)"
                    :key="role"
                    variant="secondary"
                    class="gap-1"
                  >
                    {{ role }}
                    <button
                      class="hover:text-destructive transition-colors"
                      title="Remove role"
                      @click="removeRole(u.id, role)"
                    >
                      ×
                    </button>
                  </Badge>
                  <span v-if="userRoles(u.id).length === 0" class="text-xs text-muted-foreground">—</span>
                </div>
              </TableCell>
              <TableCell class="text-right">
                <Button variant="ghost" size="sm" @click="openAssignDialog(u)">Assign role</Button>
              </TableCell>
            </TableRow>
            <TableRow v-if="users.length === 0">
              <TableCell colspan="5" class="text-center text-muted-foreground">No users found.</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </template>

    <!-- Assign role dialog -->
    <Dialog :open="dialog.open" @update:open="dialog.open = $event">
      <DialogContent class="w-80">
        <DialogHeader>
          <DialogTitle>Assign role to {{ dialog.user?.username }}</DialogTitle>
        </DialogHeader>

        <div class="space-y-4">
          <div class="space-y-1">
            <Label>Role</Label>
            <Select v-model="dialog.selectedRole">
              <SelectTrigger class="w-full">
                <SelectValue placeholder="Select a role…" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem
                  v-for="r in availableRoles(dialog.user?.id ?? 0)"
                  :key="r.name"
                  :value="r.name"
                >
                  {{ r.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <p v-if="dialog.error" class="text-sm text-destructive">{{ dialog.error }}</p>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="dialog.open = false">Cancel</Button>
          <Button :disabled="!dialog.selectedRole || dialog.loading" @click="assignRole">
            {{ dialog.loading ? 'Assigning…' : 'Assign' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { toast } from 'vue-sonner'

definePageMeta({
  middleware: ['auth'],
})

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
  const { user, selectedRole } = dialog
  try {
    await apiFetch(`/api/v1/roles/${selectedRole}/users`, {
      method: 'POST',
      body: { user_id: String(user.id) },
    })
    await load()
    dialog.open = false
    toast.success(`Role "${selectedRole}" assigned to ${user.username}`)
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error ?? 'Failed to assign role'
    dialog.error = msg
    toast.error(msg)
  } finally {
    dialog.loading = false
  }
}

async function removeRole(userId: number, roleName: string) {
  const user = users.value.find(u => u.id === userId)
  try {
    await apiFetch(`/api/v1/roles/${roleName}/users`, {
      method: 'DELETE',
      body: { user_id: String(userId) },
    })
    await load()
    toast.success(`Role "${roleName}" removed from ${user?.username ?? 'user'}`)
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error ?? 'Failed to remove role'
    toast.error(msg)
  }
}
</script>
