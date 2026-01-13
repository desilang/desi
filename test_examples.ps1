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
    [string]$Range
)

$ErrorActionPreference = "Stop"

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
$OutputDir = Join-Path $BuildDir "output"
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

# Build compiler first
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

$Desic = Join-Path $BinDir "desic.exe"

# Function to extract expected output from test file
function Get-ExpectedOutput {
    param([string]$FilePath)
    
    $inExpected = $false
    $output = ""
    
    foreach ($line in Get-Content $FilePath) {
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

# Find all test files matching [0-9]*.desi pattern
$testFiles = Get-ChildItem -Path $ExamplesDir -Filter "*.desi" -Recurse | 
    Where-Object { $_.Name -match "^\d+" } |
    Sort-Object { 
        if ($_.Name -match "^(\d+)") { [int]$Matches[1] } else { 0 }
    }

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
    
    $TotalCount++
    $relativePath = $testFile.FullName.Substring($ProjectRoot.Length + 1)
    Write-Host "[$TotalCount] Testing: $relativePath"
    
    # Check test expectations
    $firstLine = Get-Content $testFile.FullName -TotalCount 1
    $expectedFail = if ($firstLine -match "# EXPECTED:") { $firstLine } else { $null }
    $hasExpectedOutput = (Get-Content $testFile.FullName | Select-String "# EXPECTED_OUTPUT:").Count -gt 0
    
    # Build and run
    $compileSuccess = $false
    $runtimeSuccess = $false
    
    $compileLog = Join-Path $OutputDir "compile_output.log"
    $runtimeLog = Join-Path $OutputDir "runtime_output.log"
    
    # Try to compile and run
    try {
        $buildResult = & "$PSScriptRoot\build-desi.ps1" -InputFile $testFile.FullName -OutputName "test_exec" 2>&1
        if ($LASTEXITCODE -eq 0) {
            $compileSuccess = $true
            $buildResult | Out-File $compileLog -Encoding ASCII
            
            # Try to run
            $execPath = Join-Path $OutputDir "test_exec.exe"
            if (Test-Path $execPath) {
                $runtimeResult = & $execPath 2>&1
                if ($LASTEXITCODE -eq 0) {
                    $runtimeSuccess = $true
                }
                $runtimeResult | Out-File $runtimeLog -Encoding ASCII
            }
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
                $actual = (Get-Content $runtimeLog -Raw).TrimEnd()
                
                # Normalize line endings (CRLF -> LF) for cross-platform comparison
                $expected = $expected -replace "`r`n", "`n" -replace "`r", "`n"
                $actual = $actual -replace "`r`n", "`n" -replace "`r", "`n"
                
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
