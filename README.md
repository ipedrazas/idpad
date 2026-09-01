# idpad

A personal idea notebook with Google Docs–style commenting. Write an idea in a
rich-text editor, highlight any part of it and leave a threaded comment, and
attach images and links that stay with the idea.

- **Ideas** — create, list, edit and delete notes with headings, lists, bold and italic.
- **Status** — every idea sits in exactly one of draft, in progress, done or rejected. Move it from the detail view; filter the list by one state or several.
- **Comments** — anchor a thread to an exact text selection, reply, edit, resolve. Highlights are rendered over the anchored text and cycle through a colour palette.
- **Resources** — attach image and link URLs, shown as thumbnails and cards.
- **Tags** — a shared vocabulary rather than free text: spellings fold onto one slug, so "Machine Learning" and "machine-learning" are the same tag. Filter the list by one tag or stack several to narrow.
- **Connections** — link ideas with a typed relation (`references`, `expands`, `similar`, `related`). A link is stored once and shown from both ends, phrased from whichever idea you are reading.
- **Suggested tags** — one button sends the idea to a tagging service and merges what it suggests into the existing tags. Optional: without `IDPAD_TAGGER_URL` the button is not shown.
- **Theme** — light, dark, or follow the system, chosen from the nav bar and remembered.

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
| `IDPAD_TAGGER_URL` | — (off) | full URL of the tagging service endpoint |
| `IDPAD_TAGGER_TIMEOUT` | `45s` | budget for one call to that service (1s–2m) |
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
| `GET` | `/api/v1/ideas` | list ideas with counts and tags; filter with `?tag=`, `?status=`, `?q=`, `?exclude=` |
| `GET` | `/api/v1/ideas/{id}` | fetch an idea |
| `PUT` | `/api/v1/ideas/{id}` | replace title and body |
| `PATCH` | `/api/v1/ideas/{id}/status` | set `draft`, `in_progress`, `done` or `rejected` |
| `DELETE` | `/api/v1/ideas/{id}` | delete an idea and everything on it |
| `GET` | `/api/v1/ideas/{id}/comments` | threads with replies and `detached` |
| `POST` | `/api/v1/ideas/{id}/comments` | start a thread, or reply with `parent_id` |
| `PUT` | `/api/v1/comments/{id}` | edit a comment body |
| `DELETE` | `/api/v1/comments/{id}` | delete a comment (replies cascade) |
| `PATCH` | `/api/v1/comments/{id}/status` | set `open` or `resolved` |
| `GET` | `/api/v1/ideas/{id}/resources` | list resources |
| `POST` | `/api/v1/ideas/{id}/resources` | attach an image or link |
| `DELETE` | `/api/v1/resources/{id}` | remove a resource |
| `GET` | `/api/v1/tags` | the tag vocabulary with `idea_count` |
| `DELETE` | `/api/v1/tags/unused` | drop tags no idea carries |
| `GET` | `/api/v1/ideas/{id}/tags` | an idea's tags |
| `PUT` | `/api/v1/ideas/{id}/tags` | replace an idea's whole tag set |
| `GET` | `/api/v1/ideas/{id}/links` | connections in both directions |
| `POST` | `/api/v1/ideas/{id}/links` | connect this idea to another |
| `DELETE` | `/api/v1/links/{id}` | remove a connection (from either end) |
| `POST` | `/api/v1/ideas/{id}/tags/auto` | suggest tags for the idea and merge them in |
| `GET` | `/api/v1/statuses` | the four lifecycle states with `idea_count` |
| `GET` | `/api/v1/features` | which optional capabilities this server has |

The list endpoint's `tag` parameter may be repeated, and stacking narrows:
`?tag=go&tag=postgres` returns only ideas carrying both. Values are slugified,
so `?tag=Machine%20Learning` and `?tag=machine-learning` are the same filter.

`status` may be repeated too, but stacking widens: an idea is in exactly one
state, so `?status=draft&status=done` can only mean "either". The two filters
are combined with each other, so `?tag=go&status=in_progress` is the ideas that
are both.

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

Tag it, then watch two spellings fold onto one tag:

