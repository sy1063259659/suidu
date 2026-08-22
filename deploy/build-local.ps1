$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot

Push-Location (Join-Path $repoRoot 'frontend')
try {
    npm run build
} finally {
    Pop-Location
}

Push-Location (Join-Path $repoRoot 'backend')
try {
    New-Item -ItemType Directory -Force -Path (Join-Path (Get-Location) 'bin') | Out-Null
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'linux'
    $env:GOARCH = 'amd64'
    go build -trimpath -ldflags='-s -w' -o bin/suidu-api ./cmd/server
} finally {
    Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    Pop-Location
}

Write-Host 'Local release artifacts are ready:'
Write-Host (Join-Path $repoRoot 'frontend/dist')
Write-Host (Join-Path $repoRoot 'backend/bin/suidu-api')
