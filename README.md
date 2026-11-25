# gokit

Shared platform code for the Go backends (bookmarks, wheelgpt, narfy-market, schmubble).

| Package | Purpose |
|---|---|
| `apperr` | Transport independent business errors (`Kind`, `Error`) |
| `httpx` | JSON responses, `Decode` + validation, query/path params, RFC 9457 problems |
| `sqlite` | `Open` with production pragmas, goose `Migrate`, `InTx`, constraint error helpers |
| `sqlite/sqlitetest` | Fresh migrated database per test |
| `metrics` | Prometheus registry with Go/process/build/DB collectors and HTTP middleware |
| `server` | Health/ready probes, request logger, security headers, proxy IP, CSRF, serve/shutdown |
| `jobs` | `Every` for periodic background work |
| `logging` | `Setup` for the default slog logger |

## Rules

- Only code that at least two services need **unchanged** goes in here. If a
  service needs a special case, it stays in the service.
- No feature code, no sqlc output, no concrete error values, no config structs.
- Versions are `v0.x` tags. Breaking changes are fine but get a minor bump.

## Local development

Work on gokit and a service together through a Go workspace in the parent
directory (not committed):

```sh
cd ~/Repositories
go work init ./gokit ./bookmarks
```

Release a change with `git tag v0.x.y && git push --tags`, then run
`go get github.com/SoWieMarkus/gokit@v0.x.y` in each service.
