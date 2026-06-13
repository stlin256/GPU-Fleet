param(
  [Parameter(Mandatory = $true)]
  [string]$Version,
  [string]$Root = ""
)

$ErrorActionPreference = "Stop"

function Get-RepositoryRoot {
  param([string]$RequestedRoot)

  if ($RequestedRoot) {
    return (Resolve-Path -LiteralPath $RequestedRoot).Path
  }
  $gitRoot = git rev-parse --show-toplevel 2>$null
  if ($LASTEXITCODE -eq 0 -and $gitRoot) {
    return $gitRoot.Trim()
  }
  return (Resolve-Path ".").Path
}

function Normalize-ReleaseVersion {
  param([string]$Value)

  $normalized = $Value.Trim()
  if ($normalized.StartsWith("v")) {
    $normalized = $normalized.Substring(1)
  }
  if ($normalized -notmatch '^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$') {
    throw "Invalid release version '$Value'. Use a fixed semver value such as 1.0.15."
  }
  return $normalized
}

function Get-InternalVersion {
  param([string]$RepositoryRoot)

  $versionFile = Join-Path (Join-Path $RepositoryRoot "internal") "version/version.go"
  $content = Get-Content -LiteralPath $versionFile -Raw
  if ($content -match 'Version\s*=\s*"([^"]+)"') {
    return $Matches[1]
  }
  throw "Unable to parse Version from $versionFile"
}

function Assert-VersionEqual {
  param(
    [string]$Name,
    [string]$Actual,
    [string]$Expected
  )

  if ($Actual -ne $Expected) {
    throw "$Name version is '$Actual', expected '$Expected'. Fix the version before running the release workflow."
  }
}

$repoRoot = Get-RepositoryRoot -RequestedRoot $Root
$releaseVersion = Normalize-ReleaseVersion -Value $Version

$internalVersion = Get-InternalVersion -RepositoryRoot $repoRoot
Assert-VersionEqual -Name "internal/version" -Actual $internalVersion -Expected $releaseVersion

$packagePath = Join-Path (Join-Path $repoRoot "web") "package.json"
$package = Get-Content -LiteralPath $packagePath -Raw | ConvertFrom-Json
Assert-VersionEqual -Name "web/package.json" -Actual $package.version -Expected $releaseVersion

$lockPath = Join-Path (Join-Path $repoRoot "web") "package-lock.json"
if (Test-Path -LiteralPath $lockPath) {
  $lockRaw = Get-Content -LiteralPath $lockPath -Raw
  if ($lockRaw -match '"version"\s*:\s*"([^"]+)"') {
    Assert-VersionEqual -Name "web/package-lock.json" -Actual $Matches[1] -Expected $releaseVersion
  } else {
    throw "Unable to parse root version from $lockPath"
  }
  if ($lockRaw -match '(?s)"packages"\s*:\s*\{\s*""\s*:\s*\{.*?"version"\s*:\s*"([^"]+)"') {
    Assert-VersionEqual -Name "web/package-lock.json root package" -Actual $Matches[1] -Expected $releaseVersion
  }
}

$changelogPath = Join-Path $repoRoot "CHANGELOG.md"
$changelog = Get-Content -LiteralPath $changelogPath -Raw
$escapedVersion = [regex]::Escape($releaseVersion)
if ($changelog -notmatch "(?m)^## \[$escapedVersion\] - .+$") {
  throw "CHANGELOG.md does not contain a release entry for $releaseVersion."
}

Write-Host "Release version $releaseVersion is fixed across internal/version, web package metadata, and CHANGELOG.md."
