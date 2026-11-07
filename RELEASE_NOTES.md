# Release Notes

Release notes are part of the TuberSwitch release process. Before pushing a release tag, add a matching `## vX.Y.Z` section here. The release workflow publishes that section as the GitHub Release body and fails if it is missing or empty.

## Unreleased

- Adds explicit 3D/PNG detection target profiles. Detection can switch away from Default even when the target uses the current mode, while leaving active targets alone on subsequent polls. Unconfigured targets retain legacy selection behavior.
- Validates target existence and saved mode, prevents silent fallback for invalid/deleted targets, and exposes safe detection reasons/target IDs/producer timestamps in settings, logs, and SIP diagnostics. Manual cooldown and both-app conflict preferences remain supported.

- Adds the additive SIP observability contract, safe operation correlation and diagnostics.
- Adds capability-advertised SIP operations/diagnostics and request correlation. Profile activation returns requested/actual profile IDs and safe results for OBS source changes, redeem changes, and profile storage, including partial application. Avatar source switching is owned by OBS; no independent avatar connection is invented. Status reports producer-observed OBS timestamps, last disconnect code and next scheduled retry, independently of streaming activity. Successful SIP profile changes expose operation confirmation IDs/times and session-scoped revisions. Recent safe operations are retained beside configuration in `sip-operations.json` (up to 256 operations/14 days). Existing fields, local-host protections and synchronous clients remain supported.
- Adds regression tests and updates patched build/test dependencies.

## v0.7.3

### OBS Reliability

- Automatically reconnects when OBS starts after TuberSwitch or an established OBS connection drops.
- Uses serialized connection attempts with bounded exponential backoff to avoid reconnect storms.
- Adds the non-sensitive SIP `obsConnectionState` status field while preserving `obsConnected`.

### Maintenance

- Updated release metadata to `0.7.3`.

## v0.7.2

### Profile Reliability

- Reconciles outgoing and incoming profile state so OBS sources used only by the previous profile are hidden.
- Applies the selected profile's VTuber and PNGTuber visibility across its configured OBS scenes.
- Disables manageable Twitch rewards that are absent or disabled in the selected profile while leaving unmanageable rewards untouched.
- Uses the same reconciliation behavior for UI, app-detection, and SIP profile activation and reports item-specific OBS or Twitch failures.
- Keeps temporary SIP manual redeem changes separate from saved profile intent.

### Security

- Updates the pinned Go toolchain to 1.26.6 for standard-library security fixes.
- Refreshes the frontend dependency lockfile to patched Vite, PostCSS, Nano ID, and Undici releases.

### Maintenance

- Updated release metadata to `0.7.2`.

## v0.7.1

### SIP v1

- Adds `POST /api/v1/redeems/manual` for temporary Twitch redeem enable/disable without saving active profile intent.
- Validates manual redeem requests before applying Twitch changes so unknown or unmanageable rewards fail without partial updates.
- Updates SIP API docs, README endpoint listings, and Postman collections for LivePanel manual redeem control.

### Maintenance

- Updated release metadata to `0.7.1`.

## v0.7.0

### SIP v1

- Adds SIP redeem read/update endpoints for existing TuberSwitch reward mappings.
- Adds SIP redeem availability reporting, including manageable and unmanageable reward state.
- Adds a SIP Newman release smoke collection and local smoke runner.

### UX and Reliability

- Adds visible action status, warning, and error feedback for settings, profile, Twitch, OBS sync, and update workflows.
- Fixes failed save-then-action flows so controls re-enable and the original validation error remains visible.
- Updates reward mappings from legacy `is3DOnly` intent to profile-level `enabled` intent, including SIP redeem state and automatic migration for existing configs.
- Keeps profile save-as validation errors in the in-app dialog, removes duplicate recent profile options, and tightens numeric validation for OBS and app detection settings.

### Maintenance

- Updated release metadata to `0.7.0`.

## v0.6.0

### SIP v1

- Adds Service Mode with `--service` and `--show` launch behavior.
- Adds single-instance management for standalone and service launches.
- Adds SIP v1.1 localhost endpoints for app identity, health, capabilities, status, profiles, current profile, and profile activation.
- Enriches SIP status with additive OBS, redeem, and app detection summary fields for LivePanel.
- Splits SIP redeem status counts into total, manageable, and unmanageable rewards.
- Uses existing TuberSwitch profile activation paths for SIP profile switching.
- Adds SIP API reference documentation and a Postman collection.
- Adds SIP API tests for discovery, profile activation, localhost protection, JSON validation, body-size limits, error handling, and security headers.

### Maintenance

- Updated release metadata to `0.6.0`.
- Aligns Windows installer metadata and default install root with the Starsong Installer Standard.

## v0.5.0

### Highlights

- Added stream profiles for reusable 3D VTuber and PNGTuber setups.
- App Detection now applies the most recently used profile that matches the detected mode.
- Profiles now own presentation mode, OBS scene source choices, and Twitch reward enablement.
- Updated profile settings to use a single `Desired Source` selector per scene.
- Simplified reward controls to a plain `Enabled` checkbox.

### UI Polish

- Replaced browser-native profile prompts with in-app dialogs.
- Simplified the main mode panel by removing redundant status text.
- Shortened the mode switch button and aligned it with the profile selector.
- Standardized user-facing wording to `PNGTuber`.

### Maintenance

- Updated release metadata to `0.5.0`.
- Trimmed the README to focus on product setup, app detection, development, and quality gates.
- Added coverage for the profile UI changes and profile-aware app switching.
