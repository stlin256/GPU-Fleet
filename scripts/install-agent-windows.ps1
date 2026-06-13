param(
  [string]$InstallDir = "$env:ProgramFiles\GPUFleet",
  [string]$DataDir = "$env:ProgramData\GPUFleet",
  [string]$AgentPath = "",
  [Parameter(Mandatory = $true)]
  [ValidateNotNullOrEmpty()]
  [string]$ServerUrl,
  [Parameter(Mandatory = $true)]
  [ValidateNotNullOrEmpty()]
  [string]$DeviceId,
  [Parameter(Mandatory = $true)]
  [ValidateNotNullOrEmpty()]
  [string]$Secret,
  [int]$IntervalSeconds = 10,
  [int]$ConfigIntervalSeconds = 3600,
  [int]$UpdateCheckIntervalSeconds = 1800,
  [int]$QueueMaxMB = 128,
  [string]$TaskName = "GPUFleetAgent",
  [string]$MinimumVersion = "0.1.9",
  [switch]$SkipOnceCheck
)

$ErrorActionPreference = "Stop"

function Resolve-AgentPath {
  param([string]$Path)

  if ($Path) {
    return (Resolve-Path -LiteralPath $Path).Path
  }

  $candidates = @(
    (Join-Path (Resolve-Path ".").Path "bin\gpufleet-agent.exe"),
    (Join-Path (Resolve-Path ".").Path "gpufleet-agent.exe")
  )
  foreach ($candidate in $candidates) {
    if (Test-Path -LiteralPath $candidate) {
      return $candidate
    }
  }

  throw "Missing gpufleet-agent.exe. Use a release package or pass -AgentPath."
}

function Get-AgentVersion {
  param([string]$Path)

  $output = & $Path -version 2>&1
  if ($LASTEXITCODE -ne 0) {
    throw "Unable to execute $Path -version: $output"
  }
  if ($output -match 'GPUFleet\s+([0-9]+\.[0-9]+\.[0-9]+)') {
    return [version]$Matches[1]
  }
  throw "Unable to parse Agent version from: $output"
}

function Stop-ExistingAgent {
  param([string]$Name)

  Stop-ScheduledTask -TaskName $Name -ErrorAction SilentlyContinue
  Unregister-ScheduledTask -TaskName $Name -Confirm:$false -ErrorAction SilentlyContinue

  $service = Get-Service -Name $Name -ErrorAction SilentlyContinue
  if ($service) {
    Stop-Service -Name $Name -Force -ErrorAction SilentlyContinue
    sc.exe delete $Name | Out-Null
    Start-Sleep -Seconds 2
  }

  Get-CimInstance Win32_Process |
    Where-Object { $_.CommandLine -match 'gpufleet-agent|run-agent\.ps1' } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
}

function Protect-SecretFile {
  param([string]$Path)

  icacls $Path /inheritance:r | Out-Null
  icacls $Path /grant:r "*S-1-5-18:(R)" "*S-1-5-32-544:(R)" | Out-Null
}

function Write-AgentEnvFile {
  param(
    [string]$Path,
    [string]$ServerUrl,
    [string]$DeviceId,
    [string]$Secret,
    [int]$IntervalSeconds,
    [int]$ConfigIntervalSeconds,
    [int]$UpdateCheckIntervalSeconds,
    [int]$QueueMaxMB
  )

  if (Test-Path -LiteralPath $Path) {
    icacls $Path /grant:r "*S-1-5-32-544:(F)" | Out-Null
  }
  @"
GPUFLEET_SERVER_URL=$ServerUrl
GPUFLEET_DEVICE_ID=$DeviceId
GPUFLEET_SECRET=$Secret
GPUFLEET_INTERVAL=$IntervalSeconds
GPUFLEET_CONFIG_INTERVAL=$ConfigIntervalSeconds
GPUFLEET_UPDATE_CHECK_INTERVAL=$UpdateCheckIntervalSeconds
GPUFLEET_QUEUE_MAX_MB=$QueueMaxMB
"@ | Set-Content -LiteralPath $Path -Encoding ASCII
  Protect-SecretFile -Path $Path
}

function Write-RunnerScript {
  param(
    [string]$Path,
    [string]$InstallDir,
    [string]$DataDir
  )

  $escapedInstallDir = $InstallDir.Replace("'", "''")
  $escapedDataDir = $DataDir.Replace("'", "''")

  @"
`$ErrorActionPreference = "Continue"
`$installDir = '$escapedInstallDir'
`$dataDir = '$escapedDataDir'
`$exe = Join-Path `$installDir 'gpufleet-agent.exe'
`$envFile = Join-Path `$dataDir 'agent.env'
`$queuePath = Join-Path `$dataDir 'queue'
`$logDir = Join-Path `$dataDir 'logs'
`$log = Join-Path `$logDir 'agent.log'

New-Item -ItemType Directory -Force -Path `$queuePath, `$logDir | Out-Null

function Import-GPUFleetEnv {
  if (!(Test-Path -LiteralPath `$envFile)) {
    throw "Missing `$envFile"
  }
  Get-Content -LiteralPath `$envFile | ForEach-Object {
    `$line = `$_.Trim()
    if (`$line -eq "" -or `$line.StartsWith("#")) {
      return
    }
    `$parts = `$line.Split("=", 2)
    if (`$parts.Count -eq 2) {
      [Environment]::SetEnvironmentVariable(`$parts[0], `$parts[1], "Process")
    }
  }
}

