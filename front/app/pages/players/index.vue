<template>
  <div class="p-4 md:p-8 space-y-6">
    <div class="flex items-center justify-between">
      <h2 class="text-2xl font-semibold text-foreground">Players</h2>
      <Button v-if="hasPermission(Perms.CoinsWrite)" @click="openAdjust(null)">Adjust by UUID</Button>
    </div>

    <p v-if="fetchError" class="text-sm text-destructive">{{ fetchError }}</p>

    <template v-else>
      <div class="rounded-lg border overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Balance</TableHead>
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
              <TableCell class="font-medium">{{ p.balance.toLocaleString() }}</TableCell>
              <TableCell>{{ formatDate(p.created_at) }}</TableCell>
              <TableCell class="text-right">
                <div class="flex items-center justify-end gap-2">
                  <Button variant="ghost" size="sm" @click="openHistory(p)">History</Button>
                  <Button
                    v-if="hasPermission(Perms.CoinsWrite)"
                    variant="ghost"
                    size="sm"
                    @click="openAdjust(p)"
                  >Adjust</Button>
                  <Button
                    v-if="hasPermission(Perms.CoinsWrite)"
                    variant="ghost"
                    size="sm"
                    class="text-destructive hover:text-destructive"
                    @click="openDeleteDialog(p)"
                  >Delete</Button>
                </div>
              </TableCell>
            </TableRow>
            <TableRow v-if="players.length === 0">
              <TableCell colspan="4" class="text-center text-muted-foreground">No players yet.</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <!-- Pagination -->
      <div v-if="players.length === limit" class="flex justify-end">
        <Button variant="outline" size="sm" @click="loadMore">Load more</Button>
      </div>
    </template>

    <!-- Transaction history dialog -->
    <Dialog :open="historyDialog.open" @update:open="historyDialog.open = $event">
      <DialogContent class="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Transactions — {{ historyDialog.player?.username ?? historyDialog.player?.uuid }}</DialogTitle>
        </DialogHeader>

        <p v-if="historyDialog.error" class="text-sm text-destructive">{{ historyDialog.error }}</p>

        <div class="rounded-lg border overflow-x-auto max-h-96">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Date</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Source</TableHead>
                <TableHead>Description</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="t in historyDialog.transactions" :key="t.id">
                <TableCell class="text-xs whitespace-nowrap">{{ formatDate(t.created_at) }}</TableCell>
                <TableCell
                  class="font-medium"
                  :class="t.amount >= 0 ? 'text-green-600' : 'text-destructive'"
                >
                  {{ t.amount >= 0 ? '+' : '' }}{{ t.amount.toLocaleString() }}
                </TableCell>
                <TableCell>
                  <Badge variant="secondary">{{ t.source }}</Badge>
                </TableCell>
                <TableCell class="text-muted-foreground">{{ t.description ?? '—' }}</TableCell>
              </TableRow>
              <TableRow v-if="historyDialog.transactions.length === 0">
                <TableCell colspan="4" class="text-center text-muted-foreground">No transactions.</TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </DialogContent>
    </Dialog>

    <!-- Adjust balance dialog -->
    <Dialog :open="adjustDialog.open" @update:open="adjustDialog.open = $event">
      <DialogContent class="w-80">
        <DialogHeader>
          <DialogTitle>Adjust balance</DialogTitle>
          <p v-if="adjustDialog.player" class="text-sm text-muted-foreground font-mono">{{ adjustDialog.player?.username ?? adjustDialog.player?.uuid }}</p>
          <div v-else class="space-y-1 pt-1">
            <Label for="adj-uuid">Player UUID</Label>
            <Input id="adj-uuid" v-model="adjustDialog.uuid" placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" class="font-mono text-sm" />
          </div>
        </DialogHeader>

        <div class="space-y-4">
          <div class="space-y-1">
            <Label for="adj-amount">Amount</Label>
            <Input
              id="adj-amount"
              v-model.number="adjustDialog.amount"
              type="number"
              placeholder="e.g. 100 or -50"
            />
            <p class="text-xs text-muted-foreground">Positive to add, negative to deduct.</p>
          </div>
          <div class="space-y-1">
            <Label for="adj-desc">Description (optional)</Label>
            <Input id="adj-desc" v-model="adjustDialog.description" placeholder="Reason…" />
          </div>
          <p v-if="adjustDialog.error" class="text-sm text-destructive">{{ adjustDialog.error }}</p>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="adjustDialog.open = false">Cancel</Button>
          <Button
            :disabled="(!adjustDialog.player && !adjustDialog.uuid.trim()) || !adjustDialog.amount || adjustDialog.loading"
            @click="confirmAdjust"
          >
            {{ adjustDialog.loading ? 'Saving…' : 'Apply' }}
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
        <p class="text-sm">This will permanently remove the player, their balance, and all transaction history.</p>
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
import { Badge } from '@/components/ui/badge'
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
  balance: number
  created_at: string
}

