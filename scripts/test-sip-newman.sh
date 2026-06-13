#!/usr/bin/env bash
set -euo pipefail

launch=true
skip_build=false
base_url=""
collection="docs/postman/TuberSwitch-SIP-v1-newman-smoke.postman_collection.json"
timeout_seconds=90
newman_package="newman@6.2.1"
launched_pid=""
smoke_appdata_windows=""
build_repo_windows=""
launch_exe_windows=""

to_windows_path() {
    if command -v wslpath >/dev/null 2>&1; then
        wslpath -w "$1"
    elif command -v cygpath >/dev/null 2>&1; then
        cygpath -w "$1"
    else
        powershell.exe -NoProfile -Command "\$ErrorActionPreference = 'Stop'; [System.IO.Path]::GetFullPath('$1')"
    fi
}

cleanup() {
    if [[ -n "$launched_pid" ]]; then
        powershell.exe -NoProfile -Command "\$ErrorActionPreference = 'SilentlyContinue'; Stop-Process -Id ${launched_pid} -Force" >/dev/null 2>&1 || true
    fi
    if [[ -n "$smoke_appdata_windows" ]]; then
        powershell.exe -NoProfile -Command "\$ErrorActionPreference = 'SilentlyContinue'; Remove-Item -LiteralPath '${smoke_appdata_windows}' -Recurse -Force" >/dev/null 2>&1 || true
    fi
    if [[ -n "$build_repo_windows" ]]; then
        powershell.exe -NoProfile -Command "\$ErrorActionPreference = 'SilentlyContinue'; Remove-Item -LiteralPath '${build_repo_windows}' -Recurse -Force" >/dev/null 2>&1 || true
    fi
}
trap cleanup EXIT

while [[ $# -gt 0 ]]; do
    case "$1" in
        --no-launch)
            launch=false
            shift
            ;;
        --skip-build)
            skip_build=true
            shift
            ;;
        --base-url)
            if [[ -z "${2:-}" ]]; then
                echo "--base-url requires a value" >&2
                exit 2
            fi
            base_url="$2"
            shift 2
            ;;
        --collection)
            if [[ -z "${2:-}" ]]; then
                echo "--collection requires a value" >&2
                exit 2
            fi
            collection="$2"
            shift 2
            ;;
        --timeout)
            if [[ -z "${2:-}" ]]; then
                echo "--timeout requires a value" >&2
                exit 2
            fi
            timeout_seconds="$2"
            shift 2
            ;;
        *)
            echo "unknown argument: $1" >&2
            exit 2
            ;;
    esac
done

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if [[ "$launch" == true && "$skip_build" == false ]]; then
    echo "Building frontend..."
    (cd frontend && npm run build)

    echo "Building TuberSwitch desktop host..."
    repo_windows="$(to_windows_path "$repo_root" | tr -d '\r' | head -n 1)"
    build_repo_windows="$(
        powershell.exe -NoProfile -Command "\$ErrorActionPreference = 'Stop'; [System.IO.Path]::Combine(\$env:TEMP, 'TuberSwitchSipBuild-$$')" |
            tr -d '\r' |
            head -n 1
    )"
    build_repo_unix="$(wslpath -u "$build_repo_windows")"
    mkdir -p "$build_repo_unix"
    rsync -a --delete --exclude ".git" --exclude "frontend/node_modules" "$repo_root"/ "$build_repo_unix"/
    powershell.exe -NoProfile -Command "\$ErrorActionPreference = 'Stop'; \$env:Path += ';' + (& go env GOPATH) + '\bin'; Set-Location '${build_repo_windows}'; wails build -clean -s"
    powershell.exe -NoProfile -Command "\$ErrorActionPreference = 'Stop'; Copy-Item -LiteralPath '${build_repo_windows}\build\bin\TuberSwitch.exe' -Destination '${repo_windows}\build\bin\TuberSwitch.exe' -Force"
    launch_exe_windows="${build_repo_windows}\\build\\bin\\TuberSwitch.exe"
fi