while (`$true) {
  Add-Content -Path `$log -Value ("==== start " + (Get-Date -Format "yyyy-MM-dd HH:mm:ss") + " ====")
  try {
    Import-GPUFleetEnv
    & `$exe -queue-path `$queuePath *>> `$log
    Add-Content -Path `$log -Value ("==== exit code " + `$LASTEXITCODE + " " + (Get-Date -Format "yyyy-MM-dd HH:mm:ss") + " ====")
  } catch {
    Add-Content -Path `$log -Value (`$_ | Out-String)
  }
  Start-Sleep -Seconds 5
}
"@ | Set-Content -LiteralPath $Path -Encoding UTF8
}

function Register-AgentTask {
  param(
    [string]$Name,
    [string]$RunnerPath,
    [string]$DataDir
  )

  $action = New-ScheduledTaskAction `
    -Execute "powershell.exe" `
    -Argument "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$RunnerPath`"" `
    -WorkingDirectory $DataDir
  $trigger = New-ScheduledTaskTrigger -AtStartup
  $settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -ExecutionTimeLimit (New-TimeSpan -Days 3650) `
    -RestartCount 3 `
    -RestartInterval (New-TimeSpan -Minutes 1)

  Register-ScheduledTask `
    -TaskName $Name `
    -Action $action `
    -Trigger $trigger `
    -Settings $settings `
    -User "SYSTEM" `
    -RunLevel Highest `
    -Force | Out-Null
}

function Test-AgentOnce {
  param(
    [string]$Path,
    [string]$ServerUrl,
    [string]$DeviceId,
    [string]$Secret,
    [string]$QueuePath,
    [int]$QueueMaxMB
  )

  & $Path `
    -server-url $ServerUrl `
    -device-id $DeviceId `
    -secret $Secret `
    -queue-path $QueuePath `
    -queue-max-mb $QueueMaxMB `
    -once
  if ($LASTEXITCODE -ne 0) {
    throw "Agent preflight upload failed with exit code $LASTEXITCODE."
  }
}

$source = Resolve-AgentPath -Path $AgentPath
$version = Get-AgentVersion -Path $source
$minimum = [version]$MinimumVersion
if ($version -lt $minimum) {
  throw "Agent binary is GPUFleet $version. Minimum supported version is $MinimumVersion. Use a current release artifact."
}

if (!(Get-Command nvidia-smi -ErrorAction SilentlyContinue)) {
  Write-Warning "nvidia-smi was not found in PATH. The Agent can start, but GPU collection will report an error until NVIDIA drivers are available."
}

New-Item -ItemType Directory -Force -Path $InstallDir, $DataDir | Out-Null
$queuePath = Join-Path $DataDir "queue"
$logDir = Join-Path $DataDir "logs"
New-Item -ItemType Directory -Force -Path $queuePath, $logDir | Out-Null

if (!$SkipOnceCheck) {
  Test-AgentOnce `
    -Path $source `
    -ServerUrl $ServerUrl `
    -DeviceId $DeviceId `
    -Secret $Secret `
    -QueuePath $queuePath `
    -QueueMaxMB $QueueMaxMB
}

Stop-ExistingAgent -Name $TaskName

$target = Join-Path $InstallDir "gpufleet-agent.exe"
$backup = "$target.bak"
if (Test-Path -LiteralPath $target) {
  Copy-Item -LiteralPath $target -Destination $backup -Force
}
Copy-Item -LiteralPath $source -Destination $target -Force

$installedVersion = Get-AgentVersion -Path $target
if ($installedVersion -ne $version) {
  if (Test-Path -LiteralPath $backup) {
    Copy-Item -LiteralPath $backup -Destination $target -Force
  }
  throw "Installed Agent version mismatch. Expected $version, got $installedVersion."
}

$envFile = Join-Path $DataDir "agent.env"
$runner = Join-Path $InstallDir "run-agent.ps1"
Write-AgentEnvFile `
  -Path $envFile `
  -ServerUrl $ServerUrl `
  -DeviceId $DeviceId `
  -Secret $Secret `
  -IntervalSeconds $IntervalSeconds `
  -ConfigIntervalSeconds $ConfigIntervalSeconds `
  -UpdateCheckIntervalSeconds $UpdateCheckIntervalSeconds `
  -QueueMaxMB $QueueMaxMB
Write-RunnerScript -Path $runner -InstallDir $InstallDir -DataDir $DataDir

Register-AgentTask -Name $TaskName -RunnerPath $runner -DataDir $DataDir
Start-ScheduledTask -TaskName $TaskName
Start-Sleep -Seconds 3

Write-Host "Installed GPUFleet Agent $installedVersion"
Write-Host "Task: $TaskName"
Write-Host "Binary: $target"
Write-Host "Config: $envFile"
Write-Host "Log: $(Join-Path $logDir 'agent.log')"
