<#
.SYNOPSIS
    Desi vs C benchmark runner (Windows).
.DESCRIPTION
    For each benchmark pair (<name>.desi / <name>.c):
      - builds the Desi program via build-desi.ps1
      - builds the C program via clang at the SAME opt level for parity
        (default -O0; -Release compiles both sides at -O2)
      - runs each 3 times, reports best wall time and OS-reported peak
        working set (Process.PeakWorkingSet64 — exact, not sampled)
    Run from anywhere; paths resolve relative to this script.
.EXAMPLE
    .\benchmarks\run_benchmarks.ps1            # both sides -O0
    .\benchmarks\run_benchmarks.ps1 -Release   # both sides -O2
#>

param(
    [switch]$Release
)

$ErrorActionPreference = "Stop"
$BenchDir = $PSScriptRoot
$RepoRoot = Split-Path $BenchDir
$OutDir = Join-Path $BenchDir "out"
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

$COpt = if ($Release) { "-O2" } else { "-O0" }
Write-Host "Mode: $(if ($Release) { 'RELEASE (-O2 both sides)' } else { 'default (-O0 both sides)' })" -ForegroundColor Yellow

function Measure-Exe {
    param([string]$Exe)
    $bestMs = [double]::MaxValue
    $peak = 0
    $output = ""
    for ($run = 0; $run -lt 3; $run++) {
        $sw = [System.Diagnostics.Stopwatch]::StartNew()
        $p = Start-Process -FilePath $Exe -PassThru -NoNewWindow `
            -RedirectStandardOutput (Join-Path $OutDir "last_stdout.txt")
        # PeakWorkingSet64 must be sampled while the process is alive —
        # after exit the refresh fails and the property reads 0.
        while (-not $p.HasExited) {
            try { $p.Refresh(); if ($p.PeakWorkingSet64 -gt $peak) { $peak = $p.PeakWorkingSet64 } } catch {}
        }
        $sw.Stop()
        if ($sw.Elapsed.TotalMilliseconds -lt $bestMs) { $bestMs = $sw.Elapsed.TotalMilliseconds }
    }
    $output = (Get-Content (Join-Path $OutDir "last_stdout.txt") -Raw -ErrorAction SilentlyContinue)
    if ($output) { $output = $output.Trim() }
    return @{ Ms = [math]::Round($bestMs, 1); PeakMB = [math]::Round($peak / 1MB, 1); Out = $output }
}

$names = Get-ChildItem $BenchDir -Filter "*.desi" | ForEach-Object { $_.BaseName }
$results = @()

foreach ($name in $names) {
    Write-Host "==> $name" -ForegroundColor Cyan

    # Build Desi side
    if ($Release) {
        & (Join-Path $RepoRoot "build-desi.ps1") (Join-Path $BenchDir "$name.desi") -Release | Out-Null
    } else {
        & (Join-Path $RepoRoot "build-desi.ps1") (Join-Path $BenchDir "$name.desi") | Out-Null
    }
    $desiExe = Join-Path $RepoRoot "build\output\$name.exe"

    # Build C side at the same opt level as the Desi IR
    $cExe = Join-Path $OutDir "$name`_c.exe"
    & clang $COpt (Join-Path $BenchDir "$name.c") -o $cExe
    if ($LASTEXITCODE -ne 0) { throw "clang failed for $name.c" }

    $desi = Measure-Exe $desiExe
    $c    = Measure-Exe $cExe

    if ($desi.Out -ne $c.Out) {
        Write-Host "  [WARN] output mismatch: desi='$($desi.Out)' c='$($c.Out)'" -ForegroundColor Yellow
    }

    $results += [pscustomobject]@{
        Benchmark  = $name
        "Desi ms"  = $desi.Ms
        "C ms"     = $c.Ms
        "Desi MB"  = $desi.PeakMB
        "C MB"     = $c.PeakMB
        "Output"   = $desi.Out
    }
}

Write-Host ""
$results | Format-Table -AutoSize