if [[ "$launch" == true ]]; then
    exe_path="build/bin/TuberSwitch.exe"
    if [[ ! -f "$exe_path" ]]; then
        echo "TuberSwitch.exe was not found. Run without --skip-build or build the app first." >&2
        exit 1
    fi

    smoke_appdata_windows="$(
        powershell.exe -NoProfile -Command "\$ErrorActionPreference = 'Stop'; [System.IO.Path]::Combine(\$env:TEMP, 'TuberSwitchSipSmoke-$$')" |
            tr -d '\r' |
            head -n 1
    )"
    powershell.exe -NoProfile -Command "\$ErrorActionPreference = 'Stop'; \$configDir = Join-Path '${smoke_appdata_windows}' 'TuberSwitch'; New-Item -ItemType Directory -Path \$configDir -Force | Out-Null; @'
{
  \"obs\": {
    \"host\": \"127.0.0.1\",
    \"port\": 4455,
    \"allowRemote\": false
  },
  \"sources\": {},
  \"sceneMappings\": [],
  \"twitch\": {
    \"clientId\": \"\",
    \"channelId\": \"\",
    \"channelName\": \"\"
  },
  \"rewardMappings\": [
    {
      \"rewardId\": \"headpat\",
      \"rewardName\": \"Headpat\",
      \"is3DOnly\": true,
      \"manageable\": true
    },
    {
      \"rewardId\": \"hydrate\",
      \"rewardName\": \"Hydrate\",
      \"is3DOnly\": false,
      \"manageable\": true
    }
  ],
  \"profiles\": [
    {
      \"id\": \"default\",
      \"name\": \"Default\",
      \"mode\": \"PNG\",
      \"sources\": {},
      \"sceneMappings\": [],
      \"rewardMappings\": [
        {
          \"rewardId\": \"headpat\",
          \"rewardName\": \"Headpat\",
          \"is3DOnly\": true,
          \"manageable\": true
        },
        {
          \"rewardId\": \"hydrate\",
          \"rewardName\": \"Hydrate\",
          \"is3DOnly\": false,
          \"manageable\": true
        }
      ]
    },
    {
      \"id\": \"gaming\",
      \"name\": \"Gaming Stream\",
      \"mode\": \"3D\",
      \"sources\": {},
      \"sceneMappings\": [],
      \"rewardMappings\": [
        {
          \"rewardId\": \"headpat\",
          \"rewardName\": \"Headpat\",
          \"is3DOnly\": true,
          \"manageable\": true
        },
        {
          \"rewardId\": \"hydrate\",
          \"rewardName\": \"Hydrate\",
          \"is3DOnly\": true,
          \"manageable\": true
        }
      ]
    }
  ],
  \"activeProfileId\": \"default\",
  \"modeProfiles\": [
    {
      \"id\": \"3D\",
      \"displayName\": \"3D VTuber Mode\",
      \"vtuberVisible\": true,
      \"pngTuberVisible\": false,
      \"enable3DRewards\": true
    },
    {
      \"id\": \"PNG\",
      \"displayName\": \"PNGTuber Mode\",
      \"vtuberVisible\": false,
      \"pngTuberVisible\": true,
      \"enable3DRewards\": false
    }
  ],
  \"startupMode\": \"restore-last\",
  \"currentMode\": \"PNG\",
  \"refreshRewardsOnStartup\": false,
  \"appDetection\": {
    \"enabled\": false,
    \"threeDProcessName\": \"\",
    \"pngProcessName\": \"\",
    \"intervalSeconds\": 5,
    \"conflictBehavior\": \"do-nothing\",
    \"applyTwitchChanges\": false,
    \"manualOverrideCooldownSeconds\": 20
  }
}
'@ | Set-Content -LiteralPath (Join-Path \$configDir 'config.json') -Encoding ASCII"

    exe_windows="$launch_exe_windows"
    if [[ -z "$exe_windows" ]]; then
        exe_windows="$(to_windows_path "$repo_root/$exe_path" | tr -d '\r' | head -n 1)"
    fi
    launched_pid="$(
        powershell.exe -NoProfile -Command "\$ErrorActionPreference = 'Stop'; \$env:APPDATA = '${smoke_appdata_windows}'; \$process = Start-Process -FilePath '${exe_windows}' -ArgumentList '--service' -PassThru; \$process.Id" |
            tr -d '\r' |
            head -n 1
    )"
fi

run_newman_on_windows=false
if [[ -z "$base_url" ]]; then
    deadline=$((SECONDS + timeout_seconds))
    while [[ $SECONDS -lt $deadline ]]; do
        for port in {47040..47049}; do
            candidate="http://127.0.0.1:${port}"
            if curl --silent --fail --max-time 1 "${candidate}/api/v1/app" >/dev/null; then
                base_url="$candidate"
                break 2
            fi
        done

        discovered="$(
            powershell.exe -NoProfile -Command '$ErrorActionPreference = "SilentlyContinue"; foreach ($p in 47040..47049) { try { Invoke-RestMethod -Uri "http://127.0.0.1:$p/api/v1/app" -TimeoutSec 1 | Out-Null; Write-Output "http://127.0.0.1:$p"; exit 0 } catch {} }; exit 0' 2>/dev/null |
                tr -d '\r' |
                head -n 1
        )"
        if [[ -n "$discovered" ]]; then
            base_url="$discovered"
            run_newman_on_windows=true
            break
        fi

        sleep 1
    done
fi

if [[ -z "$base_url" ]]; then
    echo "Timed out waiting for TuberSwitch SIP on 127.0.0.1:47040-47049" >&2
    exit 1
fi

echo "Running SIP Newman smoke checks against ${base_url}"
if [[ "$run_newman_on_windows" == true ]]; then
    collection_windows="$(to_windows_path "$collection" | tr -d '\r' | head -n 1)"
    powershell.exe -NoProfile -Command "\$ErrorActionPreference = 'Stop'; Set-Location \$env:TEMP; & npx.cmd --yes '${newman_package}' run '${collection_windows}' --env-var 'baseUrl=${base_url}'"
elif command -v newman >/dev/null 2>&1; then
    newman_cmd=(newman)
elif command -v npx >/dev/null 2>&1; then
    newman_cmd=(npx --yes "$newman_package")
else
    echo "Newman is not installed, and npx is unavailable. Install with: npm install -g newman" >&2
    exit 1
fi

if [[ "$run_newman_on_windows" != true ]]; then
    "${newman_cmd[@]}" run "$collection" --env-var "baseUrl=${base_url}"
fi
