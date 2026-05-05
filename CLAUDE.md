# Libredesk - Customer Support Desk with AI Integration

## Project Overview
Libredesk is an open-source customer support desk application with custom RAG AI integration. This is a fork/customization of the upstream libredesk project.

## Remote Server
- **Host**: ubuntu@54.66.177.54
- **Architecture**: ARM64 (AWS Graviton), 1.8GB RAM + 2GB swap
- **Project Path**: /home/ubuntu/libredesk/
- **URL**: Access via web browser (port 9000)
- **IMPORTANT**: Never run `pnpm build` on the server — it OOM-kills the instance. Use the local deploy script instead.
- **CRITICAL**: Server source is version-controlled. All changes are committed after each deploy. When using Agent tool to modify server files, agents MUST read the file immediately before writing — never cache file contents across multiple tool calls. Stale reads cause silent overwrites of other changes.

## Current Version & Branch
- **Base Version**: v1.0.1 (upgraded 2026-02-03)
- **Branch**: `feature/openrouter-v1.0.1`
- **Includes**: OpenRouter AI provider + RAG AI assistant

## Tech Stack
- **Backend**: Go
- **Frontend**: Vue.js 3 with Vite
- **Database**: PostgreSQL 17 with pgvector extension (for RAG/semantic search)
- **Cache**: Redis 7
- **Deployment**: Docker Compose (local build, not official image)

## Architecture
```
libredesk/
├── cmd/                    # Go entry points and API handlers
│   ├── ai.go               # AI provider handlers (OpenAI, Claude, OpenRouter)
│   ├── rag.go              # RAG handlers (generate response, knowledge sources)
│   ├── ecommerce.go        # Ecommerce settings and status handlers
│   └── handlers.go         # Route registration
├── frontend/
│   ├── src/
│   │   ├── features/
│   │   │   ├── conversation/
│   │   │   │   ├── ReplyBox.vue           # Reply editor with AI prompts + Generate Response
│   │   │   │   ├── ReplyBoxContent.vue    # CC/BCC fields with clear (X) buttons
│   │   │   │   └── ReplyBoxMenuBar.vue    # Generate Response + Orders buttons
│   │   │   └── admin/inbox/
│   │   │       ├── EmailInboxForm.vue     # Inbox settings form
│   │   │       └── formSchema.js          # Zod validation schema
│   │   ├── stores/conversation.js         # Conversation state
│   │   ├── views/admin/
│   │   │   ├── ai/
│   │   │   │   ├── AISettings.vue         # AI provider settings
│   │   │   │   └── RAGSettings.vue        # Knowledge sources
│   │   │   └── ecommerce/
│   │   │       └── EcommerceSettings.vue  # Ecommerce integration settings
│   │   ├── router/index.js                # All routes
│   │   └── constants/navigation.js        # Admin menu items
│   └── dist/               # Built frontend (stuffed into binary)
├── internal/ai/
│   ├── ai.go               # AI manager
│   ├── openai.go           # OpenAI client (multimodal support)
│   ├── openrouter.go       # OpenRouter client (multimodal support)
│   └── provider.go         # Provider interface + ImageContent struct
├── internal/rag/           # RAG AI assistant
│   ├── rag.go              # RAG manager + image extraction
│   ├── models/             # RAG data models
│   └── sync/               # Knowledge source syncing (webpages, macros)
├── internal/ecommerce/     # Ecommerce integration
│   ├── provider.go         # Provider interface
│   ├── models.go           # Order, Customer, Shipment, Address
│   ├── manager.go          # Multi-stage context gathering
│   └── magento1/           # Magento 1/Maho Commerce provider
│       ├── client.go       # API client
│       └── auth.go         # OAuth2 token management
├── internal/image/
│   └── resize.go           # Image resizing for AI (500x500 max)
├── deploy.sh               # Deployment script
└── upgrade.sh              # Version upgrade script
```

## Docker Setup
```yaml
services:
  app:        # Main app on port 9000 (LOCAL BUILD, not official image)
  db:         # PostgreSQL on 127.0.0.1:5433 (port 5432 is system postgres)
  redis:      # Redis on 127.0.0.1:6380 (port 6379 is system redis)
```

**IMPORTANT**: docker-compose.yml must use local build, NOT the official image:
```yaml
app:
  build:
    context: .
    dockerfile: Dockerfile
  # NOT: image: libredesk/libredesk:latest
```

## Deployment Process

**IMPORTANT**: The server only has 1.8GB RAM. Frontend builds MUST run locally on Mac — never on the server.

**Standard deploy (use this):**
```bash
cd /Volumes/second_disk/Development/libredesk
./deploy.sh
```

