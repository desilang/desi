# Run `desic check` with a hard timeout and a memory ceiling.
#
# Git Bash's `timeout` does not reliably kill a native Windows process, so the
# earlier bounded runs were not actually bounded — desic reached 2 GB and took
# the machine with it. This kills by handle, and also aborts early if the
# process crosses the memory limit before the deadline.
param(
    [Parameter(Mandatory = $true)][string]$File,
    [int]$TimeoutSec = 10,
    [int]$MaxMB = 800
)

$desic = "C:\viral\desilang\desi\bin\desic.exe"
$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $desic
$psi.Arguments = "check `"$File`""
$psi.UseShellExecute = $false
$psi.RedirectStandardOutput = $true
$psi.RedirectStandardError = $true
$psi.CreateNoWindow = $true

$p = [System.Diagnostics.Process]::Start($psi)
$deadline = (Get-Date).AddSeconds($TimeoutSec)
$peak = 0
$verdict = ""

while (-not $p.HasExited) {
    try { $p.Refresh(); $mb = [int]($p.WorkingSet64 / 1MB); if ($mb -gt $peak) { $peak = $mb } } catch {}
    if ($peak -gt $MaxMB) { $verdict = "RUNAWAY_MEMORY"; break }
    if ((Get-Date) -gt $deadline) { $verdict = "TIMEOUT"; break }
    Start-Sleep -Milliseconds 100
}

if (-not $p.HasExited) {
    try { $p.Kill(); $p.WaitForExit(5000) | Out-Null } catch {}
} elseif ($verdict -eq "") {
    $verdict = if ($p.ExitCode -eq 0) { "OK" } else { "CHECK_ERROR" }
}

"{0}`tpeak={1}MB`t{2}" -f $verdict, $peak, (Split-Path $File -Leaf)
