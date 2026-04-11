<template>
  <div class="p-4 md:p-8 space-y-6">
    <div class="flex items-center justify-between">
      <h2 class="text-2xl font-semibold text-foreground">Personal Access Tokens</h2>
      <Button @click="openCreateDialog">New token</Button>
    </div>

    <!-- Error state -->
    <p v-if="fetchError" class="text-sm text-destructive">{{ fetchError }}</p>

    <!-- Newly created token banner -->
    <div v-if="newToken" class="rounded-lg border border-yellow-500/40 bg-yellow-50 dark:bg-yellow-950/30 p-4 space-y-2">
      <p class="text-sm font-medium text-yellow-800 dark:text-yellow-300">
        Token created — copy it now. It will not be shown again.
      </p>
      <div class="flex items-center gap-2">
        <code class="flex-1 rounded bg-yellow-100 dark:bg-yellow-900/50 px-3 py-2 text-xs font-mono break-all select-all">{{ newToken }}</code>
        <Button variant="outline" size="sm" @click="copyToken">{{ copied ? 'Copied!' : 'Copy' }}</Button>
      </div>
      <Button variant="ghost" size="sm" class="text-xs text-muted-foreground" @click="newToken = ''">Dismiss</Button>
    </div>

    <!-- Tokens table -->
    <template v-else-if="!fetchError">
      <div class="rounded-lg border overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Scopes</TableHead>
              <TableHead>Created</TableHead>
              <TableHead>Expires</TableHead>
              <TableHead>Status</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="t in tokens" :key="t.id">
              <TableCell class="font-medium">{{ t.name || '—' }}</TableCell>
              <TableCell>
                <div class="flex flex-wrap gap-1">
                  <Badge v-for="s in t.scopes" :key="s" variant="secondary" class="font-mono text-xs">{{ s }}</Badge>
                  <span v-if="t.scopes.length === 0" class="text-xs text-muted-foreground">none</span>
                </div>
              </TableCell>
              <TableCell>{{ formatDate(t.created_at) }}</TableCell>
              <TableCell>{{ formatDate(t.expires_at) }}</TableCell>
              <TableCell>
                <Badge :variant="statusVariant(t)">{{ statusLabel(t) }}</Badge>
              </TableCell>
              <TableCell class="text-right">
                <Button
                  variant="ghost"
                  size="sm"
                  :disabled="t.revoked || isExpired(t)"
                  @click="revoke(t)"
                >
                  Revoke
                </Button>
              </TableCell>
            </TableRow>
            <TableRow v-if="tokens.length === 0">
              <TableCell colspan="6" class="text-center text-muted-foreground">No tokens yet.</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </template>

    <!-- Create token dialog -->
    <Dialog :open="dialog.open" @update:open="dialog.open = $event">
      <DialogContent class="w-[28rem]">
        <DialogHeader>
          <DialogTitle>New personal access token</DialogTitle>
        </DialogHeader>

        <div class="space-y-4">
          <div class="space-y-1">
            <Label>Name</Label>
            <Input v-model="dialog.name" placeholder="e.g. ci-pipeline" />
          </div>

          <div class="space-y-1">
            <Label>Expiry</Label>
            <Input v-model="dialog.expiry" type="date" :min="minDate" :max="maxDate" />
          </div>

          <div class="space-y-2">
            <Label>Scopes</Label>
            <div class="space-y-1">
              <label
                v-for="p in allPermissions"
                :key="p"
                class="flex items-center gap-2 text-sm cursor-pointer"
              >
                <input
                  type="checkbox"
                  :value="p"
                  v-model="dialog.scopes"
                  class="rounded"
                />
                <span class="font-mono">{{ p }}</span>
              </label>
            </div>
          </div>

          <p v-if="dialog.error" class="text-sm text-destructive">{{ dialog.error }}</p>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="dialog.open = false">Cancel</Button>
          <Button :disabled="!canCreate || dialog.loading" @click="createToken">
            {{ dialog.loading ? 'Creating…' : 'Create' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
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

const { apiFetch } = useAuth()

interface AccessToken {
  id: number
  name: string
  created_at: string
  expires_at: string
  revoked: boolean
  scopes: string[]
}

const tokens = ref<AccessToken[]>([])
const allPermissions = ref<string[]>([])
const fetchError = ref('')
const newToken = ref('')
const copied = ref(false)

async function load() {
  try {
    const [t, p] = await Promise.all([
      apiFetch<AccessToken[]>('/api/v1/user/tokens'),
      apiFetch<{ permissions: string[] }>('/api/v1/permissions'),
    ])
    tokens.value = t
    allPermissions.value = p.permissions
  } catch {
    fetchError.value = 'Failed to load data. Check your permissions.'
  }
}

await load()

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString()
}

function isExpired(t: AccessToken): boolean {
  return new Date(t.expires_at) < new Date()
}

function statusLabel(t: AccessToken): string {
  if (t.revoked) return 'Revoked'
  if (isExpired(t)) return 'Expired'
  return 'Active'
}

function statusVariant(t: AccessToken): 'secondary' | 'destructive' | 'default' {
  if (t.revoked || isExpired(t)) return 'secondary'
  return 'default'
}

async function copyToken() {
  await navigator.clipboard.writeText(newToken.value)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

// Dialog state
const today = new Date()
const minDate = today.toISOString().slice(0, 10)
const maxDate = new Date(today.getFullYear() + 1, today.getMonth(), today.getDate()).toISOString().slice(0, 10)

const dialog = reactive({
  open: false,
  name: '',
  expiry: '',
  scopes: [] as string[],
  loading: false,
  error: '',
})

const canCreate = computed(() => dialog.name.trim() && dialog.expiry && dialog.scopes.length > 0)

function openCreateDialog() {
  dialog.name = ''
  dialog.expiry = ''
  dialog.scopes = []
  dialog.error = ''
  dialog.open = true
}

async function createToken() {
  dialog.loading = true
  dialog.error = ''
  try {
    const res = await apiFetch<{ token: string }>('/api/v1/user/token', {
      method: 'POST',
      body: {
        name: dialog.name.trim(),
        scopes: dialog.scopes,
        expiry: new Date(dialog.expiry).toISOString(),
      },
    })
    newToken.value = res.token
    copied.value = false
    dialog.open = false
    await load()
    toast.success('Token created')
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error ?? 'Failed to create token'
    dialog.error = msg
    toast.error(msg)
  } finally {
    dialog.loading = false
  }
}

async function revoke(t: AccessToken) {
  try {
    await apiFetch(`/api/v1/user/tokens/${t.id}`, { method: 'DELETE' })
    await load()
    toast.success(`Token "${t.name || t.id}" revoked`)
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error ?? 'Failed to revoke token'
    toast.error(msg)
  }
}
</script>
