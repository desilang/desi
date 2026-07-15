<#
.SYNOPSIS
    One-shot developer bootstrap for building Desi from source on Windows.

.DESCRIPTION
    Checks for the toolchain Desi needs, installs anything missing, then
    builds the compiler + runtime. Optionally runs the example test suite.

    Installs (only what's missing):
      - Go           (compiler + tools)
      - LLVM / Clang (LLVM IR -> object code + linking)
      - Visual Studio 2022 Build Tools, C++ workload (cl.exe for the runtime)

    Prerequisites obtained via winget when available, with direct official
    downloads as a fallback. The script self-elevates (UAC) because the
    Visual Studio Build Tools installer requires administrator rights.

    This is for building FROM SOURCE. To just USE Desi, install the
    prebuilt binaries instead:  irm https://desilang.org/install.ps1 | iex

.PARAMETER Test
    Run the example test suite (test_examples.ps1) after a successful build.

.PARAMETER SkipBuild
    Install/verify the toolchain only; do not build.

.PARAMETER Clean
    Remove previous build artifacts before building (build.ps1 -Clean).

.PARAMETER NoElevate
    Do not self-elevate. Detection and portable installs still work, but
    installing Visual Studio Build Tools will fail without admin rights.

.EXAMPLE
    .\bootstrap.ps1                # install what's missing, then build
    .\bootstrap.ps1 -Test          # ...and run the example suite
    .\bootstrap.ps1 -SkipBuild     # just set up the toolchain
    .\bootstrap.ps1 -Clean -Test   # clean build + full suite
#>
[CmdletBinding()]
param(
    [switch]$Test,
    [switch]$SkipBuild,
    [switch]$Clean,
    [switch]$NoElevate
)

$ErrorActionPreference = "Stop"
$ProjectRoot = $PSScriptRoot

# ── Console helpers ────────────────────────────────────────────────
function Write-Step($m)    { Write-Host "==> $m" -ForegroundColor Cyan }
function Write-Ok($m)      { Write-Host "[OK] $m" -ForegroundColor Green }
function Write-Warn2($m)   { Write-Host "[!]  $m" -ForegroundColor Yellow }
function Write-Info($m)    { Write-Host "     $m" -ForegroundColor Gray }

# ── Admin / self-elevation ─────────────────────────────────────────
function Test-Admin {
    $id = [Security.Principal.WindowsIdentity]::GetCurrent()
    return ([Security.Principal.WindowsPrincipal]$id).IsInRole(
        [Security.Principal.WindowsBuiltInRole]::Administrator)
}

if (-not $NoElevate -and -not (Test-Admin)) {
    Write-Step "Requesting administrator rights (needed to install Build Tools)..."
    $argList = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "`"$PSCommandPath`"")
    if ($Test)      { $argList += "-Test" }
    if ($SkipBuild) { $argList += "-SkipBuild" }
    if ($Clean)     { $argList += "-Clean" }
    try {
        Start-Process -FilePath "powershell.exe" -ArgumentList $argList -Verb RunAs
    } catch {
        Write-Warn2 "Elevation was declined. Re-run as administrator, or use -NoElevate"
        Write-Warn2 "if Go/LLVM/Build Tools are already installed."
        exit 1
    }
    exit 0
}

Set-Location $ProjectRoot

# ── PATH helpers ───────────────────────────────────────────────────
function Update-SessionPath {
    $machine = [Environment]::GetEnvironmentVariable("Path", "Machine")
    $user    = [Environment]::GetEnvironmentVariable("Path", "User")
    $env:Path = (@($machine, $user) | Where-Object { $_ }) -join ";"
}

function Add-ToMachinePath($dir) {
    $machine = [Environment]::GetEnvironmentVariable("Path", "Machine")
    if ($machine -notlike "*$dir*") {
        [Environment]::SetEnvironmentVariable("Path", "$machine;$dir", "Machine")
        Write-Info "Added $dir to system PATH"
    }
    Update-SessionPath
}

# ── winget availability ────────────────────────────────────────────
$WingetAvailable = [bool](Get-Command winget -ErrorAction SilentlyContinue)
function Invoke-Winget {
    param([string]$Id, [string]$Override)
    if (-not $WingetAvailable) { return $false }
    $args = @("install", "--id", $Id, "--exact", "--silent",
              "--accept-source-agreements", "--accept-package-agreements")
    if ($Override) { $args += @("--override", $Override) }
    Write-Info "winget install $Id ..."
    & winget @args
    # winget: 0 = ok; -1978335189 = already installed (also treat as ok)
    return ($LASTEXITCODE -eq 0 -or $LASTEXITCODE -eq -1978335189)
}

# ── Tool detection ─────────────────────────────────────────────────
function Find-Go {
    $c = @(
        (Get-Command go -ErrorAction SilentlyContinue).Source,
        "C:\Program Files\Go\bin\go.exe",
        "C:\Go\bin\go.exe"
    ) + (Get-ChildItem "C:\gosdks" -Directory -ErrorAction SilentlyContinue |
         ForEach-Object { Join-Path $_.FullName "bin\go.exe" })
    foreach ($p in $c) { if ($p -and (Test-Path $p)) { return (Split-Path $p) } }
    return $null
}

function Find-Clang {
    $cmd = Get-Command clang -ErrorAction SilentlyContinue
    if ($cmd) { return (Split-Path $cmd.Source) }
    foreach ($d in @("C:\Program Files\LLVM\bin", "C:\LLVM\bin")) {
        if (Test-Path (Join-Path $d "clang.exe")) { return $d }
    }
    return $null
}

function Find-MSVC {
    $vswhere = "${env:ProgramFiles(x86)}\Microsoft Visual Studio\Installer\vswhere.exe"
    if (-not (Test-Path $vswhere)) { return $null }
    $path = & $vswhere -latest -products * `
        -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 `
        -property installationPath 2>$null
    if ($path) { return $path.Trim() }
    return $null
}

# ── Direct-download fallbacks ──────────────────────────────────────
function Install-GoDirect {
    Write-Info "Downloading Go from go.dev ..."
    $ver = "go1.25.0"
    try {
        $rel = (Invoke-RestMethod "https://go.dev/dl/?mode=json")[0].version
        if ($rel) { $ver = $rel }
    } catch { Write-Warn2 "Version lookup failed; using pinned $ver" }
    $zip = "$ver.windows-amd64.zip"
    $tmp = Join-Path $env:TEMP $zip
    Invoke-WebRequest "https://go.dev/dl/$zip" -OutFile $tmp -UseBasicParsing
    $dest = "C:\gosdks\$ver"
    New-Item -ItemType Directory -Force -Path "C:\gosdks" | Out-Null
    if (Test-Path $dest) { Remove-Item -Recurse -Force $dest }
    tar -xf $tmp -C "C:\gosdks"
    if (Test-Path "C:\gosdks\go") { Rename-Item "C:\gosdks\go" $ver }
    Remove-Item $tmp -ErrorAction SilentlyContinue
    Add-ToMachinePath (Join-Path $dest "bin")
}

function Install-LlvmDirect {
    Write-Info "Downloading LLVM from github.com/llvm/llvm-project (~800 MB, this is slow) ..."
    $api = Invoke-RestMethod "https://api.github.com/repos/llvm/llvm-project/releases?per_page=10"
    $asset = $null
    foreach ($r in $api) {
        $asset = $r.assets | Where-Object { $_.name -like "clang+llvm-*-x86_64-pc-windows-msvc.tar.xz" } | Select-Object -First 1
        if ($asset) { break }
    }
    if (-not $asset) { throw "Could not find an LLVM Windows release asset." }
    $tmp = Join-Path $env:TEMP $asset.name
    Invoke-WebRequest $asset.browser_download_url -OutFile $tmp -UseBasicParsing
    if (Test-Path "C:\LLVM") { Remove-Item -Recurse -Force "C:\LLVM" }
    tar -xf $tmp -C "C:\"
    $extracted = Get-ChildItem "C:\" -Directory -Filter "clang+llvm-*" | Select-Object -First 1
    if ($extracted) { Rename-Item $extracted.FullName "C:\LLVM" }
    Remove-Item $tmp -ErrorAction SilentlyContinue
    Add-ToMachinePath "C:\LLVM\bin"
}

function Install-BuildToolsDirect {
    Write-Info "Downloading Visual Studio Build Tools installer ..."
    $exe = Join-Path $env:TEMP "vs_BuildTools.exe"
    Invoke-WebRequest "https://aka.ms/vs/17/release/vs_BuildTools.exe" -OutFile $exe -UseBasicParsing
    Write-Info "Installing C++ workload (this can take several minutes) ..."
    $p = Start-Process -FilePath $exe -Wait -PassThru -ArgumentList @(
        "--quiet", "--wait", "--norestart",
        "--add", "Microsoft.VisualStudio.Workload.VCTools", "--includeRecommended")
    Remove-Item $exe -ErrorAction SilentlyContinue
    if ($p.ExitCode -ne 0 -and $p.ExitCode -ne 3010) {
        throw "Build Tools installer exited with code $($p.ExitCode)."
    }
}

# ── Ensure each prerequisite ───────────────────────────────────────
Write-Host ""
Write-Host "  Desi — build-from-source bootstrap (Windows)" -ForegroundColor White
Write-Host ""

# git (needed to have cloned this repo; just verify + advise)
Write-Step "Checking Git..."
if (Get-Command git -ErrorAction SilentlyContinue) {
    Write-Ok "Git found"
} else {
    Write-Warn2 "Git not found. Install it (winget install Git.Git) and re-clone if needed."
}

# Go
Write-Step "Checking Go..."
if (Find-Go) {
    Write-Ok "Go found"
} else {
    if (-not (Invoke-Winget -Id "GoLang.Go")) { Install-GoDirect }
    Update-SessionPath
    if (Find-Go) { Write-Ok "Go installed" } else { throw "Go install did not complete." }
}

# LLVM / Clang
Write-Step "Checking LLVM / Clang..."
if (Find-Clang) {
    Write-Ok "Clang found"
} else {
    if (-not (Invoke-Winget -Id "LLVM.LLVM")) { Install-LlvmDirect }
    # winget's LLVM package may not add itself to PATH — make sure desic can find it
    if (Test-Path "C:\Program Files\LLVM\bin") { Add-ToMachinePath "C:\Program Files\LLVM\bin" }
    Update-SessionPath
    if (Find-Clang) { Write-Ok "Clang installed" } else { throw "LLVM install did not complete." }
}

# Visual Studio Build Tools (C++ / cl.exe)
Write-Step "Checking Visual Studio Build Tools (C++)..."
if (Find-MSVC) {
    Write-Ok "MSVC C++ tools found"
} else {
    $vsOverride = "--quiet --wait --norestart --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended"
    if (-not (Invoke-Winget -Id "Microsoft.VisualStudio.2022.BuildTools" -Override $vsOverride)) {
        Install-BuildToolsDirect
    }
    if (Find-MSVC) { Write-Ok "MSVC C++ tools installed" } else { throw "Build Tools install did not complete." }
}

Write-Host ""
Write-Ok "Toolchain ready."
Write-Host ""

# ── Build ──────────────────────────────────────────────────────────
if ($SkipBuild) {
    Write-Info "-SkipBuild set; not building."
} else {
    if ($Clean) {
        Write-Step "Cleaning previous build..."
        & "$ProjectRoot\build.ps1" -Clean
    }
    Write-Step "Building Desi (runtime + tools)..."
    & "$ProjectRoot\build.ps1"
    if ($LASTEXITCODE -ne 0) { throw "build.ps1 failed." }
    Write-Ok "Build complete: bin\desic.exe"

    if ($Test) {
        Write-Step "Running example test suite..."
        & "$ProjectRoot\test_examples.ps1"
        # test_examples.ps1 exits non-zero if any test fails; surface but don't throw
        if ($LASTEXITCODE -ne 0) { Write-Warn2 "Some example tests failed (see output above)." }
        else { Write-Ok "All example tests passed." }
    }
}

Write-Host ""
Write-Host "  Done. Open a NEW terminal so PATH changes take effect, then:" -ForegroundColor Green
Write-Host "    .\bin\desic.exe --version"
Write-Host "    .\build-desi.ps1 examples\000_hello_world.desi   # compile a program"
Write-Host ""

# Keep the elevated window open so output stays readable.
if ((Test-Admin) -and -not $NoElevate) {
    Read-Host "Press Enter to close this window"
}
