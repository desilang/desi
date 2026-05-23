<#
.SYNOPSIS
    Build script for Desi Language on Windows
.DESCRIPTION
    Builds the mpdecimal library, C runtime, and Go compiler tools.
    Equivalent to 'make' on macOS/Linux.
.EXAMPLE
    .\build.ps1
    .\build.ps1 -Clean
    .\build.ps1 -SkipRuntime  # Only build Go tools
#>

param(
    [switch]$Clean,
    [switch]$SkipRuntime,
    [switch]$Verbose
)

$ErrorActionPreference = "Stop"

# Directories
$ProjectRoot = $PSScriptRoot
$BuildDir = Join-Path $ProjectRoot "build"
$BinDir = Join-Path $ProjectRoot "bin"
$RuntimeSrc = Join-Path $ProjectRoot "compiler\runtime"
$DecimalSrc = Join-Path $RuntimeSrc "decimal"
$DecimalLib = Join-Path $DecimalSrc "lib"
$DecimalInclude = Join-Path $DecimalSrc "include"
$MpdecimalSrc = Join-Path $DecimalSrc "mpdecimal-4.0.1"

# Output files
$LibDesi = Join-Path $BuildDir "libdesi.lib"
$MpdecLibWin = Join-Path $DecimalLib "libmpdec.lib"

# Find Go executable
$GoExe = $null
$goPaths = @(
    "C:\gosdks\go1.25.0\bin\go.exe",
    "C:\Program Files\Go\bin\go.exe",
    "C:\Go\bin\go.exe",
    (Get-Command "go" -ErrorAction SilentlyContinue).Source
)
foreach ($p in $goPaths) {
    if ($p -and (Test-Path $p)) {
        $GoExe = $p
        break
    }
}
if (-not $GoExe) {
    throw "Go not found. Please install Go or add it to PATH."
}

function Write-Step($message) {
    Write-Host "==> $message" -ForegroundColor Cyan
}

function Write-Success($message) {
    Write-Host "[OK] $message" -ForegroundColor Green
}

function Write-Failure($message) {
    Write-Host "[FAIL] $message" -ForegroundColor Red
}

# Clean build
if ($Clean) {
    Write-Step "Cleaning build directories..."
    if (Test-Path $BuildDir) { Remove-Item -Recurse -Force $BuildDir }
    if (Test-Path $BinDir) { Remove-Item -Recurse -Force $BinDir }
    if (Test-Path (Join-Path $ProjectRoot "gen")) { Remove-Item -Recurse -Force (Join-Path $ProjectRoot "gen") }
    Write-Success "Clean complete"
    exit 0
}

# Create directories
Write-Step "Creating directories..."
New-Item -ItemType Directory -Force -Path $BuildDir | Out-Null
New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
New-Item -ItemType Directory -Force -Path $DecimalLib -ErrorAction SilentlyContinue | Out-Null
New-Item -ItemType Directory -Force -Path $DecimalInclude -ErrorAction SilentlyContinue | Out-Null

# Find Visual Studio
function Get-VsPath {
    $vswhere = "${env:ProgramFiles(x86)}\Microsoft Visual Studio\Installer\vswhere.exe"
    if (Test-Path $vswhere) {
        $vsPath = & $vswhere -latest -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath
        return $vsPath
    }
    return $null
}

function Get-VcVarsPath {
    $vsPath = Get-VsPath
    if ($vsPath) {
        return Join-Path $vsPath "VC\Auxiliary\Build\vcvars64.bat"
    }
    return $null
}

# Run command in VS Developer environment
function Invoke-VsCommand {
    param([string]$Command)
    
    $vcvars = Get-VcVarsPath
    if (-not $vcvars -or -not (Test-Path $vcvars)) {
        throw "Visual Studio Build Tools not found. Please install Visual Studio Build Tools with C++ workload."
    }
    
    $tempScript = [System.IO.Path]::GetTempFileName() + ".bat"
    
    # Build batch file content using array and join
    $batchContent = @(
        '@echo off',
        "call `"$vcvars`" >nul 2>&1",
        $Command,
        'exit /b %ERRORLEVEL%'
    )
    $batchContent -join "`r`n" | Set-Content -Path $tempScript -Encoding ASCII
    
    try {
        # Use Start-Process to avoid PowerShell treating stderr as errors
        $pinfo = New-Object System.Diagnostics.ProcessStartInfo
        $pinfo.FileName = "cmd.exe"
        $pinfo.Arguments = "/c `"$tempScript`""
        $pinfo.RedirectStandardOutput = $true
        $pinfo.RedirectStandardError = $true
        $pinfo.UseShellExecute = $false
        $pinfo.CreateNoWindow = $true
        
        $process = New-Object System.Diagnostics.Process
        $process.StartInfo = $pinfo
        $process.Start() | Out-Null
        
        $stdout = $process.StandardOutput.ReadToEnd()
        $stderr = $process.StandardError.ReadToEnd()
        $process.WaitForExit()
        
        $exitCode = $process.ExitCode
        if ($exitCode -ne 0) {
            throw "Command failed with exit code $exitCode`nStdout: $stdout`nStderr: $stderr"
        }
        return $stdout
    }
    finally {
        Remove-Item $tempScript -ErrorAction SilentlyContinue
    }
}

