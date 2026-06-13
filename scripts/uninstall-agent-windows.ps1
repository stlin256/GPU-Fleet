param(
  [string]$TaskName = "GPUFleetAgent",
  [string]$ServiceName = "GPUFleetAgent",
  [string]$InstallDir = "$env:ProgramFiles\GPUFleet",
  [string]$DataDir = "$env:ProgramData\GPUFleet",
  [switch]$RemoveFiles
)

$ErrorActionPreference = "Stop"

Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue

$existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existing) {
  Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
  sc.exe delete $ServiceName | Out-Null
}

Get-CimInstance Win32_Process |
  Where-Object { $_.CommandLine -match 'gpufleet-agent|run-agent\.ps1' } |
  ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }

Write-Host "Removed GPUFleet Agent task/service entries."

if ($RemoveFiles) {
  foreach ($path in @($InstallDir, $DataDir)) {
    if (Test-Path -LiteralPath $path) {
      Remove-Item -LiteralPath $path -Recurse -Force
      Write-Host "Removed $path"
    }
  }
}