interface Transaction {
  id: number
  player_id: number
  amount: number
  source: string
  description: string | null
  created_at: string
  actor_user_id: number | null
  actor_token_id: number | null
}

const limit = 50
const players = ref<Player[]>([])
const fetchError = ref('')
const offset = ref(0)

async function load(reset = false) {
  if (reset) {
    offset.value = 0
    players.value = []
  }
  try {
    const page = await apiFetch<Player[]>(`/api/v1/players?limit=${limit}&offset=${offset.value}`)
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

// Transaction history dialog
const historyDialog = reactive({
  open: false,
  player: null as Player | null,
  transactions: [] as Transaction[],
  error: '',
})

async function openHistory(p: Player) {
  historyDialog.player = p
  historyDialog.transactions = []
  historyDialog.error = ''
  historyDialog.open = true
  try {
    historyDialog.transactions = await apiFetch<Transaction[]>(
      `/api/v1/players/${p.uuid}/coins/transactions`
    )
  } catch {
    historyDialog.error = 'Failed to load transactions.'
  }
}

// Adjust balance dialog
const adjustDialog = reactive({
  open: false,
  player: null as Player | null,
  uuid: '',
  amount: 0,
  description: '',
  loading: false,
  error: '',
})

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
    await apiFetch(`/api/v1/players/${player.uuid}`, { method: 'DELETE' })
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

const uuidRegex = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

function openAdjust(p: Player | null) {
  adjustDialog.player = p
  adjustDialog.uuid = ''
  adjustDialog.amount = 0
  adjustDialog.description = ''
  adjustDialog.error = ''
  adjustDialog.open = true
}

async function confirmAdjust() {
  const targetUuid = adjustDialog.player?.uuid ?? adjustDialog.uuid.trim()
  if (!targetUuid || !adjustDialog.amount) return
  if (!adjustDialog.player && !uuidRegex.test(targetUuid)) {
    adjustDialog.error = 'Invalid UUID format'
    return
  }
  adjustDialog.loading = true
  adjustDialog.error = ''
  const { player, amount, description } = adjustDialog
  try {
    const res = await apiFetch<{ balance: number }>(`/api/v1/players/${targetUuid}/coins/adjust`, {
      method: 'POST',
      body: {
        amount,
        source: 'admin',
        description: description.trim() || undefined,
      },
    })
    adjustDialog.open = false
    if (player) {
      const found = players.value.find(p => p.id === player.id)
      if (found) found.balance = res.balance
    } else {
      await load(true)
    }
    toast.success(`Balance adjusted to ${res.balance.toLocaleString()}`)
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error ?? 'Failed to adjust balance'
    adjustDialog.error = msg === 'insufficient_balance' ? 'Insufficient balance' : msg
    toast.error(adjustDialog.error)
  } finally {
    adjustDialog.loading = false
  }
}
</script>
