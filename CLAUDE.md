# CLAUDE.md — developer orientation

Audience: senior engineers (and LLM agents) joining this repo. Read this once, then read the code.

## What this is

`suuntool` is a Go CLI for the **unofficial** Suunto / Sports-Tracker HTTP API. It is meant to be used by humans and by agents driving a terminal — every UX decision flows from that dual audience.

Internal reference material (endpoint inventory, signing scheme, Python reference client, golden-vector diagnostics) lives in `handoff/`. That directory is **excluded via `.git/info/exclude`** — not `.gitignore` — so it stays local but is never committed. Treat it as load-bearing reference material; don't delete it.

## Layering — strict, one-way

```
main.go
└── cmd/                 Cobra commands. Knows about session+api+output, nothing else.
    ├── root.go          persistent flags, exit codes, helpers (baseURL, authedClient, emit, pickTimeout, parseSince, useTTYColor)
    ├── login.go logout.go whoami.go profile.go doctor.go endpoints.go version.go
    ├── workouts.go      workouts list/get/count/stats/sml/fit/export/comments/react/edit/upload/delete (+ --summary, --stream, --since)
    ├── guides.go        guides list/download/upload/update/pin/unpin/priority/delete (+ --yes). Transport only — no guide.json parsing.
    ├── wellness.go      wellness sleep/activity/recovery/sleepstages NDJSON streams
    ├── wellness_sleep_pretty.go   TTY-only sleep table renderer (writeSleepFooter)
    ├── mcp.go           `suuntool mcp` stdio MCP server entry (--allow-write / --allow-destructive)
    └── login_test.go, root_test.go, wellness_sleep_pretty_test.go    end-to-end via httptest
internal/
├── auth/                signing pipeline. Zero net/http imports.
│   ├── keys.go          embedded signing constants (login parts, TOTP parts, package name, user-agent)
│   ├── obfuscator.go    KeyObfuscator XOR + lossy-UTF-8 replace; DeriveLoginSecret / DeriveTOTPMasterSecret
│   ├── totp.go          PBKDF2-HmacSHA1 + RFC 6238 HOTP → GenerateTOTP(salt, offsetMS)
│   └── signer.go        SignParams(path, []Param) → base64url(SHA-256), RandomSalt, NowMS
├── api/                 HTTP transport.
│   ├── client.go        Client + Do()/DoStream() — injects STTAuthorization + User-Agent, maps HTTP status → typed errors, transparent gzip
│   ├── envelope.go      generic AskoResponse[T] + DecodeAsko[T]
│   ├── errors.go        *Error{Code, Message, Hint, HTTP, Exit} — ExitCode() drives os.Exit
│   └── endpoints/       per-resource wrappers — add a file here for each new resource
│       ├── session.go   Login (POST /login2), Logout, RemoteUserSession + Pretty()
│       ├── user.go      Whoami, Settings, Follow, UserByName + Prettier impls
│       ├── workouts.go  List/Get/Count/Stats/SML/FIT/Delete + WorkoutList.Summary / SummaryWithWoW
│       ├── comments.go reactions.go edit.go share.go extensions.go upload.go   workout writes (x-totp via cmd layer)
│       ├── guides.go    List/Download/Create/Update/Delete/SetGuidePinned/GuidePriority — raw zip body, no x-totp, no guide.json parsing (see the file's own package doc)
│       ├── wellness.go  NDJSON stream decoders (sleep/activity/recovery/sleepstages)
│       ├── format.go    formatKm / formatDuration / renderTable(Styled) shared by Pretty()
├── session/             session.go — XDG-aware persistence, 0600 perms, ErrNoSession (+ TOTPHeaders helper)
├── output/              the single render boundary
│   ├── output.go        Render / RenderToFile, Opts, Prettier interface, resolveFormat
│   └── tty.go           IsStdoutTTY (respects NO_COLOR)
└── mcp/                 Model Context Protocol adapter (stdio server)
    ├── server.go        Run(ctx, Opts), registerAll, deps (api.Client + session)
    ├── registry.go      tier + toolRegistrar shape; per-tool registrars elsewhere
    ├── tools_read.go    read-tier registrars (whoami, profile_*, workouts_*, wellness_*, doctor, activity_type_name, guides_list, guides_download)
    ├── tools_write.go   --allow-write registrars (comment, react, edit, batch_update, share, extensions, upload, guides_upload, guides_update, guides_pin, guides_unpin)
    ├── tools_destructive.go  --allow-destructive registrars (delete, uncomment, unreact, guides_delete)
    ├── activity.go      activityId → activityName enrichment shared by workouts_list/get/stats
    └── errors.go        mapError / mapErrorToCallToolResult (preserves *api.Error structure for the LLM)
```

