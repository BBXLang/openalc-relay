param(
    [string]$JavaHome = $env:JAVA_HOME,
    [string]$GradleUserHome = $env:GRADLE_USER_HOME,
    [switch]$WithWeb
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$androidPath = Join-Path $repoRoot 'apps/android'
$workPath = Join-Path $repoRoot ('bin/poc-integration/' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $workPath -Force | Out-Null
$onWindows = [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)
$wrapper = Join-Path $androidPath $(if ($onWindows) { 'gradlew.bat' } else { 'gradlew' })
$binary = Join-Path $workPath $(if ($onWindows) { 'poc-collector.exe' } else { 'poc-collector' })
$queuePath = Join-Path $workPath 'outbox.db'
$collectorPath = Join-Path $workPath 'collector.db'
$oldJava = $env:JAVA_HOME
$oldGradleHome = $env:GRADLE_USER_HOME
$oldPocBase = $env:POC_BASE_URL
$oldPocReport = $env:POC_CHAIN_REPORT
if ($JavaHome) { $env:JAVA_HOME = $JavaHome }
if ($GradleUserHome) { $env:GRADLE_USER_HOME = $GradleUserHome }
$collector = $null
$client = [System.Net.Http.HttpClient]::new()
$client.Timeout = [TimeSpan]::FromSeconds(2)

function Invoke-PocGradle([string[]]$TaskArguments) {
    Push-Location $androidPath
    try {
        $output = & $wrapper '-PpocJvmOnly=true' '--console=plain' '--max-workers=2' @TaskArguments 2>&1
        if ($LASTEXITCODE -ne 0) {
            $output | Select-Object -Last 60 | Write-Host
            throw 'Gradle PoC check failed'
        }
        $output | Set-Content -LiteralPath (Join-Path $workPath ('gradle-' + [guid]::NewGuid().ToString('N') + '.log'))
        $jsonLine = $output | Where-Object { $_.ToString().StartsWith('{"pending":') } | Select-Object -Last 1
        if ($jsonLine) { return $jsonLine.ToString() | ConvertFrom-Json }
    } finally { Pop-Location }
}

try {
    Write-Host 'Running real Room/SQLite JVM tests'
    Invoke-PocGradle -TaskArguments @(':poc-core:jvmTest') | Out-Null
    Push-Location (Join-Path $repoRoot 'server')
    try {
        & go build -o $binary './cmd/poc-collector'
        if ($LASTEXITCODE -ne 0) { throw 'Collector build failed' }
    } finally { Pop-Location }

    $probe = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    $probe.Start()
    $port = $probe.LocalEndpoint.Port
    $probe.Stop()
    $endpoint = "http://127.0.0.1:$port/poc/v1/events"
    $baseArgs = @(':poc-core:pocHarness', "-PpocDatabase=$queuePath", "-PpocEndpoint=$endpoint")
    Write-Host 'Persisting 100 fixtures and verifying offline retention in a new JVM process'
    $seed = Invoke-PocGradle -TaskArguments ($baseArgs + @('-PpocMode=seed', '-PpocCount=100'))
    $offline = Invoke-PocGradle -TaskArguments ($baseArgs + @('-PpocMode=sync'))
    if ($seed.pending -ne 100 -or $offline.pending -ne 100 -or $offline.synced -ne 0) { throw 'Offline retention failed' }

    $startOptions = @{
        FilePath = $binary
        ArgumentList = @('-addr', "127.0.0.1:$port", '-database', ('"' + $collectorPath + '"'), '-drop-first-ack')
        PassThru = $true
        RedirectStandardOutput = (Join-Path $workPath 'collector.stdout.log')
        RedirectStandardError = (Join-Path $workPath 'collector.stderr.log')
    }
    if ($onWindows) { $startOptions.WindowStyle = 'Hidden' }
    if ($WithWeb) {
        $webDir = Join-Path $repoRoot 'apps/web/dist'
        if (-not (Test-Path -LiteralPath (Join-Path $webDir 'index.html'))) { throw 'Build Web assets before -WithWeb' }
        $startOptions.ArgumentList += @('-web-dir', ('"' + $webDir + '"'))
    }
    $collector = Start-Process @startOptions
    $available = $false
    for ($attempt = 0; $attempt -lt 50; $attempt++) {
        try {
            $response = $client.GetAsync("http://127.0.0.1:$port/poc/v1/stats").GetAwaiter().GetResult()
            if ([int]$response.StatusCode -eq 200) { $available = $true; break }
        } catch { Start-Sleep -Milliseconds 100 }
    }
    if (-not $available) { throw 'Collector did not start' }
    Write-Host 'Restoring connectivity with a deliberate ACK-loss fault'
    $recovered = $null
    for ($attempt = 0; $attempt -lt 3; $attempt++) {
        $recovered = Invoke-PocGradle -TaskArguments ($baseArgs + @('-PpocMode=sync'))
        if ($recovered.synced -eq 100) { break }
    }
    $response = $client.GetAsync("http://127.0.0.1:$port/poc/v1/stats").GetAwaiter().GetResult()
    $stats = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult() | ConvertFrom-Json
    if ($recovered.pending -ne 0 -or $recovered.synced -ne 100 -or $recovered.rejected -ne 0 -or $stats.event_count -ne 100) {
        throw 'ACK recovery or server deduplication failed'
    }
    $report = [pscustomobject]@{
        synthetic_fixtures = 100
        offline_pending = $offline.pending
        recovered_synced = $recovered.synced
        collector_event_count = $stats.event_count
        rejected = $recovered.rejected
        android_device_tested = $false
        installation_id = $seed.installation_id
        fixture_ids = $seed.fixture_ids
        web_browser_tested = $false
        collector_restart_tested = $false
        evidence_directory = $workPath
    }
    if ($WithWeb) {
        Write-Host 'Restarting collector on the same SQLite file before real-browser reads'
        Stop-Process -Id $collector.Id
        $collector.WaitForExit()
        $startOptions.RedirectStandardOutput = Join-Path $workPath 'collector-restarted.stdout.log'
        $startOptions.RedirectStandardError = Join-Path $workPath 'collector-restarted.stderr.log'
        $collector = Start-Process @startOptions
        $available = $false
        for ($attempt = 0; $attempt -lt 50; $attempt++) {
            try {
                $response = $client.GetAsync("http://127.0.0.1:$port/poc/v1/stats").GetAwaiter().GetResult()
                if ([int]$response.StatusCode -eq 200) { $available = $true; break }
            } catch { Start-Sleep -Milliseconds 100 }
        }
        if (-not $available) { throw 'Collector restart failed' }
        $report.collector_restart_tested = $true
        $env:POC_BASE_URL = "http://127.0.0.1:$port"
        $env:POC_CHAIN_REPORT = Join-Path $workPath 'result.json'
        $report | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $env:POC_CHAIN_REPORT
        Push-Location (Join-Path $repoRoot 'apps/web')
        try {
            & npm run test:e2e
            if ($LASTEXITCODE -ne 0) { throw 'Real browser verification failed' }
            Copy-Item -LiteralPath 'build/playwright-report.json' -Destination (Join-Path $workPath 'browser-report.json')
        } finally { Pop-Location }
        $report.web_browser_tested = $true
    }
    $report | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $workPath 'result.json')
    $report | Select-Object -Property * -ExcludeProperty fixture_ids | ConvertTo-Json -Depth 5 | Write-Host
} finally {
    $client.Dispose()
    if ($collector -and -not $collector.HasExited) {
        Stop-Process -Id $collector.Id
        $collector.WaitForExit()
    }
    $env:JAVA_HOME = $oldJava
    $env:GRADLE_USER_HOME = $oldGradleHome
    $env:POC_BASE_URL = $oldPocBase
    $env:POC_CHAIN_REPORT = $oldPocReport
}
