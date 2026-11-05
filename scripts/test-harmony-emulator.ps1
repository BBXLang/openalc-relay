param(
    [string]$Serial = '127.0.0.1:5555',
    [int]$Port = 18080,
    [string]$HdcPath = ''
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$runDir = Join-Path $repo ('bin/harmony-exchange/run-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $runDir -Force | Out-Null
$hdc = $HdcPath
if ($hdc -eq '') {
    $installedHdc = Get-Command hdc -ErrorAction SilentlyContinue
    if ($null -ne $installedHdc) { $hdc = $installedHdc.Source }
    else { $hdc = 'D:\ide\Huawei\DevEco Studio\sdk\default\openharmony\toolchains\hdc.exe' }
}
$admin = Join-Path $runDir 'relay-admin.exe'
$relay = Join-Path $runDir 'relay.exe'
$database = Join-Path $runDir 'relay.db'
$keyFile = Join-Path $runDir 'keys.json'
$passwordFile = Join-Path $runDir 'owner-password.txt'
$origin = "https://127.0.0.1:$Port"
$localURL = "http://127.0.0.1:$Port"
$serverProcess = $null
$tunnelOpened = $false

function Find-Node($node, [string]$text, [string]$hint = '') {
    if (($text -ne '' -and $node.attributes.text -eq $text) -or
        ($hint -ne '' -and $node.attributes.hint -eq $hint)) { return $node }
    foreach ($child in $node.children) {
        $found = Find-Node $child $text $hint
        if ($null -ne $found) { return $found }
    }
    return $null
}

function Get-AppLayout {
    & $hdc -t $Serial shell 'uitest dumpLayout -b org.openalc.relay.harmony -p /data/local/tmp/relay-app-layout.json' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Emulator layout dump failed' }
    $layoutPath = Join-Path $runDir 'layout.json'
    & $hdc -t $Serial file recv '/data/local/tmp/relay-app-layout.json' $layoutPath | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Emulator layout transfer failed' }
    return (Get-Content -LiteralPath $layoutPath -Raw -Encoding utf8 | ConvertFrom-Json -Depth 100)
}

function Node-Center($node) {
    $match = [regex]::Match($node.attributes.bounds, '^\[(\d+),(\d+)\]\[(\d+),(\d+)\]$')
    if (-not $match.Success) { throw 'Unexpected emulator layout bounds' }
    $left = [int]$match.Groups[1].Value
    $top = [int]$match.Groups[2].Value
    $right = [int]$match.Groups[3].Value
    $bottom = [int]$match.Groups[4].Value
    return @([int](($left + $right) / 2), [int](($top + $bottom) / 2))
}

function Click-Text([string]$text) {
    $layout = Get-AppLayout
    $node = Find-Node $layout $text
    if ($null -eq $node) { throw "Emulator control missing: $text" }
    $point = Node-Center $node
    & $hdc -t $Serial shell "uitest uiInput click $($point[0]) $($point[1])" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Emulator click failed: $text" }
}

function Owner-Request([string]$method, [string]$path, $body = $null) {
    $args = @{ Uri = ($localURL + $path); Method = $method; Headers = $script:ownerHeaders; TimeoutSec = 8 }
    if ($null -ne $body) {
        $args.ContentType = 'application/json'
        $args.Body = ($body | ConvertTo-Json -Compress)
    }
    return Invoke-RestMethod @args
}

try {
    if (-not (Test-Path -LiteralPath $hdc)) { throw 'DevEco hdc is unavailable' }
    $targets = & $hdc list targets
    if ($targets -notcontains $Serial) { throw 'Requested HarmonyOS emulator is not connected' }
    Push-Location (Join-Path $repo 'server')
    try {
        go build -o $admin ./cmd/relay-admin
        if ($LASTEXITCODE -ne 0) { throw 'relay-admin build failed' }
        go build -o $relay ./cmd/relay
        if ($LASTEXITCODE -ne 0) { throw 'relay build failed' }
    } finally { Pop-Location }
    $password = [Convert]::ToBase64String([System.Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
    [System.IO.File]::WriteAllText($passwordFile, $password, [System.Text.UTF8Encoding]::new($false))
    & $admin keygen -out $keyFile | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'keygen failed' }
    & $admin bootstrap -database $database -key-file $keyFile -password-file $passwordFile | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'owner bootstrap failed' }
    $env:RELAY_DATABASE_PATH = $database
    $env:RELAY_MASTER_KEY_FILE = $keyFile
    $env:RELAY_PUBLIC_ORIGIN = $origin
    $env:RELAY_HTTP_ADDR = "127.0.0.1:$Port"
    $serverProcess = Start-Process -FilePath $relay -PassThru -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $runDir 'server-out.log') `
        -RedirectStandardError (Join-Path $runDir 'server-err.log')
    $ready = $false
    for ($attempt = 0; $attempt -lt 50; $attempt++) {
        Start-Sleep -Milliseconds 100
        try {
            $health = Invoke-RestMethod -Uri ($localURL + '/readyz') -TimeoutSec 1
            if ($health.status -eq 'ok') { $ready = $true; break }
        } catch { }
    }
    if (-not $ready) { throw 'Relay server did not become ready' }
    & $hdc -t $Serial rport "tcp:$Port" "tcp:$Port" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'hdc loopback reverse tunnel failed' }
    $tunnelOpened = $true
    $login = Invoke-WebRequest -Uri ($localURL + '/v1/session') -Method Post `
        -Headers @{ Origin = $origin } -ContentType 'application/json' `
        -Body (@{ password = $password } | ConvertTo-Json -Compress) -TimeoutSec 8
    $cookieMatch = [regex]::Match(($login.Headers['Set-Cookie'] -join ';'), '__Host-relay-session=([^;]+)')
    if (-not $cookieMatch.Success) { throw 'Owner session cookie missing' }
    $session = $login.Content | ConvertFrom-Json
    $script:ownerHeaders = @{
        Cookie = '__Host-relay-session=' + $cookieMatch.Groups[1].Value
        Origin = $origin
        'X-CSRF-Token' = $session.csrf_token
    }
    $pairing = Owner-Request 'Post' '/v1/pairings'
    Write-Output "Pairing invitation created; emulator=$Serial"
    & $hdc -t $Serial shell 'aa force-stop org.openalc.relay.harmony' | Out-Null
    & $hdc -t $Serial shell "aa start -a EntryAbility -b org.openalc.relay.harmony --ps relay_pairing_id $($pairing.pairing_id)" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'HarmonyOS application did not start' }
    Click-Text '设备'
    $layout = Get-AppLayout
    if ($null -eq (Find-Node $layout $pairing.pairing_id)) { throw 'Pairing invitation did not reach emulator' }
    Click-Text '申请配对'
    $application = $null
    for ($attempt = 0; $attempt -lt 40; $attempt++) {
        Start-Sleep -Milliseconds 250
        $current = Owner-Request 'Get' ('/v1/pairings/' + $pairing.pairing_id)
        if ($null -ne $current.request) { $application = $current.request; break }
    }
    if ($null -eq $application) { throw 'Emulator did not apply for pairing' }
    $layout = Get-AppLayout
    $shown = Find-Node $layout ('核对码：' + $application.comparison_code)
    if ($null -eq $shown) { throw 'Emulator comparison code does not match owner view' }
    Owner-Request 'Post' ('/v1/pairings/' + $pairing.pairing_id + '/approve') @{
        request_id = $application.request_id
        comparison_code = $application.comparison_code
    } | Out-Null
    Click-Text '批准后完成配对'
    $device = $null
    for ($attempt = 0; $attempt -lt 40; $attempt++) {
        Start-Sleep -Milliseconds 250
        $devices = Owner-Request 'Get' '/v1/devices'
        if ($devices.devices.Count -eq 1) { $device = $devices.devices[0]; break }
    }
    if ($null -eq $device) { throw 'Emulator did not exchange the approved claim' }
    Write-Output 'Pairing approval and one-time device credential exchange passed'
    $ping = Owner-Request 'Post' ('/v1/devices/' + $device.device_id + '/synthetic-pings')
    Click-Text '拉取 ping'
    $received = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        Start-Sleep -Milliseconds 250
        $layout = Get-AppLayout
        if ($null -ne (Find-Node $layout '已收到服务端合成 ping，等待手动回执')) { $received = $true; break }
    }
    if (-not $received) { throw 'Emulator did not display the server ping' }
    $pending = Owner-Request 'Get' ('/v1/synthetic-pings/' + $ping.ping_id)
    if ($null -ne $pending.acknowledged_at) { throw 'Ping was acknowledged before explicit emulator action' }
    Click-Text '确认并回执'
    $acknowledged = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        Start-Sleep -Milliseconds 250
        $pending = Owner-Request 'Get' ('/v1/synthetic-pings/' + $ping.ping_id)
        if ($null -ne $pending.acknowledged_at) { $acknowledged = $true; break }
    }
    if (-not $acknowledged) { throw 'Server did not persist the emulator ping acknowledgement' }
    Write-Output 'Server-to-emulator ping and explicit acknowledgement passed'
    Click-Text '收件箱'
    Click-Text '↑'
    $uploaded = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        Start-Sleep -Milliseconds 250
        $events = Owner-Request 'Get' '/v1/synthetic-events'
        if ($events.events.Count -eq 1) { $uploaded = $true; break }
    }
    if (-not $uploaded) { throw 'Server did not commit the emulator synthetic event' }
    $layout = Get-AppLayout
    if ($null -eq (Find-Node $layout '合成事件已提交')) { throw 'Emulator did not show the server event ACK' }
    Write-Output 'Emulator-to-server synthetic event and committed ACK passed'
    & $hdc -t $Serial shell 'uitest screenCap -p /data/local/tmp/relay-exchange.png' | Out-Null
    & $hdc -t $Serial file recv '/data/local/tmp/relay-exchange.png' (Join-Path $runDir 'emulator.png') | Out-Null
    Owner-Request 'Post' ('/v1/devices/' + $device.device_id + '/revoke') | Out-Null
    Click-Text '设备'
    Click-Text '拉取 ping'
    $layout = Get-AppLayout
    if ($null -eq (Find-Node $layout '拉取失败，请检查连接或设备权限')) {
        throw 'Revoked emulator credential was not rejected'
    }
    Write-Output 'Device revocation rejected the next emulator sync'
    Write-Output "Synthetic evidence: $runDir"
} finally {
    if ($tunnelOpened) {
        & $hdc -t $Serial fport rm "tcp:$Port" "tcp:$Port" | Out-Null
    }
    if ($null -ne $serverProcess) {
        Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue
    }
    Remove-Item Env:RELAY_DATABASE_PATH, Env:RELAY_MASTER_KEY_FILE, Env:RELAY_PUBLIC_ORIGIN, Env:RELAY_HTTP_ADDR -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $passwordFile, $keyFile -Force -ErrorAction SilentlyContinue
}
