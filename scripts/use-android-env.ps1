# Dot-source this script to configure only the current PowerShell process.
param([string]$JavaHome, [string]$SdkRoot)

$repoRoot = Split-Path -Parent $PSScriptRoot
$tooling = Join-Path $repoRoot 'bin/android-tooling'
if (-not $JavaHome) {
    $localJdk = if (Test-Path -LiteralPath $tooling) {
        Get-ChildItem -LiteralPath $tooling -Directory -Filter 'jdk-25*' | Select-Object -First 1
    } else { $null }
    if ($localJdk) { $JavaHome = $localJdk.FullName } else { $JavaHome = $env:JAVA_HOME }
}
if (-not $JavaHome -or -not (Test-Path -LiteralPath (Join-Path $JavaHome 'bin/java.exe'))) {
    throw 'Provide -JavaHome pointing to JDK 25 for Gradle 9.1.0.'
}
$versionOutput = (& (Join-Path $JavaHome 'bin/java.exe') -version 2>&1 | Out-String)
if ($versionOutput -notmatch 'version "(\d+)\.' -or [int]$Matches[1] -ne 25) {
    throw 'This project validates its Gradle 9.1.0 build with JDK 25.'
}
if (-not $SdkRoot) {
    $candidates = @($env:ANDROID_HOME, $env:ANDROID_SDK_ROOT, (Join-Path $tooling 'sdk'))
    if ($env:LOCALAPPDATA) { $candidates += Join-Path $env:LOCALAPPDATA 'Android/Sdk' }
    $SdkRoot = $candidates | Where-Object { $_ -and (Test-Path -LiteralPath (Join-Path $_ 'platforms/android-36/android.jar')) } | Select-Object -First 1
}
if (-not $SdkRoot) { throw 'SDK platform 36 is missing. Provide -SdkRoot pointing to an installed SDK.' }
if (-not (Test-Path -LiteralPath (Join-Path $SdkRoot 'platforms/android-36/android.jar'))) {
    throw 'SDK platform 36 is missing. Provide -SdkRoot pointing to an installed SDK.'
}
foreach ($relative in @('build-tools/35.0.0', 'platform-tools', 'cmdline-tools/latest/bin/sdkmanager.bat')) {
    if (-not (Test-Path -LiteralPath (Join-Path $SdkRoot $relative))) {
        throw "Android SDK component missing: $relative"
    }
}
$env:JAVA_HOME = (Resolve-Path -LiteralPath $JavaHome).Path
$env:ANDROID_HOME = (Resolve-Path -LiteralPath $SdkRoot).Path
$env:GRADLE_USER_HOME = Join-Path $tooling 'gradle-home'
$env:ANDROID_USER_HOME = Join-Path $tooling 'android-user-home'
New-Item -ItemType Directory -Path $env:ANDROID_USER_HOME -Force | Out-Null
Write-Host 'Android environment configured for this PowerShell process only.'
