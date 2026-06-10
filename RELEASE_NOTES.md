# Release Notes

Release notes are part of the TuberSwitch release process. Before pushing a release tag, add notes for the new version here and use the same notes for the GitHub release.

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

## Upcoming

### SIP v1

- Adds Service Mode with `--service` and `--show` launch behavior.
- Adds single-instance management for standalone and service launches.
- Adds SIP v1.1 localhost endpoints for app identity, health, capabilities, status, profiles, current profile, and profile activation.
- Enriches SIP status with additive OBS, redeem, and app detection summary fields for LivePanel.
- Splits SIP redeem status counts into total, manageable, and unmanageable rewards.
- Uses existing TuberSwitch profile activation paths for SIP profile switching.
- Adds SIP API reference documentation and a Postman collection.
- Adds SIP API tests for discovery, profile activation, localhost protection, JSON validation, body-size limits, error handling, and security headers.