Dependency direction is strict: **cmd → api(/endpoints) → auth**, and **cmd → output**, never the reverse. `internal/output` does not import any Suunto type. `internal/auth` does not import `net/http`. **`internal/mcp` is a peer of `cmd/` — same layering rules: it imports `api`, `auth`, `session`, never the reverse.**

## Patterns to follow

- **One verb per file in `cmd/`.** A new command = new file, registered via that file's `init()`.
- **Endpoints stay thin.** `internal/api/endpoints/*` files return typed structs that implement `output.Prettier`. Don't put business logic there. For list-shaped responses (workouts, comments, per-activity stats), expose `Table() (headers, rows)` (the `output.Tabular` interface) so `--format tsv` can render the same data machine-readable; `Pretty()` then reuses `Table()` and runs the result through `renderTable(headers, rows)` in `endpoints/table.go` for the aligned text version. Single-record responses stay as key/value lines. Streaming endpoints (e.g. wellness sleep NDJSON) keep their TTY-only table renderers in `cmd/`.
- **Always go through `emit(v)` from `cmd/root.go:emit`.** It handles `--output` vs stdout and `--format auto|json|pretty`. Commands never call `json.Marshal` or `fmt.Println` for primary output. Status/log lines go to stderr.
- **Errors are typed.** Anything you return from `cmd/*` `RunE` that should set a non-zero exit code must be (or wrap) a `*api.Error`. `cmd.Execute` reads `ExitCode()` off it. Codes 3/4/5/6/7 are stable and documented in `--help`.
- **TDD with golden vectors for crypto.** Anything in `internal/auth` is locked to byte-identical output from the Python reference (`handoff/reference/secret_check.py`, `sign_check.py`, plus the inline TOTP capture script in `docs/superpowers/plans/2026-05-11-suuntool-v1.md` Task 3). Don't tweak the signer/TOTP without re-running those diagnostics first.
- **HTTP tests use `httptest.Server`.** No live API calls in CI. See `internal/api/client_test.go` and `cmd/login_test.go` for the pattern (env-vars `SUUNTOOL_BASE_URL` and `SUUNTOOL_SESSION_FILE` are the only knobs you need).
- **Cobra `Example:` block on every user-facing command.** Agents read help text — keep examples concrete and copy-pasteable.

## Don'ts

- **Never commit private data.** No real emails, usernames, sessionkeys, userKeys, passwords, workout IDs, GPS coordinates, or device serial numbers — not in tests, fixtures, README snippets, commit messages, or sample outputs. Use placeholders: `you@example.com`, `alice`, `SK123`, `k1`, `hunter2`. If you capture a real response for fixture work, scrub it before saving.
- **Don't commit `handoff/` or `docs/`.** They're local-only by design. Use targeted `git add <files>`; never `git add .` or `git add -A`.
- **Don't introduce another output path.** No `fmt.Println(json…)` in commands. If `emit` doesn't do what you need, extend `internal/output`, don't bypass it.
- **Don't touch git config.** Email + name are set per-repo already.
- **Don't add a feature-flag layer or "future-proof" abstraction "just in case".** Add the file when you add the endpoint.
- **Don't widen `internal/auth`'s public surface** without good reason. Helpers like `keyObfuscator`, `utf8Replace`, `pbkdf2KeyForSalt`, `hotp6` are unexported on purpose — promoting them invites parallel implementations.
- **Don't paper over a 401.** Surface it as `*api.Error{Code:"AUTH_EXPIRED", Exit:4}` with `Hint:"Run: suuntool login"`. No silent re-auth.
- **Don't run write-side smoke tests in CI.** Anything that touches `POST`/`PUT`/`DELETE` on Suunto's real backend must be exercised manually against a personal account (round-trip: do → verify → undo) and never from automation. CI runs `httptest`-backed unit tests only.
- **Don't compute x-totp inside endpoint wrappers.** The wrappers in `internal/api/endpoints/` stay session-agnostic; the cmd layer calls `totpHeaders(sess)` and passes the header in. Same rule for any header derived from session state.
- **Don't bypass `confirm()` on destructive commands.** `workouts delete` requires `--yes` on non-TTY. New destructive commands (future bulk-deletes, etc.) must follow the same pattern. The helper returns `*api.Error{Code:"USAGE", Exit:2}` when stdin is not a TTY and `--yes` is unset — propagate, don't catch.

