next

Suggested Next Steps
Immediate (cleanup):

Delete null — 22MB binary accidentally committed (likely > dev/null on Windows). Remove from git history with git rm dev/null and add to .gitignore.
Commit the current work — companion v2 client + integration tests + doc overhaul is a clean unit.
Phase 14 continuation (building on the client):
3. Companion auto-discovery (Phase 14.2) — Wire COMPANION_URL from .env + WS hassio/addon/info fallback. This unblocks all CLI commands.
4. hactl tpl ls/show/edit/create/rm (Phase 14.4) — The client methods are ready; needs cobra commands + formatters. Highest user value.
5. hactl script def/edit/create/rm — Same pattern, reuse template CLI structure.
6. hactl auto def/edit/create/rm — Completes the YAML management triple.
7. hactl config check/reload — Already have the HA API methods, just needs CLI wiring.

Quality/infra:
8. Run golangci-lint — Hasn't been run this session (wasn't requested). Worth a check before committing.
9. Tag a release — Phase 13 completion is a good milestone for v0.14.0 or similar.
10. CI/CD companion tests — Add make test-companion to the GitHub Actions workflow (currently manual-only).

Claude Opus 4.6 • 3x