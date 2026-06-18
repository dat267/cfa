$ErrorActionPreference = 'Stop'

$Repo = "dat267/cfa"
$BinaryName = "cfa"

# Detect Architecture
$Arch = "amd64"
if ([System.Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
        $Arch = "arm64"
    }
} else {
    Write-Error "Unsupported architecture: 32-bit Windows is not supported."
}

$AssetName = "${BinaryName}-windows-${Arch}.exe"

Write-Host "Fetching latest release information from GitHub..."
$Releases = Invoke-RestMethod -Uri "https://api.github.com/repos/${Repo}/releases"
$Tag = $Releases[0].tag_name

if (-not $Tag) {
    Write-Error "Could not retrieve the latest release tag."
}

Write-Host "Latest release tag: $Tag"

$DownloadUrl = "https://github.com/${Repo}/releases/download/${Tag}/${AssetName}"
$InstallDir = Join-Path $env:USERPROFILE "bin"

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

$DestPath = Join-Path $InstallDir "${BinaryName}.exe"

Write-Host "Downloading $AssetName from $DownloadUrl..."
Invoke-WebRequest -Uri $DownloadUrl -OutFile $DestPath

Write-Host ""
Write-Host "Successfully installed $BinaryName.exe to $DestPath" -ForegroundColor Green
Write-Host ""
Write-Host "To run '$BinaryName' from anywhere, make sure '$InstallDir' is in your User PATH variable." -ForegroundColor Yellow
Write-Host "You can add it by running this PowerShell command in an Administrator session (or User scope):"
Write-Host "  [System.Environment]::SetEnvironmentVariable('PATH', `$env:PATH + ';$InstallDir', 'User')" -ForegroundColor Cyan
