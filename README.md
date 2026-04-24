## hactl

# Home Assistant control, built for agentic workflows

Stop burning tokens on raw APIs. **hactl** is purpose-built for LLM-driven control of Home Assistant: minimal payloads, precise queries, no structural noise.

## What it does
- Diagnose automations: failures, trigger frequency, dead rules  
- Inspect entities: anomalies, outages, signal quality  
- Read/Write analyse attributes, scripts, labels, etc.
- Spider along your feature cluster
- Generate and update dashboards (beta)  
- More: see [manual](docs/manual.md)

## Why it’s different
- **Token-efficient by design**: trims responses to essentials (top/head logs, no bloated schemas)  
- **On-target validation**: executes against the real Jinja engine — no mock layers  
- **Deterministic safety**: rollback by default, explicit commit required  
- **Low traffic footprint**: aggressive request caching, filesystem-backed  
- **Multi-instance ready**: one directory per HA instance, `.env` scoped

## Engineering
- Go, single static binary, zero runtime deps  
- Integration-tested against real Home Assistant via ephemeral Docker instances  
- Security-checked

## Setup
Drop a `.env` with your token into a directory. Run. Done.

---



**[Manual (LLM usage guide)](docs/manual.md)**

## Install

```bash
go install github.com/swifty99/hactl/cmd/hactl@latest
# or
git clone https://github.com/swifty99/hactl && cd hactl && make build
```
Or just download the release.

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
