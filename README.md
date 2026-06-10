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

- Windows
- Microsoft WebView2 Runtime
- OBS Studio with OBS WebSocket v5 enabled

## Quick Start

1. Open `Settings > Connections`.
2. Enter the OBS host, port, and password.
3. Enter the Twitch Client ID.
4. Click `Save connection settings`, then `Login with Twitch`.
5. Complete device activation in the browser.
6. Click `Refresh Rewards`.
7. Open `Settings > Profiles`.
8. Choose the profile mode.
9. Click `Sync Scenes & Sources`.
10. For each OBS scene you want managed, choose the `Desired Source`.
11. Check `Enabled` for rewards that should be available in that profile.
12. Click `Save profile`.

Optional:

13. Open `Settings > General`.
14. Enable `App Detection`.
15. Configure at least one process name.
16. Save the settings.

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

When a configured app is detected, TuberSwitch applies the most recently used profile with the matching mode. For example, a PNG mode process applies a PNGTuber profile, while a 3D mode process applies a 3D profile. If no profile exists for the detected mode, TuberSwitch falls back to switching the mode directly.

## Service Mode

TuberSwitch can run as a managed Starsong module for LivePanel-style orchestration:

```powershell
TuberSwitch.exe --service
TuberSwitch.exe --show
```

`--service` starts TuberSwitch hidden with app detection and SIP active. `--show` restores the existing UI, or starts normally if no instance is running. Only one TuberSwitch instance runs at a time.

SIP v1.1 is exposed on localhost only across ports `47040-47049`:

- `GET /api/v1/app`
- `GET /api/v1/health`
- `GET /api/v1/capabilities`
- `GET /api/v1/status`
- `GET /api/v1/profiles`
- `GET /api/v1/profile/current`
- `POST /api/v1/profile`

Profiles are the SIP control surface. Status includes compact OBS, redeem, and app detection summaries for local dashboards, but SIP does not expose configuration APIs for OBS scenes, reward definitions, app detection rules, or profile CRUD.

See [docs/sip-api-reference.md](docs/sip-api-reference.md) for the full SIP contract. A Postman collection is available at [docs/postman/TuberSwitch-SIP-v1.postman_collection.json](docs/postman/TuberSwitch-SIP-v1.postman_collection.json).

## Development

Development requirements:

- Go `1.25+`
- Node.js with npm
- Wails CLI `v2.12.0` or compatible `v2`

```powershell
cd frontend
npm install
cd ..
wails dev
```

Build output is written to `build\bin\TuberSwitch.exe`.

## Quality Gates

Run the local release checks before tagging. Add release notes in `RELEASE_NOTES.md` and use them for the GitHub release.

```powershell
cd frontend
npm run test:coverage
npm run build
npm audit
cd ..
go test ./... -cover
govulncheck ./...
wails build
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
