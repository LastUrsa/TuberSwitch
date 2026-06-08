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
