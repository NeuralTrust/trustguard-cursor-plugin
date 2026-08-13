# Bootstrap for the TrustGuard Cursor plugin (Windows).
#
# Mirror of trustguard-hook.sh: prefers a PATH-installed trustguard-cursor,
# otherwise downloads the pinned release for this arch into
# %USERPROFILE%\.trustguard\bin, verifies its SHA-256 against the table below,
# and runs it. Every bootstrap failure fails open (the editor must never
# brick) with a warning on stderr.
#
# The $Version and $Sha256 table are updated on each cursor-v* release; the
# release workflow prints the exact block to paste here.

$Version = '0.1.0'

# Per-arch SHA-256 of the Windows release binaries (filled per release).
$Sha256 = @{
    'amd64' = '7abb7ae3ba0a45cc4c19c3382a47d05948967fcb228b77c9040b19ff5247951f'
    'arm64' = '4363b5a61cd0d0cbe8c84b0ea803748063a3141adb6af2ee0e9e4b0d1c8004bf'
}

# Read a single line: the hook payload is one JSON document and Cursor may
# keep stdin open, so ReadToEnd would wait forever for an EOF that never comes.
$Stdin = [Console]::In.ReadLine()

function Invoke-Hook([string]$Exe) {
    $out = $Stdin | & $Exe hook
    if ($null -ne $out) { Write-Output $out }
    exit $LASTEXITCODE
}

function Exit-FailOpen([string]$Message) {
    [Console]::Error.WriteLine("trustguard-cursor bootstrap: $Message - allowing without evaluation")
    Write-Output '{"continue":true,"permission":"allow"}'
    exit 0
}

try {
    # 1. A binary on the PATH always wins (manual, MDM or package-manager install).
    $onPath = Get-Command 'trustguard-cursor' -ErrorAction SilentlyContinue
    if ($onPath) { Invoke-Hook $onPath.Source }

    $binDir = if ($env:TRUSTGUARD_CURSOR_BIN_DIR) { $env:TRUSTGUARD_CURSOR_BIN_DIR } else { Join-Path $env:USERPROFILE '.trustguard\bin' }
    $baseUrl = if ($env:TRUSTGUARD_CURSOR_DOWNLOAD_BASE) { $env:TRUSTGUARD_CURSOR_DOWNLOAD_BASE } else { 'https://github.com/NeuralTrust/trustguard-cursor-plugin/releases/download' }

    # 2. Cached pinned version.
    $bin = Join-Path $binDir "trustguard-cursor-$Version.exe"
    if (Test-Path $bin) { Invoke-Hook $bin }

    # 3. Download, verify, install, run.
    $arch = switch ($env:PROCESSOR_ARCHITECTURE) {
        'AMD64' { 'amd64' }
        'ARM64' { 'arm64' }
        default { Exit-FailOpen "unsupported arch $($env:PROCESSOR_ARCHITECTURE); install trustguard-cursor manually" }
    }
    $wantSha = $Sha256[$arch]
    if (-not $wantSha) {
        Exit-FailOpen "no pinned checksum for windows/$arch (release $Version not published yet?); install trustguard-cursor manually"
    }

    $url = "$baseUrl/v$Version/trustguard-cursor_${Version}_windows_$arch.exe"
    New-Item -ItemType Directory -Force -Path $binDir | Out-Null
    $tmp = "$bin.download.$PID"
    try {
        Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $tmp -TimeoutSec 60
    } catch {
        Remove-Item -Force -ErrorAction SilentlyContinue $tmp
        Exit-FailOpen "download failed: $url"
    }

    $gotSha = (Get-FileHash -Algorithm SHA256 -Path $tmp).Hash.ToLowerInvariant()
    if ($gotSha -ne $wantSha.ToLowerInvariant()) {
        Remove-Item -Force -ErrorAction SilentlyContinue $tmp
        Exit-FailOpen "checksum mismatch for $url (got $gotSha, want $wantSha)"
    }

    Move-Item -Force $tmp $bin
    [Console]::Error.WriteLine("trustguard-cursor bootstrap: installed $bin")
    Invoke-Hook $bin
} catch {
    Exit-FailOpen "unexpected error: $($_.Exception.Message)"
}
