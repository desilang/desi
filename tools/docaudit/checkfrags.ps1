# Check every documentation fragment, bounded, and separate the two kinds of
# failure.
#
# A fragment is a few lines lifted out of prose, so it usually cannot stand
# alone: it references things defined earlier on the page, or is a bare
# signature. Type errors from that are expected and say nothing about the docs.
#
# A *parse* error is different. Syntax is wrong no matter what surrounds it, so
# DPE codes are the signal worth acting on.
#
# Bounded per file by handle-kill and a memory ceiling: Git Bash's `timeout`
# does not reliably kill a native Windows process, which is how an earlier run
# reached 2 GB.
param(
    [string]$Index = "$env:TEMP\dblocks\frags.tsv",
    [int]$TimeoutSec = 10,
    [int]$MaxMB = 800
)

$desic = "C:\viral\desilang\desi\bin\desic.exe"
$parseErrors = @()
$otherFail = 0
$ok = 0
$runaway = @()
$n = 0

foreach ($line in Get-Content $Index) {
    $parts = $line -split "`t"
    if ($parts.Count -lt 3) { continue }
    $src = "$($parts[0]):$($parts[1])"
    $file = $parts[2] -replace '^/tmp/', "$env:TEMP\" -replace '/', '\'
    if (-not (Test-Path $file)) { continue }
    $n++

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $desic
    $psi.Arguments = "check `"$file`""
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.CreateNoWindow = $true

    $p = [System.Diagnostics.Process]::Start($psi)
    $sbOut = $p.StandardOutput.ReadToEndAsync()
    $sbErr = $p.StandardError.ReadToEndAsync()
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    $peak = 0
    $killed = $false
    while (-not $p.HasExited) {
        try { $p.Refresh(); $mb = [int]($p.WorkingSet64 / 1MB); if ($mb -gt $peak) { $peak = $mb } } catch {}
        if ($peak -gt $MaxMB -or (Get-Date) -gt $deadline) {
            try { $p.Kill() } catch {}
            $killed = $true
            break
        }
        Start-Sleep -Milliseconds 40
    }
    $p.WaitForExit(5000) | Out-Null

    if ($killed) { $runaway += "$src (peak ${peak}MB)"; continue }

    $out = ($sbOut.Result + $sbErr.Result)
    if ($p.ExitCode -eq 0) { $ok++; continue }

    $dpe = [regex]::Matches($out, 'error\[(DPE\d+)\]') | ForEach-Object { $_.Groups[1].Value } | Select-Object -First 1
    if ($dpe) { $parseErrors += "$src`t$dpe" } else { $otherFail++ }
}

"checked          : $n"
"clean            : $ok"
"parse errors     : $($parseErrors.Count)   <-- real doc defects"
"context failures : $otherFail   (expected: fragments lack surrounding code)"
"runaway/timeout  : $($runaway.Count)"
if ($runaway.Count) { ""; "RUNAWAY:"; $runaway }
if ($parseErrors.Count) {
    ""
    "PARSE ERRORS:"
    $parseErrors | Sort-Object | ForEach-Object { "  $_" }
}
