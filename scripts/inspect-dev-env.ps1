# Read-only inventory; safe to run before installing SDKs or initializing Git.
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot

$tools = foreach ($name in @('git', 'go', 'node', 'npm', 'java')) {
    $command = Get-Command $name -ErrorAction SilentlyContinue | Select-Object -First 1
    [pscustomobject]@{ tool = $name; found = [bool]$command; path = $(if ($command) { $command.Source } else { $null }) }
}

$sdkCandidates = @($env:ANDROID_HOME, $env:ANDROID_SDK_ROOT, (Join-Path $repoRoot 'bin/android-tooling/sdk'))
if ($env:LOCALAPPDATA) { $sdkCandidates += Join-Path $env:LOCALAPPDATA 'Android/Sdk' }
$sdks = foreach ($sdk in ($sdkCandidates | Where-Object { $_ } | Select-Object -Unique)) {
    [pscustomobject]@{
        sdk = $sdk
        platform_36 = Test-Path -LiteralPath (Join-Path $sdk 'platforms/android-36/android.jar')
        build_tools_35 = Test-Path -LiteralPath (Join-Path $sdk 'build-tools/35.0.0')
        platform_tools = Test-Path -LiteralPath (Join-Path $sdk 'platform-tools')
        command_line_tools = Test-Path -LiteralPath (Join-Path $sdk 'cmdline-tools/latest/bin/sdkmanager.bat')
    }
}
[pscustomobject]@{
    powershell = $PSVersionTable.PSVersion.ToString()
    tools = @($tools)
    java_home = $env:JAVA_HOME
    android_home = $env:ANDROID_HOME
    gradle_user_home = $env:GRADLE_USER_HOME
    project_jdk_25 = [bool](Get-ChildItem -LiteralPath (Join-Path $repoRoot 'bin/android-tooling') -Directory -Filter 'jdk-25*' -ErrorAction SilentlyContinue | Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'bin/java.exe') } | Select-Object -First 1)
    sdk_candidates = @($sdks)
} | ConvertTo-Json -Depth 4
Write-Host 'Requirements: Go >=1.26; Node >=22.12; JDK 25 with the Gradle 9.1.0 wrapper.'
Write-Host 'This inventory does not install tools, start adb, access phones or verify version compatibility.'
