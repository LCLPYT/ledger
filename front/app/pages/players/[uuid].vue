<template>
  <div class="p-4 md:p-8 space-y-6">
    <!-- Back link -->
    <NuxtLink to="/players" class="text-sm text-muted-foreground hover:text-foreground transition-colors">
      ← Players
    </NuxtLink>

    <div v-if="fetchError" class="text-sm text-destructive">{{ fetchError }}</div>

    <template v-else-if="player">
      <!-- Player header -->
      <div>
        <h2 class="text-2xl font-semibold text-foreground">{{ player.username ?? player.uuid }}</h2>
        <p v-if="player.username" class="font-mono text-sm text-muted-foreground">{{ player.uuid }}</p>
        <p class="text-sm text-muted-foreground">Registered {{ formatDate(player.created_at) }}</p>
      </div>

      <!-- Coins section -->
      <div class="rounded-lg border p-4 space-y-3">
        <h3 class="text-sm font-medium text-muted-foreground uppercase tracking-wide">Coins</h3>
        <div class="flex items-center justify-between">
          <span class="text-2xl font-semibold">
            {{ coinsError ? '—' : balance.toLocaleString() }}
          </span>
          <div class="flex gap-2">
            <Button variant="outline" size="sm" @click="openHistory">History</Button>
            <Button
              v-if="hasPermission(Perms.CoinsWrite)"
              size="sm"
              @click="openAdjust"
            >Adjust</Button>
          </div>
        </div>
        <p v-if="coinsError" class="text-xs text-muted-foreground">{{ coinsError }}</p>
      </div>
    </template>

    <!-- Transaction history dialog -->
    <Dialog :open="historyDialog.open" @update:open="historyDialog.open = $event">
      <DialogContent class="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Coin transactions</DialogTitle>
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
          <p class="text-sm text-muted-foreground font-mono">{{ player?.username ?? player?.uuid }}</p>
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
            :disabled="!adjustDialog.amount || adjustDialog.loading"
            @click="confirmAdjust"
          >
            {{ adjustDialog.loading ? 'Saving…' : 'Apply' }}
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

const route = useRoute()
const uuid = route.params.uuid as string
const { apiFetch, hasPermission } = useAuth()

interface Player {
  id: number
  uuid: string
  username: string | null
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

const player = ref<Player | null>(null)
const balance = ref(0)
const fetchError = ref('')
const coinsError = ref('')

async function loadPlayer() {
  try {
    player.value = await apiFetch<Player>(`/api/v1/minecraft/players/${uuid}`)
  } catch (e: unknown) {
    const msg = (e as { data?: { error?: string } })?.data?.error
    fetchError.value = msg === 'player not found' ? 'Player not found.' : 'Failed to load player.'
  }
}

async function loadCoins() {
  try {
    const res = await apiFetch<{ balance: number }>(`/api/v1/minecraft/players/${uuid}/coins`)
    balance.value = res.balance
  } catch {
    coinsError.value = 'No balance yet.'
  }
}

await Promise.all([loadPlayer(), loadCoins()])

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString()
}

// Transaction history dialog
const historyDialog = reactive({
  open: false,
  transactions: [] as Transaction[],
  error: '',
})

async function openHistory() {
  historyDialog.transactions = []
  historyDialog.error = ''
  historyDialog.open = true
  try {
    historyDialog.transactions = await apiFetch<Transaction[]>(
      `/api/v1/minecraft/players/${uuid}/coins/transactions`
    )
  } catch {
    historyDialog.error = 'Failed to load transactions.'
  }
}

// Adjust balance dialog
const adjustDialog = reactive({
  open: false,
  amount: 0,
  description: '',
  loading: false,
  error: '',
})

function openAdjust() {
  adjustDialog.amount = 0
  adjustDialog.description = ''
  adjustDialog.error = ''
  adjustDialog.open = true
}

async function confirmAdjust() {
  if (!adjustDialog.amount) return
  adjustDialog.loading = true
  adjustDialog.error = ''
  try {
    const res = await apiFetch<{ balance: number }>(`/api/v1/minecraft/players/${uuid}/coins/adjust`, {
      method: 'POST',
      body: {
        amount: adjustDialog.amount,
        source: 'admin',
        description: adjustDialog.description.trim() || undefined,
      },
    })
    adjustDialog.open = false
    balance.value = res.balance
    coinsError.value = ''
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
