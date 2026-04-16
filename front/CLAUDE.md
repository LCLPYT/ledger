# CLAUDE.md — front

## Stack

- **Nuxt 4** (Vue 3, `app/` source dir)
- **Tailwind CSS v4** via `@tailwindcss/vite`
- **shadcn-vue** (new-york style, neutral base, CSS variables, `reka-ui` v2 primitives)
- **Pinia** (`@pinia/nuxt`)
- **lucide-vue-next** for icons

## Commands

```sh
npm run dev       # dev server
npm run build     # production build
npm run generate  # static generation
npx nuxt typecheck  # type-check without building
```

## shadcn components

Install new components with:
```sh
yes N | npx shadcn-vue@latest add <component>
```
The `yes N` skips overwrite prompts for already-installed components.

Components live in `app/components/ui/<name>/`.

Config: `components.json`. Aliases: `@/components/ui`, `@/lib`, `@/composables`.

## Key files

| Path | Role |
|---|---|
| `app/pages/login.vue` | Login form (no layout) |
| `app/pages/index.vue` | Dashboard (auth middleware) |
| `app/pages/users/index.vue` | Users table + role management |
| `app/layouts/default.vue` | Responsive layout: fixed sidebar on `md+`, hamburger drawer on mobile |
| `app/composables/useAuth.ts` | login, logout, fetchUser, apiFetch |
| `app/middleware/auth.ts` | Route guard + session refresh |
| `app/assets/css/main.css` | Tailwind entry + CSS variable theme |
| `app/lib/utils.ts` | `cn()` helper (clsx + tailwind-merge) |

## Patterns

**API calls** — use `apiFetch` from `useAuth`, not `$fetch` directly. It attaches the Bearer token.

**Auth state** — `useAuth()` exposes `user`, `login`, `logout`, `fetchUser`, `apiFetch`, `permissions`, `fetchPermissions`, `hasPermission`.

**Permission constants** — `app/utils/permissions.ts` exports `Perms` (e.g. `Perms.UsersRead`), auto-imported everywhere. Mirrors the backend `perms` package. Use `hasPermission(Perms.Foo)` to gate UI elements; `permissions` is `null` until loaded (treat as "not yet known", not "none granted").

**Permission-gated UI** — use `v-if="hasPermission(Perms.Foo)"` on buttons/actions. `permissions` is fetched once after login and cached in `useState`; no per-page fetch needed.

**shadcn Dialog** — always use `:open` + `@update:open`, never `v-if` on `<Dialog>`. `v-if` breaks Escape-key close.

**shadcn Select** — `v-model` goes on `<Select>`, not `<SelectTrigger>`. Placeholder lives in `<SelectValue placeholder="...">`, not an empty `<SelectItem>`.

**Color theme** — uses oklch CSS variables (`--background`, `--foreground`, `--primary`, `--sidebar-*`, etc.) defined in `main.css`. Light and dark modes supported via `.dark` class.

**Responsive layout** — sidebar hidden below `md` (768px). Mobile shows a top bar with a hamburger that opens a `Sheet` drawer. Route changes auto-close the drawer via `watch(route, ...)`. Breakpoint pattern: `hidden md:flex` on sidebar, `flex md:hidden` on mobile header.