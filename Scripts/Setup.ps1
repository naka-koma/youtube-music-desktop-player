param(
    [switch]$InstallYTMDesktop2,
    [switch]$InstallGo,
    [string]$ApiBaseUrl = 'http://127.0.0.1:13091',
    [string]$RainmeterRoot
)

$ErrorActionPreference = 'Stop'
$ApiBaseUrl = $ApiBaseUrl.TrimEnd('/')
$projectRoot = Split-Path -Parent $PSScriptRoot

function Find-YTMDesktop2 {
    $candidates = @(
        (Join-Path $env:LOCALAPPDATA 'Programs\YouTube Music for Desktop\ytmdesktop.exe'),
        (Join-Path $env:LOCALAPPDATA 'Programs\YouTube Music for Desktop\YouTube Music for Desktop.exe'),
        (Join-Path $env:ProgramFiles 'YouTube Music for Desktop\ytmdesktop.exe'),
        (Join-Path $env:LOCALAPPDATA 'Programs\ytmdesktop2\YouTube Music for Desktop.exe'),
        (Join-Path $env:ProgramFiles 'YouTube Music for Desktop\YouTube Music for Desktop.exe')
    )
    foreach ($candidate in $candidates) {
        if ($candidate -and (Test-Path -LiteralPath $candidate)) { return $candidate }
    }
    foreach ($hive in @('HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*', 'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*', 'HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*')) {
        $entry = Get-ItemProperty $hive -ErrorAction SilentlyContinue | Where-Object { $_.DisplayName -match 'YouTube Music for Desktop|YTMDesktop2' } | Select-Object -First 1
        if ($entry -and $entry.InstallLocation) {
            foreach ($exeName in @('ytmdesktop.exe', 'YouTube Music for Desktop.exe')) {
                $exe = Join-Path $entry.InstallLocation $exeName
                if (Test-Path -LiteralPath $exe) { return $exe }
            }
        }
    }
    $running = Get-CimInstance Win32_Process -Filter "Name = 'ytmdesktop.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.ExecutablePath -and $_.ExecutablePath -match '\\YouTube Music for Desktop\\ytmdesktop\.exe$' } |
        Select-Object -First 1
    if ($running) { return $running.ExecutablePath }
    return $null
}

function Install-YTMDesktop2FromOfficialRelease {
    Write-Host 'Looking up the latest stable YTMDesktop2 Windows installer on the official GitHub release page...'
    $release = Invoke-RestMethod -Uri 'https://api.github.com/repos/Venipa/ytmdesktop2/releases/latest' -Headers @{ 'User-Agent' = 'AmbientMusicDashboard-Setup' }
    $asset = $release.assets | Where-Object { $_.name -match '-setup\.exe$' } | Select-Object -First 1
    if (-not $asset) { throw 'The latest stable release did not include a Windows setup.exe asset.' }
    $installer = Join-Path ([IO.Path]::GetTempPath()) $asset.name
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $installer
    Write-Host "Starting the official installer: $($asset.name)"
    Start-Process -FilePath $installer -Wait
    return (Find-YTMDesktop2)
}

function Install-GoIfRequested {
    if (Get-Command go -ErrorAction SilentlyContinue) { return }
    if (-not $InstallGo) {
        Write-Host 'Go is required to build the local bridge.'
        $answer = Read-Host 'Install Go now with winget? [Y/n]'
        if ($answer -match '^(n|no)$') {
            throw 'Setup stopped. Install Go from https://go.dev/dl/, then run setup again.'
        }
    }
    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) { throw 'winget was not found; install Go from https://go.dev/dl/ and rerun Setup.ps1.' }
    & winget install --id GoLang.Go --exact --accept-source-agreements --accept-package-agreements
    if ($LASTEXITCODE -ne 0) { throw 'winget could not install Go. Install it manually from https://go.dev/dl/.' }
    $machinePath = [Environment]::GetEnvironmentVariable('Path', 'Machine')
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $env:Path = "$machinePath;$userPath"
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw 'Go was installed. Open a new PowerShell window and rerun Setup.ps1.' }
}

function Save-TokenForCurrentUser([string]$Token) {
    $dataDir = Join-Path $env:LOCALAPPDATA 'AmbientMusicDashboard'
    New-Item -ItemType Directory -Force -Path $dataDir | Out-Null
    $tokenPath = Join-Path $dataDir 'token'
    [IO.File]::WriteAllText($tokenPath, $Token, [Text.Encoding]::ASCII)
    $acl = Get-Acl -LiteralPath $tokenPath
    $acl.SetAccessRuleProtection($true, $false)
    $sid = [Security.Principal.WindowsIdentity]::GetCurrent().User
    $rule = New-Object Security.AccessControl.FileSystemAccessRule($sid, 'Read,Write', 'Allow')
    $acl.SetAccessRule($rule)
    Set-Acl -LiteralPath $tokenPath -AclObject $acl
    Write-Host "Saved the API token outside the repository with current-user-only file access: $tokenPath"
}

