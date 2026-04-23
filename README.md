# hactl

Go CLI for Home Assistant — built for LLM tool_use. Every command returns compact, token-efficient output that agents can parse in one shot. Humans work fine too.

Single binary. No runtime deps. One HA instance = one directory with a `.env`.

**[Manual (LLM usage guide)](docs/manual.md)**

## Install

```bash
go install github.com/swifty99/hactl/cmd/hactl@latest
# or
git clone https://github.com/swifty99/hactl && cd hactl && make build
```

## Setup

```bash
mkdir -p ~/ha/home && cd ~/ha/home
cat > .env << 'EOF'
HA_URL=http://homeassistant.local:8123
HA_TOKEN=<long_lived_access_token>
EOF

hactl health
# → HA 2026.4.3  state=RUNNING  recorder=ok  errors=4
#   location=Home  tz=Europe/Berlin
```

Instance discovery: `--dir` flag → `HACTL_DIR` env → CWD (if `.env` exists) → `~/.hactl/default/`.

## Commands

```
hactl health                              # version, recorder, error count
hactl health --json                       # structured JSON output
hactl issues                              # active repairs
hactl changes --since 24h                 # recent state changes (logbook)

hactl auto ls [--failing] [--pattern] [--json]  # automations table
hactl auto show <id>                      # summary + last 5 traces
hactl auto diff <id> -f new.yaml          # diff local vs remote
hactl auto apply <id> -f new.yaml         # dry-run (default)
hactl auto apply <id> -f new.yaml --confirm  # write + reload
hactl rollback [automation-id]            # restore last backup

hactl trace show <id>                     # condensed steps (trigger/cond/action)
hactl trace show <id> --full              # raw JSON

hactl ent ls --pattern sensor.wp_*        # glob-filtered entities
hactl ent show <id>                       # entity profile
hactl ent show <id> --full                # profile + all attributes
hactl ent hist <id> --since 7d            # ~50 resampled points
hactl ent hist <id> --resample 5m         # custom bucket size
hactl ent anomalies <id>                  # gaps, stuck values, spikes

hactl log --errors --unique               # deduplicated errors with count
hactl log --component zha                 # filter by integration
hactl log --json --full                   # full JSON log output
hactl cc ls                               # custom components + versions
hactl tpl eval '{{ states("sensor.x") }}' # Jinja eval via HA
hactl tpl eval -f template.j2

hactl svc call <domain>.<service>         # call HA service
hactl svc call group.set -d '{...}'       # with inline JSON data
hactl svc call group.set -d @data.json    # read JSON from file

hactl script ls [--pattern] [--json]      # scripts table
hactl script show <id>                    # summary + last 5 traces

hactl cache status                        # age + size per category
hactl cache refresh [traces|logs]
hactl cache clear
hactl version
```

### Global flags

`--dir <path>` · `--since 24h` · `--top 10` · `--full` · `--json` · `--color`

### Filtering

`--pattern` on `auto ls`, `ent ls`, and `script ls` accepts either a **substring** or a **glob**:

```bash
hactl auto ls --pattern ess          # substring: matches anything containing "ess"
hactl auto ls --pattern "ess_*"      # glob: matches IDs starting with "ess_"
hactl ent ls --pattern sensor.wp_*   # glob: sensor entities starting with wp_
```

If the pattern contains `*` or `?` it is treated as a glob. Otherwise it does a case-sensitive substring match.

### Write safety

All writes go through the HA Config API (no filesystem access). `auto apply` without `--confirm` is a dry-run. Backups are created automatically in `backups/`. `rollback` restores the last one. Config is validated via `check_config` before reload.

## How it works

hactl talks to HA via REST + WebSocket. Logs are fetched via WS `system_log/list` (preferred) with automatic fallback to REST `/api/error_log`. It caches traces (SQLite) and logs (JSONL) locally. Every item gets a **stable short ID** (`trc:a7`, `anom:g3`) persisted in `cache/ids.json` for follow-up calls. Output defaults to compact tables — one header, one line per item, `…+N more` for overflow. No emojis, no color unless `--color`.

```
id                state  runs_24h  errors  last_err
climate_schedule  on     12        3       09:42 cond_false
alarm_morning     off    0         1       03:11 tmpl_err
vent_boost        on     47        1       11:05 srv_timeout
```

```
trc:a7  automation.climate_schedule  2026-04-16 09:42  FAIL
 1 trigger  time         09:42:00
 2 cond     state==home  true
 3 cond     tmpl         FAIL  → 'unknown' not float
 X action   service_call skipped
```

### Project structure

```
cmd/hactl/           main
internal/
  cmd/               cobra subcommands
  config/            .env loading, instance discovery
  haapi/             REST + WebSocket client (auth, retry)
  cache/             SQLite traces, JSONL logs, TTL refresh
  analyze/           trace condenser, log deduper, anomaly detection
  format/            table renderer (compact/full/JSON)
  writer/            YAML diff, backup, validate, apply, rollback
pkg/ids/             stable ID generator (hash-based, short)
testdata/            fixtures + golden files + sample traces
```

## Testing

121 unit tests + 120 integration tests. All green against real HA Docker containers.

```bash
make test           # unit tests (no Docker)
make test-int       # integration tests against HA container (~2 min)
make lint           # golangci-lint, must be 0 findings
```

### Integration tests

Real HA in Docker via [testcontainers-go](https://golang.testcontainers.org/). No mocks. Three fixtures:

- **basic** — 3 automations, 3 scripts (Kino, standby), standard config. Shared across ~90 tests.
- **faulty** — broken template, disabled automation. Separate container, lazy-init.
- **realistic** — 11 automations, template sensors, input helpers, `system_log` integration. Modelled after a real-world 381-automation installation. Separate container with seeded entity history for `ent hist`, `ent anomalies`, and WS `system_log/list` testing.

Tests run hactl in-process via `cmd.RunWithOutput()` (no subprocess). Golden files in `testdata/golden/` catch output regressions — regenerate with `HACTL_UPDATE_GOLDEN=1 make test-int`.

Contract tests validate HA API schemas (`/api/config`, `/api/states`, WS `trace/list`, WS `system_log/list`, etc.) to catch breaking changes across HA versions. CI matrix runs against HA stable + stable-1.

See [docs/end2end-testing.md](docs/end2end-testing.md) for architecture details.

