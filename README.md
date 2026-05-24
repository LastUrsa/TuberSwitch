# TuberSwitch

TuberSwitch is a Windows desktop app for VTubers who switch between a 3D VTuber source and a PNG Tuber source in OBS, while also keeping Twitch Channel Point rewards in sync with the active mode.

It is built with Wails, Go, and React.

## What It Does

TuberSwitch coordinates two systems from one mode switch:

- OBS source visibility through OBS WebSocket v5
- Twitch Channel Point reward availability through the Twitch Helix API

When you switch modes, the app can:

- show the configured 3D source and hide the configured PNG source
- or hide the configured 3D source and show the configured PNG source
- enable Twitch rewards marked as `3D Only` in 3D mode
- disable those same rewards in PNG mode

Unchecked rewards are left alone.

## Current Feature Set

- Compact desktop UI with a single primary mode switch
- OBS and Twitch connection status indicators in the app header
- Resizable settings experience that expands the native window when settings open
- Tabbed settings layout:
  - `General`
  - `OBS Settings`
  - `Twitch Settings`
- Dark and light themes with persisted theme selection
- Built-in update check against GitHub Releases
- OBS scene and source sync
- Scene-by-scene source mapping for both 3D and PNG modes
- Per-scene enable/disable control
- Optional filter to show only selected scenes
- Startup mode options:
  - restore last mode
  - always 3D
  - always PNG
- Twitch Device Code login flow
- Reward refresh from Twitch
- Create new rewards under the connected Twitch app
- Separate manageable and unmanageable rewards
- Mark manageable rewards as `3D Only`
- Secure OBS password storage in the OS credential store
- Secure Twitch token storage in the OS credential store
- Local config and log files
- Windows app icon / packaged desktop executable

## Requirements

### Runtime

- Windows
- Microsoft WebView2 Runtime
- OBS Studio with OBS WebSocket v5 enabled

### Development

- Go `1.25+`
- Node.js with npm
- Wails CLI `v2.12.0` or compatible `v2`

Install Wails if needed:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor
```

If a fresh terminal does not see `go`, `node`, or `wails`, make sure these are on `PATH`:

```text
C:\Program Files\Go\bin
C:\Program Files\nodejs
%USERPROFILE%\go\bin
```

## Development

Install frontend dependencies:

```powershell
cd frontend
npm install
```

Run the app in development mode:

```powershell
wails dev
```

Build the Windows desktop app:

```powershell
wails build
```

The packaged executable is written to:

```text
build\bin\TuberSwitch.exe
```

Useful validation commands:

```powershell
go test ./...
cd frontend
npm test -- --run
npm run build
```

## OBS Setup

OBS Studio 28+ includes OBS WebSocket v5 by default.

In OBS:

1. Open `Tools > WebSocket Server Settings`.
2. Enable the WebSocket server.
3. Keep the default port `4455` unless you need another port.
4. Set or copy the WebSocket password.
5. Make sure each target scene contains the sources you want to switch between.

In TuberSwitch:

1. Open `Settings`.
2. Go to `OBS Settings`.
3. Enter the OBS host, port, and password.
4. Click `Save`.
5. Click `Sync Scenes & Sources`.
6. For each scene you want managed, choose:
   - `VTuber Source`
   - `PNG Tuber Source`
7. Enable the scenes you want TuberSwitch to control.

Notes:

- The OBS password is stored in the OS credential store, not in the JSON config file.
- `Sync Scenes & Sources` saves settings first, so validation still runs before OBS sync.

## Twitch Setup

Create an application in the Twitch Developer Console.

Recommended desktop setup:

- public client
- Device Code Grant flow
- scopes:

```text
channel:read:redemptions
channel:manage:redemptions
```

If Twitch requires a redirect URI during app registration, use a placeholder such as:

```text
https://localhost
```

TuberSwitch does not rely on a local redirect listener for login. It uses the Twitch device activation flow.

In TuberSwitch:

1. Open `Settings`.
2. Go to `Twitch Settings`.
3. Enter the Twitch Client ID.
4. Click `Save`.
5. Click `Login with Twitch`.
6. Complete the device activation flow in the browser.
7. Return to the app and click `Refresh Rewards` if needed.

Notes:

- The Twitch Client ID is shown as a masked field in the UI, but it is not a secret in the same sense as a client secret.
- Twitch access and refresh tokens are stored in the OS credential store.

## Reward Management

Twitch only allows an application to update custom Channel Point rewards that were created by that same Twitch application/client ID.

That means:

- rewards created manually in the Twitch dashboard may be visible
- rewards created by another tool may be visible
- but those rewards may not be manageable by this app

TuberSwitch handles this by splitting rewards into:

- `Manageable Rewards`
- `Unmanageable Rewards`

Use `Create Reward` inside the app if you want a reward that TuberSwitch can reliably enable and disable later.

## General Settings

The `General` tab currently includes:

- theme selection
- app version display
- update check against GitHub Releases

If an update is available, the app can open the GitHub Releases page directly.

## First-Time Workflow

1. Launch TuberSwitch.
2. Open `Settings > OBS Settings`.
3. Save OBS connection details.
4. Sync scenes and sources.
5. Configure the scenes you want to manage.
6. Open `Settings > Twitch Settings`.
7. Save the Twitch Client ID.
8. Login with Twitch.
9. Refresh rewards.
10. Mark any rewards that should be `3D Only`.
11. Use the main toggle to switch between modes.

Expected behavior:

- 3D mode shows the VTuber source, hides the PNG source, and enables checked `3D Only` rewards
- PNG mode hides the VTuber source, shows the PNG source, and disables checked `3D Only` rewards
- rewards not marked `3D Only` are not changed

## Local Files

Runtime files are stored outside the repo in the user config directory:

```text
%APPDATA%\TuberSwitch\config.json
%APPDATA%\TuberSwitch\tuberswitch.log
```

Secrets are not stored in `config.json`. Sensitive values are stored in the OS credential store.

## Troubleshooting

### OBS not connecting

- Confirm OBS is running.
- Confirm OBS WebSocket is enabled.
- Confirm host, port, and password.
- Confirm firewall rules allow the connection you are attempting.
- Re-sync scenes and sources after major OBS scene/source changes.

### Scene mapping issues

- Click `Sync Scenes & Sources`.
- Confirm the selected scene contains the selected sources.
- OBS source names must match exactly.
- If a scene is not enabled, TuberSwitch will not manage it.

### Twitch login fails

- Confirm the Client ID.
- Confirm the Twitch app is set up for the Device Code flow you intend to use.
- If the device code expires, start login again.

### Rewards do not update

- Confirm the authenticated Twitch user owns the channel.
- Confirm the app has `channel:manage:redemptions`.
- Confirm the reward was created by this Twitch app/client ID if you expect it to be manageable.
- Some rewards may remain visible but read-only.

### Tokens or saved credentials seem stale

- Re-run Twitch login if token refresh fails.
- Re-enter the OBS password if the saved credential is no longer valid.

## Testing

Current automated coverage includes:

- Go unit tests for app logic, config handling, OBS/Twitch behavior, secret handling, and update-check behavior
- Frontend tests for mode switching, settings flows, reward management, validation handling, and update-check UI behavior

Run them with:

```powershell
go test ./...
cd frontend
npm test -- --run
```