## Key code to read first (in this order)

1. `internal/auth/obfuscator.go` — `DeriveLoginSecret`, `utf8Replace`. The whole signing scheme hinges on matching Java's lossy `new String(bytes, UTF-8)` semantics; the test in `obfuscator_test.go` proves it.
2. `internal/auth/signer.go` — `SignParams`. Build string verbatim, no URL-encoding, SHA-256, base64url no-padding. Mirrors the upstream signing routine.
3. `internal/auth/totp.go` — `GenerateTOTP`. Java PBEKeySpec quirk: only the low byte of each password char is used.
4. `internal/api/client.go` — `Do()` is the only place HTTP status codes are mapped to exit codes; if you're surprised by a CLI exit code, start here.
5. `internal/api/endpoints/session.go` — `Login`. Note that `/login2` returns the session at the top level, NOT in an AskoResponse envelope — every other endpoint does.
6. `cmd/root.go` (bottom half) — `baseURL`, `authedClient`, `renderOpts`, `emit`, `pickTimeout`. Every other command leans on these four.
7. `cmd/login_test.go` — the end-to-end pattern (env-var overrides + httptest + piped stdin password).

## Adding a new endpoint — the recipe

1. New file under `internal/api/endpoints/<resource>.go` with the typed struct + a `Pretty()` method + a function `Foo(ctx, *api.Client, args…) (*T, error)` that calls `c.Do` and `api.DecodeAsko[T]`.
2. New file under `cmd/<verb>.go` that calls `authedClient()` → endpoint → `emit(v)`.
3. Add a row to `endpointTable` in `cmd/endpoints.go` so agents discover it.
4. Test the endpoint wrapper with `httptest.Server` (mirror `endpoints/session_test.go`).
5. If the endpoint takes a `x-totp` header (reactions, comments, settings-safe, email/phone change), generate it with `auth.GenerateTOTP(session.Email, session.OffsetMS)` and pass via the `headers` map of `client.Do`. From the cmd or MCP layer, use `session.TOTPHeaders(sess)`.
6. **Expose it via MCP.** Pick the right tier file (`tools_read.go`, `tools_write.go`, `tools_destructive.go`) and append a `toolRegistrar` closure that calls `sdkmcp.AddTool` with a typed args struct (`json` + `jsonschema` tags), the same endpoint wrapper from step 1, and `authGate(d)` if the endpoint is authed. Return the structured payload directly; `mapErrorToCallToolResult` wraps `*api.Error`s. Add a happy-path test in the matching `tools_*_test.go` using the in-memory MCP transport. `login`/`logout` stay CLI-only by design — don't port them.

## Failure modes — what each exit code looks like

All exit codes are set centrally by `internal/api/client.go:Do()` (see lines ~85–103). When triaging a CLI exit, start there.

| Exit | Code | Trigger | Hint shown? | Typical cause |
|------|------|---------|-------------|---------------|
| 2 | `USAGE` | bad flag combo, missing required arg, `confirm()` on non-TTY without `--yes` | no | fix the invocation |
| 3 | `NETWORK` | DNS/TCP/TLS failure, request timeout (`context.DeadlineExceeded`) | no | check connectivity; bump `--timeout`. **Distinct from server errors** — never retry blindly. |
| 4 | `AUTH_EXPIRED` | HTTP 401 | yes: `Run: suuntool login` | session aged out (~30d) or signing-key drift after Suunto app bump. If `suuntool login` itself returns 4, suspect key rotation — see below. |
| 5 | `SERVER` | HTTP 5xx **or** any other non-2xx/3xx without a dedicated code | no | Suunto-side. 429 also lands here (no Retry-After parsing yet). Treat as transient. |
| 6 | `NOT_FOUND` | HTTP 404 | no | wrong workout key, deleted resource, wrong username on `workouts/{username}/stats` |
| 7 | `FORBIDDEN` | HTTP 403 | no | privacy/sharingFlags mismatch, deleted account, or an x-totp the server refused (clock skew > 30s — re-check `OffsetMS`) |