function Complete-ApiPairing {
    $root = Invoke-RestMethod -Uri "$ApiBaseUrl/" -TimeoutSec 4
    if (-not $root.authRequired) {
        Write-Host 'YTMDesktop2 API is reachable and authorization is not required; no token is needed.'
        return
    }
    $tokenPath = Join-Path $env:LOCALAPPDATA 'AmbientMusicDashboard\token'
    if (Test-Path -LiteralPath $tokenPath) {
        $token = [IO.File]::ReadAllText($tokenPath).Trim()
        if ($token) {
            try {
                $null = Invoke-RestMethod -Uri "$ApiBaseUrl/track/state" -Headers @{ Authorization = "Bearer $token" } -TimeoutSec 4
                Write-Host 'Using the existing YTMDesktop2 API authorization.'
                return
            } catch { }
        }
    }
    $codeResponse = Invoke-RestMethod -Method Post -Uri "$ApiBaseUrl/auth/requestcode" -ContentType 'application/json' -Body (@{
        appId = 'ambientmusic'
        appName = 'YouTube Music Desktop Player'
        appVersion = '1.0.0'
    } | ConvertTo-Json -Compress)
    Write-Host ''
    Write-Host "Pairing code: $($codeResponse.code)" -ForegroundColor Cyan
    Write-Host 'In YTMDesktop2, open Settings > API & Integrations > Authentication and approve this request.'
    Read-Host 'Press Enter after you have approved it' | Out-Null
    $tokenResponse = Invoke-RestMethod -Method Post -Uri "$ApiBaseUrl/auth/request" -ContentType 'application/json' -Body (@{
        appId = 'ambientmusic'
        code = $codeResponse.code
    } | ConvertTo-Json -Compress)
    if (-not $tokenResponse.token) { throw 'YTMDesktop2 did not return an API token after approval.' }
    Save-TokenForCurrentUser $tokenResponse.token
}

$ytmExe = Find-YTMDesktop2
if (-not $ytmExe) {
    Write-Host 'YTMDesktop2 was not found.' -ForegroundColor Yellow
    if (-not $InstallYTMDesktop2) {
        Write-Host 'The installer will be downloaded from the official stable GitHub release.'
        $answer = Read-Host 'Install YTMDesktop2 now? [Y/n]'
        if ($answer -match '^(n|no)$') {
            Write-Host 'Setup stopped. You can install it later from https://youtube-music.app/ and rerun setup.'
            exit 2
        }
    }
    $ytmExe = Install-YTMDesktop2FromOfficialRelease
    if (-not $ytmExe) { throw 'The installer finished but YTMDesktop2 was not found. Start it once, then rerun setup.' }
}

$bundledBridge = Join-Path $projectRoot 'Bridge\bin\YouTubeMusicDesktopPlayerBridge.exe'
$bridgeBinary = $null
if (Test-Path -LiteralPath $bundledBridge -PathType Leaf) {
    $bridgeBinary = $bundledBridge
} else {
    Install-GoIfRequested
}

if (-not $RainmeterRoot) {
    $rainmeterProcess = Get-Process Rainmeter -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($rainmeterProcess) { $RainmeterRoot = Split-Path -Parent $rainmeterProcess.Path }
}
if (-not $RainmeterRoot) {
    foreach ($candidate in @((Join-Path $env:ProgramFiles 'Rainmeter'), (Join-Path $env:LOCALAPPDATA 'Rainmeter'))) {
        if (Test-Path (Join-Path $candidate 'Rainmeter.exe')) { $RainmeterRoot = $candidate; break }
    }
}
if (-not $RainmeterRoot -or -not (Test-Path (Join-Path $RainmeterRoot 'Rainmeter.exe'))) {
    throw 'Rainmeter was not found. Rerun with -RainmeterRoot pointing to its installation folder.'
}

$bridgeExe = & (Join-Path $PSScriptRoot 'Install-PlayerSkin.ps1') -RainmeterRoot $RainmeterRoot -BridgeBinary $bridgeBinary
Write-Host "Installed the YouTube Music Desktop Player skin and bridge."

$runningYtm = Get-Process -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $ytmExe } | Select-Object -First 1
if (-not $runningYtm) {
    Write-Host 'Starting YTMDesktop2...'
    Start-Process -FilePath $ytmExe
}

$apiReady = $false
try {
    $null = Invoke-RestMethod -Uri "$ApiBaseUrl/" -TimeoutSec 2
    $apiReady = $true
} catch { }
if (-not $apiReady) {
    Write-Host ''
    Write-Host "Waiting for the YTMDesktop2 API at $ApiBaseUrl." -ForegroundColor Yellow
    Write-Host 'In YTMDesktop2, open Settings > API & Integrations > API and enable the API.'
    Write-Host 'Keep the bind address on 127.0.0.1. This setup will continue automatically when the API is Running.'
    $waitSeconds = 0
    while (-not $apiReady) {
        Start-Sleep -Seconds 2
        $waitSeconds += 2
        try {
            $null = Invoke-RestMethod -Uri "$ApiBaseUrl/" -TimeoutSec 2
            $apiReady = $true
        } catch {
            if ($waitSeconds % 10 -eq 0) { Write-Host "Still waiting for the local API... ($waitSeconds seconds)" }
        }
    }
}

Complete-ApiPairing

$listener = Get-NetTCPConnection -LocalAddress 127.0.0.1 -LocalPort 38471 -State Listen -ErrorAction SilentlyContinue
if (-not $listener) {
    Start-Process -FilePath $bridgeExe -WindowStyle Hidden
    Start-Sleep -Milliseconds 700
}
try {
    $null = Invoke-RestMethod -Uri 'http://127.0.0.1:38471/state' -TimeoutSec 4
} catch {
    throw "The bridge did not become ready at http://127.0.0.1:38471. $($_.Exception.Message)"
}

Write-Host ''
Write-Host 'Setup is complete. In Rainmeter, load YouTube Music Desktop Player\YouTube Music Desktop Player.ini from Manage Rainmeter > Skins.' -ForegroundColor Green
Write-Host 'YTMDesktop2 remains available in the system tray while the bridge serves the Rainmeter skin.'
