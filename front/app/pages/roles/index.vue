<template>
  <div class="p-4 md:p-8 space-y-6">
    <div class="flex items-center justify-between">
      <h2 class="text-2xl font-semibold text-foreground">Roles</h2>
      <Button v-if="hasPermission(Perms.RolesCreate)" @click="openCreateDialog">New role</Button>
    </div>

    <!-- Error state -->
    <p v-if="fetchError" class="text-sm text-destructive">{{ fetchError }}</p>

    <!-- Roles table -->
    <template v-else>
      <div class="rounded-lg border overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Members</TableHead>
              <TableHead>Permissions</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="r in roles" :key="r.id">
              <TableCell class="font-medium">
                <div class="flex items-center gap-2">
                  <Lock v-if="r.protected" class="h-3.5 w-3.5 text-muted-foreground" />
                  {{ r.name }}
                  <Badge v-if="r.protected" variant="outline" class="text-xs">protected</Badge>
                </div>
              </TableCell>
              <TableCell>
                <span class="text-sm text-muted-foreground">{{ r.members.length }}</span>
              </TableCell>
              <TableCell>
                <div class="flex flex-wrap gap-1">
                  <Badge
                    v-for="p in rolePermissions[r.name] ?? []"
                    :key="p"
                    variant="secondary"
                    class="font-mono text-xs"
                  >{{ p }}</Badge>
                  <span v-if="(rolePermissions[r.name] ?? []).length === 0" class="text-xs text-muted-foreground">—</span>
                </div>
              </TableCell>
              <TableCell class="text-right">
                <div class="flex items-center justify-end gap-2">
                  <Button
                    v-if="hasPermission(Perms.RolesCreate)"
                    variant="ghost"
                    size="sm"
                    :disabled="r.protected"
                    :title="r.protected ? 'Protected roles cannot be edited' : undefined"
                    @click="openPermDialog(r)"
                  >
                    Permissions
                  </Button>
                  <Button
                    v-if="hasPermission(Perms.RolesCreate)"
                    variant="ghost"
                    size="sm"
                    :disabled="r.protected"
                    :title="r.protected ? 'Protected roles cannot be deleted' : undefined"
                    class="text-destructive hover:text-destructive"
                    @click="deleteRole(r)"
                  >
                    Delete
                  </Button>
                </div>
              </TableCell>
            </TableRow>
            <TableRow v-if="roles.length === 0">
              <TableCell colspan="4" class="text-center text-muted-foreground">No roles found.</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </template>

    <!-- Create role dialog -->
    <Dialog :open="createDialog.open" @update:open="createDialog.open = $event">
      <DialogContent class="w-80">
        <DialogHeader>
          <DialogTitle>New role</DialogTitle>
        </DialogHeader>

        <div class="space-y-4">
          <div class="space-y-1">
            <Label>Name</Label>
            <Input v-model="createDialog.name" placeholder="e.g. editor" @keydown.enter="createRole" />
          </div>
          <p v-if="createDialog.error" class="text-sm text-destructive">{{ createDialog.error }}</p>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="createDialog.open = false">Cancel</Button>
          <Button :disabled="!createDialog.name.trim() || createDialog.loading" @click="createRole">
            {{ createDialog.loading ? 'Creating…' : 'Create' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Manage permissions dialog -->
    <Dialog :open="permDialog.open" @update:open="permDialog.open = $event">
      <DialogContent class="w-[28rem]">
        <DialogHeader>
          <DialogTitle>Permissions — {{ permDialog.role?.name }}</DialogTitle>
        </DialogHeader>

        <div class="space-y-2">
          <label
            v-for="p in allPermissions"
            :key="p"
            class="flex items-center gap-2 text-sm cursor-pointer"
          >
            <input
              type="checkbox"
              :value="p"
              :checked="permDialog.selected.includes(p)"
              :disabled="permDialog.loading"
              class="rounded"
              @change="togglePermission(p, ($event.target as HTMLInputElement).checked)"
            />
            <span class="font-mono">{{ p }}</span>
          </label>
        </div>

        <p v-if="permDialog.error" class="text-sm text-destructive">{{ permDialog.error }}</p>

        <DialogFooter>
          <Button @click="permDialog.open = false">Done</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { Lock } from 'lucide-vue-next'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { toast } from 'vue-sonner'

definePageMeta({
  middleware: ['auth'],
})

const { apiFetch, permissions: allPermissions, hasPermission } = useAuth()

interface Role {
  id: number
  name: string
  created_at: string
  protected: boolean
  members: string[]
}

const roles = ref<Role[]>([])
const rolePermissions = ref<Record<string, string[]>>({})
const fetchError = ref('')

async function load() {
  try {
    const r = await apiFetch<Role[]>('/api/v1/roles')
    roles.value = r

    const permsMap: Record<string, string[]> = {}
    await Promise.all(
      r.map(async (role) => {
        try {
          permsMap[role.name] = await apiFetch<string[]>(`/api/v1/roles/${role.name}/permissions`)
        } catch {
          permsMap[role.name] = []
        }
      }),
    )
    rolePermissions.value = permsMap
  } catch {
    fetchError.value = 'Failed to load data. Check your permissions.'
  }
}

await load()

// Create role
const createDialog = reactive({
  open: false,
  name: '',
  loading: false,
  error: '',
})

function openCreateDialog() {
  createDialog.name = ''
  createDialog.error = ''
  createDialog.open = true
}

async function createRole() {
  if (!createDialog.name.trim()) return
  createDialog.loading = true
  createDialog.error = ''
  try {
    await apiFetch('/api/v1/roles', {
      method: 'POST',
      body: { name: createDialog.name.trim() },
    })
    createDialog.open = false
    await load()
    toast.success(`Role "${createDialog.name.trim()}" created`)
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error ?? 'Failed to create role'
    createDialog.error = msg
    toast.error(msg)
  } finally {
    createDialog.loading = false
  }
}

// Delete role
async function deleteRole(r: Role) {
  try {
    await apiFetch(`/api/v1/roles/${r.name}`, { method: 'DELETE' })
    await load()
    toast.success(`Role "${r.name}" deleted`)
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error ?? 'Failed to delete role'
    toast.error(msg)
  }
}

// Manage permissions dialog
const permDialog = reactive({
  open: false,
  role: null as Role | null,
  selected: [] as string[],
  loading: false,
  error: '',
})

function openPermDialog(r: Role) {
  permDialog.role = r
  permDialog.selected = [...(rolePermissions.value[r.name] ?? [])]
  permDialog.error = ''
  permDialog.open = true
}

async function togglePermission(perm: string, checked: boolean) {
  if (!permDialog.role) return
  permDialog.loading = true
  permDialog.error = ''
  const roleName = permDialog.role.name
  try {
    if (checked) {
      await apiFetch(`/api/v1/roles/${roleName}/permissions`, {
        method: 'POST',
        body: { permission: perm },
      })
      permDialog.selected.push(perm)
    } else {
      await apiFetch(`/api/v1/roles/${roleName}/permissions`, {
        method: 'DELETE',
        body: { permission: perm },
      })
      permDialog.selected = permDialog.selected.filter(p => p !== perm)
    }
    rolePermissions.value[roleName] = [...permDialog.selected]
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error ?? 'Failed to update permission'
    permDialog.error = msg
    toast.error(msg)
  } finally {
    permDialog.loading = false
  }
}
</script>
