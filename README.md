# idpad

A personal idea notebook with Google Docs–style commenting. Write an idea in a
rich-text editor, highlight any part of it and leave a threaded comment, and
attach images and links that stay with the idea.

- **Ideas** — create, list, edit and delete notes with headings, lists, bold and italic.
- **Comments** — anchor a thread to an exact text selection, reply, edit, resolve. Highlights are rendered over the anchored text and cycle through a colour palette.
- **Resources** — attach image and link URLs, shown as thumbnails and cards.

**Stack:** React 19 + TypeScript + Vite + Tailwind 4 + TipTap 3 · Go 1.25 (chi, pgx, golang-migrate, `log/slog`) · PostgreSQL 16.

---

## Quickstart

```bash
cp .env.example .env      # optional: the defaults work as they are
task up
```

| Service | URL |
|---|---|
| Web app | <http://localhost:5173> |
| API | <http://localhost:8080/api/v1/ideas> |
| Health | <http://localhost:8080/healthz> |
| Postgres | `localhost:5432` (`idpad` / `idpad`) |

`task down` stops the stack and keeps your data; `task clean` also drops the
volume. `task logs` follows all services (`SERVICE=api task logs` narrows it).

### Working on the code

```bash
task dev      # Postgres in Docker, API with hot reload (air), Vite on :5173
task test     # Go unit + testcontainers integration tests, and the web suite
task lint     # go vet, gofmt, golangci-lint, tsc --noEmit, eslint
task build    # build both container images locally
```

`task dev` runs the API on the host, so the Vite dev server proxies `/api` to
`http://localhost:8080` — the browser only ever talks to one origin, in
development and in production alike.

### Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | — (required) | pgx connection string |
| `IDPAD_ADDR` | `:8080` | listen address |
| `IDPAD_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `IDPAD_LOG_FORMAT` | `json` | `json` or `text` |
| `IDPAD_CORS_ORIGINS` | `*` | comma-separated origins, or `*` |
| `IDPAD_MIGRATE_ON_START` | `true` | apply pending migrations on boot |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | `idpad` | database credentials |
| `API_PORT` / `WEB_PORT` / `POSTGRES_PORT` | `8080` / `5173` / `5432` | published host ports |

### Migrations

The SQL files in `api/migrations/` are the single source of truth for the
schema. They are embedded in the server binary and applied on start-up, and the
same files drive the CLI:

```bash
task migrate                       # apply everything pending
task migrate-down                  # roll back one step
task migrate-new NAME=add_tags     # scaffold the next numbered pair
```

### Using a PostgreSQL you already run

`task up` needs no database setup — the postgres container creates the role and
the database from the `POSTGRES_*` variables the first time its volume is
initialised. To point idpad at a server you already have instead (a managed
instance, a Homebrew install, a shared development box), create the role and
database with:

```bash
task db-create
```

It reads the same `.env` as everything else, is idempotent, and makes the role
the owner of the database's public schema — which PostgreSQL 15 and later no
longer grant implicitly. It uses `psql` when that is on `PATH` and otherwise
runs the same statements inside a throwaway `postgres:16-alpine` container, so
Docker on its own is enough. Applying the schema stays a separate step:

```bash
task db-create && task migrate
```

Connect as a superuser other than the local `postgres` role by setting the
usual variables, in `.env` or inline:

```bash
PGHOST=db.internal PGSUPERUSER=admin PGSUPERPASSWORD=secret task db-create
```

`task db-drop` removes the database and the role again (it asks you to type the
database name first), and `task db-reset` drops, recreates and re-migrates in
one step. Run `scripts/create-db.sh --help` for the full set of variables.

---

## API

REST under `/api/v1`, JSON only, snake_case keys, RFC 3339 timestamps.
Successes are `{"data": …}`; failures are
`{"error": {"code": "…", "message": "…"}}` with a meaningful status: 400 for
validation, 404 for a missing row, 409 for a conflict, 422 when a well-formed
request breaks a domain rule.

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/v1/ideas` | create an idea |
| `GET` | `/api/v1/ideas` | list ideas with `comment_count` and `resource_count` |
| `GET` | `/api/v1/ideas/{id}` | fetch an idea |
| `PUT` | `/api/v1/ideas/{id}` | replace title and body |
| `DELETE` | `/api/v1/ideas/{id}` | delete an idea and everything on it |
| `GET` | `/api/v1/ideas/{id}/comments` | threads with replies and `detached` |
| `POST` | `/api/v1/ideas/{id}/comments` | start a thread, or reply with `parent_id` |
| `PUT` | `/api/v1/comments/{id}` | edit a comment body |
| `DELETE` | `/api/v1/comments/{id}` | delete a comment (replies cascade) |
| `PATCH` | `/api/v1/comments/{id}/status` | set `open` or `resolved` |
| `GET` | `/api/v1/ideas/{id}/resources` | list resources |
| `POST` | `/api/v1/ideas/{id}/resources` | attach an image or link |
| `DELETE` | `/api/v1/resources/{id}` | remove a resource |

