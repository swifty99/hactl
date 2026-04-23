# hactl – End-to-End Testing

> Integration tests run hactl commands against a real Home Assistant instance
> in a Docker container. No mocks, no fakes – real HA, real API.

## Quick Start

```bash
# Prerequisites: Docker running
make test-int        # runs all integration tests (~2 min first run, ~60s cached)
make test            # unit tests only (fast, no Docker)
```

## Architecture

```
internal/
  hatest/             ← Test helper: container lifecycle + headless onboarding
    hatest.go
  integration/        ← Integration test suite (one package, shared container)
    main_test.go      ← TestMain: starts HA, shares instance across all tests
    helpers_test.go   ← runHactl() + variants, assertContains, sanitizeGolden
    golden_test.go    ← Golden-file comparison framework (assertGolden)
    health_test.go    ← health command E2E
    auto_test.go      ← auto ls/show E2E + JSON schema validation
    ent_test.go       ← ent ls/show/hist/anomalies + WebSocket E2E
    tpl_test.go       ← template eval E2E
    log_test.go       ← log command E2E + component filter
    trace_test.go     ← trace show condensed/full + automation trigger
    cache_test.go     ← cache status/refresh/clear E2E
    changes_test.go   ← changes command E2E
    issues_test.go    ← issues command E2E
    cc_test.go        ← custom component commands E2E
    svc_test.go       ← service call E2E + @file support
    script_test.go    ← script ls/show E2E + pattern matching
    version_test.go   ← version output format E2E
    write_test.go     ← auto diff/apply/rollback write-path E2E
    faulty_test.go    ← Error-path tests with faulty fixture
    realistic_test.go ← Realistic fixture: WS logs, health --json, seeded history
    error_test.go     ← Invalid input + edge case error paths
    golden_capture_test.go ← Golden-file snapshot tests
    contract_test.go  ← HA API schema contract tests
testdata/
  fixtures/
    basic/            ← Minimal HA config: automations, entities
      configuration.yaml
      automations.yaml
    faulty/           ← Intentional errors for error-path testing
      configuration.yaml
      automations.yaml
    realistic/        ← Real-world-like config with template sensors + input helpers
      configuration.yaml
      automations.yaml
  golden/             ← Golden-file snapshots (auto-generated)
  traces/             ← Sample trace JSON for unit tests
```

## How It Works

### Container Lifecycle

One HA container per test package, started in `TestMain`, shared across all tests:

```go
var ha *hatest.Instance

func TestMain(m *testing.M) {
    var code int
    ha, code = hatest.StartMain(m, hatest.WithFixture("basic"))
    if code != 0 { os.Exit(code) }
    exitCode := m.Run()
    ha.Stop()
    os.Exit(exitCode)
}
```

**Why one container?** HA takes 30-60s to boot. Sharing avoids paying that cost per test.
Tests are independent (read-only against HA) so sharing is safe.

### Headless Onboarding

A fresh HA container requires onboarding (creating an owner account) before the API
is usable. There is no env var to skip this. `hatest` automates the full flow:

1. **Start container** – `ghcr.io/home-assistant/home-assistant:stable`, fixtures mounted to `/config/`
2. **Wait for readiness** – poll `GET /api/onboarding` (unauthenticated endpoint)
3. **Create owner** – `POST /api/onboarding/users` → returns `auth_code`
4. **Get tokens** – `POST /auth/token` with `grant_type=authorization_code`
5. **Skip setup steps** – `POST /api/onboarding/core_config` + `/api/onboarding/analytics`
6. **Long-lived token** – WebSocket `auth/long_lived_access_token` (only available via WS)
7. **Ready** – returns `Instance` with `URL()`, `Token()`, `Dir()`

### Running Commands

Tests execute hactl commands through the cobra command tree (no subprocess):

```go
func TestHealthCommand(t *testing.T) {
    out := runHactl(t, "health")
    if !strings.Contains(out, "HA ") {
        t.Errorf("unexpected: %s", out)
    }
}
```

`runHactl()` sets `HACTL_DIR` to the instance's temp directory (containing `.env` with
URL + token), then calls `cmd.RunWithOutput()` which drives cobra directly.

**Helper variants:**

| Helper | Purpose |
|--------|---------|
| `runHactl(t, args...)` | Run command, fatal on error |
| `runHactlDir(t, dir, args...)` | Run against a specific instance directory |
| `runHactlErr(t, args...)` | Run command, return `(output, error)` — for error-path tests |
| `runHactlJSON[T](t, args...)` | Run with `--json`, unmarshal into T |
| `runHactlLines(t, args...)` | Run command, return trimmed non-empty lines |
| `assertContains(t, s, sub)` | Assert string contains substring |
| `assertNotContains(t, s, sub)` | Assert string does NOT contain substring |

### Fixtures

Fixtures in `testdata/fixtures/<name>/` are bind-mounted as `/config/` in the container.

**`basic/configuration.yaml`** – minimal setup with `default_config:` (enables REST API,
recorder, automation engine, etc.) and `automation: !include automations.yaml`.

