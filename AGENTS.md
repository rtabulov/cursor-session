## Agent skills

### Issue tracker

Issues live in GitHub Issues for `rtabulov/cursor-session`; use the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Canonical roles map 1:1 to label strings (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout: root `CONTEXT.md` + `docs/adr/`. See `docs/agents/domain.md`.

## Cursor Cloud specific instructions

`cursor-session` is a single Go CLI (module `github.com/rtabulov/cursor-session`, Go 1.23) that extracts and exports Cursor IDE / cursor-agent chat sessions. There is no server or GUI; verify it via the terminal.

- Standard commands live in the `Makefile` (`make build`, `make test`, `make test-coverage`) and `README.md`. CI (`.github/workflows/ci.yml`) runs `go vet ./...`, `golangci-lint run`, and `go test ./... -v`.
- `golangci-lint` installs to `$(go env GOPATH)/bin` (i.e. `~/go/bin`), which is not on `PATH` by default. Run it as `"$(go env GOPATH)/bin/golangci-lint" run --timeout=5m ./...` or prepend that dir to `PATH` for the session.
- Running the CLI end-to-end does not require a real Cursor install: point `--storage` at a `state.vscdb` SQLite file whose `cursorDiskKV(key,value)` table holds `composerData:<id>` and `bubbleId:<chatId>:<bubbleId>` JSON rows (see `testutil/fixtures.go` / `testutil/mockdb.go` for the schema). Bubble `chatId` must equal the composer id for `show`/`export` to reconstruct a conversation. Then run `list`, `show <id>`, `export --format md --out <dir>`.
- `list`/`export` cache to `~/.cursor-session-cache`; pass `--clear-cache` when changing the seed DB so you don't read stale results.
