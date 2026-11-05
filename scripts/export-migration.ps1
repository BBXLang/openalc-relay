param(
    [string]$OutputDirectory,
    [string]$Label = (Get-Date -Format 'yyyy-MM-dd'),
    [switch]$SourceOnly
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
if ($Label -notmatch '^\d{4}-\d{2}-\d{2}$') { throw 'Label must be YYYY-MM-DD' }
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $repoRoot 'bin/migration' }
$outputPath = [System.IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Path $outputPath -Force | Out-Null
$kind = if ($SourceOnly) { 'source' } else { 'migration' }
$zipPath = Join-Path $outputPath ("openalc-relay-$kind-$Label.zip")
if (Test-Path -LiteralPath $zipPath) { throw 'Archive already exists; choose another output directory rather than overwriting it.' }

# Include untracked source: this repository currently has no commits or remote.
$files = @(& git -C $repoRoot -c core.quotepath=false ls-files --cached --others --exclude-standard | Sort-Object -Unique)
if ($LASTEXITCODE -ne 0 -or $files.Count -eq 0) { throw 'Cannot enumerate source files' }
$apkPath = 'bin/openalc-relay-0.0.2-poc-zh.apk'
if (-not $SourceOnly) {
    if (-not (Test-Path -LiteralPath (Join-Path $repoRoot $apkPath))) { throw 'Chinese APK missing; use -SourceOnly instead' }
    $files += $apkPath
}
$manifestFiles = [System.Collections.Generic.List[object]]::new()
foreach ($relative in $files) {
    if ([System.IO.Path]::IsPathRooted($relative) -or $relative.Contains('\') -or $relative.Split('/') -contains '..') { throw 'Unsafe source path' }
    if ($relative -ne '.env.example' -and ($relative -match '(^|/)\.env($|\.)|\.(keystore|jks|p12|pfx|pem|key|db|sqlite|sqlite3|zip)$|(^|/)\.git/')) {
        throw 'Sensitive or archive file encountered; review the source list before exporting'
    }
    $absolute = Join-Path $repoRoot $relative
    $item = Get-Item -LiteralPath $absolute
    if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) { throw 'Linked files are not supported in this export' }
    $manifestFiles.Add([pscustomobject]@{
        path = $relative
        bytes = $item.Length
        sha256 = (Get-FileHash -LiteralPath $absolute -Algorithm SHA256).Hash.ToLowerInvariant()
    })
}
$manifest = [ordered]@{
    version = 1
    snapshot_date = $Label
    kind = $kind
    git_metadata_included = $false
    private_keys_included = $false
    files = $manifestFiles.ToArray()
}

# Copy streams instead of loading SDK trees or large APKs into memory.
$stream = [System.IO.File]::Open($zipPath, [System.IO.FileMode]::CreateNew)
$archive = [System.IO.Compression.ZipArchive]::new($stream, [System.IO.Compression.ZipArchiveMode]::Create)
try {
    foreach ($file in $manifestFiles) {
        $entry = $archive.CreateEntry('openalc-relay/' + $file.path, [System.IO.Compression.CompressionLevel]::Fastest)
        $inputStream = [System.IO.File]::OpenRead((Join-Path $repoRoot $file.path))
        $entryStream = $entry.Open()
        try { $inputStream.CopyTo($entryStream) } finally { $entryStream.Dispose(); $inputStream.Dispose() }
    }
    $entry = $archive.CreateEntry('openalc-relay/MIGRATION-MANIFEST.json')
    $writer = [System.IO.StreamWriter]::new($entry.Open(), [System.Text.UTF8Encoding]::new($false))
    try { $writer.Write(($manifest | ConvertTo-Json -Depth 5)) } finally { $writer.Dispose() }
} finally { $archive.Dispose(); $stream.Dispose() }

# Read every compressed entry back and compare against the source snapshot hashes.
$readStream = [System.IO.File]::OpenRead($zipPath)
$readArchive = [System.IO.Compression.ZipArchive]::new($readStream, [System.IO.Compression.ZipArchiveMode]::Read)
try {
    if ($readArchive.Entries.Count -ne $manifestFiles.Count + 1) { throw 'Archive entry count mismatch' }
    foreach ($file in $manifestFiles) {
        $entry = $readArchive.GetEntry('openalc-relay/' + $file.path)
        if (-not $entry -or $entry.Length -ne $file.bytes) { throw 'Archive entry missing or truncated' }
        $entryStream = $entry.Open()
        $hasher = [System.Security.Cryptography.SHA256]::Create()
        try { $hash = [System.BitConverter]::ToString($hasher.ComputeHash($entryStream)).Replace('-', '').ToLowerInvariant() }
        finally { $hasher.Dispose(); $entryStream.Dispose() }
        if ($hash -ne $file.sha256) { throw 'Archive content does not match snapshot' }
    }
} finally { $readArchive.Dispose(); $readStream.Dispose() }

$zipHash = (Get-FileHash -LiteralPath $zipPath -Algorithm SHA256).Hash.ToLowerInvariant()
($zipHash + '  ' + [System.IO.Path]::GetFileName($zipPath)) | Set-Content -LiteralPath ($zipPath + '.sha256') -Encoding ascii
[pscustomobject]@{ archive = $zipPath; files = $manifestFiles.Count; bytes = (Get-Item -LiteralPath $zipPath).Length; sha256 = $zipHash; archive_readback_verified = $true }
