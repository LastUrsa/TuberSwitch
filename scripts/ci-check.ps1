$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Split-Path -Parent $PSScriptRoot

Write-Host "Running frontend dependency install..." -ForegroundColor Cyan
Push-Location (Join-Path $repoRoot "frontend")
try {
    npm ci

    Write-Host "Running frontend tests with coverage..." -ForegroundColor Cyan
    npm run test:coverage

    Write-Host "Building frontend..." -ForegroundColor Cyan
    npm run build
}
finally {
    Pop-Location
}

Write-Host "Running Go tests with coverage..." -ForegroundColor Cyan
Push-Location $repoRoot
try {
    go test ./... -cover
}
finally {
    Pop-Location
}

Write-Host "CI-equivalent checks passed." -ForegroundColor Green
