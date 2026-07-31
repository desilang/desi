<#
.SYNOPSIS
    Test all .desi example files (Windows)
.DESCRIPTION
    Runs all numbered .desi example files through the compiler and checks for expected behavior.
    Equivalent to './test_examples.sh' on macOS/Linux.
.PARAMETER Start
    Starting test number (default: 0)
.PARAMETER End
    Ending test number (default: 9999)
.PARAMETER Range
    Test range in format "start,end" (e.g., "0,25")
.EXAMPLE
    .\test_examples.ps1              # Run all tests
    .\test_examples.ps1 -Start 26    # Run tests from 26 to end
    .\test_examples.ps1 -Range 0,25  # Run tests 0 to 25
#>

param(
    [int]$Start = 0,
    [int]$End = 9999,
    [string]$Range,

    # Parallel sharding (used by test_examples_parallel.ps1). With
    # -ShardCount N, this process runs only the tests whose position in the
    # sorted list satisfies (index % N == Shard), and keeps its compile
    # intermediates in build\shard<Shard> so concurrent shards never share
    # program.ll or test_exec.exe. Defaults leave behavior unchanged.
    [int]$Shard = -1,
    [int]$ShardCount = 0,

    # Skip rebuilding desic (the parallel driver builds it once up front so
    # shards don't race writing bin\desic.exe).
    [switch]$NoBuild,

    # Per-test wall-clock limit. Without this a single hanging example blocks
    # the whole suite forever (there was no timeout at all before).
    [int]$TimeoutSec = 120
)

$ErrorActionPreference = "Stop"

# Force UTF-8 for output comparison — Desi programs always output UTF-8
# (SetConsoleOutputCP(CP_UTF8) is called in entry.c), so the test runner
# must read that output as UTF-8 or emoji/non-ASCII comparisons will fail.
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding          = [System.Text.Encoding]::UTF8

# Parse range if provided
if ($Range) {
    $parts = $Range -split ","
    $Start = [int]$parts[0]
    if ($parts.Length -gt 1) {
        $End = [int]$parts[1]
    }
}

# Directories
$ProjectRoot = $PSScriptRoot
$BuildDir = Join-Path $ProjectRoot "build"
# Each shard compiles into its own scratch dir; unsharded runs use build\.
$ScratchDir = if ($ShardCount -gt 0) { Join-Path $BuildDir "shard$Shard" } else { $BuildDir }
$OutputDir = Join-Path $ScratchDir "output"
$BinDir = Join-Path $ProjectRoot "bin"
$ExamplesDir = Join-Path $ProjectRoot "examples"

# Create output directory
New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null

# Find Go executable (same logic as build.ps1)
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
    Write-Host "Go not found. Please install Go or add it to PATH." -ForegroundColor Red
    exit 1
}

# Track results
$FailedTests = @()
$PassedCount = 0
$TotalCount = 0

Write-Host "=========================================="
Write-Host "Running Desi Example Tests ($Start to $End)"
Write-Host "=========================================="
Write-Host ""

# Build compiler first (skipped for shards — the driver already built it)
if (-not $NoBuild) {
    Write-Host "Building compiler..."
    Push-Location $ProjectRoot
    try {
        & $GoExe build -o (Join-Path $BinDir "desic.exe") ./compiler/cmd/desic
        if ($LASTEXITCODE -ne 0) {
            Write-Host "[FAIL] Compiler build failed!" -ForegroundColor Red
            exit 1
        }
    }
    finally {
        Pop-Location
    }
    Write-Host "[OK] Compiler built successfully" -ForegroundColor Green
    Write-Host ""
}

$Desic = Join-Path $BinDir "desic.exe"

# Function to extract expected output from test file
function Get-ExpectedOutput {
    param([string]$FilePath)

    $inExpected = $false
    $output = ""

    foreach ($line in Get-Content $FilePath -Encoding UTF8) {
        if ($line -eq "# EXPECTED_OUTPUT:") {
            $inExpected = $true
            continue
        }
        if ($inExpected) {
            if ($line -match "^# (.*)$") {
                $output += $Matches[1] + "`n"
            }
            elseif ($line -eq "#") {
                $output += "`n"
            }
            else {
                break
            }
        }
    }

    # Remove trailing newline for comparison
    return $output.TrimEnd("`n")
}

# Function to extract expected warning codes from test file header
# Returns array of warning codes like @("DW0004", "DW0008") or empty array
function Get-ExpectedWarnings {
    param([string]$FilePath)

    foreach ($line in Get-Content $FilePath -Encoding UTF8 -TotalCount 5) {
        if ($line -match "^#\s*WARNINGS:\s*(.+)$") {
            # Split on comma/space to get individual codes
            return ($Matches[1] -split '[,\s]+' | Where-Object { $_ -ne '' })
        }
    }
    return @()
}

