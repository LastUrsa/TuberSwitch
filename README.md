# TuberSwitch

TuberSwitch is a Windows desktop app for VTubers who switch between a 3D VTuber source and a PNG Tuber source.

The app controls two things from one toggle:

- OBS source visibility through OBS WebSocket v5.
- Twitch Channel Point reward availability through the Twitch Helix API.

The MVP is a standalone Wails desktop app, not a native OBS plugin.

## Current MVP Features

- 3D/PNG mode toggle.
- OBS connection test.
- Sync scenes and sources from OBS.
- Scene/source dropdown selection.
- Stores OBS scene item IDs after selection/sync.
- OBS-only `Test 3D` and `Test PNG` buttons.
- Twitch OAuth login through Twitch Device Code Grant flow.
- Fetch custom Channel Point rewards.
- Mark rewards as `3D Only`.
- Hide reward IDs from normal UI.
- Show manageable and unmanageable rewards separately.
- Create new rewards under this Twitch app/client ID.
- Enable `3D Only` rewards in 3D mode.
- Disable `3D Only` rewards in PNG mode.
- Leave all other rewards untouched.
- Optional Twitch reward refresh on startup.
- Startup mode: restore last, always 3D, or always PNG.
- Local JSON config and local log file.

## Prerequisites

Install:

- Go
- Node.js with npm
- Wails v2 CLI
- WebView2 Runtime

On this machine, Wails Doctor reported the system ready with:

- Go 1.26.3
- Node 24.16.0
- npm 11.13.0
- Wails 2.12.0
- WebView2 installed

Install Wails if needed:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor
```

If a fresh terminal does not see `go`, `node`, or `wails`, restart PowerShell or add these to PATH:

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

Useful validation commands:

```powershell
go test ./...
cd frontend
npm run build
```

## OBS Setup

OBS Studio 28+ includes OBS WebSocket v5 by default.

In OBS:

1. Open `Tools > WebSocket Server Settings`.
2. Enable the WebSocket server.
3. Keep the default port `4455` unless you need another port.
4. Set or copy the WebSocket password.
5. Make sure the target scene contains both sources:
   - VTuber source
   - PNG Tuber source

In TuberSwitch:

1. Open `Settings`.
2. Enter OBS host, port, and password.
3. Click `Test OBS`.
4. Click `Sync Scenes & Sources`.
5. Select the scene, VTuber source, and PNG Tuber source from dropdowns.
6. Click `Save`.
7. Use `Test 3D` and `Test PNG` to verify OBS switching without touching Twitch rewards.

## Twitch App Setup

Create a Twitch application in the Twitch Developer Console.

Recommended 2026 desktop flow:

- Use Twitch Device Code Grant flow.
- Register the application as a public client if the console offers a client type.
- No redirect URI is used by TuberSwitch during login.
- No public domain is required.
- If the Developer Console requires an HTTPS redirect URI during registration, enter an unused placeholder such as `https://localhost`. TuberSwitch will not listen on that URL or use it for OAuth.

Required scopes:

```text
channel:read:redemptions
channel:manage:redemptions
```

Important Twitch limitation:

Twitch only allows an app to update custom Channel Point rewards that were created by the same Twitch application/client ID. Rewards created manually in the Twitch dashboard or by another tool may be visible to the broadcaster, but this app cannot enable or disable them through Helix.

TuberSwitch handles this by:

- Fetching all custom rewards for visibility.
- Fetching the manageable subset from Twitch.
- Showing app-created rewards in `Manageable Rewards`.
- Showing dashboard/other-app rewards in `Unmanageable Rewards` as read-only.
- Providing `Create Reward` so new rewards are created under this app's Client ID and can be toggled later.

In TuberSwitch:

1. Open `Settings`.
2. Enter the Twitch Client ID.
   The Twitch Client Secret is optional and not needed for public Device Code login.
3. Click `Save`.
4. Click `Login with Twitch`.
5. The browser opens Twitch's device activation page.
6. Approve the requested scopes.
7. Return to TuberSwitch after the app reports that Twitch is connected.

Token note: the MVP stores Twitch tokens in the local JSON config. A later hardening pass should move tokens to Windows Credential Manager.

## First-Time Workflow

1. Launch TuberSwitch.
2. Configure and test OBS.
3. Sync scenes and sources.
4. Select the scene, VTuber source, and PNG Tuber source.
5. Use `Test 3D` and `Test PNG` to confirm OBS behavior.
6. Configure Twitch Client ID.
7. Login with Twitch.
8. Click `Refresh Rewards`.
9. Create any rewards that TuberSwitch should control, or use existing manageable rewards.
10. Check `3D Only` for rewards that should only be available in 3D mode.
11. Flip the main toggle.

Expected behavior:

- 3D mode shows the VTuber source, hides the PNG Tuber source, and enables checked rewards.
- PNG mode hides the VTuber source, shows the PNG Tuber source, and disables checked rewards.
- Unchecked rewards are untouched.

## Local Files

Runtime files are stored outside the repo in the user config directory:

```text
%APPDATA%\TuberSwitch\config.json
%APPDATA%\TuberSwitch\tuberswitch.log
```

## Troubleshooting

OBS disconnected:

- Confirm OBS is running.
- Confirm OBS WebSocket is enabled.
- Confirm host, port, and password.
- Confirm firewall rules allow localhost connections.

Source not found:

- Click `Sync Scenes & Sources`.
- Confirm the selected scene contains the selected source.
- OBS source names must match exactly.

Twitch login fails:

- Confirm the Client ID.
- Confirm the app is allowed to use Device Code Grant flow.
- If the browser does not open automatically, copy the activation URL from the app/log and open it manually.
- If the device code expires, click `Login with Twitch` again.

Rewards do not update:

- Confirm the authenticated Twitch user owns the channel.
- Confirm the app has `channel:manage:redemptions`.
- Confirm the reward was created by this Twitch app/client ID.
- Rewards created manually in Twitch's dashboard or by another app cannot be updated by TuberSwitch.
- Some rewards may not be updateable; failures are logged and other rewards continue processing.

Token expired:

- The app attempts refresh automatically.
- If refresh fails, login with Twitch again.
