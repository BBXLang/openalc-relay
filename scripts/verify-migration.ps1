param([string]$Root = (Split-Path -Parent $PSScriptRoot))

$ErrorActionPreference = 'Stop'
$rootPath = (Resolve-Path -LiteralPath $Root).Path
$prefix = $rootPath.TrimEnd([char[]]'\/') + [System.IO.Path]::DirectorySeparatorChar
$manifestPath = Join-Path $rootPath 'MIGRATION-MANIFEST.json'
if (-not (Test-Path -LiteralPath $manifestPath)) { throw 'Migration manifest missing; run this on an extracted migration package.' }
$manifest = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
if ($manifest.version -ne 1 -or $manifest.files.Count -lt 1) { throw 'Unsupported or empty migration manifest' }
$seen = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
$issues = [System.Collections.Generic.List[string]]::new()
foreach ($file in $manifest.files) {
    $relative = [string]$file.path
    $candidate = [System.IO.Path]::GetFullPath((Join-Path $rootPath $relative))
    if ([System.IO.Path]::IsPathRooted($relative) -or $relative.Contains('\') -or
        -not $candidate.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase) -or
        -not $seen.Add($relative)) { throw 'Invalid or duplicate manifest path' }
    if (-not (Test-Path -LiteralPath $candidate -PathType Leaf)) { $issues.Add('missing: ' + $relative); continue }
    $item = Get-Item -LiteralPath $candidate
    if ($item.Length -ne $file.bytes) { $issues.Add('size mismatch: ' + $relative); continue }
    if ((Get-FileHash -LiteralPath $candidate -Algorithm SHA256).Hash -ne $file.sha256) { $issues.Add('hash mismatch: ' + $relative) }
}
if ($issues.Count) { throw ($issues -join [Environment]::NewLine) }
Write-Host ('Migration integrity verified: ' + $manifest.files.Count + ' files. This is a transfer check, not a build/device test.')