This script:
1. Syncs frontend source from server → local `.frontend-build/`
2. Runs `pnpm build` locally on Mac (fast, no OOM risk)
3. Uploads `dist/` to server via rsync
4. Builds Go binary on server (lightweight, ~200MB)
5. Runs stuffbin + Docker rebuild + restart

**Manual steps (if deploy.sh fails):**
```bash
# On Mac: build frontend locally
cd /Volumes/second_disk/Development/libredesk/.frontend-build
pnpm install --frozen-lockfile
pnpm build
rsync -az --delete dist/ ubuntu@54.66.177.54:/home/ubuntu/libredesk/frontend/dist/

# On server: Go build + stuffbin + Docker
ssh ubuntu@54.66.177.54
cd /home/ubuntu/libredesk
export PATH=$PATH:/home/ubuntu/go/bin
VERSION=$(git describe --tags --always)
CGO_ENABLED=0 go build -ldflags "-s -w -X 'main.buildString=$VERSION' -X 'main.versionString=$VERSION'" -o libredesk ./cmd/
stuffbin -a stuff -in libredesk -out libredesk frontend/dist i18n schema.sql static
docker compose build --no-cache app
docker compose up -d --force-recreate app
```

### Stuffbin Details
- **Location**: `/home/ubuntu/go/bin/stuffbin`
- **Purpose**: Embeds static assets (frontend, i18n, schema) into the Go binary
- **Verify bundled assets**: `stuffbin -a id -in libredesk`
- **If stuffbin not found**: `go install github.com/knadh/stuffbin/...@latest`

### Common Deployment Issues
- **502 Error after deploy**: Binary is dynamically linked. Rebuild with `CGO_ENABLED=0`
- **Frontend changes not showing**: stuffbin wasn't run, or old binary in container
- **New routes/pages missing**: Check router/navigation files, rebuild frontend, re-stuff

## AI Features

### Two AI Systems

**1. Default AI (Built-in v1.0.1)**
- **Purpose**: Transform/enhance selected text in replies
- **How to use**: Write text → Select it → Choose AI prompt from BubbleMenu
- **Location**: ReplyBoxMenuBar.vue, TextEditor.vue BubbleMenu
- **Providers**: OpenAI, Claude, OpenRouter

**2. RAG AI Assistant (Custom)**
- **Purpose**: Generate full AI response from knowledge base
- **How to use**: Click "Generate Response" button in ReplyBox
- **Location**: ReplyBox.vue → handleGenerateResponse()
- **Features**:
  - Uses conversation context
  - Searches knowledge base (pgvector similarity)
  - Generates contextual reply
  - Knowledge Sources: webpages, macros

### Provider Configuration
- **Settings Page**: `/admin/ai`
- **Knowledge Sources**: `/admin/knowledge-sources`
- **Database Table**: `ai_providers`
- **Supported**: OpenAI, Claude, OpenRouter (100+ models)

## Key Customizations

### 1. Ticket ID in Header (UI Fix)
**File**: `frontend/src/stores/conversation.js`
```js
// currentContactName shows: "Matthew Campbell #105 - Subject here"
return conversation.data?.contact.first_name + ' ' + conversation.data?.contact.last_name
  + (conversation.data?.reference_number ? ' #' + conversation.data.reference_number : '')
  + (conversation.data?.subject ? ' - ' + conversation.data.subject : '')
```

### 2. Simple Name in Sidebar
**File**: `frontend/src/features/conversation/sidebar/ConversationSideBarContact.vue`
```vue
<!-- Shows only: "Matthew Campbell" (no ticket ID/subject to avoid overlay) -->
{{ conversation?.contact?.first_name + ' ' + conversation?.contact?.last_name }}
```

### 3. OpenRouter Support
**Files**:
- `internal/ai/openrouter.go` - OpenRouter client
- `internal/ai/provider.go` - Added `ProviderOpenRouter`
- `internal/ai/ai.go` - Added OpenRouter case with encryption
- `cmd/ai.go` - API handlers for provider management
- `frontend/src/views/admin/ai/AISettings.vue` - Settings UI

### 4. RAG AI Assistant
**Files**:
- `cmd/rag.go` - RAG API handlers
- `internal/rag/` - RAG manager and models
- `internal/rag/sync/` - Knowledge source syncing
- `frontend/src/features/conversation/ReplyBox.vue` - Generate Response button
- `frontend/src/views/admin/ai/RAGSettings.vue` - Knowledge sources UI

### 5. Ecommerce Integration (Magento/Maho Commerce)
**Settings Page**: `/admin/ecommerce`
**Files**:
- `internal/ecommerce/provider.go` - Provider interface
- `internal/ecommerce/models.go` - Order, Customer, Shipment models
- `internal/ecommerce/manager.go` - Multi-stage context gathering
- `internal/ecommerce/magento1/` - Magento 1/Maho Commerce provider
- `cmd/ecommerce.go` - API handlers
- `frontend/src/views/admin/ecommerce/EcommerceSettings.vue` - Settings UI

