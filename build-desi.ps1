<#
.SYNOPSIS
    Build a Desi program into an executable (Windows)
.DESCRIPTION
    Compiles a .desi file to LLVM IR, then to an object file, and links it
    with the Desi runtime library to create an executable.
    Equivalent to './build-desi.sh' on macOS/Linux.
.EXAMPLE
    .\build-desi.ps1 examples\01_hello_world.desi
    .\build-desi.ps1 examples\01_hello_world.desi hello
#>

param(
    [Parameter(Mandatory=$true, Position=0)]
    [string]$InputFile,
    
    [Parameter(Position=1)]
    [string]$OutputName
)

$ErrorActionPreference = "Stop"

# Directories
$ProjectRoot = $PSScriptRoot
$BuildDir = Join-Path $ProjectRoot "build"
$OutputDir = Join-Path $BuildDir "output"
$BinDir = Join-Path $ProjectRoot "bin"
$DecimalLib = Join-Path $ProjectRoot "compiler\runtime\decimal\lib"

# Tools
$Desic = Join-Path $BinDir "desic.exe"
$LibDesi = Join-Path $BuildDir "libdesi.lib"
$MpdecLib = Join-Path $DecimalLib "libmpdec.lib"

# Derive output name from input if not specified
if (-not $OutputName) {
    $OutputName = [System.IO.Path]::GetFileNameWithoutExtension($InputFile)
}

# Validate prerequisites
if (-not (Test-Path $Desic)) {
    Write-Host "Error: Compiler not found at $Desic" -ForegroundColor Red
    Write-Host "Please run '.\build.ps1' first to build the compiler." -ForegroundColor Yellow
    exit 1
}

if (-not (Test-Path $LibDesi)) {
    Write-Host "Error: Runtime library not found at $LibDesi" -ForegroundColor Red
    Write-Host "Please run '.\build.ps1' first to build the runtime." -ForegroundColor Yellow
    exit 1
}

if (-not (Test-Path $InputFile)) {
    Write-Host "Error: Input file not found: $InputFile" -ForegroundColor Red
    exit 1
}

# Check for LLVM clang - search common paths
$llvmPaths = @(
    "C:\Program Files\LLVM\bin",
    "C:\LLVM\bin",
    $env:LLVM_PATH
)

$ClangExe = $null

# Try PATH first
$clangCmd = Get-Command "clang" -ErrorAction SilentlyContinue
if ($clangCmd) { $ClangExe = $clangCmd.Source }

# Search common LLVM paths if not found
if (-not $ClangExe) {
    foreach ($p in $llvmPaths) {
        if ($p -and (Test-Path $p)) {
            if (Test-Path (Join-Path $p "clang.exe")) {
                $ClangExe = Join-Path $p "clang.exe"
                break
            }
        }
    }
}

if (-not $ClangExe) {
    Write-Host "Error: 'clang' not found. Please ensure LLVM is installed." -ForegroundColor Red
    Write-Host "  Download from: https://releases.llvm.org/" -ForegroundColor Yellow
    exit 1
}

# Create output directory
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

# Intermediate files
$LlvmIr = Join-Path $BuildDir "program.ll"
$Executable = Join-Path $OutputDir "$OutputName.exe"

try {
    # Step 1: Compile Desi to LLVM IR
    Write-Host "==> Compiling Desi to LLVM IR..." -ForegroundColor Cyan
    $irOutput = & $Desic emit-ir $InputFile 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Error: Desi compilation failed" -ForegroundColor Red
        Write-Host $irOutput
        exit 1
    }
    $irOutput | Set-Content -Path $LlvmIr -Encoding ASCII
    
    # Step 2: Compile LLVM IR and link executable (clang can do both in one step)
    Write-Host "==> Compiling and linking executable..." -ForegroundColor Cyan
    # Clang can compile LLVM IR directly, no need for separate llc step
    # Temporarily disable ErrorActionPreference to prevent stderr warnings from causing exceptions
    $oldEAP = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $clangOutput = & $ClangExe $LlvmIr $LibDesi -o $Executable -ffunction-sections -fdata-sections -Wl,--gc-sections 2>&1
    $clangExit = $LASTEXITCODE
    $ErrorActionPreference = $oldEAP
    if ($clangExit -ne 0) {
        Write-Host "Error: Compilation/Linking failed" -ForegroundColor Red
        Write-Host $clangOutput
        exit 1
    }
    
    # Cleanup intermediate files
    Write-Host "==> Cleaning up intermediate files..." -ForegroundColor Cyan
    Remove-Item $LlvmIr -ErrorAction SilentlyContinue
    
    Write-Host "[OK] Built executable: $Executable" -ForegroundColor Green
    Write-Host "  Run with: $Executable" -ForegroundColor Gray
}
catch {
    Write-Host "Error: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}
