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

    # The archive holds bin\ and lib\. Both matter: lib\ carries libdesi.lib
    # and entry.obj, which every compiled program links against, and desic
    # looks for them at ..\lib relative to itself — so the two must stay
    # siblings under the install directory.
    $srcDir = Join-Path $tmpDir $filename
    $srcBin = Join-Path $srcDir "bin"
    $srcLib = Join-Path $srcDir "lib"
    if (-not (Test-Path $srcBin) -or -not (Test-Path $srcLib)) {
        throw "Archive layout unexpected: $filename has no bin\ and lib\. Please report this at https://github.com/desilang/desi/issues"
    }

    $libDir = Join-Path $installDir "lib"
    New-Item -ItemType Directory -Path $libDir -Force | Out-Null

    Copy-Item (Join-Path $srcBin "*") -Destination $binDir -Recurse -Force
    Copy-Item (Join-Path $srcLib "*") -Destination $libDir -Recurse -Force

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

    # Desi compiles to native code: desic emits LLVM IR and then calls clang
    # to assemble and link it. Without clang the install still succeeds, but
    # the first `desic build` fails — so say so here rather than there.
    if (-not (Get-Command clang -ErrorAction SilentlyContinue)) {
        Write-Host "  clang was not found on your PATH." -ForegroundColor Yellow
        Write-Host "  Desi compiles through LLVM, so clang is required to build programs."
        Write-Host "  Install it with: " -NoNewline
        Write-Host "winget install LLVM.LLVM" -ForegroundColor Cyan
        Write-Host "  You also need the MSVC linker, from Visual Studio Build Tools"
        Write-Host "  with the 'Desktop development with C++' workload."
        Write-Host ""
    }

    Write-Host "  Run " -NoNewline
    Write-Host "desic version" -ForegroundColor Cyan -NoNewline
    Write-Host " to verify."
    Write-Host "  Get started: " -NoNewline
    Write-Host "https://desilang.org/getting-started/" -ForegroundColor Cyan
    Write-Host ""
}

Install-Desi
