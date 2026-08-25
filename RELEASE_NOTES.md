# Release Notes

Release notes are part of the TuberSwitch release process. Before pushing a release tag, add a matching `## vX.Y.Z` section here. The release workflow publishes that section as the GitHub Release body and fails if it is missing or empty.

## Unreleased

### Profile Reliability

- Reconciles outgoing and incoming profile state so OBS sources used only by the previous profile are hidden.
- Applies the selected profile's VTuber and PNGTuber visibility across its configured OBS scenes.
- Disables manageable Twitch rewards that are absent or disabled in the selected profile while leaving unmanageable rewards untouched.
- Uses the same reconciliation behavior for UI, app-detection, and SIP profile activation and reports item-specific OBS or Twitch failures.
- Keeps temporary SIP manual redeem changes separate from saved profile intent.

### Security

- Updates the pinned Go toolchain to 1.26.6 for standard-library security fixes.
- Refreshes the frontend dependency lockfile to patched Vite, PostCSS, Nano ID, and Undici releases.

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
