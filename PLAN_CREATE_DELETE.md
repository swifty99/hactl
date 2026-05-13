# PLAN: `hactl` Create/Delete Commands

**Branch:** `feat/create-delete`  
**Goal:** Full CRUD for every HA entity type via CLI, backed by companion endpoints where needed.

---

## Scope Overview

| Entity Type | CLI Commands to Add | Backend |
|---|---|---|
| **Automations** | `auto create -f`, `auto delete <id>` | Companion (endpoints exist) |
| **Scripts** | `script create -f`, `script delete <id>` | Companion (endpoints exist) |
| **Templates** | `tpl create -f --domain <d>`, `tpl delete <id>` | Companion (endpoints exist) |
| **Helpers** | `helper create <domain> -f`, `helper delete <entity_id>` | Companion (NEW endpoints) |
| **Scenes** | `scene create -f`, `scene delete <id>` | HA WebSocket/REST |
| **Labels** | `label delete <name>` | HA WebSocket (create exists) |
| **Areas** | `area create <name>`, `area delete <name>` | HA WebSocket |
| **Floors** | `floor create <name>`, `floor delete <name>` | HA WebSocket |

**UX contract (all commands):**
- Input via `-f <file.yaml>` 
- Dry-run by default (preview what will happen)
- `--confirm` to execute
- Auto-backup before destructive actions
- Rollback support for created entities

---

## Step-by-Step Plan

### 1. Branch & Prep
- [ ] Create branch `feat/create-delete` from `main`
- [ ] Verify local dev setup: `make build && make test` green

---

### 2. OpenAPI Spec — Companion Helpers Endpoints
- [ ] Add to `hactl_companion/openapi/companion-v1.yaml`:
  - `POST /v1/config/helpers` — create helper (query: `?domain=input_boolean|input_number|...`)
  - `GET /v1/config/helpers` — list helpers (query: `?domain=...`)
  - `GET /v1/config/helpers?id=<id>` — get single helper
  - `PUT /v1/config/helpers?id=<id>` — update helper
  - `DELETE /v1/config/helpers?id=<id>` — delete helper
- [ ] Define request/response schemas per helper domain

### 3. Companion — Implement Helper Routes
- [ ] Create `hactl_companion/src/companion/routes/helpers.py`
  - Use HA WebSocket API: `config/helpers/registry/create`, `config/helpers/registry/delete`, `config/helpers/registry/update`
  - Support domains: `input_boolean`, `input_number`, `input_select`, `input_text`, `input_datetime`, `counter`, `timer`, `schedule`
- [ ] Register route in `hactl_companion/src/companion/routes/__init__.py`
- [ ] Add validation (schema per domain)
- [ ] Unit tests: `hactl_companion/tests/test_helpers.py`
- [ ] Run: `make test-companion` → green

---

### 4. Go CLI — API Client Extensions

#### 4a. Companion Client Methods
- [ ] Add to `hactl/internal/companion/` (or wherever the companion HTTP client lives):
  - `CreateAutomation(yaml []byte) (id string, err error)`
  - `DeleteAutomation(id string) error`
  - `CreateScript(yaml []byte) (id string, err error)`
  - `DeleteScript(id string) error`
  - `CreateTemplate(yaml []byte, domain string) (id string, err error)`
  - `DeleteTemplate(id string) error`
  - `CreateHelper(yaml []byte, domain string) (id string, err error)`
  - `DeleteHelper(id string) error`

#### 4b. WebSocket Client Methods  
- [ ] Add to `hactl/internal/haapi/websocket.go`:
  - `SceneCreate(config map[string]any) error`
  - `SceneDelete(id string) error`
  - `LabelRegistryDelete(id string) error` (create exists)
  - `AreaRegistryCreate(name string) (id string, err error)`
  - `AreaRegistryDelete(id string) error`
  - `FloorRegistryCreate(name string) (id string, err error)`
  - `FloorRegistryDelete(id string) error`

---

### 5. Go CLI — Commands

#### 5a. Automations
- [ ] `hactl auto create -f <file.yaml> [--confirm]`
  - Dry-run: parse YAML, show summary (alias, trigger count, action count)
  - Confirm: POST to companion, print new ID, auto-backup
- [ ] `hactl auto delete <id> [--confirm]`
  - Dry-run: fetch current config, show summary
  - Confirm: backup first, DELETE via companion, confirm deletion

#### 5b. Scripts
- [ ] `hactl script create -f <file.yaml> [--confirm]`
- [ ] `hactl script delete <id> [--confirm]`
  - Same pattern as automations

#### 5c. Templates
- [ ] `hactl tpl create -f <file.yaml> --domain <sensor|binary_sensor> [--confirm]`
- [ ] `hactl tpl delete <id> [--confirm]`

#### 5d. Helpers (NEW command group)
- [ ] `hactl helper ls [--domain <domain>]`
- [ ] `hactl helper show <entity_id>`
- [ ] `hactl helper create <domain> -f <file.yaml> [--confirm]`
  - Domains: `input_boolean`, `input_number`, `input_select`, `input_text`, `input_datetime`, `counter`, `timer`, `schedule`
- [ ] `hactl helper delete <entity_id> [--confirm]`
- [ ] New file: `hactl/internal/cmd/helper.go`

#### 5e. Scenes (NEW command group)
- [ ] `hactl scene ls`
- [ ] `hactl scene show <id>`
- [ ] `hactl scene create -f <file.yaml> [--confirm]`
- [ ] `hactl scene delete <id> [--confirm]`
- [ ] New file: `hactl/internal/cmd/scene.go`

