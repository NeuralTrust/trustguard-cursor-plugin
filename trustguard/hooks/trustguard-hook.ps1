# Bootstrap for the TrustGuard Cursor plugin (Windows).
#
# Mirror of trustguard-hook.sh: prefers a PATH-installed trustguard-cursor,
# otherwise installs the pinned release for this arch into
# %USERPROFILE%\.trustguard\bin in the background, verifying its SHA-256
# against the table below. Every bootstrap failure fails open (the editor must
# never brick) with a warning on stderr.
#
# -InstallOnly is how the script re-enters itself as the detached installer; it
# takes no hook input and answers nothing.
#
# The $Version and $Sha256 table are updated automatically by the Release
# workflow on every push to main.

param([switch]$InstallOnly)

$Version = '0.1.2'

# Per-arch SHA-256 of the Windows release binaries (filled per release).
$Sha256 = @{
    'amd64' = '5c946f59c77efec27dae82fb075a9a41c437e3b7e536cbb10ffd01cefee86d72'
    'arm64' = '644b3fad82b61c27ead4fbcdf0f6e6157d77bfe5cb441962303bf86224c1f33f'
}

# Read a single line: the hook payload is one JSON document and Cursor may
# keep stdin open, so ReadToEnd would wait forever for an EOF that never comes.
$Stdin = if ($InstallOnly) { $null } else { [Console]::In.ReadLine() }

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

function Install-Binary([string]$Url, [string]$Target, [string]$WantSha) {
    $tmp = "$Target.download.$PID"
    try {
        Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $tmp -TimeoutSec 300
        $gotSha = (Get-FileHash -Algorithm SHA256 -Path $tmp).Hash.ToLowerInvariant()
        if ($gotSha -ne $WantSha.ToLowerInvariant()) { throw "checksum mismatch (got $gotSha, want $WantSha)" }
        Move-Item -Force $tmp $Target
    } catch {
        Remove-Item -Force -ErrorAction SilentlyContinue $tmp
        [Console]::Error.WriteLine("trustguard-cursor bootstrap: install failed: $($_.Exception.Message)")
    }
}

try {
    # 1. A binary on the PATH always wins (manual, MDM or package-manager install).
    $onPath = Get-Command 'trustguard-cursor' -ErrorAction SilentlyContinue
    if ($onPath -and -not $InstallOnly) { Invoke-Hook $onPath.Source }

    $binDir = if ($env:TRUSTGUARD_CURSOR_BIN_DIR) { $env:TRUSTGUARD_CURSOR_BIN_DIR } else { Join-Path $env:USERPROFILE '.trustguard\bin' }
    $baseUrl = if ($env:TRUSTGUARD_CURSOR_DOWNLOAD_BASE) { $env:TRUSTGUARD_CURSOR_DOWNLOAD_BASE } else { 'https://github.com/NeuralTrust/trustguard-cursor-plugin/releases/download' }

    # 2. Cached pinned version.
    $bin = Join-Path $binDir "trustguard-cursor-$Version.exe"
    if ((Test-Path $bin) -and -not $InstallOnly) { Invoke-Hook $bin }

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
    $lock = Join-Path $binDir "install-$Version.lock"

    if ($InstallOnly) {
        Install-Binary $url $bin $wantSha
        Remove-Item -Force -Recurse -ErrorAction SilentlyContinue $lock
        exit 0
    }

    # 3. Not cached: install detached and fail open now. Downloading inline would
    # freeze the editor on every event, and Cursor kills the hook long before the
    # request gives up, so the binary would never land. A lock left behind by a
    # killed process is reclaimed after ten minutes.
    if ((Test-Path $lock) -and ((Get-Item $lock).CreationTime -lt (Get-Date).AddMinutes(-10))) {
        Remove-Item -Force -Recurse -ErrorAction SilentlyContinue $lock
    }
    if (-not (Test-Path $lock)) {
        New-Item -ItemType Directory -Path $lock -ErrorAction SilentlyContinue | Out-Null
        Start-Process -FilePath 'powershell' -WindowStyle Hidden -ArgumentList @(
            '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $PSCommandPath, '-InstallOnly'
        )
    }
    Exit-FailOpen "trustguard-cursor $Version not installed yet; fetching it in the background"
} catch {
    Exit-FailOpen "unexpected error: $($_.Exception.Message)"
}
