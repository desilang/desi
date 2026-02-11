# Desi Installer for Windows — https://desilang.org
# Usage: irm https://desilang.org/install.ps1 | iex
#
# Installs desic, desifmt, and desilsp to %USERPROFILE%\.desi\bin
# and adds it to the user PATH.
#
# Environment variables:
#   DESI_INSTALL_DIR  — Override install directory (default: $env:USERPROFILE\.desi)
#   DESI_VERSION      — Install a specific version (default: latest)

$ErrorActionPreference = "Stop"

# ── Detect Architecture ────────────────────────────────────────────
function Get-Platform {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
    switch ($arch) {
        "X64"   { return "amd64" }
        "Arm64" { return "arm64" }
        default { throw "Unsupported architecture: $arch" }
    }
}

# ── Resolve Version ────────────────────────────────────────────────
function Get-LatestVersion {
    if ($env:DESI_VERSION) {
        return $env:DESI_VERSION
    }

    Write-Host "▸ Fetching latest release..." -ForegroundColor Cyan
    try {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/desilang/desi/releases/latest" -Headers @{ Accept = "application/json" }
        return $release.tag_name
    } catch {
        throw "Could not determine latest version. Set `$env:DESI_VERSION manually."
    }
}

# ── Main Install ───────────────────────────────────────────────────
function Install-Desi {
    Write-Host ""
    Write-Host "  Desi Installer" -ForegroundColor White -BackgroundColor DarkRed
    Write-Host ""

    $arch = Get-Platform
    $version = Get-LatestVersion
    $platform = "windows-$arch"

    $installDir = if ($env:DESI_INSTALL_DIR) { $env:DESI_INSTALL_DIR } else { Join-Path $env:USERPROFILE ".desi" }
    $binDir = Join-Path $installDir "bin"
    New-Item -ItemType Directory -Path $binDir -Force | Out-Null

    # Download
    $filename = "desi-$version-$platform"
    $archive = "$filename.zip"
    $url = "https://github.com/desilang/desi/releases/download/$version/$archive"

    Write-Host "▸ Downloading Desi $version for $platform..." -ForegroundColor Cyan
    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) "desi-install-$(Get-Random)"
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
    $archivePath = Join-Path $tmpDir $archive

    try {
        Invoke-WebRequest -Uri $url -OutFile $archivePath -UseBasicParsing
    } catch {
        throw "Download failed. Check version ($version) and platform ($platform)."
    }

    # Extract
    Write-Host "▸ Extracting..." -ForegroundColor Cyan
    Expand-Archive -Path $archivePath -DestinationPath $tmpDir -Force

    # Copy binaries
    $srcDir = Join-Path $tmpDir $filename
    foreach ($tool in @("desic.exe", "desifmt.exe", "desilsp.exe")) {
        $src = Join-Path $srcDir $tool
        if (Test-Path $src) {
            Copy-Item $src -Destination (Join-Path $binDir $tool) -Force
        }
    }

    # Cleanup
    Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue

    Write-Host "✓ Installed to $binDir" -ForegroundColor Green

    # Update PATH
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($currentPath -notlike "*$binDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$binDir;$currentPath", "User")
        Write-Host "⚠ Added $binDir to user PATH — restart your terminal to apply." -ForegroundColor Yellow
    }

    Write-Host ""
    Write-Host "  Desi $version installed successfully!" -ForegroundColor Green
    Write-Host ""
    Write-Host "  Run " -NoNewline
    Write-Host "desic --version" -ForegroundColor Cyan -NoNewline
    Write-Host " to verify."
    Write-Host "  Get started: " -NoNewline
    Write-Host "https://desilang.org/getting-started/" -ForegroundColor Cyan
    Write-Host ""
}

Install-Desi
