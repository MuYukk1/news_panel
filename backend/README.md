# news_panel core (P1-01)

Imported from 0x2E/fusion v1.3.0 (MIT), commit
`e90198ae06d03d5820d97c126c161d411fa24313`. See root LICENSE and NOTICE.
The Go module is `news_panel`; `cmd/fusion` and `FUSION_*` reader settings remain
compatible. Go 1.26 and Node 24+ are required for source builds.

## Build and run

From the repository root: `./scripts.sh build` (Go + pnpm 10), or
`docker build -t news-panel .`. The multi-stage Dockerfile bundles the React 19
SPA into a CGO-disabled Go binary; the final Alpine container runs as non-root.
Only `/data` needs persistent storage. A new Docker named volume is initialized
with the runtime user's permissions.

```sh
docker run --rm -p 8080:8080 -v news-panel-data:/data \
  -e FUSION_PASSWORD=choose-a-reader-password \
  -e INGEST_TOKEN=choose-a-separate-plugin-token news-panel
```

`INGEST_TOKEN` protects plugin writes, independently of reader sessions. An
unset/empty token always returns 401 on plugin writes, even if reader auth has
been explicitly disabled using `FUSION_ALLOW_EMPTY_PASSWORD=true`.

## v0.1 HTTP contract

The authoritative schema is `../docs/PLUGIN_SPEC.md`. Both write endpoints
require `Authorization: Bearer <INGEST_TOKEN>` and accept JSON bodies up to
2 MiB. Unknown keys (including incorrect casing), missing top-level fields,
invalid JSON, and batches outside 1..100 return 400; larger bodies return 413.
Individual invalid items are counted as rejected in a 200 response. Valid
replays count as deduplicated; new stored items count as accepted. Errors are
item-level `{external_id, reason}` records; `errors` is always an array.

- `POST /api/v1/ingest`: standard source envelope and items from PLUGIN_SPEC.
- `POST /api/v1/heartbeat`: source/version/status/count/UTC timestamp envelope;
  upserts the latest received heartbeat for each source. `error_message` may
  be absent or null. The server does not echo supplied credentials into errors.
- `GET /api/items` (also `/api/v1/items`) and `GET /api/items/:id` (also
  `/api/v1/items/:id`): original reader session auth and response envelopes.
  Lists support `triage_status`, `domain`, `priority`, `min_score` (0..100),
  existing feed/group/unread filters and cursor pagination. Totals use the same
  filters and exclude the page cursor. `min_score` excludes unscored items.
- `GET /api/v1/plugins/heartbeats`: reader auth; `{data: [...], total: N}` with
  schema fields and server `updated_at` in UTC. Ingest tokens grant no reader
  access. Reader auth follows Fusion's existing password/OIDC configuration.
- Bookmark reads preserve content snapshots and expose live linked `triage`
  metadata; the same explicit triage filters run before bookmark pagination.
  Orphaned snapshots have `triage: null`, and remain available without filters.

## Storage and processing boundary

Migration 006 adds plugin/triage columns and heartbeats without changing Fusion's
actual RSS `UNIQUE(feed_id, guid)` index. The referenced v1.3.0 schema has no
`deleted_at` column. The new unique index covers non-empty `(source_id,
external_id)` only, so RSS rows with empty plugin keys coexist normally.

Each HTTP source has a suspended feed using a private `urn:news_panel:source:`
URI. This reuses existing feed/list/bookmark/read-state associations without
putting plugin sources into the RSS polling loop. Incoming author, source type,
and raw metadata are retained. Source content is rendered as text; legacy RSS
HTML continues through Fusion's sanitizer. Existing and new RSS items are kept
and remain visible in the frontend's default kept view.

`internal/triage.TriageEvaluator` accepts a normalized `triage.Input` and returns
`triage.Result`. `handler.New(store, config, puller)` installs `PassThrough`;
P1-03 may pass its evaluator as the fourth argument. No `internal/ai` package is
created. The stub keeps items with domain `other`, priority `normal`, null score,
empty tags and summary. A failed/invalid evaluator result is stored as `failed`,
and still counts as accepted because the item was saved successfully.

Storage commits pending state before evaluation, then writes the result. The full
UTC publication timestamp is retained separately from legacy second-based
`pub_date`, so a retry preserves fractional precision.
A replay of a pending row resumes evaluation using the original stored envelope;
completed duplicates do not invoke the evaluator. This recovers request or
process interruptions when the source retries. There is no background replay
queue in P1-01; P1-03 should bound LLM timeouts and may add pending-row recovery
on startup if needed. Concurrent retries can evaluate the same pending item;
storage remains unique and evaluation must tolerate at-least-once delivery.

The frontend displays metadata in the existing list/detail components. URL query
parameters mirror the read filters, e.g. `/unread?domain=ai&priority=high&min_score=80`.
Default item lists show `kept`; explicit statuses expose audit rows. Bookmark
views preserve saved snapshots by default and apply explicitly selected filters.
P1-04's `components/panel/` directory is untouched.
