<#
.SYNOPSIS
    Run the Desi example suite in parallel (Windows)
.DESCRIPTION
    Splits the example suite across N worker processes and aggregates the
    results. Each worker is the ordinary test_examples.ps1 running with
    -Shard/-ShardCount, so the compile/run/compare logic has exactly one
    implementation — this script only partitions work and sums the totals.

    Every test is a full compile + link against libdesi.lib, which is CPU
    bound and almost entirely serial otherwise. Workers keep their
    intermediates in build\shard<N>\ so concurrent compiles never share
    program.ll or test_exec.exe.
.PARAMETER Jobs
    Worker count. Defaults to half the logical processors (each worker also
    spawns clang/lld, which are themselves multi-threaded).
.PARAMETER Release
    Run the optimized (-O2 + LTO) leg by setting DESI_RELEASE=1 in workers.
.EXAMPLE
    .\test_examples_parallel.ps1
    .\test_examples_parallel.ps1 -Jobs 8
    .\test_examples_parallel.ps1 -Release
#>

param(
    [int]$Jobs = 0,
    [int]$Start = 0,
    [int]$End = 9999,
    [string]$Range,
    [int]$TimeoutSec = 120,
    [switch]$Release
)

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

# Mirror build-desi.ps1: DESI_RELEASE=1 selects the optimized leg, so CI can
# pick debug vs release with one environment variable instead of a flag.
if ($env:DESI_RELEASE -eq "1") { $Release = $true }

$ProjectRoot = $PSScriptRoot
$BinDir = Join-Path $ProjectRoot "bin"
$BuildDir = Join-Path $ProjectRoot "build"

if ($Range) {
    $parts = $Range -split ","
    $Start = [int]$parts[0]
    if ($parts.Length -gt 1) { $End = [int]$parts[1] }
}

if ($Jobs -le 0) {
    $cpu = [Environment]::ProcessorCount
    $Jobs = [Math]::Max(2, [Math]::Floor($cpu / 2))
}

Write-Host "=========================================="
Write-Host "Desi Example Tests - parallel ($Jobs workers)"
Write-Host "=========================================="
Write-Host ""

# Build desic ONCE. Workers run with -NoBuild so they never race on
# writing bin\desic.exe (concurrent go build would corrupt it).
$GoExe = $null
foreach ($p in @("C:\gosdks\go1.25.0\bin\go.exe",
                 "C:\Program Files\Go\bin\go.exe",
                 "C:\Go\bin\go.exe",
                 (Get-Command "go" -ErrorAction SilentlyContinue).Source)) {
    if ($p -and (Test-Path $p)) { $GoExe = $p; break }
}
if (-not $GoExe) { Write-Host "Go not found." -ForegroundColor Red; exit 1 }

Write-Host "Building compiler..."
Push-Location $ProjectRoot
try {
    & $GoExe build -o (Join-Path $BinDir "desic.exe") ./compiler/cmd/desic
    if ($LASTEXITCODE -ne 0) { Write-Host "[FAIL] Compiler build failed!" -ForegroundColor Red; exit 1 }
}
finally { Pop-Location }
Write-Host "[OK] Compiler built successfully" -ForegroundColor Green
Write-Host ""

# Fresh scratch dirs so a previous run's exe can never be mistaken for this one's
for ($i = 0; $i -lt $Jobs; $i++) {
    $sd = Join-Path $BuildDir "shard$i"
    Remove-Item -Recurse -Force $sd -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force -Path (Join-Path $sd "output") | Out-Null
}

$script = Join-Path $ProjectRoot "test_examples.ps1"
$sw = [System.Diagnostics.Stopwatch]::StartNew()

$procs = @()
for ($i = 0; $i -lt $Jobs; $i++) {
    $log = Join-Path $BuildDir "shard$i\shard.log"
    $args = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $script,
              "-Shard", $i, "-ShardCount", $Jobs, "-NoBuild",
              "-Start", $Start, "-End", $End, "-TimeoutSec", $TimeoutSec)
    # Launch via cmd so the worker's stdout+stderr land in its log file
    # directly. Draining redirected pipes from PowerShell risks deadlocking
    # a chatty worker, and the file is what you want for debugging anyway.
    $inner = ($args | ForEach-Object { if ($_ -match '\s') { '"' + $_ + '"' } else { $_ } }) -join ' '
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = "cmd.exe"
    $psi.Arguments = '/d /c "powershell.exe ' + $inner + ' > "' + $log + '" 2>&1"'
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true
    $psi.WorkingDirectory = $ProjectRoot
    if ($Release) { $psi.EnvironmentVariables["DESI_RELEASE"] = "1" }
    $p = [System.Diagnostics.Process]::Start($psi)
    $procs += [pscustomobject]@{ Proc = $p; Index = $i; Log = $log }
    Write-Host "  worker $i started (pid $($p.Id))"
}

Write-Host ""
Write-Host "Running $(if ($Release) { 'RELEASE (-O2 + LTO)' } else { 'debug (-O0)' }) leg..."
foreach ($w in $procs) { $w.Proc.WaitForExit() }
$sw.Stop()

# Aggregate
$total = 0; $passed = 0
$failed = @()
foreach ($w in $procs) {
    $txt = if (Test-Path $w.Log) { Get-Content $w.Log -Encoding UTF8 } else { @() }
    foreach ($line in $txt) {
        if ($line -match "^Total:\s+(\d+)")  { $total  += [int]$Matches[1] }
        if ($line -match "^Passed:\s+(\d+)") { $passed += [int]$Matches[1] }
        if ($line -match "^\s+- (examples.+)$") { $failed += $Matches[1].Trim() }
    }
}

Write-Host ""
Write-Host "=========================================="
Write-Host "Test Summary (parallel, $Jobs workers)"
Write-Host "=========================================="
Write-Host "Elapsed: $([math]::Round($sw.Elapsed.TotalMinutes,2)) min"
Write-Host "Total:  $total"
Write-Host "Passed: $passed" -ForegroundColor Green
$failCount = $total - $passed
if ($failCount -eq 0) { Write-Host "Failed: 0" -ForegroundColor Green }
else { Write-Host "Failed: $failCount" -ForegroundColor Red }
Write-Host ""

if ($failed.Count -gt 0) {
    Write-Host "Failed tests:" -ForegroundColor Red
    foreach ($t in ($failed | Sort-Object)) { Write-Host "  - $t" }
    Write-Host ""
    Write-Host "Per-worker logs: build\shard<N>\shard.log"
    exit 1
}
else {
    Write-Host "[OK] All tests passed!" -ForegroundColor Green
    exit 0
}
