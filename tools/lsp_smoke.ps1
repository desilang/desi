<#
.SYNOPSIS
    Smoke test for desilsp (Windows)
.DESCRIPTION
    Drives a real LSP session over stdio with Content-Length framing:
    initialize, didOpen on a file containing a deliberate type error, then the
    requests an editor issues constantly, then shutdown/exit.

    Verifies the server answers every request id, publishes diagnostics, and
    exits rather than hanging. A crash in any handler shows up as missing
    responses for every id after it, because the panic takes down the process
    the editor is talking to — which is exactly how the completion panic
    (nil type component in a signature) was found.
.EXAMPLE
    .\tools\lsp_smoke.ps1
#>

$ErrorActionPreference = "Continue"
$repo = Split-Path -Parent $PSScriptRoot
$lsp  = Join-Path $repo "bin\desilsp.exe"
if (-not (Test-Path $lsp)) {
    Write-Host "desilsp not found at $lsp - run .\build.ps1 first" -ForegroundColor Red
    exit 1
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("desi_lsp_" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
try {
    $doc  = Join-Path $tmp "lsptest.desi"
    $text = "def greet(name: str) -> str:`n`t`"hello `" + name`n`ndef main() -> int:`n`tlet x: str = 42`n`tprint(greet(`"desi`"))`n`t0`n"
    $enc  = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($doc, $text, $enc)
    $uri = "file:///" + ($doc -replace '\\','/')

    function New-Msg($obj) {
        $json  = $obj | ConvertTo-Json -Depth 20 -Compress
        $bytes = [System.Text.Encoding]::UTF8.GetByteCount($json)
        return "Content-Length: $bytes`r`n`r`n$json"
    }

    $msgs = @()
    $msgs += New-Msg @{ jsonrpc="2.0"; id=1; method="initialize"; params=@{ processId=$PID; rootUri=$null; capabilities=@{} } }
    $msgs += New-Msg @{ jsonrpc="2.0"; method="initialized"; params=@{} }
    $msgs += New-Msg @{ jsonrpc="2.0"; method="textDocument/didOpen"; params=@{
        textDocument=@{ uri=$uri; languageId="desi"; version=1; text=$text } } }
    $msgs += New-Msg @{ jsonrpc="2.0"; id=2; method="textDocument/hover"; params=@{
        textDocument=@{ uri=$uri }; position=@{ line=5; character=8 } } }
    $msgs += New-Msg @{ jsonrpc="2.0"; id=3; method="textDocument/documentSymbol"; params=@{ textDocument=@{ uri=$uri } } }
    $msgs += New-Msg @{ jsonrpc="2.0"; id=4; method="textDocument/completion"; params=@{
        textDocument=@{ uri=$uri }; position=@{ line=5; character=8 } } }
    $msgs += New-Msg @{ jsonrpc="2.0"; id=5; method="textDocument/definition"; params=@{
        textDocument=@{ uri=$uri }; position=@{ line=5; character=8 } } }
    $msgs += New-Msg @{ jsonrpc="2.0"; id=6; method="textDocument/formatting"; params=@{
        textDocument=@{ uri=$uri }; options=@{ tabSize=4; insertSpaces=$false } } }
    $msgs += New-Msg @{ jsonrpc="2.0"; id=7; method="textDocument/semanticTokens/full"; params=@{ textDocument=@{ uri=$uri } } }
    $msgs += New-Msg @{ jsonrpc="2.0"; id=8; method="shutdown"; params=$null }
    $msgs += New-Msg @{ jsonrpc="2.0"; method="exit"; params=$null }

    $inPath  = Join-Path $tmp "in.bin"
    $outPath = Join-Path $tmp "out.txt"
    $errPath = Join-Path $tmp "err.txt"
    [System.IO.File]::WriteAllText($inPath, ($msgs -join ""), $enc)

    $p = Start-Process -FilePath $lsp -PassThru -NoNewWindow `
            -RedirectStandardInput $inPath -RedirectStandardOutput $outPath -RedirectStandardError $errPath
    $exited = $p.WaitForExit(30000)

    $fail = 0
    if (-not $exited) {
        $p.Kill()
        Write-Host "[FAIL] server did not exit after 'exit' notification" -ForegroundColor Red
        $fail = 1
    }

    $out = Get-Content $outPath -Raw -Encoding UTF8
    if ($null -eq $out) { $out = "" }

    foreach ($id in 1..8) {
        if ($out -match ('"id"\s*:\s*' + $id + '\b')) {
            Write-Host ("  [OK]   response id={0}" -f $id) -ForegroundColor Green
        } else {
            Write-Host ("  [FAIL] no response for id={0}" -f $id) -ForegroundColor Red
            $fail = 1
        }
    }
    if ($out -match "publishDiagnostics") {
        Write-Host "  [OK]   diagnostics published" -ForegroundColor Green
    } else {
        Write-Host "  [FAIL] no diagnostics published" -ForegroundColor Red
        $fail = 1
    }

    $errTxt = Get-Content $errPath -Raw
    if ($errTxt -and $errTxt -match "panic:") {
        Write-Host "  [FAIL] server panicked:" -ForegroundColor Red
        Write-Host $errTxt.Trim()
        $fail = 1
    }

    Write-Host ""
    if ($fail -eq 0) {
        Write-Host "[OK] desilsp smoke test passed" -ForegroundColor Green
        exit 0
    }
    Write-Host "[FAIL] desilsp smoke test failed" -ForegroundColor Red
    exit 1
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
