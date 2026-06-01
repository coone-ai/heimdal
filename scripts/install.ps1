Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# Heimdal CLI installer for Windows PowerShell
# Usage:
#   irm https://raw.githubusercontent.com/coone-ai/heimdal/main/scripts/install.ps1 | iex
#
# Optional environment variables:
#   HEIMDAL_REPO      GitHub repo in owner/name format (default: coone-ai/heimdal)
#   HEIMDAL_VERSION   Release tag to install, e.g. v0.0.1 (default: latest)
#   HEIMDAL_INSTALL   Install directory (default: %LOCALAPPDATA%\Programs\heimdal)

$Repo = if ($env:HEIMDAL_REPO) { $env:HEIMDAL_REPO } else { "coone-ai/heimdal" }
$Version = $env:HEIMDAL_VERSION
$InstallDir = if ($env:HEIMDAL_INSTALL) { $env:HEIMDAL_INSTALL } else { Join-Path $env:LOCALAPPDATA "Programs\heimdal" }
$BinaryName = "heimdal.exe"
$AliasName = "coval.exe"

if (-not $Version) {
  Write-Host "Resolving latest release tag..."
  $latest = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
  $Version = $latest.tag_name
}

if (-not $Version) {
  throw "Failed to resolve release tag from GitHub."
}

$archRaw = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
switch ($archRaw) {
  "x64" { $Arch = "amd64" }
  "arm64" { $Arch = "arm64" }
  default { throw "Unsupported architecture: $archRaw" }
}

$Archive = "heimdal_${Version}_windows_${Arch}.zip"
$Url = "https://github.com/$Repo/releases/download/$Version/$Archive"

Write-Host "Downloading $Url"
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("heimdal-install-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempRoot | Out-Null

try {
  $zipPath = Join-Path $tempRoot $Archive
  $extractDir = Join-Path $tempRoot "extract"
  Invoke-WebRequest -Uri $Url -OutFile $zipPath
  Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force

  $sourceBin = Join-Path $extractDir $BinaryName
  if (-not (Test-Path $sourceBin)) {
    throw "Binary not found in archive: $Archive"
  }

  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  $targetBin = Join-Path $InstallDir $BinaryName
  $aliasBin = Join-Path $InstallDir $AliasName
  Copy-Item -Path $sourceBin -Destination $targetBin -Force
  Copy-Item -Path $sourceBin -Destination $aliasBin -Force

  $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
  if (-not $userPath) {
    $userPath = ""
  }
  $pathParts = $userPath -split ";" | Where-Object { $_ -ne "" }
  if (-not ($pathParts -contains $InstallDir)) {
    $newPath = if ($userPath) { "$userPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "Added to user PATH: $InstallDir"
  }

  Write-Host ""
  Write-Host "Installed: $targetBin"
  Write-Host "Alias: $aliasBin"
  & $targetBin version
  Write-Host ""
  Write-Host "Open a new terminal if 'heimdal' or 'coval' is not found in your current session."
}
finally {
  Remove-Item -Path $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}
