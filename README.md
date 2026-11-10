# TuberSwitch

TuberSwitch is a Windows desktop app for switching OBS scenes and Twitch Channel Point rewards between 3D VTuber and PNGTuber stream profiles.

Built with Wails, Go, and React.

## Features

- Switch between 3D and PNGTuber profiles from a compact desktop control.
- Save reusable profiles with their own mode, OBS scene source choices, and reward enablement.
- Sync OBS scenes through OBS WebSocket v5.
- Manage Twitch Channel Point reward availability through the Twitch Helix API.
- Optionally auto-switch profiles when configured avatar apps are detected.
- Store OBS passwords and Twitch tokens in the OS credential store.

## Requirements

- Windows x64
- Microsoft WebView2 Runtime
- OBS Studio with OBS WebSocket v5 enabled

## Quick Start

1. Open `Settings > Connections`, enter OBS and Twitch details, then save.
2. Click `Login with Twitch`, complete device activation, and refresh rewards.
3. Open `Settings > Profiles` and choose the profile mode.
4. Click `Sync Scenes & Sources`.
5. Choose each managed scene's `Desired Source`.
6. Check `Enabled` for rewards this profile should manage during mode changes.
7. Click `Save profile`.

Optional:

8. Open `Settings > General`.
9. Enable `App Detection`.
10. Configure at least one process name.
11. Assign optional 3D/PNG target profiles after checking their saved modes, then save the settings.

## App Detection

App Detection is optional and disabled by default.

When enabled, TuberSwitch checks for up to two process names:

- `3D Mode Process`
- `PNG Mode Process`

You can configure a process name by:

- typing it manually
- using `Select Running App`
- using `Browse Executable`

TuberSwitch stores only the executable filename, such as `AvatarApp.exe`. Matching is case-insensitive.

In Settings → General → App Detection, choose a **3D Detection Profile** and a **PNG Detection Profile** to assign a saved profile to each detected app. For example, assign VNyan to your 3D avatar profile and veadotube-mini to your PNG profile. The target profile must have the matching saved mode; profile names do not determine mode. Correct a profile's mode in Settings → Profiles and save the profile before assigning it as a target.

With an explicit target, detection checks both the active profile ID and mode. It activates the target even when Default already uses the same mode, and does not repeatedly apply an already active target. Missing/deleted or mismatched targets block switching and show an explanation; they never silently fall back to Default. Deleting a target preserves its reference until you choose a replacement or clear the selector.

Leave a selector at **Automatic (existing mode selection)** to retain the previous behavior for that mode: when the mode changes, apply the most recently used matching profile, or switch mode directly if none exists. An already active mode remains unchanged. Existing installations keep this behavior until explicit targets are selected.

The detection status explains no matching app, a conflict, manual override cooldown, invalid targets, or integration failure. The existing both-app conflict preference and manual cooldown still apply. An integration failure remains visible while the selected target is active; polling does not repeatedly reapply its integrations. Check the integrations and manually reapply the profile to retry. Logs record decision transitions with hashed target references and safe reason/error codes. SIP exposes safe producer observations through `appDetection` in status and `dependencies.appDetection` in diagnostics; it excludes process inventories and raw errors.

## Service Mode

TuberSwitch can run as a managed Starsong module for LivePanel-style orchestration:

```powershell
TuberSwitch.exe --service
TuberSwitch.exe --show
```

`--service` starts TuberSwitch hidden with app detection and SIP active. `--show` restores the existing UI, or starts normally if no instance is running. Only one TuberSwitch instance runs at a time.

SIP v1 is exposed on localhost only across ports `47040-47049`:

- `GET /api/v1/app`
- `GET /api/v1/health`
- `GET /api/v1/capabilities`
- `GET /api/v1/status`
- `GET /api/v1/diagnostics`
- `GET /api/v1/operations/{id}`
- `GET /api/v1/profiles`
- `GET /api/v1/profile/current`
- `POST /api/v1/profile`
- `GET /api/v1/redeems`
- `POST /api/v1/redeems`
- `POST /api/v1/redeems/manual`

