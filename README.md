# TuberSwitch

TuberSwitch is a Windows desktop app for switching between a 3D VTuber source and a PNG Tuber source in OBS while keeping Twitch Channel Point rewards in sync with the active mode.

Built with Wails, Go, and React.

## What It Does

TuberSwitch coordinates:

- OBS source visibility through OBS WebSocket v5
- Twitch Channel Point reward availability through the Twitch Helix API

Main capabilities:

- one-click switching between 3D and PNG modes
- per-scene OBS source mappings
- Twitch reward management with `3D Only` rewards
- optional App Detection based on running process names
- secure storage for the OBS password and Twitch tokens

## Requirements

Runtime:

- Windows
- Microsoft WebView2 Runtime
- OBS Studio with OBS WebSocket v5 enabled

Development:

- Go `1.25+`
- Node.js with npm
- Wails CLI `v2.12.0` or compatible `v2`

## Quick Start

1. Open `Settings > OBS Settings`.
2. Enter the OBS host, port, and password.
3. Click `Save`, then `Sync Scenes & Sources`.
4. Configure the `VTuber Source` and `PNG Tuber Source` for the scenes you want managed.
5. Open `Settings > Twitch Settings`.
6. Enter the Twitch Client ID.
7. Click `Save`, then `Login with Twitch`.
8. Complete device activation in the browser.
9. Click `Refresh Rewards`.
10. Mark any rewards that should be `3D Only`.

Optional:

11. Open `Settings > General`.
12. Enable `App Detection`.
13. Configure at least one process name.
14. Save the settings.

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

The running-app picker filters aggressively by default to reduce noise, including likely-avatar-only filtering, visible-window filtering, and hiding common system/helper processes. If the app you want is missing, disable filters and refresh the list.

## Development

Core desktop app code is split by responsibility:

- `app.go` wires the app, shared interfaces, and lifecycle hooks
- `app_runtime.go` contains runtime state and mode-switch orchestration
- `app_integrations.go` contains OBS, Twitch, update, and secret-management integrations

Install frontend dependencies:

```powershell
cd frontend
npm install
```

Run in development mode:

```powershell
wails dev
```

Build the app:

```powershell
wails build
```

Output:

```text
build\bin\TuberSwitch.exe
```

## Testing

Before pushing, run the same checks CI runs:

```powershell
.\scripts\ci-check.ps1
```

Equivalent manual commands:

```powershell
cd frontend
npm ci
npm run test:coverage
npm run build
cd ..
go test ./... -cover
```

Security pass commands:

```powershell
cd frontend
npm audit
cd ..
govulncheck ./...
```

If `govulncheck` reports standard-library vulnerabilities, upgrade to a patched Go toolchain before shipping. As of June 2026, the fixes for the current reachable findings are in Go `1.25.11+` and Go `1.26.4+`.

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
