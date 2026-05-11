# demo-seeder

Single-binary Go seeder that loads a fresh libredesk install with realistic
data exercising every fork feature documented in the top-level `README.md`.
Built so a reviewer can clone the repo, `docker compose up`, run
`./seed.sh`, and have enough data to screenshot or screen-record every fork
feature without manual click-through.

## Quick start

```bash
# from the repo root
cd tools/demo-seeder
./seed.sh
```

That's it. Default target is `http://localhost:9000` with System creds
`System` / `changeme` (the docker-compose default after running
`docker exec -it libredesk_app ./libredesk --set-system-user-password`).

## Flags

| Flag | Default | What it does |
|---|---|---|
| `--url <url>` | `http://localhost:9000` | Base URL of the libredesk API |
| `--user <user>` | `System` | System user to authenticate as |
| `--pass <pass>` | `changeme` | System user password |
| `--reset` | off | Wipe existing demo data first (best-effort) |
| `--allow-non-localhost` | off | Required to point at non-loopback hosts |
| `--skip-ecommerce` | off | Skip ecommerce settings (handy if you've already configured it) |
| `--skip-push-tokens` | off | Skip FCM push-token seed |
| `-v` | off | Verbose HTTP logging |

## Safety

The seeder refuses to run against any host that doesn't resolve to
`127.0.0.1` / `::1` unless you pass `--allow-non-localhost`. This is a
guardrail against fat-fingering a production URL — running this against
production would create dozens of garbage tickets and a fake demo admin.

The fake email inbox is created with `enabled=false` and IMAP/SMTP creds
set to `DISABLED-BY-DEMO-SAFETY` so even if you somehow flip it on, it
can't actually authenticate against a real mail server.

The fake FCM push token is a literal string `demo-fcm-token-DISABLED-...`
that Firebase will reject immediately — no actual mobile devices receive
notifications.

## What gets created

Run the seeder and read the summary at the end. Roughly:

- 3 agents (`admin@demo.local`, `agent1@demo.local`, `agent2@demo.local`,
  all password `DemoPassw0rd!`)
- 2 teams (`Demo Sales`, `Demo Support`)
- 2 inboxes (`Demo Email`, `Demo Chat Widget`)
- 5 macros across folder prefixes `[Demo: Order Status]`, `[Demo: Returns]`,
  `[Demo: Greetings]`
- 2 knowledge sources (webpage + macro-based)
- 1 shared view (`Demo: Unassigned + Open`)
- 6 tags (`vip`, `complaint`, `feature-request`, `spam-rescued`,
  `duplicate`, `demo`)
- ~20 conversations exercising spam, trash, merging, customer history,
  inline images, PCI redaction, voicemail transcripts, mentions, varied
  statuses/priorities/ages, and team/agent assignments
- Ecommerce config (Magento1 type, fake creds — Connection Test will fail
  but the settings page renders for screenshots)
- 1 FCM push token (fake, for the mobile demo)

For the per-feature screenshot recipe see `docs/DEMO_SETUP.md` at the repo
root.

## Idempotency

Re-running without `--reset` is safe. The seeder detects existing rows by
name/email and skips them. `--reset` deletes anything matching the demo
patterns:

- Agents with email ending `@demo.local`
- Inboxes named `Demo ...`
- Teams named `Demo Sales` / `Demo Support`
- Macros named `[Demo: ...]`
- RAG sources / shared views prefixed `Demo: `

**Conversations are NOT wiped by `--reset`** (no safe per-row delete via
the API — conversations go via the Trash flow). Re-running on a populated
DB will just add more conversations.

## Design notes

- **Stdlib only** — a third-party fake-data library felt disproportionate
  for a tool that runs a handful of times. We hand-curate a small pool of
  names/subjects/messages and use a deterministic `rand.NewSource(20260507)`
  so output is reproducible.
- **Real API endpoints** — same paths the Vue frontend uses. Keeps the
  seeder honest against schema migrations and incidentally serves as
  informal API documentation. The `--reset` path is the same way (DELETE
  endpoints), not raw SQL.
- **Single Go module** — `tools/demo-seeder/go.mod` is separate from the
  parent module so this tool can't drag dependencies into the main app.
