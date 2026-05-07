# v1.0.3 Production Backlog

Tracks pending changes for the **v1.0.3-plus-enhancements** fork (the live production branch at `desk.tenniswarehouse.com.au`). This is the *production* fork, not the v2 port — the v2 port is tracked separately at `docs/superpowers/specs/2026-04-27-v103-port-design.md` in the `v2.1.1-plus-enhancements` repo.

(Note: this lives at repo root as `BACKLOG.md` because the fork's `.gitignore` excludes the entire `docs/` tree.)

## Scope

Use this doc for:
- Bug fixes that need to land on production (v1.0.3) before — or instead of — v2.
- New v1.0.3-only features the user wants now without waiting for the v2 port.
- Backports of fixes/improvements that originated in v2 work and would also help production.

**Don't** use this doc for the v1.0.3 → v2 port direction. That's the v2 spec's job.

## How to add a task

1. Pick the next free `B##` ID (sequential).
2. Add a row to the relevant section's table with status `pending`.
3. Set effort: `S` = ~30 min, `M` = ~1–2h, `L` = ~3–5h, `XL` = day-long.
4. List dependencies on other backlog rows in the **Deps** column (or `–` if standalone).
5. **Notes** column should answer: what changes, where, why. Be specific — file paths and the *reason* for the change. Future agents pick up from this doc cold.

## Task intake from external agents (no git access required)

If you're an agent running outside this repo (e.g. on the Maho codebase, or PicoClaw, or any other Claude session) and you want to drop a task here, POST to the shared memory API. The libredesk Claude session will pull these in at session start and triage them into the tables below as real `B##` rows.

**Convention:**

```bash
curl -s -X POST https://mem.mageaustralia.com.au/memory \
  -H "Authorization: Bearer USW8pKBPUTu6Y21a7wSWajB6YgufSw2ARGqG1qDnzx4" \
  -H "Content-Type: application/json" \
  -d '{
    "content": "TASK-v1.0.3: <one-line title>. Details: <what changes, where, why>. Priority: high/normal/low.",
    "source": "task-libredesk-v103",
    "author": "<your-agent-name>"
  }'
```

The `source: task-libredesk-v103` tag is the key — it scopes incoming tasks so the libredesk Claude can pull them without dragging in unrelated session summaries.

**On the libredesk side**, at session start (or on demand):

```bash
curl -s -H "Authorization: Bearer USW8pKBPUTu6Y21a7wSWajB6YgufSw2ARGqG1qDnzx4" \
  "https://mem.mageaustralia.com.au/memory/search?q=TASK-v1.0.3&n=10"
```

After triaging an entry into a `B##` row, optionally `DELETE /memory/{id}` to clear it (or just leave it — search results decay over time).

## How to execute a task

1. Read the row's Notes column for full context.
2. Implement on the server (the v1.0.3 source of truth lives at `ubuntu@54.66.177.54:/home/ubuntu/libredesk/`). Per CLAUDE.md, agents must re-read files immediately before writing — never cache file contents across multiple tool calls.
3. Commit on `v1.0.3-plus-enhancements` with a conventional-commit-style message.
4. Update this doc: change status to `done <commit-hash-short>`. Commit the doc change separately (`docs(backlog): mark B## done <hash>`).
5. Push to `mageaustralia v1.0.3-plus-enhancements`.
6. If user-facing: deploy via `./deploy.sh` from local Mac (`/Volumes/second_disk/Development/libredesk/`). **Never** run `pnpm build` on the server — 1.8 GB RAM, OOMs the instance.

## Backports from v2 (low-risk improvements that originated in v2 port work)

| ID  | Title | v2 source | Effort | Deps | Status | Notes |
|-----|-------|-----------|--------|------|--------|-------|
| B01 | Filter persistence: replace sessionStorage with `useStorage` for cross-tab sync | v2 FS3 / FS8 | S | – | pending | v1.0.3 `frontend/src/views/search/SearchView.vue` saves `searchQuery` / `searchResults` to sessionStorage manually. v2 uses `@vueuse/core`'s `useStorage` which gives reactive cross-tab sync for free. Light refactor, no behaviour change for single-tab users. Verify the dep is already in `package.json` before assuming. |
| B02 | Tag filter: rename `not_contains` → `not contains` in admin filter UI | v2 FS-misc | S | – | pending | Cosmetic — the underlying SQL operator stays. v2 spec noted the snake_case label leaked into a user-visible dropdown. Find the i18n key in `i18n/en.json` (likely `filter.operator.notContains` or similar) and the dropdown render site. |
| B03 | Inline-image orphan cleanup on conversation send | v2 IP-misc | M | – | pending | Inline images uploaded into the editor but not actually referenced in the final HTML stay orphaned in the media table. v2 has a sweep pass on send. Port the same: when `handleSendOutgoing` finalises message HTML, scan for `<img src=…/uploads/…>` tags, diff against the media rows associated with the conversation draft, soft-delete the unreferenced ones. |

## Bugs

| ID  | Title | Effort | Deps | Status | Notes |
|-----|-------|--------|------|--------|-------|
| _(none currently)_ | | | | | |

## UX / Frontend

| ID  | Title | Effort | Deps | Status | Notes |
|-----|-------|--------|------|--------|-------|
| _(none currently)_ | | | | | |

## Backend / API

| ID  | Title | Effort | Deps | Status | Notes |
|-----|-------|--------|------|--------|-------|
| _(none currently)_ | | | | | |

## Security

| ID  | Title | Effort | Deps | Status | Notes |
|-----|-------|--------|------|--------|-------|
| _(none currently)_ | | | | | |

## Done (recent — for context, not exhaustive)

Older completed work lives in `git log` rather than here. Move rows out of the active tables once they're shipped + verified in production for ~7 days, otherwise this file grows unbounded.
