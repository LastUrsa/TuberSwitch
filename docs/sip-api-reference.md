# TuberSwitch SIP API Reference

TuberSwitch exposes SIP v1 for local Starsong module integration. This API is intended for same-machine tools such as LivePanel, not public network use.

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
- No authentication in SIP v1
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
- Mutating SIP endpoints are limited to activating an existing TuberSwitch profile by name and enabling or disabling existing redeems.
- The SIP listener enforces a small request-body limit.
- Responses include `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`, `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'; base-uri 'none'`, `Referrer-Policy: no-referrer`, and `X-Frame-Options: DENY`.

SIP v1 does not define authentication. If future endpoints expose higher-impact actions or data, add a fresh security review before implementation.

## Endpoints

### GET `/api/v1/app`

Returns application identity and runtime mode.

Response:

```json
{
  "appId": "tuberswitch",
  "appName": "TuberSwitch",
  "name": "TuberSwitch",
  "version": "0.6.0",
  "mode": "standalone",
  "protocolVersion": 1,
  "capabilities": [
    "profiles",
    "redeems"
  ]
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

Returns SIP capability names and legacy feature flags.

Response:

```json
{
  "protocolVersion": 1,
  "capabilities": [
    "profiles",
    "redeems"
  ],
  "supportsProfiles": true,
  "supportsStatusReporting": true,
  "supportsRedeems": true
}
```

Consumers should prefer `capabilities` for feature discovery and ignore unknown future capability names or flags.

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
  "activeProfileName": "Default",
  "mode": "png",
  "activeMode": "png",
  "obsSummary": "Connected: Gaming / PNG",
  "obsConnected": true,
  "activeScene": "Gaming",
  "activeSource": "PNG",
  "redeemsEnabled": true,
  "redeemCount": 2,
  "manageableRedeemCount": 1,
  "unmanageableRedeemCount": 1,
  "appDetectionStatus": "PNG app detected",
  "appDetectionEnabled": true,
  "currentModeLabel": "PNGTuber Mode",
  "activeProfileLastUsed": "2026-06-10T12:00:00Z"
}
```

Known states include:

- `ready`: TuberSwitch is ready and profile state is available
- `error`: TuberSwitch could not report profile state

`mode` is `png`, `3d`, or `unknown`. `activeMode` is retained for compatibility and is `png` or `3d` when an active profile is available.

Additional status fields are additive and may be omitted when details are unavailable:

- `obsSummary`: Short OBS connection/configuration summary for compact UI display
- `obsConnected`: Whether TuberSwitch is currently connected to OBS
- `activeScene`: Primary enabled scene mapping for the active profile or current config
- `activeSource`: Primary source for the current mode in `activeScene`
- `redeemsEnabled`: Whether Twitch reward switching is configured with an access token and at least one manageable reward
- `redeemCount`: Total number of configured reward mappings for the active profile or current config
- `manageableRedeemCount`: Number of configured reward mappings TuberSwitch can edit through Twitch
- `unmanageableRedeemCount`: Number of configured reward mappings TuberSwitch can see but cannot edit through Twitch
- `appDetectionStatus`: Human-readable app detection state
- `appDetectionEnabled`: Whether app detection is enabled in TuberSwitch
- `currentModeLabel`: Display label for the current mode
- `activeProfileLastUsed`: Timestamp stored on the active profile, when available

### GET `/api/v1/redeems`

Returns known redeems and their current enabled and operational availability state for the active profile.

Response:

```json
{
  "redeems": [
    {
      "id": "headpat",
      "name": "Headpat",
      "available": true,
      "enabled": true
    },
    {
      "id": "hydrate",
      "name": "Hydrate",
      "available": false,
      "enabled": false
    }
  ]
}
```

`enabled` is user intent and can be modified through SIP. `available` is read-only operational readiness determined by TuberSwitch. Redeems remain visible when unavailable so clients can show diagnostics instead of hiding configured redeems.

### POST `/api/v1/redeems`

Enables or disables existing redeems. TuberSwitch remains responsible for redeem creation, deletion, validation, and Twitch ownership rules.

Request:

```json
{
  "redeems": [
    {
      "id": "headpat",
      "enabled": false
    }
  ]
}
```

Response:

```json
{
  "success": true
}
```

Redeem updates persist to the active profile. Unknown redeem IDs return `RedeemNotFound`. Enabling a read-only or unmanageable redeem returns `InvalidRequest`.

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

- `400 Bad Request`: invalid JSON, unknown request fields, empty profile name, empty redeem IDs, or disallowed redeem updates
- `403 Forbidden`: non-localhost host header
- `404 Not Found`: requested profile or redeem does not exist
- `405 Method Not Allowed`: endpoint does not support the requested HTTP method
- `413 Payload Too Large`: request body exceeds the SIP body limit
- `415 Unsupported Media Type`: missing or non-JSON `Content-Type` for profile activation or redeem updates