#### 5f. Labels — Add Delete
- [ ] `hactl label delete <name> [--confirm]`

#### 5g. Areas — Add Create/Delete
- [ ] `hactl area create <name> [--icon <icon>] [--floor <floor>]`
- [ ] `hactl area delete <name> [--confirm]`

#### 5h. Floors — Add Create/Delete
- [ ] `hactl floor create <name> [--icon <icon>]`
- [ ] `hactl floor delete <name> [--confirm]`

---

### 6. Rollback Support for Create
- [ ] Extend `hactl/internal/writer/` to track created entity IDs
  - Store in `backups/created_<timestamp>_<type>_<id>.yaml`
- [ ] `hactl rollback` lists both updated and created entities
- [ ] Rollback of a created entity = delete it + remove backup record

---

### 7. Unit Tests
- [ ] `hactl/internal/cmd/auto_test.go` — test create/delete subcommands
- [ ] `hactl/internal/cmd/script_test.go` — test create/delete
- [ ] `hactl/internal/cmd/helper_test.go` — test full helper command group
- [ ] `hactl/internal/cmd/scene_test.go` — test full scene command group
- [ ] `hactl/internal/cmd/label_test.go` — test delete
- [ ] `hactl/internal/cmd/area_test.go` — test create/delete
- [ ] `hactl/internal/cmd/floor_test.go` — test create/delete
- [ ] `hactl/internal/haapi/websocket_test.go` — test new WS methods
- [ ] `hactl/internal/writer/writer_test.go` — test rollback for create
- [ ] Run: `make test` → green
- [ ] Run: `make lint` → green

---

### 8. Integration Tests
- [ ] Add integration test cases in `hactl/internal/integration/`:
  - Create automation → verify exists → delete → verify gone
  - Create script → verify exists → delete → verify gone
  - Create template → verify exists → delete → verify gone
  - Create helper (each domain) → verify → delete → verify
  - Create scene → verify → delete → verify
  - Label delete, area CRUD, floor CRUD
- [ ] Run: `make test-int` → green (local Docker)

---

### 9. Manual & RTFM
- [ ] Update `hactl/docs/manual.md`:
  - Add `auto create`, `auto delete` to Automations section
  - Add `script create`, `script delete` to Scripts section
  - Add `tpl create`, `tpl delete` to Templates section
  - Add new **Helpers** section with all subcommands
  - Add new **Scenes** section with all subcommands
  - Add `label delete` to Labels section
  - Add `area create`, `area delete` to Areas section
  - Add `floor create`, `floor delete` to Floors section
  - Add rollback-for-create documentation
- [ ] Update `hactl/README.md` command summary table
- [ ] Verify: `hactl rtfm` renders correctly

---

### 10. Companion Docs
- [ ] Update `hactl_companion/README.md` with new helper endpoints
- [ ] Update `hactl_companion/docs/testing.md` if needed

---

### 11. Local Docker Testing
- [ ] `docker compose up` with HA + companion
- [ ] Manual smoke tests:
  - `hactl auto create -f test_auto.yaml` → dry-run preview
  - `hactl auto create -f test_auto.yaml --confirm` → created
  - `hactl auto ls` → new automation visible
  - `hactl auto delete <new_id> --confirm` → deleted
  - Repeat for scripts, templates, helpers, scenes
  - `hactl label delete <name> --confirm`
  - `hactl area create "Test Area" --confirm`
  - `hactl area delete "Test Area" --confirm`
  - `hactl floor create "Test Floor" --confirm`
  - `hactl floor delete "Test Floor" --confirm`
  - `hactl rollback` → shows created entities
- [ ] `make test-int` → green against local Docker

---

### 12. Push to GitHub
- [ ] `git push origin feat/create-delete`
- [ ] Open PR against `main`
- [ ] PR description: summary of changes, new commands, breaking changes (none expected)

---

### 13. CI/CD — Iterate Until Green
- [ ] CI lint → green
- [ ] CI unit tests → green
- [ ] CI integration tests (HA stable/prev/dev matrix) → green
- [ ] CI companion tests → green
- [ ] CI vulnerability scan → green
- [ ] Fix any failures, push, re-run

---

### 14. Merge to Main
- [ ] All CI checks green
- [ ] Self-review or peer review
- [ ] Squash-merge to `main`
- [ ] Verify release pipeline triggers (if tag-based)

---

## Test Data Files Needed
Create in `hactl/testdata/fixtures/`:
- `create_automation.yaml` — minimal automation for testing
- `create_script.yaml` — minimal script
- `create_template.yaml` — minimal template sensor
- `create_helper_input_boolean.yaml` — minimal input_boolean
- `create_helper_input_number.yaml` — minimal input_number
- `create_scene.yaml` — minimal scene

---

## Risk Assessment
| Risk | Mitigation |
|---|---|
| Helper WS API undocumented | Test against real HA in Docker first |
| Breaking existing `auto apply` flow | Only add new subcommands, don't change `apply` |
| Rollback complexity for create | Simple: store created ID, rollback = delete |
| Large PR size | Comprehensive commit messages, logical grouping |

---

## Estimated Complexity
- **Companion (Python):** ~200 LOC new (helpers route + tests)
- **OpenAPI spec:** ~150 lines additions
- **Go CLI commands:** ~600 LOC new (8 command files touched/created)
- **Go API client:** ~200 LOC new (companion + websocket methods)
- **Tests:** ~400 LOC new
- **Manual:** ~100 lines additions
- **Total:** ~1650 LOC
