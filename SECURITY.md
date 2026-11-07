# Security


## SIP Observability

SIP remains loopback-only with existing Host validation, request limits, cache/security headers and JSON validation. New IDs are bounded and validated before echoing/logging. Safe operation snapshots omit request payloads, credential material, raw upstream errors, rendered content and receipt URLs; legacy display fields are never copied into diagnostic messages. Operation history is bounded and created with private permissions where supported. Diagnostic IDs and operational timing can still identify a streaming session; review them before sharing. No endpoint reads arbitrary files or performs a real post to check credential readiness.