if (-not $SkipRuntime) {
    # Build mpdecimal if not already built
    if (-not (Test-Path $MpdecLibWin)) {
        Write-Step "Building mpdecimal library (first time setup)..."
        
        # Check if mpdecimal source exists
        if (-not (Test-Path $MpdecimalSrc)) {
            throw "mpdecimal source not found at $MpdecimalSrc"
        }
        
        $libmpdecDir = Join-Path $MpdecimalSrc "libmpdec"
        
        # Compile mpdecimal using MSVC
        Push-Location $libmpdecDir
        try {
            # Copy Makefile.vc to Makefile
            Copy-Item "Makefile.vc" "Makefile" -Force
            
            # Build using nmake in VS environment
            Write-Host "  Compiling mpdecimal with MSVC..." -ForegroundColor Gray
            # Run nmake clean (ignore errors) then nmake
            try { Invoke-VsCommand "cd /d `"$libmpdecDir`" `& nmake clean" } catch { }
            $result = Invoke-VsCommand "cd /d `"$libmpdecDir`" `& nmake MACHINE=x64"
            if ($Verbose) { Write-Host $result }
            
            # Copy the built library
            $builtLib = Join-Path $libmpdecDir "libmpdec-4.0.1.lib"
            if (Test-Path $builtLib) {
                Copy-Item $builtLib $MpdecLibWin -Force
                Copy-Item (Join-Path $libmpdecDir "mpdecimal.h") $DecimalInclude -Force
                Write-Success "mpdecimal built successfully"
            } else {
                throw "mpdecimal build failed - library not found"
            }
        }
        finally {
            Pop-Location
        }
    } else {
        Write-Host "  mpdecimal already built, skipping..." -ForegroundColor Gray
    }
    
    # Build Desi runtime
    Write-Step "Building Desi runtime library..."
    
    # Get all .c files in runtime directory (excluding decimal subdirectory)
    # Skip files that require Unix-only APIs (sockets, signals, POSIX regex, etc.)
    $windowsExcludes = @(
        'desi_host.c', 'http_server.c', 'tls.c', 'net.c', 'websocket.c',
        'signal_handler.c', 'reload.c', 'fs.c', 'os.c', 'path.c',
        'process.c', 'random.c', 're.c', 'shell.c', 'uuid.c'
    )
    $runtimeFiles = Get-ChildItem -Path $RuntimeSrc -Filter "*.c" -File |
        Where-Object { $windowsExcludes -notcontains $_.Name }
    $decimalWrapper = Join-Path $DecimalSrc "desi_decimal.c"
    
    $objectFiles = @()
    
    # Compile each runtime .c file
    foreach ($cFile in $runtimeFiles) {
        $objFile = Join-Path $BuildDir ($cFile.BaseName + ".obj")
        Write-Host "  Compiling $($cFile.Name)..." -ForegroundColor Gray
        
        $compileCmd = "cl.exe /nologo /c /O2 /std:c17 /experimental:c11atomics /DNDEBUG `"$($cFile.FullName)`" /Fo`"$objFile`""
        $result = Invoke-VsCommand $compileCmd
        if ($Verbose) { Write-Host $result }
        
        $objectFiles += $objFile
    }
    
    # Compile decimal wrapper
    $decimalObj = Join-Path $BuildDir "desi_decimal.obj"
    Write-Host "  Compiling desi_decimal.c..." -ForegroundColor Gray
    $compileCmd = "cl.exe /nologo /c /O2 /std:c17 /experimental:c11atomics /DNDEBUG /I`"$DecimalSrc`" /I`"$DecimalInclude`" `"$decimalWrapper`" /Fo`"$decimalObj`""
    $result = Invoke-VsCommand $compileCmd
    if ($Verbose) { Write-Host $result }
    $objectFiles += $decimalObj
    
    # Create static library
    Write-Host "  Creating static library..." -ForegroundColor Gray
    $objList = ($objectFiles | ForEach-Object { "`"$_`"" }) -join " "
    $libCmd = "lib.exe /nologo /OUT:`"$LibDesi`" $objList `"$MpdecLibWin`""
    $result = Invoke-VsCommand $libCmd
    if ($Verbose) { Write-Host $result }
    
    if (Test-Path $LibDesi) {
        Write-Success "Runtime library built: $LibDesi"
    } else {
        throw "Failed to create runtime library"
    }
}

# Build Go compiler tools
Write-Step "Building desic..."
& $GoExe build -ldflags="-s -w" -o (Join-Path $BinDir "desic.exe") ./compiler/cmd/desic
if ($LASTEXITCODE -ne 0) { throw "Failed to build desic" }

Write-Step "Building desifmt..."
& $GoExe build -ldflags="-s -w" -o (Join-Path $BinDir "desifmt.exe") ./compiler/cmd/desifmt
if ($LASTEXITCODE -ne 0) { throw "Failed to build desifmt" }

Write-Step "Building desirepl..."
& $GoExe build -ldflags="-s -w" -o (Join-Path $BinDir "desirepl.exe") ./compiler/cmd/desirepl
if ($LASTEXITCODE -ne 0) { throw "Failed to build desirepl" }

Write-Step "Building desilsp..."
& $GoExe build -ldflags="-s -w" -o (Join-Path $BinDir "desilsp.exe") ./compiler/cmd/desilsp
if ($LASTEXITCODE -ne 0) { throw "Failed to build desilsp" }

Write-Success "All tools built successfully!"

# Summary
Write-Host ""
Write-Host "Build complete!" -ForegroundColor Green
Write-Host "  Compiler:  $BinDir\desic.exe"
Write-Host "  Formatter: $BinDir\desifmt.exe"
Write-Host "  REPL:      $BinDir\desirepl.exe"
Write-Host "  LSP:       $BinDir\desilsp.exe"
if (-not $SkipRuntime) {
    Write-Host "  Runtime:   $LibDesi"
}
Write-Host ""
Write-Host "To compile a Desi program:"
Write-Host "  .\build-desi.ps1 your_program.desi"
