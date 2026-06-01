param(
  [string]$ServiceName = "GPUFleetAgent",
  [switch]$RemoveFiles,
  [string]$InstallDir = "C:\Program Files\GPUFleet"
)

$ErrorActionPreference = "Stop"

$existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existing) {
  Stop-Service -Name $ServiceName -ErrorAction SilentlyContinue
  sc.exe delete $ServiceName | Out-Null
  Write-Host "Removed $ServiceName"
}

if ($RemoveFiles -and (Test-Path $InstallDir)) {
  Remove-Item -LiteralPath $InstallDir -Recurse -Force
  Write-Host "Removed $InstallDir"
}

