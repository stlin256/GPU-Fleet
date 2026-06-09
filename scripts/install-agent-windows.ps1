param(
  [string]$InstallDir = "C:\Program Files\GPUFleet",
  [string]$ServerUrl = "http://127.0.0.1:8080",
  [Parameter(Mandatory = $true)]
  [ValidateNotNullOrEmpty()]
  [string]$DeviceId,
  [Parameter(Mandatory = $true)]
  [ValidateNotNullOrEmpty()]
  [string]$Secret,
  [int]$IntervalSeconds = 10,
  [int]$QueueMaxMB = 128,
  [string]$ServiceName = "GPUFleetAgent"
)

$ErrorActionPreference = "Stop"

$source = Join-Path (Resolve-Path ".") "bin\gpufleet-agent.exe"
if (!(Test-Path $source)) {
  throw "Missing $source. Build it first: go build -o bin\gpufleet-agent.exe .\cmd\gpufleet-agent"
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$target = Join-Path $InstallDir "gpufleet-agent.exe"
Copy-Item -LiteralPath $source -Destination $target -Force

$queuePath = Join-Path $InstallDir "queue"
$args = @(
  "-server-url", $ServerUrl,
  "-device-id", $DeviceId,
  "-secret", $Secret,
  "-interval", $IntervalSeconds,
  "-queue-path", $queuePath,
  "-queue-max-mb", $QueueMaxMB
)

$existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existing) {
  Stop-Service -Name $ServiceName -ErrorAction SilentlyContinue
  sc.exe delete $ServiceName | Out-Null
  Start-Sleep -Seconds 1
}

New-Service `
  -Name $ServiceName `
  -BinaryPathName ('"{0}" {1}' -f $target, ($args -join " ")) `
  -DisplayName "GPUFleet Agent" `
  -Description "Read-only NVIDIA GPU telemetry agent for GPUFleet." `
  -StartupType Automatic

Start-Service -Name $ServiceName
Write-Host "Installed and started $ServiceName"