**Features**:
- OAuth2 client_credentials authentication
- Customer lookup by email
- Order lookup by email or order number
- Multi-stage context gathering (always fetches customer + recent orders, scans conversation for order numbers, fetches full details for mentioned orders)
- "+ Orders" button in ReplyBox (only shows when configured)

### 6. Image Support in RAG
**Files**:
- `internal/image/resize.go` - Resize images to 500x500 for AI context
- `internal/ai/provider.go` - `ImageContent` struct for multimodal prompts
- `internal/ai/openai.go`, `openrouter.go` - Multimodal request formatting

**Features**:
- Extracts images from conversation attachments
- Resizes to max 500x500 preserving aspect ratio
- Includes as base64 in multimodal AI prompts

### 7. Auto-assign on Reply (Per-inbox setting)
**Files**:
- `frontend/src/features/admin/inbox/EmailInboxForm.vue` - Toggle UI
- `frontend/src/features/admin/inbox/formSchema.js` - Zod validation (includes `auto_assign_on_reply`)
- `internal/inbox/inbox.go` - Backend handling

### 8. Forward Message
**Purpose**: Forward individual conversation messages to external parties (suppliers, freight companies, etc.)
**How it works**:
- Forward button on each non-activity, non-private message in the thread
- Opens ReplyBox in "Forward" mode with the message content pre-populated
- TO field is empty — agent enters the external recipient
- Prior messages shown in collapsible `...` thread (editable, same as reply)
- Sent email gets "Fwd:" subject prefix and starts a new email thread
- Activity note logged: "Agent forwarded to recipient@example.com"
- Forwarded messages show "Forwarded to:" badge in the UI

**Backend files**:
- `cmd/messages.go` - `ForwardedTo` field, meta handling, activity note
- `internal/conversation/message.go` - Fwd: subject prefix, clear threading headers, `InsertForwardActivityNote()`

**Frontend files**:
- `frontend/src/features/conversation/message/MessageBubble.vue` - Forward button + forwarded badge
- `frontend/src/features/conversation/ReplyBox.vue` - Forward mode, content population, thread building
- `frontend/src/features/conversation/ReplyBoxContent.vue` - Forward tab
- `frontend/src/features/conversation/Conversation.vue` - Forward event wiring

**Mobile API**: Same endpoint, add `forwarded_to` array:
```json
POST /api/v1/conversations/:uuid/messages
{"message": "<p>FYI</p>", "private": false, "sender_type": "agent", "forwarded_to": ["recipient@example.com"]}
```

### 9. Client-side Email Threading
**Purpose**: Quoted thread in replies/forwards is built client-side so agents can edit it before sending.
- Server-side thread append removed — all threading is in the frontend
- `_buildThread()` in ReplyBox.vue builds last 3 messages as quoted blocks
- Shown via `...` toggle in ReplyBoxContent.vue (collapsed by default)
- Thread is editable when expanded
- Included in sent email with `<!-- thread -->` marker

## Upgrade Workflow

### v1.0.3 → v2 port (in progress)

The full feature-parity port from `v1.0.3-plus-enhancements` to upstream v2 is tracked as a structured spec, **not** a one-shot cherry-pick. Always check the spec before doing v2-related work.

- **Branch (port target)**: `v2.1.1-plus-enhancements` on `mageaustralia/libredesk`
- **Working dir**: `/Volumes/second_disk/Development/libredesk-staging/repo/`
- **Design spec**: `docs/superpowers/specs/2026-04-27-v103-port-design.md` — sectioned task list (5.0 already-done, 5.1 Tier 1, 5.2 Email composer, 5.3 Email rendering, 5.4 Filters, 5.5 Macros/per-inbox, 5.6 IMAP plumbing, 5.7 Security, 5.8 Recent activities, 5.9 UX polish, 5.10–5.14 Tier 3). IDs like T1a, EC1, ER1, IP1 etc. with status `pending` / `done <hash>`.
- **Tier plans**: `docs/superpowers/plans/` — per-tier execution plans.
- **When adding a new v1.0.3 feature** that hasn't been ported yet: add a row to the right section of the spec with `pending` status, deps on prerequisite IDs, and a Notes column describing the change. Don't try to apply features to v2 ad-hoc.
- **Per-session protocol**: read spec § 6. End-of-session push to `mageaustralia/v2.1.1-plus-enhancements` and checkpoint to shared memory under source `checkpoint-libredesk-port`.

### Legacy cherry-pick workflow (only for hotfixes against same major)

When a new upstream version is released:

