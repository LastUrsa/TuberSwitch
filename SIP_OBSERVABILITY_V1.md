# SIP Observability v1 (additive protocol-v1 extension)

All apps retain existing endpoints, fields, synchronous behavior, loopback binding,
Host validation and no-store/security headers. Browser Origin/cross-site requests are rejected before any operation history is created. New capabilities are opt-in.

## Common fields and headers

- `X-Request-ID`: optional 1..64 ASCII letters/digits/`._:-`; invalid values return
  400. The server generates a random ID when absent, echoes it in the header and
  JSON `requestId`, and includes it in safe operational logs.
- Every JSON response adds `sessionId` (server launch ID), `requestId`, and
  `observedAt` (UTC RFC3339). `updatedAt` denotes actual snapshot change.
- Actions add `operationId` and `operation`. A safe operation has `id`, `kind`
  (route), `requestId`, `sessionId`, `startedAt`, `updatedAt`, optional
  `completedAt`, `phase`, `outcome`, `errorCode`, `message`, `retryable`, optional
  `retryAfterSeconds`, and bounded `results`. Outcomes: `running`, `success`,
  `partial`, `failed`, `canceled`, `unknown`, `confirmation_required`.
- `errorCode` uses stable uppercase codes. `message` is a fixed, explicitly safe
  explanation. Legacy `error` remains display-only and must not be persisted.
- Capability `operations` enables `GET /api/v1/operations/{id}` returning
  `{operation: {...}}`. Unknown/expired IDs return 404 `OPERATION_NOT_FOUND`.
- Capability `diagnostics` enables `GET /api/v1/diagnostics`: safe app/build/
  protocol version, server session ID, start time, uptime, and owner-specific
  dependency observations. No secrets, content, private paths or arbitrary reads.
- A successful applied change exposes `confirmedAt`, `confirmedOperationId`, and
  monotonic `revision` on the corresponding current-state response. An accepted
  request is not a confirmation of applied state.

## Safe error codes

`INVALID_REQUEST`, `FORBIDDEN`, `NOT_FOUND`, `OPERATION_NOT_FOUND`, `BUSY`,
`INTERNAL_ERROR`, `UNAVAILABLE`, `TIMEOUT`, `CANCELED`, `AUTH_REJECTED`,
`MISSING_CREDENTIALS`, `VALIDATION_FAILED`, `INTEGRATION_DISABLED`,
`PUBLISH_FAILED`, `FOLLOW_UP_FAILED`, `PARTIAL_FAILURE`,
`DUPLICATE_CONFIRMATION_REQUIRED`, `CONFIRMATION_EXPIRED`,
`IDEMPOTENCY_CONFLICT`, `HISTORY_UNAVAILABLE`, `HISTORY_WRITE_FAILED`, `INTERRUPTED`.

`retryable` describes whether an action can be attempted again safely. Publishing
transport uncertainty is never automatically retryable; inspect its operation.

## StreamSignal publishing

- Capability `async-publishing`: add `Prefer: respond-async` to existing announce,
  confirm or end-stream POST routes. Return 202, `Location`, and an operation;
  execution continues independently of the client's connection under a bounded
  server lifetime. Poll the operation. Legacy callers remain synchronous.
- `Idempotency-Key` uses the same validated ID grammar (max 64). Repeated same
  key/route/payload returns the same operation without executing again. A changed
  route/payload returns 409 `IDEMPOTENCY_CONFLICT`. Persist a hash of the key and
  request fingerprint, never request content. History is bounded to 256 recent
  operations/14 days; keys cannot guarantee deduplication after expiry.
- Persist safe operations atomically before executing; refuse publishing if this
  write fails. After restart, unfinished persisted work is `unknown/INTERRUPTED`
  and is not automatically replayed.
- Destination results: `destinationId`, `platform`, `state`, `phase`, `errorCode`,
  `message`, `retryable`, `published` (true/false/null for unknown),
  `startedAt`, `completedAt`, and bounded `stages` for publication, Live Now and
  history work. Exclude rendered content, destination names and receipt URLs.
- Credential readiness is non-posting, safe state only: configured/missing/
  rejected/unknown; local configuration does not prove remote authentication.

## App-specific observations

TuberSwitch exposes actual OBS observation/connection transition times, a safe
last disconnect code, next retry time, and per-integration profile outcomes plus
requested/confirmed active profile IDs. Streaming activity is independent of OBS
WebSocket connection state.

TideReader exposes separately observed overlay and provider/browser health with
safe reasons, last update/success timestamps and stale state. Observations must
come from the actual producer lifecycle, not from the time a client polls.
