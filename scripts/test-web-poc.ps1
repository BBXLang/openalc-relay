param(
    [string]$JavaHome = $env:JAVA_HOME,
    [string]$GradleUserHome = $env:GRADLE_USER_HOME
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location (Join-Path $repoRoot 'apps/web')
try {
    & npm ci --no-fund
    if ($LASTEXITCODE -ne 0) { throw 'Web dependency installation failed' }
    & npm run build
    if ($LASTEXITCODE -ne 0) { throw 'Web build failed' }
    & npx playwright install chromium
    if ($LASTEXITCODE -ne 0) { throw 'Chromium installation failed' }
} finally { Pop-Location }
& (Join-Path $PSScriptRoot 'test-android-poc.ps1') -JavaHome $JavaHome -GradleUserHome $GradleUserHome -WithWeb
