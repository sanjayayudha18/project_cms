<#
.SYNOPSIS
  Run the CMS frontends locally without Docker (Vite dev server).

.DESCRIPTION
  Starts the Vite dev server for the internal Company portal, the Vendor
  portal, or both. Docker is only needed for the production (Nginx) image;
  for day-to-day UI work this runs the raw dev server with hot reload.

  The /api proxy in each vite.config.ts forwards to the ATM backend on
  http://localhost:8080, so keep that backend running (Docker or `air`) if
  you need real API data. Pure UI / mock-data screens work with no backend.

.PARAMETER Target
  Which portal to start: company (default), vendor, or all.

.PARAMETER Install
  Run `pnpm install` in the target(s) before starting.

.PARAMETER Preview
  Build then serve the production bundle (pnpm build + pnpm preview)
  instead of the dev server. Useful to sanity-check the built output
  without touching Docker.

.EXAMPLE
  ./dev.ps1                 # internal Company portal, dev server
  ./dev.ps1 vendor          # Vendor portal, dev server
  ./dev.ps1 all             # both portals in parallel
  ./dev.ps1 company -Install
  ./dev.ps1 company -Preview
#>
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [ValidateSet("company", "vendor", "all")]
    [string]$Target = "company",

    [switch]$Install,

    [switch]$Preview
)

$ErrorActionPreference = "Stop"

$root = $PSScriptRoot
$portals = @{
    company = @{ Name = "CompanyPortal-Vite (internal)"; Path = Join-Path $root "CompanyPortal-Vite" }
    vendor  = @{ Name = "VendorPortal-Vite (vendor)";    Path = Join-Path $root "VendorPortal-Vite" }
}

if (-not (Get-Command pnpm -ErrorAction SilentlyContinue)) {
    Write-Error "pnpm not found on PATH. Install it first: npm install -g pnpm  (or: corepack enable)"
}

# Resolve which portals to act on.
$selected = if ($Target -eq "all") { @("company", "vendor") } else { @($Target) }

# Command to run inside each portal.
$script = if ($Preview) { "run build; if (`$LASTEXITCODE -eq 0) { pnpm run preview }" } else { "run dev" }
$mode = if ($Preview) { "production preview (build + preview)" } else { "dev server (hot reload)" }

foreach ($key in $selected) {
    $portal = $portals[$key]
    if (-not (Test-Path $portal.Path)) {
        Write-Error "Portal path not found: $($portal.Path)"
    }
    if ($Install) {
        Write-Host "==> Installing dependencies for $($portal.Name)" -ForegroundColor Cyan
        Push-Location $portal.Path
        pnpm install
        Pop-Location
    }
}

if ($selected.Count -eq 1) {
    $portal = $portals[$selected[0]]
    Write-Host "==> Starting $($portal.Name) - $mode" -ForegroundColor Green
    Write-Host "    (Ctrl+C to stop)" -ForegroundColor DarkGray
    Push-Location $portal.Path
    try {
        Invoke-Expression "pnpm $script"
    }
    finally {
        Pop-Location
    }
}
else {
    # Run each portal in its own PowerShell window so both stay interactive.
    Write-Host "==> Starting both portals - $mode (each in its own window)" -ForegroundColor Green
    foreach ($key in $selected) {
        $portal = $portals[$key]
        Write-Host "    - $($portal.Name)" -ForegroundColor DarkGray
        Start-Process pwsh -ArgumentList @(
            "-NoExit",
            "-Command",
            "Set-Location '$($portal.Path)'; Write-Host 'Running $($portal.Name)' -ForegroundColor Green; pnpm $script"
        )
    }
    Write-Host "==> Launched. Close each window (or Ctrl+C inside it) to stop a portal." -ForegroundColor Cyan
}
