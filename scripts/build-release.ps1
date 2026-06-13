param(
  [string]$Version = "",
  [string]$OutputDir = "dist/release",
  [ValidateSet("full", "core")]
  [string]$TargetSet = "full",
  [string[]]$Targets = @(),
  [switch]$SkipWebBuild
)

$ErrorActionPreference = "Stop"

function Get-RepoRoot {
  $root = git rev-parse --show-toplevel 2>$null
  if ($LASTEXITCODE -eq 0 -and $root) {
    return $root.Trim()
  }
  return (Resolve-Path ".").Path
}

function Get-SourceVersion {
  param([string]$Root)

  $versionFile = Join-Path $Root "internal/version/version.go"
  $content = Get-Content -LiteralPath $versionFile -Raw
  if ($content -match 'Version\s*=\s*"([^"]+)"') {
    return $Matches[1]
  }
  throw "Unable to parse version from $versionFile"
}

function New-CleanDirectory {
  param([string]$Path)

  if (Test-Path -LiteralPath $Path) {
    Remove-Item -LiteralPath $Path -Recurse -Force
  }
  New-Item -ItemType Directory -Force -Path $Path | Out-Null
}

function Copy-ReleaseCommonFiles {
  param(
    [string]$Root,
    [string]$PackageDir,
    [string[]]$ScriptNames
  )

  foreach ($file in @("README.md", "README-en.md", "CHANGELOG.md")) {
    Copy-Item -LiteralPath (Join-Path $Root $file) -Destination $PackageDir -Force
  }

  $docsDir = Join-Path $PackageDir "docs"
  New-Item -ItemType Directory -Force -Path $docsDir | Out-Null
  foreach ($doc in @("14-installation.md", "12-operations.md")) {
    Copy-Item -LiteralPath (Join-Path $Root "docs/$doc") -Destination $docsDir -Force
  }

  $scriptsDir = Join-Path $PackageDir "scripts"
  New-Item -ItemType Directory -Force -Path $scriptsDir | Out-Null
  foreach ($script in $ScriptNames) {
    Copy-Item -LiteralPath (Join-Path $Root "scripts/$script") -Destination $scriptsDir -Force
  }
}

function Compress-Package {
  param(
    [string]$PackageDir,
    [string]$ArchivePath,
    [string]$Format
  )

  if ($Format -eq "zip") {
    Compress-Archive -Path (Join-Path $PackageDir "*") -DestinationPath $ArchivePath -Force
    return
  }

  $parent = Split-Path -Parent $PackageDir
  $leaf = Split-Path -Leaf $PackageDir
  tar -C $parent -czf $ArchivePath $leaf
  if ($LASTEXITCODE -ne 0) {
    throw "tar failed with exit code $LASTEXITCODE"
  }
}

function Add-Checksum {
  param(
    [string]$ArchivePath,
    [System.Collections.Generic.List[string]]$Lines
  )

  $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $ArchivePath).Hash.ToLowerInvariant()
  $name = Split-Path -Leaf $ArchivePath
  $Lines.Add("$hash  $name")
}

$repoRoot = Get-RepoRoot
$versionValue = $Version
if (!$versionValue) {
  $versionValue = Get-SourceVersion -Root $repoRoot
}

$commit = "dev"
$gitCommit = git -C $repoRoot rev-parse HEAD 2>$null
if ($LASTEXITCODE -eq 0 -and $gitCommit) {
  $commit = $gitCommit.Trim()
}
$buildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags = "-X gpufleet/internal/version.Commit=$commit -X gpufleet/internal/version.BuildTime=$buildTime"

if (!$SkipWebBuild) {
  Push-Location (Join-Path $repoRoot "web")
  try {
    npm ci
    if ($LASTEXITCODE -ne 0) {
      throw "npm ci failed with exit code $LASTEXITCODE"
    }
    npm run build
    if ($LASTEXITCODE -ne 0) {
      throw "npm run build failed with exit code $LASTEXITCODE"
    }
  } finally {
    Pop-Location
  }
}

$webDist = Join-Path $repoRoot "web/dist"
if (!(Test-Path -LiteralPath (Join-Path $webDist "index.html"))) {
  throw "Missing web/dist/index.html. Build the frontend or pass -SkipWebBuild only when web/dist already exists."
}

$releaseRoot = Join-Path $repoRoot $OutputDir
New-CleanDirectory -Path $releaseRoot