```bash
ssh ubuntu@54.66.177.54
cd /home/ubuntu/libredesk

# 1. Fetch new tags
git fetch origin --tags

# 2. Create new feature branch from new version
git checkout -b feature/openrouter-v1.0.2 v1.0.2

# 3. Cherry-pick our customization commits
git cherry-pick <openrouter-commit-hash>
git cherry-pick <rag-commit-hash>

# 4. Resolve conflicts if any (keep new version + add our code)

# 5. Fix docker-compose.yml (always check these):
# - dockerfile: Dockerfile (not Dockerfile.custom)
# - redis port: 127.0.0.1:6380:6379

# 6. Deploy
./deploy.sh

# 7. Push to your fork
git push trabulium feature/openrouter-v1.0.2
```

### Current Custom Commits to Cherry-pick
```bash
# Get these from: git log --oneline feature/openrouter-v1.0.1
# Typically 2-3 commits:
# - OpenRouter support commit
# - RAG AI assistant commit
# - Any UI fixes
```

### Conflict Resolution Tips
- **ai.go conflicts**: Keep new version's encryption, add OpenRouter case
- **frontend conflicts**: Keep both - merge v1.0.x features + our custom code
- **docker-compose.yml**: Always verify local build + correct ports
- **router/navigation**: Add our routes to new structure

### Manual Fixes After Cherry-pick
Some things may need manual fixing each upgrade:
1. Check `docker-compose.yml` uses `Dockerfile` not `Dockerfile.custom`
2. Check Redis port is `6380` not `6379`
3. Verify AI Settings and Knowledge Sources routes exist
4. Test both AI features after deploy

## Git Remotes
- `origin` = upstream (abhinavxd/libredesk)
- `trabulium` = your fork (Trabulium/libredesk)

## Troubleshooting

### Wrong Version Showing (v0.8.6-beta)
- docker-compose.yml is using `image: libredesk/libredesk:latest` instead of local build
- Fix: Change to `build: context: . dockerfile: Dockerfile`

### Redis Port Conflict
- System Redis uses 6379
- docker-compose.yml should use: `"127.0.0.1:6380:6379"`

### AI Settings Not Visible
- Hard refresh browser (Ctrl+Shift+R)
- Check you have `ai:manage` permission
- Verify routes exist in handlers.go

### Generate Response Not Working
- Check RAG handlers registered in cmd/handlers.go
- Verify `internal/rag/` package exists
- Check `internal/setting/setting.go` has `GetAISettings()` method
- Verify AI provider is configured in database

### Build Errors
- Frontend: Check for conflict markers (`<<<<<<`, `======`, `>>>>>>`)
- Backend: Check Go imports match package structure
- SCSS: Check brace matching in Vue component styles

### IMAP Authentication Error in Logs
- Email inbox credentials need updating
- Check Admin → Inboxes → Edit

## Notes
- Server has limited memory - use `NODE_OPTIONS='--max-old-space-size=4096'` for frontend builds
- Frontend build takes ~1 minute
- The `stuffbin` tool embeds static assets into the Go binary
- Backups stored in `/home/ubuntu/libredesk/backups/`
- Version string in logs may be blank (deploy.sh doesn't set ldflags)
- RAG sync runs hourly (configurable in coordinator.go)

## Local Working Files
This local directory contains working files from development sessions:
- `AISettings.vue` / `AISettings-fixed.vue` - AI settings component work
- `TextEditor.vue` - Text editor component
- `import_magento_to_libredesk.py` - Script to import Magento customers
- `openrouter-support.patch` - Original OpenRouter patch (39KB)

## Recent Changes (2026-02-03)
- Upgraded from v0.8.0-beta to v1.0.1
- Rebased OpenRouter support onto v1.0.1 (resolved ai.go encryption conflict)
- Added RAG AI assistant feature (cherry-picked from rag-ai-assistant branch)
- Fixed docker-compose.yml to use local build (not official image)
- Fixed Redis port to 6380 (system Redis uses 6379)
- Created upgrade.sh and custom-patches infrastructure
- Applied ticket ID fix to conversation header
- Added Knowledge Sources admin page

## Recent Changes (2026-02-05)
- Added image support for RAG (extracts images from attachments, resizes to 500x500, multimodal AI prompts)
- Added ecommerce integration (Magento 1/Maho Commerce provider with multi-stage context)
- Added ecommerce settings page at `/admin/ecommerce`
- Added "+ Orders" button in ReplyBox for AI responses with ecommerce context
- Added clear (X) button to CC/BCC email fields
- Fixed auto-assign on reply setting not saving (missing from formSchema.js and NewInbox.vue)

## Recent Changes (2026-03-19)
- Added forward message feature (per-message forward button, Forward mode in ReplyBox, Fwd: subject, activity notes)
- Moved email threading from server-side to client-side (editable quoted thread with ... toggle)