# Find all test files matching [0-9]*.desi pattern
$testFiles = Get-ChildItem -Path $ExamplesDir -Filter "*.desi" -Recurse | 
    Where-Object { $_.Name -match "^\d+" } |
    Sort-Object { 
        if ($_.Name -match "^(\d+)") { [int]$Matches[1] } else { 0 }
    }

$fileIndex = -1
foreach ($testFile in $testFiles) {
    # Extract number from filename
    if ($testFile.Name -match "^(\d+)") {
        $num = [int]$Matches[1]
    }
    else {
        continue
    }

    # Skip if outside range
    if ($num -lt $Start -or $num -gt $End) {
        continue
    }

    # Round-robin shard selection: interleaving (rather than contiguous
    # blocks) keeps slow clusters like the db/ORM range spread evenly.
    $fileIndex++
    if ($ShardCount -gt 0 -and ($fileIndex % $ShardCount) -ne $Shard) {
        continue
    }

    $TotalCount++
    $relativePath = $testFile.FullName.Substring($ProjectRoot.Length + 1)
    
    # Check if test should be skipped (search first 5 lines)
    $first5Lines = Get-Content $testFile.FullName -TotalCount 5 -Encoding UTF8
    # "# REQUIRES: database" tests need a live PostgreSQL/MySQL and only run
    # when DESI_DB_TESTS=1 says the servers are available. build.ps1 does build
    # compiler/runtime/db/*.c, so the db module links on Windows; what it lacks
    # is TLS, because no OpenSSL include path is passed. Point PG_HOST/MYSQL_HOST
    # at a server that does not require TLS — see the wsl-db-test-rig notes.
    $needsDb = $first5Lines | Where-Object { $_ -match "# REQUIRES: database" }
    if ($needsDb -and $env:DESI_DB_TESTS -ne "1") {
        Write-Host "[$TotalCount] Testing: $relativePath  [SKIP] (needs a database)" -ForegroundColor Yellow
        $PassedCount++
        Write-Host ""
        continue
    }
    $isSkip = $first5Lines | Where-Object { $_ -match "# EXPECTED: SKIP" }
    if (-not $needsDb -and $isSkip) {
        Write-Host "[$TotalCount] Testing: $relativePath  [SKIP]" -ForegroundColor Yellow
        $PassedCount++
        Write-Host ""
        continue
    }
    
    Write-Host "[$TotalCount] Testing: $relativePath"
    
    # Check test expectations
    $firstLine = Get-Content $testFile.FullName -TotalCount 1
    $expectedFail    = if ($firstLine -match "# EXPECTED:") { $firstLine } else { $null }
    $hasExpectedOutput = (Get-Content $testFile.FullName -Encoding UTF8 | Select-String "# EXPECTED_OUTPUT:").Count -gt 0
    $expectedWarnings  = Get-ExpectedWarnings $testFile.FullName
    $isWarningsTest    = $expectedWarnings.Count -gt 0
    
    # Build and run
    $compileSuccess = $false
    $runtimeSuccess = $false
    
    $compileLog = Join-Path $OutputDir "compile_output.log"
    $runtimeLog = Join-Path $OutputDir "runtime_output.log"
    
    # Try to compile and run
    try {
        $buildResult = & "$PSScriptRoot\build-desi.ps1" -InputFile $testFile.FullName -OutputName "test_exec" -WorkDir $ScratchDir 2>&1
        if ($LASTEXITCODE -eq 0) {
            $compileSuccess = $true
            $buildResult | Out-File $compileLog -Encoding UTF8

            # Try to run — through cmd.exe so stdout+stderr merge into the log
            # as raw UTF-8 bytes, mirroring test_examples.sh ('> log 2>&1').
            # Capturing with 2>&1 in PowerShell 5.1 throws on the first stderr
            # line under ErrorActionPreference=Stop and mangles non-ASCII output.
            $execPath = Join-Path $OutputDir "test_exec.exe"
            if (Test-Path $execPath) {
                # Run under a wall-clock limit. A hung example used to block
                # the entire suite (no timeout existed); now it is killed —
                # with its whole process tree — and reported as a failure.
                # ProcessStartInfo (not Start-Process) so the cmd redirect is
                # passed through verbatim: Start-Process re-quotes arguments
                # containing spaces, which breaks '"exe" > "log" 2>&1'.
                $psi = New-Object System.Diagnostics.ProcessStartInfo
                $psi.FileName = "cmd.exe"
                $psi.Arguments = '/d /c ""' + $execPath + '" > "' + $runtimeLog + '" 2>&1"'
                $psi.UseShellExecute = $false
                $psi.CreateNoWindow = $true
                $runProc = [System.Diagnostics.Process]::Start($psi)
                if ($runProc.WaitForExit($TimeoutSec * 1000)) {
                    if ($runProc.ExitCode -eq 0) { $runtimeSuccess = $true }
                }
                else {
                    & taskkill /T /F /PID $runProc.Id *>$null
                    "TIMEOUT after ${TimeoutSec}s" | Out-File $runtimeLog -Append -Encoding UTF8
                }
            }
        }
        elseif ($isWarningsTest) {
            # Compile failed — check if it's because of expected warnings
            # (some compilers exit non-zero on warnings)
            $buildOutput = $buildResult -join "`n"
            $allFound = $true
            foreach ($wcode in $expectedWarnings) {
                if ($buildOutput -notmatch [regex]::Escape($wcode)) {
                    $allFound = $false; break
                }
            }
            if ($allFound) {
                # Treat as compile success for WARNINGS tests
                $compileSuccess = $true
            }
            $buildResult | Out-File $compileLog -Encoding UTF8
        }
        else {
            $buildResult | Out-File $compileLog -Encoding ASCII
        }
    }
    catch {
        $_.Exception.Message | Out-File $compileLog -Encoding ASCII
    }
    
    # Determine if test passed based on expectations
    if ($expectedFail) {
        if ($expectedFail -match "COMPILE_ERROR") {
            if (-not $compileSuccess) {
                Write-Host "  [PASS] (expected compile error)" -ForegroundColor Green
                $PassedCount++
            }
            else {
                Write-Host "  [FAIL] (expected compile error, but compiled successfully)" -ForegroundColor Red
                $FailedTests += "$relativePath (unexpected success)"
            }
        }
        elseif ($expectedFail -match "RUNTIME_ERROR") {
            if ($compileSuccess -and -not $runtimeSuccess) {
                Write-Host "  [PASS] (expected runtime error)" -ForegroundColor Green
                $PassedCount++
            }
            else {
                Write-Host "  [FAIL] (expected runtime error, but ran successfully)" -ForegroundColor Red
                $FailedTests += "$relativePath (unexpected success)"
            }
        }
    }
    else {
        # Expected to pass
        if ($compileSuccess -and $runtimeSuccess) {
            # Check expected output if specified
            if ($hasExpectedOutput) {
                $expected = Get-ExpectedOutput $testFile.FullName
                # Get-Content -Raw returns $null for an empty file (PS 5.1)
                $actualRaw = Get-Content $runtimeLog -Raw -Encoding UTF8
                $actual = if ($null -ne $actualRaw) { $actualRaw.TrimEnd() } else { "" }

                # If expected section is empty, treat as "just verify it runs" (no output check)
                if ($expected -eq "") {
                    Write-Host "  [PASS]" -ForegroundColor Green
                    $PassedCount++
                }
                else {
                    # Normalize line endings (CRLF -> LF) for cross-platform comparison
                    $expected = $expected -replace "`r`n", "`n" -replace "`r", "`n"
                    $actual   = $actual   -replace "`r`n", "`n" -replace "`r", "`n"

                    if ($expected -eq $actual) {
                        Write-Host "  [PASS]" -ForegroundColor Green
                        $PassedCount++
                    }
                    else {
                        Write-Host "  [FAIL] (output mismatch)" -ForegroundColor Red
                        Write-Host "    Expected:" -ForegroundColor Yellow
                        $expected -split "`n" | ForEach-Object { Write-Host "      $_" }
                        Write-Host "    Actual:" -ForegroundColor Yellow
                        $actual -split "`n" | ForEach-Object { Write-Host "      $_" }
                        $FailedTests += "$relativePath (output)"
                    }
                }
            }
            else {
                Write-Host "  [PASS]" -ForegroundColor Green
                $PassedCount++
            }
        }
        elseif (-not $compileSuccess) {
            Write-Host "  [FAIL] (compile error)" -ForegroundColor Red
            Get-Content $compileLog | ForEach-Object { Write-Host "    $_" }
            $FailedTests += "$relativePath (compile)"
        }
        else {
            Write-Host "  [FAIL] (runtime error)" -ForegroundColor Red
            if (Test-Path $runtimeLog) {
                Get-Content $runtimeLog | ForEach-Object { Write-Host "    $_" }
            }
            $FailedTests += "$relativePath (runtime)"
        }
    }
    Write-Host ""
}

# Summary
Write-Host "=========================================="
Write-Host "Test Summary"
Write-Host "=========================================="
Write-Host "Total:  $TotalCount"
Write-Host "Passed: $PassedCount" -ForegroundColor Green
$failedCount = $TotalCount - $PassedCount
if ($failedCount -eq 0) {
    Write-Host "Failed: 0" -ForegroundColor Green
} else {
    Write-Host "Failed: $failedCount" -ForegroundColor Red
}
Write-Host ""

if ($FailedTests.Count -gt 0) {
    Write-Host "Failed tests:" -ForegroundColor Red
    foreach ($test in $FailedTests) {
        Write-Host "  - $test"
    }
    exit 1
}
else {
    Write-Host "[OK] All tests passed!" -ForegroundColor Green
    exit 0
}
