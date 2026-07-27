<#
.SYNOPSIS
    Smoke test every shipped Desi binary (Windows)
.DESCRIPTION
    The example suite exercises the compiler, but the other binaries we ship
    — desirepl, desifmt, desilsp, and the desic subcommands — had no coverage
    at all. Everything checked here is something that was actually broken:

      * desic build produced a file with no .exe extension, which Windows
        cannot execute
      * desic init created an empty tests/, so desic test failed on a project
        the tool had just generated
      * desirepl printed "ok" instead of evaluating anything
      * desifmt corrupted f-strings, raw strings, and dropped keywords
      * desilsp panicked on the first completion request

    Fast (a few seconds) and self-contained: everything runs in a temp
    directory that is removed afterwards.
.EXAMPLE
    .\tools\smoke.ps1
#>

$ErrorActionPreference = "Continue"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

$repo   = Split-Path -Parent $PSScriptRoot
$desic  = Join-Path $repo "bin\desic.exe"
$fmt    = Join-Path $repo "bin\desifmt.exe"
$repl   = Join-Path $repo "bin\desirepl.exe"
$lsp    = Join-Path $repo "bin\desilsp.exe"
$enc    = New-Object System.Text.UTF8Encoding($false)

foreach ($b in @($desic, $fmt, $repl, $lsp)) {
    if (-not (Test-Path $b)) {
        Write-Host "missing $b - run .\build.ps1 first" -ForegroundColor Red
        exit 1
    }
}

$fails = 0
function Check($name, $ok, $detail) {
    if ($ok) {
        Write-Host ("  [OK]   " + $name) -ForegroundColor Green
    } else {
        Write-Host ("  [FAIL] " + $name + $(if ($detail) { " - $detail" } else { "" })) -ForegroundColor Red
        $script:fails++
    }
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("desi_smoke_" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

try {
    # ---------- desic: the documented init -> run -> test -> build path ----------
    Write-Host "desic (project workflow)"
    Push-Location $tmp
    & $desic init myapp *>$null
    Check "init scaffolds a project" (Test-Path (Join-Path $tmp "myapp\desi.mod"))
    Check "init scaffolds a test"    (Test-Path (Join-Path $tmp "myapp\tests\main_test.desi"))
    Pop-Location

    $app = Join-Path $tmp "myapp"
    Push-Location $app
    $runOut = & $desic run 2>&1 | Out-String
    Check "run prints program output" ($runOut -match "Hello, Desi!") $runOut.Trim()

    $testOut = & $desic test 2>&1 | Out-String
    Check "test passes on a fresh project" ($LASTEXITCODE -eq 0 -and $testOut -match "passed") $testOut.Trim()

    & $desic build *>$null
    $exe = Join-Path $app "build\output\myapp.exe"
    Check "build produces a runnable .exe" (Test-Path $exe)
    if (Test-Path $exe) {
        $artifactOut = & $exe 2>&1 | Out-String
        Check "built exe runs" ($artifactOut -match "Hello, Desi!") $artifactOut.Trim()
    }
    Pop-Location

    # ---------- desic run on a bare file (README quick start) ----------
    $bare = Join-Path $tmp "bare.desi"
    [System.IO.File]::WriteAllText($bare, 'print("Hello, Desi!")' + "`n", $enc)
    $bareOut = & $desic run $bare 2>&1 | Out-String
    Check "README quick start (top-level statement)" ($bareOut -match "Hello, Desi!") $bareOut.Trim()

    # ---------- desifmt ----------
    Write-Host "desifmt"
    $src = Join-Path $tmp "fmt.desi"
    $text = "class C:`n`tpub mut static n: int = 0`n`ndef noop() -> none:`n`tpass`n`ndef main() -> int:`n`tlet raw = r#`"say `"hi`"`"#`n`tlet name = `"desi`"`n`tprint(f`"hello {name}`")`n`tprint(raw)`n`t0`n"
    [System.IO.File]::WriteAllText($src, $text, $enc)

    $once = Join-Path $tmp "fmt1.desi"
    $twice = Join-Path $tmp "fmt2.desi"
    cmd /d /c "`"$fmt`" `"$src`" > `"$once`"" | Out-Null
    Check "formats a file" ($LASTEXITCODE -eq 0)
    cmd /d /c "`"$fmt`" `"$once`" > `"$twice`"" | Out-Null

    $b1 = [System.IO.File]::ReadAllBytes($once)
    $b2 = [System.IO.File]::ReadAllBytes($twice)
    $idem = $b1.Length -eq $b2.Length
    if ($idem) { for ($i=0; $i -lt $b1.Length; $i++) { if ($b1[$i] -ne $b2[$i]) { $idem = $false; break } } }
    Check "formatting is idempotent" $idem

    $formatted = Get-Content $once -Raw -Encoding UTF8
    Check "keeps 'static'"            ($formatted -match "static")
    Check "keeps 'pass'"              ($formatted -match "pass")
    Check "keeps f-strings intact"    ($formatted -match [regex]::Escape('f"hello {name}"'))
    Check "keeps raw strings intact"  ($formatted -match [regex]::Escape('r#"say "hi""#'))

    # ---------- desirepl ----------
    Write-Host "desirepl"
    $replIn  = Join-Path $tmp "repl_in.txt"
    $replOut = Join-Path $tmp "repl_out.txt"
    $session = "1 + 2`nlet x = 10`nx * 4`nlet name = `"desi`"`nprint(f`"hi {name}`")`nundefined_thing`n1 + 1`n:quit`n"
    [System.IO.File]::WriteAllText($replIn, $session, $enc)
    $p = Start-Process -FilePath $repl -PassThru -NoNewWindow `
            -RedirectStandardInput $replIn -RedirectStandardOutput $replOut -RedirectStandardError (Join-Path $tmp "repl_err.txt")
    $exited = $p.WaitForExit(120000)
    if (-not $exited) { $p.Kill() }
    Check "repl exits on :quit" $exited
    $ro = Get-Content $replOut -Raw -Encoding UTF8
    if ($null -eq $ro) { $ro = "" }
    Check "repl evaluates expressions"     ($ro -match "\b3\b")
    Check "repl keeps bindings across lines" ($ro -match "\b40\b")
    Check "repl runs print"                ($ro -match "hi desi")
    Check "repl survives a bad input"      ($ro -match "\b2\b")

    # ---------- desilsp ----------
    Write-Host "desilsp"
    & (Join-Path $PSScriptRoot "lsp_smoke.ps1") | Out-Null
    Check "lsp session completes without crashing" ($LASTEXITCODE -eq 0)

    # ---------- desic watch (documented as unsupported here) ----------
    Write-Host "desic watch"
    $wOut = Join-Path $tmp "watch.txt"
    $wp = Start-Process -FilePath $desic -ArgumentList "watch", $bare -PassThru -NoNewWindow `
            -RedirectStandardOutput $wOut -RedirectStandardError (Join-Path $tmp "watch_err.txt")
    $wExited = $wp.WaitForExit(15000)
    if (-not $wExited) { $wp.Kill() }
    $wTxt = Get-Content $wOut -Raw
    Check "watch exits with a clear message on Windows" ($wExited -and $wTxt -match "not supported on Windows")
}
finally {
    Pop-Location -ErrorAction SilentlyContinue
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

Write-Host ""
if ($fails -eq 0) {
    Write-Host "[OK] all tooling smoke checks passed" -ForegroundColor Green
    exit 0
}
Write-Host "[FAIL] $fails smoke check(s) failed" -ForegroundColor Red
exit 1