### Walkthrough with curl

Create an idea whose body is a TipTap document:

```bash
IDEA=$(curl -sX POST localhost:8080/api/v1/ideas \
  -H 'Content-Type: application/json' \
  -d '{
        "title": "A notebook that comments back",
        "body": {"type":"doc","content":[
          {"type":"paragraph","content":[
            {"type":"text","text":"We should definitely ship this."}]}]}
      }' | jq -r .data.id)
```

Comment on the word "definitely". The anchor is the block index plus the
character offsets of the selection, and `snippet` is the literal selected text
— the server rejects the request with 422 if they disagree:

```bash
THREAD=$(curl -sX POST localhost:8080/api/v1/ideas/$IDEA/comments \
  -H 'Content-Type: application/json' \
  -d '{
        "body": "Do we have the capacity?",
        "anchor": {"block_index":0,"start_offset":10,"end_offset":20,"snippet":"definitely"}
      }' | jq -r .data.id)
```

Reply, then resolve the thread:

```bash
curl -sX POST localhost:8080/api/v1/ideas/$IDEA/comments \
  -H 'Content-Type: application/json' \
  -d "{\"parent_id\":\"$THREAD\",\"body\":\"Yes, next sprint.\"}" | jq .

curl -sX PATCH localhost:8080/api/v1/comments/$THREAD/status \
  -H 'Content-Type: application/json' \
  -d '{"status":"resolved"}' | jq .
```

Attach a resource and read the threads back:

```bash
curl -sX POST localhost:8080/api/v1/ideas/$IDEA/resources \
  -H 'Content-Type: application/json' \
  -d '{"type":"link","url":"https://tiptap.dev","label":"TipTap docs"}' | jq .

curl -s localhost:8080/api/v1/ideas/$IDEA/comments | jq '.data[] | {body, detached}'
```

Now edit the anchored words away and read the comments again — the thread comes
back with `"detached": true`:

```bash
curl -sX PUT localhost:8080/api/v1/ideas/$IDEA \
  -H 'Content-Type: application/json' \
  -d '{"title":"A notebook that comments back",
       "body":{"type":"doc","content":[{"type":"paragraph","content":[
         {"type":"text","text":"We should ship this."}]}]}}' > /dev/null

curl -s localhost:8080/api/v1/ideas/$IDEA/comments | jq '.data[] | {body, detached}'
```

---

## Testing

```bash
task test          # everything
task test:api      # Go: unit tests plus testcontainers integration tests
task test:web      # Vitest: anchor logic and a component test
```