**`basic/automations.yaml`** – three test automations: `climate_schedule` (time-based +
template condition), `alarm_morning` (time trigger), `vent_boost` (numeric state trigger).

**`faulty/configuration.yaml`** – same base config, named "Faulty Home".

**`faulty/automations.yaml`** – intentional error cases: `broken_template` (Jinja error
referencing undefined sensor), `always_off` (disabled automation), `working_simple`
(baseline comparison).

To add a new fixture set, create `testdata/fixtures/<name>/` with at least a
`configuration.yaml`. Pass `hatest.WithFixture("<name>")` to `StartMain`.

**`realistic/configuration.yaml`** – template sensors (outdoor temp, humidity, power,
energy), input helpers (booleans, numbers, select), explicit `system_log` integration,
Europe/Berlin timezone. Modelled after a real 381-automation installation.

**`realistic/automations.yaml`** – 11 diverse automations (door light, climate schedule,
humidity-based ventilation, morning/night routines, power spike alert, guest/vacation
modes, disabled legacy automation).

Realistic tests seed entity history via `CallService` (setting input_number values
with timestamps) to exercise `ent hist`, `ent anomalies`, and WS `system_log/list`.

### Golden Files

Golden-file tests capture hactl output and compare against committed snapshots in
`testdata/golden/`. This catches unintended output format changes.

**How it works:**
1. `assertGolden(t, "test_name", got)` compares sanitized output against `testdata/golden/test_name.txt`
2. Dynamic values (timestamps, HA versions, ports) are replaced with placeholders
3. Mismatches fail the test with a diff

**Regenerating golden files:**
```bash
HACTL_UPDATE_GOLDEN=1 make test-int
```

Golden files are committed to the repo — changes appear in PR diffs for review.

### Faulty Fixture Tests

Error-path tests use a **separate HA container** with the `faulty` fixture:

```go
func TestFaultyAutoLs(t *testing.T) {
    inst := getFaultyHA(t)
    out := runHactlDir(t, inst.Dir(), "auto", "ls")
    assertContains(t, out, "id")
}
```

The faulty container is lazily initialized (via `sync.Once`) and reused across all
faulty tests to avoid the ~30-60s boot cost per test.

### Write-Path Tests

Write-path tests exercise the full apply/rollback cycle:

1. Create a temp YAML with modified automation config
2. `auto diff` — verify diff output
3. `auto apply` (dry-run) — verify no actual change
4. `auto apply --confirm` — verify write + backup creation
5. `rollback` — verify original config restored

These tests mutate HA state, so they run in a defined order within `write_test.go`.

## Build Tags

Integration tests use `//go:build integration`. This keeps them out of `make test`:

| Command | What runs | Docker needed |
|---------|-----------|---------------|
| `make test` | Unit tests only | No |
| `make test-int` | Unit + integration tests | Yes |

## API Reference (`hatest` package)

### `StartMain(m *testing.M, opts ...Option) (*Instance, int)`
For TestMain. Returns instance + exit code (non-zero = setup failed).

### `Start(t *testing.T, opts ...Option) *Instance`
For single tests. Registers t.Cleanup automatically.

### Options
- `WithFixture(name string)` – mount `testdata/fixtures/<name>/`
- `WithImage(image string)` – override Docker image (default: `stable`)
- `WithTimeout(d time.Duration)` – override startup timeout (default: 3 min)

### Instance
- `URL() string` – `http://localhost:<port>`
- `Token() string` – long-lived access token
- `Dir() string` – temp dir with `.env` (for `--dir` or `HACTL_DIR`)
- `Stop()` – terminate container

## Adding Tests

1. Add a new `*_test.go` in `internal/integration/`
2. Use the `//go:build integration` build tag
3. Use `runHactl(t, "command", "subcommand", "--flag")` to run commands
4. Use `runHactlErr(t, ...)` for tests that expect errors
5. Use `runHactlJSON[T](t, ...)` for structured JSON validation
6. Assert with `assertContains` / `assertNotContains` for readable failures
7. For golden-file tests, use `assertGolden(t, "name", output)`
8. For tests needing the faulty fixture, use `getFaultyHA(t)` + `runHactlDir`
9. For direct API access, use `loadConfig(t)` to get URL + token

## Troubleshooting

**Container doesn't start:** Check Docker is running. First run pulls ~1GB image.

**Tests timeout:** Increase timeout: `go test -tags=integration -timeout 600s ./internal/integration/`

**Fixture changes not picked up:** HA loads config at boot. The container must restart
to see fixture changes (re-run the test).

**Port conflicts:** testcontainers uses random ports. No conflicts possible.

**Orphaned containers:** testcontainers' Ryuk sidecar auto-removes containers after
test process exits (even on crash/kill).

## Future: CI Matrix

When ready, the CI workflow will test against multiple HA versions:

```yaml
strategy:
  matrix:
    ha-version: [stable, 2025.3, beta]
```

Pass `hatest.WithImage("ghcr.io/home-assistant/home-assistant:" + version)`.
`beta` failures are warnings only; `stable` failures block merge.
