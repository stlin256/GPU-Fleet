param(
  [Parameter(Mandatory = $true)]
  [string]$Version,
  [string]$ChangelogPath = "CHANGELOG.md",
  [string]$OutputPath = ""
)

$ErrorActionPreference = "Stop"

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

function Get-ChangelogSection {
  param(
    [string]$ReleaseVersion,
    [string]$Path
  )

  $resolvedPath = (Resolve-Path -LiteralPath $Path).Path
  $content = Get-Content -LiteralPath $resolvedPath -Raw
  $escapedVersion = [regex]::Escape($ReleaseVersion)
  $headingPattern = "(?m)^## \[$escapedVersion\] - .+$"
  $headingMatch = [regex]::Match($content, $headingPattern)
  if (!$headingMatch.Success) {
    throw "Unable to find CHANGELOG.md entry for $ReleaseVersion in $resolvedPath"
  }

  $afterHeading = $headingMatch.Index + $headingMatch.Length
  $remaining = $content.Substring($afterHeading)
  $nextHeading = [regex]::Match($remaining, "(?m)^## \[[^\]]+\] - .+$")
  $end = $content.Length
  if ($nextHeading.Success) {
    $end = $afterHeading + $nextHeading.Index
  }
  return $content.Substring($headingMatch.Index, $end - $headingMatch.Index).Trim()
}

$releaseVersion = Normalize-ReleaseVersion -Value $Version
$section = Get-ChangelogSection -ReleaseVersion $releaseVersion -Path $ChangelogPath
$checksumName = "gpufleet_${releaseVersion}_checksums.txt"
$notes = @"
$section

## Release Assets / 发布包

This release attaches prebuilt Server and Agent packages for the selected target matrix, plus $checksumName for SHA256 verification.

本次发布会附加所选目标矩阵的 Server 与 Agent 预编译包，并提供 $checksumName 用于 SHA256 校验。
"@

if ($OutputPath) {
  $parent = Split-Path -Parent $OutputPath
  if ($parent) {
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
  }
  Set-Content -LiteralPath $OutputPath -Value $notes -Encoding UTF8
  Write-Host "Release notes written to $OutputPath"
} else {
  Write-Output $notes
}