Endpoints to babysit:

- **`workouts list --stream` / `export --bundle`** — pages forever and pulls SML (~5 MB each). The server rate-limits aggressively here; expect 429 → exit 5 on long backfills. Throttle with `--limit` or split by `--since`.
- **`workouts/{key}/sml`** and **`workout/exportFit/{key}`** — large bodies, gzip'd, streamed via `DoStream`. Timeouts here are common on slow links; use `--timeout` or `-o file` and re-run.
- **Anything with `x-totp`** (comments post, react/unreact, settings-safe, email/phone change, workout edit/delete) — server validates the TOTP against its clock. If the laptop clock drifts >30s, every write returns 403 until you re-`login` (refreshes `OffsetMS`).
- **`/login2`** — does *not* return an AskoResponse envelope. Decoding failures here mean Suunto changed the login response shape, not a normal error.

What a signing-key rotation feels like, in practice: `login` succeeds against your password but every subsequent call returns 401 (exit 4) with a generic body, *even immediately after a fresh login*. Or `login` itself starts returning 401 / 403 with no useful body. That's the canary — confirm by re-running `handoff/reference/secret_check.py` against the current app release; if the goldens have moved, follow the `CONTRIBUTING.md` rotation procedure.

## `--summary` aggregation math (workouts list)

`workouts list --summary` replaces the per-row table with `WorkoutSummary` totals over **whatever pages got fetched in the current invocation**. The math lives in `internal/api/endpoints/workouts.go:Summary()` and `SummaryWithWoW()` — read those before changing behavior. Quick reference so agents don't have to:

- **Scope.** The aggregate covers exactly the items in the `WorkoutList` returned to the cmd layer. With `--since` + pagination, that's every workout whose `startTime ≥ since`. Without `--since`, it's one server page (`--limit`, default 20, max 100). The summary is *not* a server-side rollup — it's a client-side fold over the items the CLI already pulled. If you didn't paginate, the totals only describe one page.
- **"Active time" = `TotalTime`** (the workout's `totalTime` field, seconds) summed verbatim. There is **no separate moving-vs-paused split** in the wire model — Suunto reports a single `totalTime` per workout and that's what we add. Pretty() renders it via `formatDuration` (h/m/s).
- **Distance** is meters (`totalDistance`) summed and rendered as km. Ascent/descent are meters, summed as-is.
- **Multi-activity sessions roll up by `activityId`**, not by session. Each workout has exactly one `activityId` on the wire (the parent/primary activity); sub-activity legs are not separate rows on this endpoint, so there is no double-counting at the list level. `ByActivity[id]` accumulates `Count`, `Distance`, `Duration` for that activity ID; the top-level scalars (`TotalDistance`, `TotalTime`, `TotalAscent`, `TotalDescent`) are the sum across all activities. Sum of `ByActivity[*].Distance == TotalDistance`. Same for duration.
- **`SummaryWithWoW(nowMS)`** adds a per-activity `WeekOverWeek.Count` delta = (count in `[nowMS-7d, nowMS)`) − (count in `[nowMS-14d, nowMS-7d)`). Buckets use the workout's `startTime`. Items older than 14 days still count toward the totals but contribute 0 to the delta. `nowMS == 0` → `time.Now().UnixMilli()`. The cmd layer passes 0 in production; tests pin it for determinism.
- **Rendering.** `Pretty()` prints the scalar block (workouts/distance/time/ascent/descent) then a per-activity table via `Table()`. When `WeekOverWeek` is populated, `Table()` appends a `ΔWoW` column (signed; `formatDelta` uses `+N` for positive, `0` for zero). On a TTY, `cmd/workouts.go:ansiDeltaColor` colors that column green/red — JSON output flattens the embedded struct and drops the color flag.

## Signing-key rotation

Suunto rotates the embedded keys on major app version bumps. See `CONTRIBUTING.md` for the procedure — short version: refresh the key material per the new app release, update the five constants in `internal/auth/keys.go`, regenerate goldens from the Python diagnostics, and update the `expected*` constants in `internal/auth/*_test.go`. The test suite is the canary.
