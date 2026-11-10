# TuberSwitch 1.0.1 Release Validation

Candidate validation: 2026-11-09 (America/New_York).

- All 43 frontend tests passed with coverage (85.62% statements, 81.07% branches, 88.43% lines); TypeScript/Vite production build passed using lockfile dependencies.
- Full Go race/coverage tests and `go vet ./...` passed. The process-awareness decision and executable-name matching functions have 100% statement coverage.
- Native Windows test binaries passed. Linux and Windows `govulncheck` found no known vulnerabilities; `npm audit --audit-level=moderate` found zero vulnerabilities.
- Regressions cover no retries while OBS is closed, detecting startup/shutdown, clearing retry schedules, remote hosts, process-detection failures, manual connection attempts, executable names, and SIP status.
- Wails product metadata, runtime/SIP version, UI fallback, and current API examples are aligned to 1.0.1. The internal frontend package retains its existing independent 0.0.0 package version. SIP protocol remains version 1.

Before tagging v1.0.1, the manual Release workflow must pass on this revision with publishing disabled. It builds Windows packages and runs isolated SIP smoke. The tag workflow repeats packaging and smoke before publishing installer, portable ZIP, executable, and checksums. GitHub Actions records provide the evidence for the exact revision.

No live OBS/Twitch session or production install/upgrade acceptance is claimed. Historical validation boundaries below still apply.

## Historical: TuberSwitch 1.0.0 Release Validation

Candidate validation date: 2026-11-07 (America/New_York). This records automated evidence, not a claim that every hardware/account/OS combination has been manually tested.

## Local automated verification

- Frontend: 43 tests passed with coverage (88.43% lines); TypeScript/Vite production build passed.
- Full Go tests, Linux race tests, go vet, and native Windows production build/tests passed. Go vulnerability scans cover Linux and Windows.
- Frontend `npm audit`: zero reported vulnerabilities.
- Dependency scans reported no known vulnerabilities in the tested dependency graph. This is a point-in-time result, not a guarantee against unknown issues.
- Release workflow YAML passed actionlint. Version metadata and current API examples are aligned to 1.0.0; SIP protocol remains version 1.

## Packaging validation

Before pushing `v1.0.0`, run the release workflow manually with publishing disabled. Its packaging/SIP smoke steps must pass on the candidate revision. Tag workflows repeat the release checks and publish assets only after their prerequisites pass. GitHub Actions run history and artifacts provide the packaging evidence for the exact commit.

## Validation boundaries

Tests use isolated fixtures/source mirrors. Local validation did not launch the production app, change personal configuration/credentials, install over a production copy, or modify live OBS/Twitch/media state. Manual first-install/upgrade acceptance, real OBS scenes/rewards, browser/media behavior and OS trust prompts require an interactive platform check; automated tests do not establish those results.

The release workflow may use isolated app instances for SIP smoke. TuberSwitch detection targets require explicit configuration to replace legacy mode-based selection. Profile names do not set their saved mode. Windows packages currently have no configured signing step.