if ($Targets.Count -eq 0) {
  if ($TargetSet -eq "core") {
    $Targets = @(
      "windows/amd64",
      "windows/arm64",
      "linux/amd64",
      "linux/arm64",
      "darwin/amd64",
      "darwin/arm64"
    )
  } else {
    $Targets = @(
      "windows/386",
      "windows/amd64",
      "windows/arm64",
      "linux/386",
      "linux/amd64",
      "linux/arm/v5",
      "linux/arm/v6",
      "linux/arm/v7",
      "linux/arm64",
      "linux/loong64",
      "linux/mips",
      "linux/mips64",
      "linux/mips64le",
      "linux/mipsle",
      "linux/ppc64",
      "linux/ppc64le",
      "linux/riscv64",
      "linux/s390x",
      "darwin/amd64",
      "darwin/arm64",
      "freebsd/386",
      "freebsd/amd64",
      "freebsd/arm/v6",
      "freebsd/arm/v7",
      "freebsd/arm64",
      "freebsd/riscv64"
    )
  }
}

$checksums = [System.Collections.Generic.List[string]]::new()

foreach ($targetName in $Targets) {
  if ($targetName -notmatch '^([^/]+)/([^/]+)(?:/([^/]+))?$') {
    throw "Invalid target '$targetName'. Use GOOS/GOARCH or GOOS/GOARCH/variant, for example linux/amd64 or linux/arm/v7."
  }
  $targetOS = $Matches[1]
  $targetArch = $Matches[2]
  $targetVariant = $Matches[3]
  $targetArchLabel = $targetArch
  if ($targetVariant) {
    $targetArchLabel = "$targetArch$targetVariant"
  }
  $targetExt = if ($targetOS -eq "windows") { ".exe" } else { "" }
  $targetFormat = if ($targetOS -eq "windows") { "zip" } else { "tar.gz" }

  $env:GOOS = $targetOS
  $env:GOARCH = $targetArch
  $env:CGO_ENABLED = "0"
  Remove-Item Env:\GOARM -ErrorAction SilentlyContinue
  if ($targetArch -eq "arm" -and $targetVariant) {
    $env:GOARM = $targetVariant.TrimStart("v")
  }

  foreach ($component in @("server", "agent")) {
    $packageName = "gpufleet-$component`_$versionValue`_$targetOS`_$targetArchLabel"
    $packageDir = Join-Path $releaseRoot $packageName
    New-CleanDirectory -Path $packageDir

    $binDir = Join-Path $packageDir "bin"
    New-Item -ItemType Directory -Force -Path $binDir | Out-Null
    $binaryName = "gpufleet-$component$targetExt"
    $binaryPath = Join-Path $binDir $binaryName

    go build -trimpath -ldflags $ldflags -o $binaryPath "./cmd/gpufleet-$component"
    if ($LASTEXITCODE -ne 0) {
      throw "go build failed for gpufleet-$component $targetOS/$targetArch"
    }

    if ($component -eq "server") {
      New-Item -ItemType Directory -Force -Path (Join-Path $packageDir "web") | Out-Null
      Copy-Item -LiteralPath $webDist -Destination (Join-Path $packageDir "web") -Recurse -Force
      Copy-ReleaseCommonFiles `
        -Root $repoRoot `
        -PackageDir $packageDir `
        -ScriptNames @("install-server-linux.sh", "backup-server-linux.sh", "restore-server-linux.sh")
    } else {
      Copy-ReleaseCommonFiles `
        -Root $repoRoot `
        -PackageDir $packageDir `
        -ScriptNames @("install-agent-windows.ps1", "uninstall-agent-windows.ps1", "install-agent-linux.sh", "uninstall-agent-linux.sh", "gpufleet-agent.service")
    }

    $archiveExt = if ($targetFormat -eq "zip") { ".zip" } else { ".tar.gz" }
    $archivePath = Join-Path $releaseRoot "$packageName$archiveExt"
    Compress-Package -PackageDir $packageDir -ArchivePath $archivePath -Format $targetFormat
    Add-Checksum -ArchivePath $archivePath -Lines $checksums
    Remove-Item -LiteralPath $packageDir -Recurse -Force
  }
}

Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
Remove-Item Env:\GOARM -ErrorAction SilentlyContinue

$checksumPath = Join-Path $releaseRoot "gpufleet_$versionValue`_checksums.txt"
$checksums | Set-Content -LiteralPath $checksumPath -Encoding ASCII

Write-Host "Release artifacts written to $releaseRoot"
Get-ChildItem -LiteralPath $releaseRoot | Select-Object Name, Length