The Go integration tests start a real `postgres:16-alpine` through
[testcontainers](https://golang.testcontainers.org/), run the migrations
against it, and exercise the router end to end. **They need a running Docker
daemon but no local Postgres** — a clean checkout with Docker available is
enough. One container is started per package and each test truncates the
tables, so the suite stays fast.

The frontend tests cover the pure anchor module (flattening, matching,
offset ↔ position mapping, detachment) and the comment thread component.

---

## Architecture notes

### Why bodies are `jsonb`

An idea's body is stored as the TipTap/ProseMirror document JSON, verbatim, in
a `jsonb` column. The API validates only that it is a node of type `doc` and
never normalises it, so no mark, attribute or future node type is lost in a
round-trip. `jsonb` (rather than `text`) keeps the column queryable — counting
nodes or indexing on document structure later needs no migration of existing
rows — while still handing the client back exactly what it sent.

### How comments anchor to text

A thread is anchored by `{block_index, start_offset, end_offset}` plus the
`snippet` that was selected. Offsets are character positions into the block's
**flattened text**: the concatenation of every descendant text node of one
top-level block, in document order. So a paragraph made of three differently
marked runs is one string, and an anchor into it is independent of how the
marks happen to be split.

Two details make this work in practice:

- **Offsets are UTF-16 code units.** That is the unit JavaScript string indices
  already use, so the browser and the Go validator agree byte-for-byte even
  when the text contains emoji. The Go side converts explicitly rather than
  counting runes.
- **The snippet is the source of truth.** `snippet` is stored alongside the
  offsets, so validity is a pure function of the current document: does the
  text at those offsets still equal the snippet?

Creating a thread validates the anchor server-side and returns 422 if it does
not match, so a thread never starts life broken. On every read the server
re-checks each anchor against the current body and returns `detached: true`
when it no longer matches. **A detached comment is kept, not deleted**: it stays
in the sidebar with a "text changed" note and renders no highlight.
Re-anchoring heuristics are out of scope for v1 — a clean detach is the correct
behaviour, and because matching is computed on read, restoring the text
re-attaches the thread automatically.

In the editor, highlights are **ProseMirror decorations, never marks**. The
document is never mutated to record a comment, which is what lets an anchor
detach without leaving an orphan mark behind. The decoration plugin re-resolves
every anchor whenever the document changes, so a highlight disappears the
instant its text is edited. All the offset arithmetic lives in
`web/src/lib/anchor.ts`, a module with no editor imports, and the plugin feeds
it plain data read from the live document — which is why that logic is unit
tested without mounting an editor at all.

### Other decisions

- **UUIDv7 generated in the application.** IDs are time-orderable, so `order by
  created_at, id` is stable, and no database extension is required.
- **Cascades in the database.** Deleting an idea removes its comments and
  resources through `on delete cascade`; deleting a thread root removes its
  replies the same way. The application never sweeps children by hand.
- **`updated_at` set explicitly.** Every update writes the timestamp in the
  same statement rather than through a trigger, so the behaviour is visible in
  the query.
- **A URL may only be attached to an idea once**, enforced by a unique
  constraint and surfaced as 409.
- **Threads are one level deep.** A reply cannot be a parent; the API returns
  422 rather than growing an unbounded tree.
- **Autosave.** The detail view saves the title and body 1.2 s after the last
  keystroke, and flushes pending edits before creating a comment so the server
  is always anchoring against text it can see.

### No authentication (yet)

v1 is single-user: there is no login, and comments render with a placeholder
author. The schema is ready for it — adding a `users` table and an `author_id`
column on `comments` (nullable at first, backfilled, then made `not null`) is a
migration and a handler change, with no restructuring of the anchoring or
threading model.

---

## Repository layout

```
api/                  Go REST API
  cmd/server/         entrypoint, graceful shutdown, self health-probe
  internal/config/    environment-based configuration
  internal/model/     domain types and the pure anchor logic
  internal/store/     pgx repository layer, connection and migration helpers
  internal/httpapi/   router, middleware, handlers, JSON envelopes
  internal/testutil/  testcontainers Postgres helper
  migrations/         numbered SQL migrations, embedded in the binary
web/                  React SPA
  src/api/            typed fetch client and wire types
  src/lib/            anchor math and formatting — pure, unit tested
  src/hooks/          TanStack Query hooks with optimistic updates
  src/components/     editor, highlight plugin, sidebar, resources
scripts/create-db.sh  role + database bootstrap for an existing server
compose.yaml          local stack (builds from source)
compose.prod.yaml     the same stack from images on GHCR
Taskfile.yml          every development task
```

## Continuous integration

`.github/workflows/ci.yml` runs two jobs. **lint-and-test** (on pull requests
and pushes to main) runs `go vet`, `gofmt`, `golangci-lint` and `go test -race`
for the backend — the testcontainers suite runs unchanged on stock
`ubuntu-latest` — then `npm ci`, `tsc --noEmit`, `eslint`, `vitest` and a
production build for the frontend. **build-and-push** (on pushes to main and on
`v*` tags) builds `linux/amd64` and `linux/arm64` images with a GitHub Actions
layer cache and pushes them to
`ghcr.io/ipedrazas/idpad/idpad-api` and `…/idpad-web`, tagged `latest`, the
commit SHA, and the semver version for release tags. It authenticates with the
built-in `GITHUB_TOKEN`, so no PAT is needed.

Run the published images with `task prod` (set `POSTGRES_PASSWORD` in `.env`
first — `compose.prod.yaml` refuses to start without one).