```bash
curl -sX PUT localhost:8080/api/v1/ideas/$IDEA/tags \
  -H 'Content-Type: application/json' \
  -d '{"tags":["Product","Machine Learning"]}' | jq '.data[].slug'

OTHER=$(curl -sX POST localhost:8080/api/v1/ideas \
  -H 'Content-Type: application/json' \
  -d '{"title":"Embedding search"}' | jq -r .data.id)

curl -sX PUT localhost:8080/api/v1/ideas/$OTHER/tags \
  -H 'Content-Type: application/json' \
  -d '{"tags":["machine-learning"]}' > /dev/null

# One tag, carried by two ideas — not two tags.
curl -s localhost:8080/api/v1/tags | jq '.data[] | {slug, name, idea_count}'

# Stacking tags narrows the list.
curl -s 'localhost:8080/api/v1/ideas?tag=machine-learning&tag=product' | jq '.data[].title'
```

Move an idea along its lifecycle:

```bash
# Every idea starts as a draft.
curl -sX PATCH localhost:8080/api/v1/ideas/$IDEA/status \
  -H 'Content-Type: application/json' \
  -d '{"status":"in_progress"}' | jq '.data | {status, status_changed_at, updated_at}'

# Stacking statuses widens: an idea is in exactly one of them.
curl -s 'localhost:8080/api/v1/ideas?status=draft&status=in_progress' | jq '.data[].title'

# Every state, empty ones included, which is what the filter chips render.
curl -s localhost:8080/api/v1/statuses | jq '.data[] | {status, idea_count}'
```

Ask the tagging service to suggest tags, if one is configured:

```bash
# Merged into whatever the idea already carries, never replacing it.
curl -sX POST localhost:8080/api/v1/ideas/$IDEA/tags/auto | jq '.data[].name'

# Whether the button is available at all.
curl -s localhost:8080/api/v1/features | jq .data
```

Connect the two ideas, and read the link from each end:

```bash
curl -sX POST localhost:8080/api/v1/ideas/$OTHER/links \
  -H 'Content-Type: application/json' \
  -d "{\"target_idea_id\":\"$IDEA\",\"relation\":\"expands\",
       \"note\":\"takes the notebook idea further\"}" > /dev/null

# The source declared it: "expands -> A notebook that comments back"
curl -s localhost:8080/api/v1/ideas/$OTHER/links \
  | jq '.data[] | {direction, relation, other_title}'

# The target sees the same row from the other side, as incoming.
curl -s localhost:8080/api/v1/ideas/$IDEA/links \
  | jq '.data[] | {direction, relation, other_title}'
```

---

## Testing