Profiles and existing redeem state are the SIP control surface. Status includes compact OBS, redeem, and app detection summaries for local dashboards, but SIP does not expose configuration APIs for OBS scenes, reward definitions, app detection rules, or profile CRUD.

Starting in v1.0.1, for local OBS on Windows, TuberSwitch checks executable names every three seconds while OBS is closed and pauses WebSocket connection attempts until OBS starts. It reports `not_running` / `OBS not running` during this wait. When OBS is running but unavailable, TuberSwitch reconnects automatically with bounded backoff. Remote OBS hosts, unsupported platforms, and process-detection errors retain bounded-backoff retries. SIP status continues to expose `obsConnected` and also reports a non-sensitive `obsConnectionState` (`not_configured`, `not_running`, `disconnected`, `reconnecting`, or `connected`).

Profile activation reconciles both the outgoing and incoming profiles. OBS sources used only by the outgoing profile are hidden, incoming VTuber/PNGTuber visibility is applied, and every affected manageable Twitch reward is enabled or disabled according to the incoming profile. Unmanageable rewards are left unchanged. Activation errors identify failed OBS sources or Twitch rewards so callers can surface partial reconciliation failures.

See [docs/sip-api-reference.md](docs/sip-api-reference.md) for the full SIP contract. A Postman collection is available at [docs/postman/TuberSwitch-SIP-v1.postman_collection.json](docs/postman/TuberSwitch-SIP-v1.postman_collection.json).

## Development

Development requirements:

- Go `1.26.10+` (the pinned toolchain includes required standard-library security fixes)
- Node.js 24 with npm
- Wails CLI `v2.13.0` or compatible `v2`

```powershell
cd frontend
npm install
cd ..
wails dev
```

Build output is written to `build\bin\TuberSwitch.exe`.

## Quality Gates

Run the local release checks before tagging. Add a `## vX.Y.Z` section to `RELEASE_NOTES.md`; the release workflow publishes that section as the GitHub Release body and fails if it is missing or empty.

```powershell
cd frontend
npm run test:coverage
npm run build
npm audit
cd ..
go test ./... -cover
govulncheck ./...
wails build
./scripts/test-sip-newman.sh --skip-build
```

## Local Files

```text
%APPDATA%\TuberSwitch\config.json
%APPDATA%\TuberSwitch\tuberswitch.log
```

Secrets are not stored in `config.json`. Sensitive values are stored in the OS credential store.

## Troubleshooting

- OBS issues: confirm OBS is running, OBS WebSocket is enabled, and the host/port/password are correct.
- Scene issues: re-run `Sync Scenes & Sources` and confirm the selected scene contains the selected sources.
- Twitch issues: confirm the Client ID and retry the device login flow.
- Reward issues: confirm the app has `channel:manage:redemptions` and that manageable rewards were created by this Twitch app/client ID.
- App Detection issues: launch the target app first, then retry `Select Running App`, relax picker filters, or use `Browse Executable`.


## SIP Troubleshooting and Operation Metadata

Adds capability-advertised SIP operations/diagnostics and request correlation. Profile activation returns requested/actual profile IDs and safe results for OBS source changes, redeem changes, and profile storage, including partial application. Avatar source switching is owned by OBS; no independent avatar connection is invented. Status reports producer-observed OBS timestamps, last disconnect code and next scheduled retry, independently of streaming activity. Successful SIP profile changes expose operation confirmation IDs/times and session-scoped revisions. Recent safe operations are retained beside configuration in `sip-operations.json` (up to 256 operations/14 days). Existing fields, local-host protections and synchronous clients remain supported.

See [SIP Observability v1](./SIP_OBSERVABILITY_V1.md) for the common headers, operation/error schema, retry rules, capabilities, and endpoint examples. Legacy `error` fields remain display-only; clients should log the fixed `message`/`errorCode` and operational IDs instead.

Build and validate with Go 1.26.10 or newer. Frontend build/test dependencies and Wails have been updated for security advisories.

See [1.0 release validation](RELEASE_VALIDATION.md) for the verified checks, packaging gates, and manual-validation boundaries.
