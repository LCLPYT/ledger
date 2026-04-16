<template>
  <div class="p-4 md:p-8 space-y-6">
    <div class="flex items-center justify-between">
      <h2 class="text-2xl font-semibold text-foreground">Players</h2>
      <div class="flex gap-2">
        <Button v-if="hasPermission(Perms.PlayerWrite)" @click="openAddByUUID">Add by UUID</Button>
        <Button v-if="hasPermission(Perms.PlayerWrite)" @click="openAddByName">Add by name</Button>
      </div>
    </div>

    <Input
      v-model="search"
      placeholder="Search by name or UUID…"
      class="max-w-sm"
      @input="onSearchInput"
    />

    <p v-if="fetchError" class="text-sm text-destructive">{{ fetchError }}</p>

    <template v-else>
      <div class="rounded-lg border overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Registered</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="p in players" :key="p.id">
              <TableCell>
                <span class="font-medium">{{ p.username ?? p.uuid }}</span>
                <span v-if="p.username" class="block font-mono text-xs text-muted-foreground">{{ p.uuid }}</span>
              </TableCell>
              <TableCell>{{ formatDate(p.created_at) }}</TableCell>
              <TableCell class="text-right">
                <div class="flex items-center justify-end gap-2">
                  <Button variant="ghost" size="sm" as-child>
                    <NuxtLink :to="`/players/${p.uuid}`">Manage</NuxtLink>
                  </Button>
                  <Button
                    v-if="hasPermission(Perms.PlayerWrite)"
                    variant="ghost"
                    size="sm"
                    class="text-destructive hover:text-destructive"
                    @click="openDeleteDialog(p)"
                  >Delete</Button>
                </div>
              </TableCell>
            </TableRow>
            <TableRow v-if="players.length === 0">
              <TableCell colspan="3" class="text-center text-muted-foreground">No players yet.</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <!-- Pagination -->
      <div v-if="players.length === limit" class="flex justify-end">
        <Button variant="outline" size="sm" @click="loadMore">Load more</Button>
      </div>
    </template>

    <!-- Add by UUID dialog -->
    <Dialog :open="addByUUIDDialog.open" @update:open="addByUUIDDialog.open = $event">
      <DialogContent class="w-80">
        <DialogHeader>
          <DialogTitle>Add player by UUID</DialogTitle>
        </DialogHeader>
        <div class="space-y-1">
          <Label for="add-uuid">Player UUID</Label>
          <Input id="add-uuid" v-model="addByUUIDDialog.uuid" placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" class="font-mono text-sm" />
        </div>
        <p v-if="addByUUIDDialog.error" class="text-sm text-destructive">{{ addByUUIDDialog.error }}</p>
        <DialogFooter>
          <Button variant="outline" @click="addByUUIDDialog.open = false">Cancel</Button>
          <Button :disabled="!addByUUIDDialog.uuid.trim() || addByUUIDDialog.loading" @click="confirmAddByUUID">
            {{ addByUUIDDialog.loading ? 'Adding…' : 'Add' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Add by name dialog -->
    <Dialog :open="addByNameDialog.open" @update:open="addByNameDialog.open = $event">
      <DialogContent class="w-80">
        <DialogHeader>
          <DialogTitle>Add player by name</DialogTitle>
        </DialogHeader>
        <div class="space-y-1">
          <Label for="add-name">Player name</Label>
          <Input id="add-name" v-model="addByNameDialog.name" placeholder="e.g. Notch" />
        </div>
        <p v-if="addByNameDialog.error" class="text-sm text-destructive">{{ addByNameDialog.error }}</p>
        <DialogFooter>
          <Button variant="outline" @click="addByNameDialog.open = false">Cancel</Button>
          <Button :disabled="!addByNameDialog.name.trim() || addByNameDialog.loading" @click="confirmAddByName">
            {{ addByNameDialog.loading ? 'Adding…' : 'Add' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete player dialog -->
    <Dialog :open="deleteDialog.open" @update:open="deleteDialog.open = $event">
      <DialogContent class="w-80">
        <DialogHeader>
          <DialogTitle>Delete player</DialogTitle>
          <p class="text-sm text-muted-foreground">
            {{ deleteDialog.player?.username ?? deleteDialog.player?.uuid }}
          </p>
        </DialogHeader>
        <p class="text-sm">This will permanently remove the player and all their data.</p>
        <p v-if="deleteDialog.error" class="text-sm text-destructive">{{ deleteDialog.error }}</p>
        <DialogFooter>
          <Button variant="outline" @click="deleteDialog.open = false">Cancel</Button>
          <Button variant="destructive" :disabled="deleteDialog.loading" @click="confirmDelete">
            {{ deleteDialog.loading ? 'Deleting…' : 'Delete' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

<script setup lang="ts">
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { toast } from 'vue-sonner'

definePageMeta({
  middleware: ['auth'],
})

const { apiFetch, hasPermission } = useAuth()

interface Player {
  id: number
  uuid: string
  username: string | null
  created_at: string
}

const limit = 50
const players = ref<Player[]>([])
const fetchError = ref('')
const offset = ref(0)
const search = ref('')
let searchTimer: ReturnType<typeof setTimeout> | null = null

function onSearchInput() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => load(true), 300)
}

async function load(reset = false) {
  if (reset) {
    offset.value = 0
    players.value = []
  }
  try {
    const url = `/api/v1/minecraft/players?limit=${limit}&offset=${offset.value}` +
      (search.value ? `&search=${encodeURIComponent(search.value)}` : '')
    const page = await apiFetch<Player[]>(url)
    players.value = reset ? page : [...players.value, ...page]
    offset.value += page.length
  } catch {
    fetchError.value = 'Failed to load players. Check your permissions.'
  }
}

function loadMore() {
  load(false)
}

await load(true)

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString()
}

// Add by UUID dialog
const addByUUIDDialog = reactive({
  open: false,
  uuid: '',
  loading: false,
  error: '',
})

function openAddByUUID() {
  addByUUIDDialog.uuid = ''
  addByUUIDDialog.error = ''
  addByUUIDDialog.open = true
}

async function confirmAddByUUID() {
  const uid = addByUUIDDialog.uuid.trim()
  if (!uid) return
  addByUUIDDialog.loading = true
  addByUUIDDialog.error = ''
  try {
    const p = await apiFetch<Player>(`/api/v1/minecraft/players/${uid}`)
    addByUUIDDialog.open = false
    if (!players.value.some(existing => existing.uuid === p.uuid)) {
      players.value = [p, ...players.value]
    }
    toast.success(`Player ${p.username ?? p.uuid} added`)
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error ?? 'Failed to add player'
    addByUUIDDialog.error = msg === 'player not found' ? 'Player not found on Mojang' : msg
    toast.error(addByUUIDDialog.error)
  } finally {
    addByUUIDDialog.loading = false
  }
}

// Add by name dialog
const addByNameDialog = reactive({
  open: false,
  name: '',
  loading: false,
  error: '',
})

function openAddByName() {
  addByNameDialog.name = ''
  addByNameDialog.error = ''
  addByNameDialog.open = true
}

async function confirmAddByName() {
  const name = addByNameDialog.name.trim()
  if (!name) return
  addByNameDialog.loading = true
  addByNameDialog.error = ''
  try {
    const p = await apiFetch<Player>(`/api/v1/minecraft/players/lookup?name=${encodeURIComponent(name)}`)
    addByNameDialog.open = false
    if (!players.value.some(existing => existing.uuid === p.uuid)) {
      players.value = [p, ...players.value]
    }
    toast.success(`Player ${p.username ?? p.uuid} added`)
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error ?? 'Failed to add player'
    addByNameDialog.error = msg === 'player not found' ? 'Player not found on Mojang' : msg
    toast.error(addByNameDialog.error)
  } finally {
    addByNameDialog.loading = false
  }
}

// Delete player dialog
const deleteDialog = reactive({
  open: false,
  player: null as Player | null,
  loading: false,
  error: '',
})

function openDeleteDialog(p: Player) {
  deleteDialog.player = p
  deleteDialog.error = ''
  deleteDialog.open = true
}

async function confirmDelete() {
  if (!deleteDialog.player) return
  deleteDialog.loading = true
  deleteDialog.error = ''
  const { player } = deleteDialog
  try {
    await apiFetch(`/api/v1/minecraft/players/${player.uuid}`, { method: 'DELETE' })
    deleteDialog.open = false
    players.value = players.value.filter(p => p.id !== player.id)
    toast.success('Player deleted')
  } catch (e: unknown) {
    deleteDialog.error = (e as { data?: { error?: string } })?.data?.error ?? 'Failed to delete player'
    toast.error(deleteDialog.error)
  } finally {
    deleteDialog.loading = false
  }
}
</script>