```bash
task test          # everything
task test:api      # Go: unit tests plus testcontainers integration tests
task test:web      # Vitest: anchor, slug and relation logic, plus component tests
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

### Status is a column, not a tag

Draft, in progress, done and rejected went in as a `status` column with a check
constraint rather than four tags. A tag set is open and many-valued: nothing
would stop an idea being tagged both `done` and `rejected`, and four
process labels would sit in the vocabulary index alongside the subjects the
ideas are actually about. A status is closed and single-valued, and the
database is the right place to say so.

It moves through `PATCH /ideas/{id}/status`, its own endpoint rather than a
field on the `PUT`. The editor autosaves title and body every 1.2 s; if status
rode along on that request, a tab left open on a stale idea would quietly
restore the status it was showing. Separate endpoints make the two writes
independent in both directions.

`status_changed_at` is a second timestamp so `updated_at` keeps meaning "the
body was edited". Marking an idea done changes nothing it says, and if it
bumped `updated_at` the newest-first list would reshuffle every time something
was filed away. Re-setting the status an idea already has leaves the timestamp
alone, so the transition it records is a real one.

### Tags are a vocabulary, not strings

Tags live in their own table keyed by a `slug` — lower-cased, accent-stripped,
with punctuation runs collapsed to hyphens — while `name` keeps the spelling
someone typed. Writing "Machine Learning" on one idea and "machine-learning" on
another therefore produces one tag carried by two ideas, not two tags. That is
what makes the tag index, the usage counts and the filter meaningful.

The rule is implemented twice: `model.Slugify` in Go is authoritative, and
`web/src/lib/slug.ts` mirrors it so the chip editor can recognise a duplicate
without a round trip. Both are tested against the same table of cases, and the
server's response is always what gets rendered.

`PUT /ideas/{id}/tags` replaces the whole set rather than adding one, which is
the shape a chip editor naturally produces — and sending `[]` is how the last
tag is removed. Tags outlive the ideas that used them (deleting an idea leaves
the vocabulary intact); `DELETE /tags/unused` is the deliberate cleanup.

### Links are stored once and read from both ends

A connection is one row, directed `source → target`, with a relation. The idea
that declared it reads the active phrasing, and the idea at the other end reads
the passive one: "references" on one page is "referenced by" on the other. The
two symmetric relations — `similar`, `related` — are their own inverse and read
identically either way, so they get a two-headed arrow rather than a direction
they do not carry.

Storing one row rather than two is what keeps the ends from disagreeing: there
is no second row to fall out of step, either end can delete the link, and
`on delete cascade` on both foreign keys means an idea's removal takes its
connections with it. Rendering the inverse is entirely the client's job
(`web/src/lib/relations.ts`); the API only ever stores and returns the four
real relations, and rejects an inverse like `referenced_by` as a relation to
write.

### Suggested tags merge, and the service is never trusted blindly

`POST /ideas/{id}/tags/auto` flattens the idea — title first, then the body's
blocks — and posts `{"text": …}` to the service named by `IDPAD_TAGGER_URL`,
expecting `{"tags": [...]}` back. It **merges** what comes back into the idea's
existing tags rather than replacing them: a suggestion should never silently
discard a tag someone chose by hand, and an unwanted one is a single click to
remove. Existing names come first, so a suggestion can never displace the
spelling already in use.

The service is not bound by this API's tag rules, so each suggestion is judged
on its own — one unusable name is skipped rather than costing the good ones
alongside it — and the whole merged set still goes through the same validation
and slug-folding as a hand-typed tag. Suggestions that fold onto a tag the idea
already carries are dropped.

The call is proxied through the API rather than made from the browser: the
service's location stays server-side, there is no CORS round trip, and an
upstream failure is logged with its cause but reported to the client as a plain
502 — the upstream's status and body are never echoed back.

Two timeouts matter here. The service is much slower on a cold start (tens of
seconds) than when warm (a second or two), well past the global 30 s request
timeout, so this handler runs on its own `IDPAD_TAGGER_TIMEOUT` budget rather
than the shared one. Detaching also means a caller who navigates away still
gets the tags written, which is the useful outcome rather than wasted work. The
server's write timeout is derived from that budget, so raising one cannot leave
the other cutting the response off.

Because the endpoint is optional, `GET /features` reports whether it is
configured and the UI hides the button when it is not, rather than offering one
that always fails. Note that the service is not deterministic: pressing the
button twice can add tags the first pass did not return.

### The theme is resolved in JS, not by a media query

Dark mode is driven only by a `dark` class on `<html>`. The OS preference is
deliberately *not* part of Tailwind's `dark` variant, because a variant that
also matched `prefers-color-scheme: dark` would override an explicit choice of
light on a dark-themed machine — the one case a theme toggle exists for.

So `system` is resolved once, in JS, and the result is written to the class.
An inline script in `index.html` applies the stored preference before the first
paint, which is what stops a dark-theme reload flashing white; it mirrors
`resolveTheme`/`applyTheme` in `web/src/theme/theme.ts`, and the two have to be
changed together. Choosing `system` keeps following the OS live through a
`matchMedia` listener, rather than resolving once at load.

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
  internal/tagger/    client for the optional external tagging service
  internal/httpapi/   router, middleware, handlers, JSON envelopes
  internal/testutil/  testcontainers Postgres helper
  migrations/         numbered SQL migrations, embedded in the binary
web/                  React SPA
  src/api/            typed fetch client and wire types
  src/lib/            anchor math, slugs, relation phrasing — pure, unit tested
  src/hooks/          TanStack Query hooks with optimistic updates
  src/components/     editor, highlight plugin, sidebar, resources, tags, links
  src/theme/          theme preference, resolution and provider
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
