# Ideation v1 spec

This spec is build-ready. v1 can be built from it without making any further decisions. The vocabulary is in [`CONTEXT.md`](../CONTEXT.md) (Idea, Tag, Attribute, Status, Capture, Version, Filter, Review), and decisions that are hard to reverse are in [`docs/adr/`](adr/). Each section links the ticket that decided it; the ticket's resolution holds the reasoning. When this spec and a ticket disagree, this spec wins.

## Contents

1. [Overview](#1-overview)
2. [Data model](#2-data-model)
3. [Storage](#3-storage)
4. [Filter language](#4-filter-language)
5. [HTTP API](#5-http-api)
6. [Daemon lifecycle](#6-daemon-lifecycle)
7. [CLI](#7-cli)
8. [Review TUI](#8-review-tui)
9. [Agent skill](#9-agent-skill)
10. [nvim plugin](#10-nvim-plugin)
11. [Out of scope](#11-out-of-scope)

## 1. Overview

Ideation is a single-user, local service that is the one home for ideas. It runs on macOS only.

**Deliverables.**

| Deliverable | Where | What |
|---|---|---|
| `idea` binary | Go module `github.com/NurramoX/ideation` | One binary that is the daemon (`idea daemon`), the CLI (every other verb) and the Review TUI (`idea review`, bare `idea`). |
| Agent skill | `skill/SKILL.md` | The `idea` skill for Claude Code. It is embedded in the binary and installed by `idea install`. |
| nvim plugin | `nvim/` | `:e idea://<id>` read/write and `:NewIdea`, built on the CLI. |

**Stack.** Go, `modernc.org/sqlite` (cgo-free, FTS5 built in), bubbletea, bubbles, lipgloss and glamour **v2** (`charm.land/...`). The shape mirrors `~/Projects/cdf`: one binary, launchd owns the lifecycle, hand-rolled verb dispatch. Unlike cdf, the daemon speaks HTTP.

**Principles.**
- The server is the only home of an idea. There are no local copies and nothing to sync.
- Writes overwrite. There is no revision history.
- Agents make most edits.
- Text a user or agent wrote leaves through the server or stdout, never as a leftover file.

## 2. Data model

Decided in [Idea data model](https://github.com/NurramoX/ideation/issues/2).

An **idea** carries: `id`, `title`, `body`, a set of `tags`, a set of `attributes`, a `version`, and the timestamps `created_at`, `updated_at` and `reviewed_at`, all set by the server. Every violation of the rules below is a `422`.

| Field | Rules |
|---|---|
| `id` | A server-assigned integer, never reused (`INTEGER PRIMARY KEY AUTOINCREMENT`). Plain decimal on the wire. |
| `title` | Separate from the body; the server never parses the body for it. 1–400 characters after trimming, single line, no control characters, not unique. |
| `body` | Markdown, stored byte-exact, valid UTF-8, at most 10 MB (`413` over). May be empty, stored as `""`, never null. No frontmatter. |
| tag | A bare label, its own concept rather than an attribute. `[a-z0-9][a-z0-9_-]*` with input lowercased, at most 128 bytes, at most 128 per idea. Returned sorted. |
| attribute | `key → value`, exactly one value per key. The key follows the tag rules. The value is single-line UTF-8, 1–2000 bytes, trimmed, case preserved; an empty value is rejected (remove the key instead). At most 128 per idea. Returned sorted by key. |
| reserved keys | `id`, `title`, `body`, `tag`, `has`, `created`, `updated`, `reviewed` can never be attribute keys. |
| `version` | An integer. It advances, together with `updated_at`, only on a real change; a no-op write moves neither. |
| `created_at`, `updated_at`, `reviewed_at` | RFC 3339 UTC with millisecond precision (`2026-09-22T14:03:07.412Z`). `reviewed_at` is null until the first review. |

**Status** is the only built-in attribute: an ordinary attribute in the API that the server validates and defaults, and that cannot be removed (`422`). Input is lowercased, and an unknown value is a `422`. Any value may move to any other.

| Value | Meaning | |
|---|---|---|
| `raw` | captured, not yet through Review (the default on create) | open |
| `active` | alive, however slowly it moves | open |
| `done` | realised or concluded | closed |
| `dropped` | decided against | closed |

**Reviewed.** Only the explicit mark-reviewed operation sets `reviewed_at`.
- It sets server time and only ever moves forward.
- It needs no `If-Match` and is allowed on any idea.
- It never moves `version` or `updated_at`, and an edit never moves `reviewed_at`.

**Vocabulary** is derived: a tag or key exists exactly while at least one idea carries it.

**Delete** is a hard delete. There are no tombstones; the id then answers `404`.

## 3. Storage

Decided in [Full-text search with modernc.org/sqlite](https://github.com/NurramoX/ideation/issues/5) and [Daemon lifecycle and local HTTP exposure](https://github.com/NurramoX/ideation/issues/4). Only the daemon opens the database.

**Connection.**
- Pragmas: `journal_mode=WAL`, `foreign_keys=ON`, `busy_timeout=5000`, `synchronous=FULL`.
- A single connection (`SetMaxOpenConns(1)`), so everything serialises.

**Migrations.**
- Ordered, forward-only and embedded in the binary.
- Tracked in `PRAGMA user_version`, and each runs in a transaction at daemon start.
- If the database is newer than the binary knows, the daemon refuses to start.

**Schema.** The table names are free; these constraints are not:
- `idea(id INTEGER PRIMARY KEY AUTOINCREMENT, title, body, version, created_at, updated_at, reviewed_at)`, plus tables for tags (`idea_id, tag`) and attributes (`idea_id, key, value, value_folded`), both `ON DELETE CASCADE`. Status is an attribute row.
- Rows of `idea` change only by `UPDATE` or upsert, **never** `INSERT OR REPLACE` / `REPLACE INTO`. External-content FTS5 treats REPLACE as ABORT, and REPLACE skips the delete trigger.
- `value_folded` is the value under Unicode simple case folding, computed in Go on write. Filters compare against it. Don't use SQLite `NOCASE`, which is ASCII-only.

**Full-text index.** An external-content FTS5 table kept in sync by triggers, so the index and the content commit atomically:

```sql
CREATE VIRTUAL TABLE idea_fts USING fts5(
  title, body,
  content='idea', content_rowid='id',
  tokenize='porter unicode61 remove_diacritics 2'
);
CREATE TRIGGER idea_ai AFTER INSERT ON idea BEGIN
  INSERT INTO idea_fts(rowid, title, body) VALUES (new.id, new.title, new.body);
END;
CREATE TRIGGER idea_ad AFTER DELETE ON idea BEGIN
  INSERT INTO idea_fts(idea_fts, rowid, title, body) VALUES ('delete', old.id, old.title, old.body);
END;
CREATE TRIGGER idea_au AFTER UPDATE ON idea BEGIN
  INSERT INTO idea_fts(idea_fts, rowid, title, body) VALUES ('delete', old.id, old.title, old.body);
  INSERT INTO idea_fts(rowid, title, body) VALUES (new.id, new.title, new.body);
END;
INSERT INTO idea_fts(idea_fts, rank) VALUES('rank', 'bm25(10.0, 1.0)');  -- title outweighs body
```

The FTS5 facts the filter compiler depends on:
- **`MATCH` only works as a top-level AND-ed term** in a query that joins `idea_fts.rowid = idea.id`. Only that join form can use `rank`, `bm25`, `snippet` and `highlight`.
- **Text under `or` or `-`** compiles to `id [NOT] IN (SELECT rowid FROM idea_fts WHERE idea_fts MATCH ?)`. It filters, but can't be ranked or snippeted.
- **Quoting:** every text term is sent as one quoted phrase (`"..."`, with inner `"` doubled), bound as a parameter, never interpolated. Raw input such as `key=value`, `foo-bar` or an empty string is a MATCH syntax error.
- **`rank`** is negative, and more negative is better: `ORDER BY rank` ascending. The scores are unstable and are never exposed.
- **Snippets** are plain text. Use `snippet(idea_fts, -1, '', '', '…', 16)`, with no highlight markers, because markers would land inside the markdown.
- **Maintenance:** `'rebuild'` and `'integrity-check'` repair and verify the index. Run `'rebuild'` after any migration that touches `idea`.

## 4. Filter language

Decided in [Filter language](https://github.com/NurramoX/ideation/issues/6); see [ADR 0002](adr/0002-compact-filter-syntax.md). A Filter is one string. Only the server parses it; clients pass it through opaquely.

```
filter := or                          empty filter matches every idea
or     := and ( OR and )*             OR is the word `or` in any case; loosest
and    := term+                       juxtaposition is AND
term   := '-' term                    negation; binds tightest
        | '(' or ')'
        | WORD | STRING               text term over title and body
        | KEY ':' value (',' value)*  any-of: `status:raw,active`
value  := WORD | STRING
WORD   := run of characters other than whitespace ( ) : , "
STRING := "..." with "" for a literal quote
```

**Syntax rules.**
- Keys match lowercased.
- A value or text term needs quotes when it contains whitespace, `(`, `)`, `:`, `,` or `"`, starts with `-`, or is the word `or`.
- There are no `<`/`>` operators, no wildcards, no prefix match and no ordering on attribute values.

| Key | Example | Meaning |
|---|---|---|
| any attribute, including `status` | `effort:small` | attribute equals value, case-insensitively (Unicode simple folding) |
| `tag` | `tag:rust` | the idea carries the tag |
| `id` | `id:12` | the idea's id |
| `has` | `has:effort`, `has:tag`, `has:reviewed` | attribute present; any tag present; reviewed at least once |
| `title`, `body` | `title:borrow` | text term restricted to that FTS column |
| `created`, `updated`, `reviewed` | `created:2026-09`, `reviewed:..90d` | date within a period or range |
| none | `borrow`, `"borrow checker"` | text term over title and body |

**Text.**
- Each text term becomes one quoted FTS5 phrase (see [Storage](#3-storage)).
- Top-level AND text terms are **ranked**: they enable `snippet` and `sort=rank`.
- Text terms under `or` or `-` are allowed but unranked.

**Dates.**
- A **period** is `2026`, `2026-09`, `2026-09-01`, `today` or a full RFC 3339 instant; `created:2026-09` means during September.
- A **range** is `a..b`, `a..` or `..b`, both ends inclusive. An end is a period or a **relative instant**, `90d`, `2w`, `6m` or `1y`, meaning that long before now.
- A lone relative instant (`created:7d`) is a parse error that points at the range spelling.
- Periods are read in the machine's local time zone.
- A null `reviewed` satisfies no date term, so "never reviewed" is `-has:reviewed`, and the **Review queue** is `status:raw,active -reviewed:90d..`.

**Errors.**
- A parse error is a `400` carrying `position`, the 1-based rune position of the offending character.
- An unknown tag or key is **valid and matches nothing**: a filter's validity never depends on data. Clients catch typos with a vocabulary hint (see [CLI](#7-cli)).

## 5. HTTP API

Decided in [HTTP API surface](https://github.com/NurramoX/ideation/issues/7), with amendments from the data model. Plain HTTP/1.1 over the Unix socket ([ADR 0001](adr/0001-unix-socket-only.md)): no auth and no version prefix.

| Method and path | Purpose |
|---|---|
| `GET /` | `{"service":"ideation","api":1}`: compatibility check and liveness probe. |
| `POST /ideas` | JSON `{title, body?, tags?, attributes?}`, where `title` is required → `201` with `Location`, `ETag` and the envelope. May set `status`. |
| `GET /ideas?filter=&sort=&order=&limit=&offset=` | List: `{ideas: [metadata], total}`. Never includes bodies. |
| `GET`/`HEAD /ideas/{id}` | Envelope `{id, title, tags, attributes, body, version, created_at, updated_at, reviewed_at}`. |
| `PATCH /ideas/{id}` | Merge update (RFC 7396): absent means untouched, `tags` replaces the set, `attributes: {k: null}` removes a key, and `body` is replaced if present → `200` with the envelope (body omitted unless it was sent). There is no JSON `PUT`. |
| `GET`/`HEAD`/`PUT /ideas/{id}/body` | Raw `text/markdown; charset=utf-8`. `PUT` → `204`. |
| `PUT`/`DELETE /ideas/{id}/tags/{tag}` | Idempotent → `204`, including when already present or absent. |
| `PUT`/`DELETE /ideas/{id}/attributes/{key}` | The request body is the value. Idempotent → `204`. |
| `PUT /ideas/{id}/reviewed` | Mark reviewed → `204`. No body and no `If-Match`. Doesn't touch `version` or `updated_at`, so it returns no new `ETag`. |
| `DELETE /ideas/{id}` | Hard delete → `204`. |
| `GET /tags`, `GET /attributes`, `GET /attributes/{key}` | Vocabulary: `[{tag, count}]`, `[{key, count}]`, `[{value, count}]`. |

**Lists.**
- Metadata is the envelope without `body`.
- A plain-text `snippet` appears only when the filter has a ranked text term.
- `sort` is one of `updated|created|reviewed|title|rank` and `order` is `asc|desc`. The default is `rank` when a ranked text term is present, otherwise `updated desc`. `sort=rank` without a ranked term is a `400`.
- `sort=reviewed` treats never-reviewed as oldest.
- `limit` and `offset` exist, with no default limit and no cursors.
- An absent `filter` means all ideas.

**Version.**
- Every response exposes the Version as `ETag` and as `version`, and every successful write returns the new `ETag`.
- `If-Match` is **required** on `PATCH`, `PUT /body` and `DELETE /ideas/{id}`:
  - missing → `428`
  - stale → `412` with `current_version`
  - `If-Match: *` forces the write
- `If-Match` is optional but honoured on the tag and attribute operations.
- Reads support `If-None-Match` → `304`.

**Bodies.** Returned byte-exact. Invalid UTF-8 → `422`, over 10 MB → `413`, a `Content-Type` other than `text/markdown` → `415`.

**Ids.** A non-numeric id → `400`. An unknown or deleted id → `404`.

**Errors.** `application/problem+json` (RFC 9457) on every endpoint, raw ones included:

| Status | When | Extra member |
|---|---|---|
| `400` | malformed request, id or filter | `position` for filter errors |
| `404` | unknown or deleted id | |
| `412` | stale `If-Match` | `current_version` |
| `413` | body over 10 MB | |
| `415` | wrong `Content-Type` | |
| `422` | invalid content | |
| `428` | missing `If-Match` | |

## 6. Daemon lifecycle

Decided in [Daemon lifecycle and local HTTP exposure](https://github.com/NurramoX/ideation/issues/4).

**Home.** `~/Library/Application Support/ideation/` holds exactly `ideation.db` (plus WAL files), `ideation.sock` and `daemon.lock`.
- The daemon creates it and `chmod`s it to `0700` on every start.
- `IDEATION_HOME`, read the same way by daemon and CLI, replaces the root for tests and dev builds.
- The launchd job never sets `IDEATION_HOME`.

**Socket.** `<home>/ideation.sock`, `chmod`ed to `0600` after bind. There is never a TCP listener, and there is no token or peer check. The daemon refuses to start if the socket path exceeds 104 bytes.

**Single instance.** The first act of `idea daemon` is `flock(LOCK_EX|LOCK_NB)` on `daemon.lock`, held for the life of the process.
- If another process holds the lock, the daemon exits non-zero with "already running" and touches nothing.
- Only the lock holder may unlink a stale socket and bind.

**Shutdown.** On `SIGTERM`/`SIGINT`, in order:
1. Stop accepting new connections.
2. `http.Server.Shutdown`, draining for up to 5 s, then force-close the stragglers.
3. `PRAGMA wal_checkpoint(TRUNCATE)`.
4. Close the DB.
5. Remove the socket.
6. Release the lock.
7. Exit 0.

**Logs.** Stderr only: lifecycle events and errors. No access log, never titles or bodies, no rotation.

**launchd.** The daemon runs as a LaunchAgent with label `io.github.nurramox.ideation` and plist `~/Library/LaunchAgents/io.github.nurramox.ideation.plist`.

| Key | Value |
|---|---|
| `Label` | `io.github.nurramox.ideation` |
| `ProgramArguments` | `[os.Executable() (symlinks not resolved), "daemon"]` |
| `RunAtLoad` | true |
| `KeepAlive` | true |
| `StandardOutPath`, `StandardErrorPath` | `~/Library/Logs/ideation/daemon.log` |
| `ExitTimeOut` | 10 |

- Deliberately absent: `EnvironmentVariables`, `ProcessType`, `ThrottleInterval` and `Sockets` (no socket activation).
- `idea daemon` itself knows nothing about launchd.
- A `gui/<uid>` agent runs only while the user is logged in graphically.

## 7. CLI

Decided in [CLI command surface](https://github.com/NurramoX/ideation/issues/9), with amendments from the daemon, filter and Review TUI tickets.

**Dispatch.**
- Verbs are dispatched by hand, verb first: `idea <verb> [args]`.
- Bare `idea` is `idea review`.
- An unknown verb is exit 2, never an implicit Filter.
- Flags may appear anywhere after the verb, and `--` ends them. There are no global flags.
- Ids are plain decimal; anything else is exit 2.
- The CLI validates nothing about tags, keys or values: it relays the server's `422`.

**Verbs.**

| Verb | Behaviour |
|---|---|
| `add <title> [-t <tag>]... [-s <k>=<v>]... [--body-file <path>\|-] [--edit]` | One `POST /ideas`. Reads the body from stdin only with an explicit `-`, never by sniffing whether stdin is a tty; otherwise the body is empty. `--edit` opens the editor first and creates on save. Prints exactly the new id. |
| `show <id>...` | Human output: a header block (title, id, status, tags, attributes, version, dates), a blank line, then the body; several ideas are separated by `---`. `--json`: one envelope per line (JSON Lines). |
| `ls [<filter words>...] [--sort updated\|created\|reviewed\|title\|rank] [--asc\|--desc] [--limit n] [--offset n] [-q\|--json]` | The Filter is the positionals joined by single spaces. An unknown single-dash word (`-has:effort`) is a negated term, here and in `review`; `--` guards one that is also a flag (`-q`, `-h`). Human output: an aligned table of id, status, title, tags and snippet, plus `N ideas` on stderr when `total` exceeds what is shown. `-q`: ids only. `--json`: `{ideas, total}` verbatim. `-q` with `--json` → exit 2. An empty result is exit 0 plus the vocabulary hint. |
| `body <id>` | The raw body, byte-exact. |
| `write <id> --version <n>\|--force [--body-file <path>]` | Reads the body from stdin by default. Refuses (exit 2) without `--version` or `--force`. Empty input writes an empty body. |
| `replace <id> <old> <new> [--old-file p] [--new-file p] [--all]` | Read, literal byte-exact replace, guarded write. `<old>` must occur exactly once unless `--all`; zero or several matches → exit 5 with the count. An empty `<old>` → exit 2, and an empty `<new>` deletes. On a `412` it re-reads and retries the whole operation up to 3 times, then exits 4. It is the only verb that retries. |
| `edit <id>` | `$EDITOR` round-trip, see below. |
| `title <id> <words...>` | Words joined by one space. Guarded. |
| `tag <id> <tag>...`, `untag`, `set <id> <k>=<v>...`, `unset <id> <key>...`, `status <id> <value>` | One unguarded idempotent call per tag or key, in order, stopping at the first failure. `set` splits on the first `=`, and an empty value → exit 2 pointing at `unset`. `status` is `set status=<value>`. |
| `rm <id>... [-y] [--force]` | Hard delete. On a terminal it prompts `delete idea 42 "<title>"? [y/N]`. Without a terminal and without `-y` it refuses (exit 2). |
| `review [<filter words>...]` | The Review TUI ([section 8](#8-review-tui)). |
| `reviewed <id>...` | Mark reviewed. |
| `tags`, `attrs [<key>]` | Vocabulary with counts. `--json` prints the API arrays. |
| `daemon` | Runs the server in the foreground. This is what launchd executes. |
| `install` | Idempotent, and also the upgrade step. It creates the home and log directories, writes the plist, runs `launchctl enable`, `bootout` if loaded, then `bootstrap gui/<uid>`. It waits for `GET /`, **writes the embedded skill to `~/.claude/skills/idea/SKILL.md`**, and prints the socket, DB, log and skill paths. |
| `uninstall` | `bootout`, then removes the plist and `~/.claude/skills/idea/`. It never touches the home or the logs, and prints where they remain. |
| `stop` / `start` | `launchctl bootout` / `bootstrap` of the installed plist, never HTTP. `start` without a plist → error pointing at `install`. |
| `ping` | `GET /`: prints the service and api number. Exit 0, or exit 7 when the daemon is down. |
| `help [<verb>]`, `-h`/`--help`, `version` | Help goes to stdout; each verb's help carries one agent-oriented example. `version` prints the binary version, plus the daemon api when reachable, warning on a mismatch. |
| `completion fish\|zsh\|bash`, hidden `complete <kind> [<prefix>]` | See Completion below. |

**Batch and `IDEATION_HOME` rules.**
- Only `show`, `rm` and `reviewed` take several ids. They process each in order, continue past failures (one stderr line each), and exit with the code of the first failure. Bulk work is a shell loop over `idea ls -q`.
- `install`, `uninstall`, `start` and `stop` refuse (exit 2) while `IDEATION_HOME` is set.

**Version flags.**
- Every guarded verb accepts `--version <n>` (sent as `If-Match`) and `--force` (`If-Match: *`).
- `title`, `rm`, `replace` and `edit` GET first when given neither; `--force` skips that GET.
- `write` demands one of the two.
- `tag`, `untag`, `set`, `unset` and `status` send `If-Match` only when given `--version`.
- A `412` prints `idea 42 changed: you had version 7, it is now 9` and exits 4 (only `replace` retries).

**Output.**
- Human output by default. `--json` is explicit, never sniffed, and prints the API's own shapes.
- Mutating verbs print nothing on success, with two exceptions:
  - `add` prints the id.
  - With `--json`, `add`, `write`, `replace`, `edit`, `title`, `tag`, `untag`, `set`, `unset` and `status` print `{"id":<n>,"version":<n>}`.
- Diagnostics go to stderr.
- No colour when stdout is not a terminal or when `NO_COLOR` is set.
- The CLI never adds or strips a trailing newline in `body`, `write`, `replace` or `edit`.

**Exit codes.**

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | unexpected or internal error, including `428` |
| 2 | usage error, never sent to the server |
| 3 | not found (`404`) |
| 4 | stale Version (`412`) |
| 5 | rejected content (`422`, `413`, `415`), or a `replace` match-count failure |
| 6 | malformed request or Filter (`400`); filter errors are shown with a caret under `position` |
| 7 | daemon unreachable, with a diagnosis |
| 8 | daemon api mismatch: "run `idea install`" |

Humans see `idea: <title>: <detail>` on stderr. With `--json`, the problem+json document goes to stderr verbatim.

**Reaching the daemon.** Every invocation first calls `GET /`.
- **api mismatch:** exit 8, before anything else is sent.
- **Connect failure:** the CLI runs `launchctl print gui/<uid>/<label>`.
  - Job not loaded → exit 7 "not installed: run `idea install`".
  - Job loaded → retry with backoff for up to 2 s, then exit 7 "installed but not responding, see <log path>".
- **With `IDEATION_HOME` set:** no launchctl and no retry; exit 7 "no daemon at <home>: run `idea daemon`".
- The CLI never starts the daemon.

**Vocabulary hint.** When `ls` or `review` gets an empty result, the CLI checks the filter's tags and keys against `/tags` and `/attributes`. It prints e.g. `no ideas have the key 'tga'` on stderr.

**Edit round-trip.** Shared by `edit`, `add --edit` and the TUI's `e`.
- **Temp file:** it holds the body only, at `$TMPDIR/idea-<id>-*.md`, mode `0600`, inside a `0700` directory. It is always removed, including on SIGINT and SIGTERM.
- **Editor:** `$VISUAL`, then `$EDITOR`, then `vi`.
- **No write:** when the content is unchanged or the editor exits non-zero.
- **On a `412`,** the CLI prompts:
  - `[o]verwrite`
  - `[r]e-edit`: prints the other party's current body to stderr, reopens the user's text, and guards against the new Version. There are no conflict markers.
  - `[a]bort`: prints the user's text to stdout.
- **Non-interactive `412`:** abort, text to stdout, exit 4.

**Completion.** The hidden `idea complete <kind> [<prefix>]` prints one candidate per line, and exits 0 silently when the daemon is down, with no retry.

| Kind | Used for | Candidates |
|---|---|---|
| `verbs` | the verb | every verb |
| `tags` | `tag`, `untag`, `add -t` | existing tags |
| `keys` | `set`, `unset` | existing attribute keys |
| `values <key>` | after `set <id> key=` | existing values of that key; the four Status values for `status` |
| `ids` | id arguments | `<id>\t<title>` for the 50 most recently updated ideas |

The Filter gets no completion. `idea completion fish|zsh|bash` prints the script; fish comes first, and zsh and bash behave the same.

## 8. Review TUI

Decided in [Review TUI](https://github.com/NurramoX/ideation/issues/8). It is telescope-style: a filter bar and list on the left, a preview on the right.

**Layout.**
- **Left:** a one-line filter bar showing the Filter verbatim, with the list under it. Each row shows a status letter, the title, dimmed tags and, right-aligned, the age of the current sort field. `✓` marks rows reviewed this session, and `▸` marks the selected row.
- **Right:** the preview. A header (title, id, status, tags, attributes, created/updated/reviewed ages, version), then the body rendered with glamour at pane width. `m` toggles rendered ↔ raw.
- **Bottom:** a status line with `n/total`, the sort order, errors and a `?` hint.
- **Proportions:** the list takes about 40% of the width, at least 30 columns. Under 80 columns only one pane shows, and `tab` switches between them.

**Opening.** `idea review [<filter words>...]`, or bare `idea`. The bar is prefilled with the given Filter, or with the **Review queue** `status:raw,active -reviewed:90d..` when none is given. The 90-day window is fixed. Focus starts on the list.

**Order.** The default is `sort=updated` desc. `o` cycles:
1. updated (newest first)
2. reviewed (longest unseen first, never-reviewed first)
3. created (newest first)
4. random (a client-side shuffle)

**Filter bar.** `/` or `f` focuses it with the cursor at the end.
- **Live requery:** typing requeries after about 150 ms of debounce.
- **Moving while typing:** `ctrl-n`/`ctrl-p` and the arrow keys move the selection while in the bar.
- **Leaving the bar:** `enter` or `esc` returns to the list and keeps the filter.
- **Errors:** a `400` keeps the last good list and shows the error with a caret under `position`.
- **Empty result:** shows the vocabulary hint.
- **Empty filter:** lists everything.

**Snapshot.** The list is the result of the last query.
- **Actions don't reorder:** an action updates its row in place, and nothing reorders or disappears until the next requery.
- **What requeries:** a filter edit, `o`, or `ctrl-r`.
- **Selection:** a requery keeps the selected idea when it is still present, and otherwise selects the top row.

**Keys (list focus).**

| Key | Does | Then |
|---|---|---|
| `j`/`k`, arrows, `g`/`G` | move the selection; never marks reviewed | |
| `J`/`K`, `ctrl-d`/`ctrl-u` | scroll the preview | |
| `space` `enter` `n` | skip: mark reviewed | advance |
| `z` | later: does not mark reviewed | advance |
| `a` `d` `x` `r` | status → active / done / dropped / raw, then mark reviewed | advance |
| `t` | retag inline (edit the space-separated tag list; `enter` applies per-tag `PUT`/`DELETE`) | stay |
| `e` | body round-trip in the editor (see [CLI](#7-cli)) | stay |
| `D` | delete after `y`/N, with `If-Match` set to the Version shown | advance |
| `/` `f` | focus the filter bar | |
| `o` | cycle order | |
| `ctrl-r` | requery | |
| `m` | rendered ↔ raw | |
| `?` | help | |
| `q` `ctrl-c` | quit | |

"Advance" moves the selection to the next row. Attributes are read-only in Review.

**Data.** The list uses `GET /ideas`. The preview `GET`s the selected idea with a short debounce and revalidates with `If-None-Match`; that response's Version guards `e` and `D`.

**Conflicts.**
- A `412` on delete shows "changed, not deleted" and refreshes the preview.
- A `412` after the editor offers `[o]verwrite / [r]e-edit / [a]bort`. **Abort** prints the user's text to stdout after the TUI exits and the terminal is restored.

## 9. Agent skill

Decided in [Agent capture and editing flow](https://github.com/NurramoX/ideation/issues/3).

Agents use the `idea` CLI through one skill, `idea`. There is no MCP server and no raw HTTP. The skill's source is `skill/SKILL.md`; it is embedded in the binary and installed by `idea install` (see [CLI](#7-cli)). It carries **judgment only**: for syntax it points at `idea help <verb>` and never restates the verb table. Its description triggers on phrases like "capture this", "save this as an idea", "add this to my idea about…" and "what ideas do I have on…". The skill's prose is the builder's to write. The rules below are normative and must be in it.

**Capture.**
- **When:** only when the user asks. The agent never captures, or offers to, on its own.
- **Search first:** before every `idea add`, run a text Filter on the idea's key terms (`idea ls borrow checker tag:rust`).
  - On a clear match, ask whether to fold the discussion into that idea or create a new one. This is the only confirmation in the capture path.
  - With no match, capture without asking.
- **Body:** a distilled, self-contained markdown document: the idea, the reasoning that survived, and the open questions.
  - No transcript, no link to the conversation, no "as discussed".
  - No H1 repeating the title.
  - The first paragraph is the gist.
  - Open questions go under a closing heading.
- **Title and tags:** the agent picks the title. It runs `idea tags` and reuses existing tags, minting a new one only when nothing fits, and says so when it does.
- **Status** stays `raw` unless the user names one.
- **Attributes:** no provenance attribute. The agent sets an attribute only when the user names it, or when a key already in `idea attrs` plainly applies. It never creates a new key unprompted.
- **One idea per distinct idea.** A discussion with two separable ideas becomes two captures, and both ids are reported. Cross-references are plain prose ("see idea 42").
- **Report:** after uploading, one line: id, title, tags. There is no draft approval; Review is the approval step.

**Editing.**
- **Finding the idea:** `idea ls <filter> --json`. With one clear match, proceed and name the idea touched. With several, list them and ask. Never guess.
- **Tools:** `idea replace` for local changes. `idea show --json` then `idea write --version <n>` to restructure.
- **Integrate new material:** the document stays a current statement of the idea, with no dated log sections.
- **No history, so no silent loss:** never remove or contradict existing content unless asked. When removing, quote what was removed in the report.
- **Title and status** change only when the user asks.
- **On exit 4:** re-read the idea, reapply the change to the fresh body, and retry.

**Boundaries.**
- **Forbidden:** `--force` and `rm`.
  - Asked to delete, the agent hands the user `! idea rm <id>`, and offers `idea status <id> dropped` if the user only wants the idea out of the open set.
- **Reading** is unrestricted, but only when the user refers to ideas. The agent never searches them unprompted.
- **Several ideas:** list the affected ideas, wait for the go-ahead, then work one idea at a time.
- **On exit 7:** report the failure and print the finished document into the conversation. Never start the daemon and never write a fallback file.

## 10. nvim plugin

Decided in [nvim plugin](https://github.com/NurramoX/ideation/issues/10).

**Layout.** `nvim/plugin/idea.lua` and `nvim/lua/idea/`, loaded with lazy.nvim `dir = "~/Projects/ideation/nvim"`. It shells out to `idea` through `vim.system` and never speaks HTTP, so the CLI's api check, daemon diagnosis and messages all apply.

**`BufReadCmd idea://*`.**
- The part after `idea://` must be a decimal id.
- It runs `idea show --json <id>` and loads `body` into the buffer. A trailing `\n` sets `eol`; its absence sets `noeol` + `nofixeol`, so the round-trip is byte-exact.
- It sets `fileformat=unix`, `buftype=acwrite`, `filetype=markdown`, `noswapfile`, `b:idea_version` and `b:idea_title`.
- `:e!` reloads through the same path.

**`BufWriteCmd idea://*`.**
- It pipes the buffer to `idea write <id> --version <b:idea_version> --json`. The buffer is the lines joined by `\n`, plus a trailing `\n` only when `eol` is set.
- On success it stores the new `version` and clears `modified`.
- On failure it shows the CLI's stderr through `vim.notify` at error level and leaves the buffer modified. A stale Version (exit 4) is just that error, and `:e!` is the way out.
- It never forces a write.

**`:NewIdea <title>`.**
- It runs `idea add <title>` and opens `idea://<id>`.
- An empty title is an error.

## 11. Out of scope

Out of scope for v1:
- Multi-user use, remote access and cross-machine sync.
- A mountable filesystem.
- A web UI. Any future web UI gets its own listener ([ADR 0001](adr/0001-unix-socket-only.md)).
- Revision history.
- A designed migration of existing ideas; that is done ad hoc with an agent and the CLI.
- Bulk API operations, tag rename/merge, `sort=attr:<key>`, server-side partial body edits, change notifications and auth.
- Resurfacing beyond Review's ordering: reminders and nudges.
- Backup and export. `idea stop` gives a safe window to copy the DB file.
- An nvim picker and nvim conflict handling.
- Platforms other than macOS.
