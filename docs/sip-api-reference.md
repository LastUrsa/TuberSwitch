# TuberSwitch SIP API Reference

TuberSwitch exposes SIP v1.1 for local Starsong module integration. This API is intended for same-machine tools such as LivePanel, not public network use.

## Runtime

TuberSwitch starts a dedicated SIP HTTP listener on `127.0.0.1` using the reserved TuberSwitch port range:

```text
47040-47049
```

The first available port is used. In a typical local run, the base URL is:

```text
http://127.0.0.1:47040
```

Supported launch modes:

```text
TuberSwitch.exe
TuberSwitch.exe --service
TuberSwitch.exe --show
```

`--service` starts TuberSwitch without showing the main window and reports SIP mode `service`. A normal launch reports `standalone`. `--show` asks an existing instance to restore its UI, or launches standalone if no instance is running.

## Transport Rules

- JSON request and response bodies
- Localhost only
- No authentication in SIP v1.1
- No generic command or action endpoints
- `POST /api/v1/profile` requires `Content-Type: application/json`
- Unknown request fields are rejected
- SIP request bodies are limited to 4096 bytes
- Responses include no-store and browser hardening headers

## Security Posture

SIP is a local integration API. It is intentionally narrow and does not expose profile CRUD, OBS scene configuration APIs, reward definition APIs, app detection rules, credentials, filesystem paths, or generic command execution.

Current safeguards:

- SIP binds to `127.0.0.1`, not a public interface.
- Requests with non-localhost `Host` headers are rejected.
- CORS is not enabled for the SIP listener.
- JSON `POST` requests reject unknown fields.
- The only mutating SIP endpoint activates an existing TuberSwitch profile by name.
- The SIP listener enforces a small request-body limit.
- Responses include `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`, `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'; base-uri 'none'`, `Referrer-Policy: no-referrer`, and `X-Frame-Options: DENY`.

SIP v1.1 does not define authentication. If future endpoints expose higher-impact actions or data, add a fresh security review before implementation.

## Endpoints

### GET `/api/v1/app`

Returns application identity and runtime mode.

Response:

```json
{
  "appId": "tuberswitch",
  "name": "TuberSwitch",
  "version": "0.5.0",
  "mode": "standalone",
  "protocolVersion": "1.1"
}
```

`mode` is either `standalone` or `service`.

### GET `/api/v1/health`

Returns SIP readiness and application health.

Response:

```json
{
  "status": "ready",
  "message": "TuberSwitch operational"
}
```

If profiles are unavailable, health reports `error` with a short message.

### GET `/api/v1/capabilities`

Returns SIP feature flags.

Response:

```json
{
  "supportsProfiles": true,
  "supportsStatusReporting": true
}
```

Consumers should ignore unknown future capability flags.

### GET `/api/v1/status`

Returns lightweight runtime status and the active profile. This endpoint does not expose full application settings.

Response:

```json
{
  "state": "ready",
  "message": "Profile active",
  "healthy": true,
  "activeProfile": "Default",
  "activeProfileId": "default",
  "activeMode": "png"
}
```

Known states include:

- `ready`: TuberSwitch is ready and profile state is available
- `error`: TuberSwitch could not report profile state

`activeMode` is `png` or `3d` when an active profile is available.

### GET `/api/v1/profiles`

Returns available TuberSwitch profile names.

Response:

```json
{
  "profiles": [
    "Default",
    "Ursa PNGTuber"
  ]
}
```

Only profile names are returned. Profile settings and CRUD operations remain TuberSwitch UI responsibilities.

### GET `/api/v1/profile/current`

Returns the active TuberSwitch profile.

Response:

```json
{
  "id": "default",
  "name": "Default"
}
```

Empty state:

```json
{
  "id": "",
  "name": ""
}
```

### POST `/api/v1/profile`

Activates an existing TuberSwitch profile by name.

Request:

```json
{
  "profile": "Ursa PNGTuber"
}
```

Response:

```json
{
  "success": true,
  "profile": "Ursa PNGTuber",
  "profileId": "ursa-pngtuber"
}
```

Profile names are matched case-insensitively. Activation uses TuberSwitch's existing profile path, so the active mode, OBS scene/source choices, and reward enablement update as if the profile were selected through the UI.

## Errors

Standard error response:

```json
{
  "success": false,
  "error": "ProfileNotFound"
}
```

Common statuses:

- `400 Bad Request`: invalid JSON, unknown request fields, or empty profile name
- `403 Forbidden`: non-localhost host header
- `404 Not Found`: requested profile does not exist
- `405 Method Not Allowed`: endpoint does not support the requested HTTP method
- `413 Payload Too Large`: request body exceeds the SIP body limit
- `415 Unsupported Media Type`: missing or non-JSON `Content-Type` for profile activation
