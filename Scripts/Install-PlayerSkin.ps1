param(
    [Parameter(Mandatory)][string]$RainmeterRoot
)

$ErrorActionPreference = 'Stop'
$RainmeterRoot = (Resolve-Path -LiteralPath $RainmeterRoot).Path
$skinName = 'YouTube Music Desktop Player'
$skinFile = "$skinName.ini"
$projectRoot = Split-Path -Parent $PSScriptRoot
$skinSource = Join-Path (Join-Path $projectRoot 'Rainmeter') $skinName
$bridgeSource = Join-Path $projectRoot 'Bridge'
$rainmeterExe = Join-Path $RainmeterRoot 'Rainmeter.exe'
$rainmeterSettings = Join-Path $RainmeterRoot 'Rainmeter.ini'
if (-not (Test-Path -LiteralPath $rainmeterSettings)) {
    $rainmeterSettings = Join-Path $env:APPDATA 'Rainmeter\Rainmeter.ini'
}
$settings = if (Test-Path -LiteralPath $rainmeterSettings) { [IO.File]::ReadAllText($rainmeterSettings) } else { '' }
$skinPath = [regex]::Match($settings, '(?mi)^SkinPath=(.+)$').Groups[1].Value.Trim().Trim('"')
if (-not $skinPath) { $skinPath = Join-Path $RainmeterRoot 'Skins' }
New-Item -ItemType Directory -Force -Path $skinPath | Out-Null
$skinPath = (Resolve-Path -LiteralPath $skinPath).Path
$legacy = Join-Path $skinPath 'AmbientMusic'
$target = Join-Path $skinPath $skinName
$migrate = Test-Path -LiteralPath $legacy -PathType Container
if ($migrate -and (Test-Path -LiteralPath $target)) {
    throw "Both player folders exist under $skinPath. Resolve the duplicate before migrating."
}
if ($migrate -and ((Get-Item -LiteralPath $legacy).Attributes -band [IO.FileAttributes]::ReparsePoint)) {
    throw "The legacy player folder is a link: $legacy"
}
$sourceInstall = if ($migrate) { $legacy } else { $target }
$renameVariant = Test-Path -LiteralPath (Join-Path $sourceInstall 'AmbientMusic.ini') -PathType Leaf
if ($renameVariant -and (Test-Path -LiteralPath (Join-Path $sourceInstall $skinFile))) {
    throw "Both player INI files exist under $sourceInstall. Resolve the duplicate before migrating."
}
$legacyHeaders = '(?mi)^\[AmbientMusic(?=\\|])'
$newHeaders = '(?mi)^\[YouTube Music Desktop Player(?=\\|])'
if ($migrate -and $settings -match $legacyHeaders -and $settings -match $newHeaders) {
    throw "Both player configurations exist in $rainmeterSettings. Resolve the duplicate before migrating."
}

$stagedBridge = Join-Path ([IO.Path]::GetTempPath()) ("yt-music-player-" + [guid]::NewGuid().ToString('N') + ".exe")
try {
    Push-Location $bridgeSource
    try {
        & go build -trimpath -ldflags '-H=windowsgui' -o $stagedBridge .
        if ($LASTEXITCODE -ne 0) { throw 'Go bridge build failed.' }
    } finally { Pop-Location }

    $rainmeterRunning = Get-Process -Name Rainmeter -ErrorAction SilentlyContinue |
        Where-Object { $_.Path -eq $rainmeterExe }
    $previousConfig = if ($migrate) { 'AmbientMusic' } else { $skinName }
    $activeConfigs = @()
    if (($migrate -or $renameVariant) -and $rainmeterRunning) {
        foreach ($config in "$previousConfig\Fullscreen", $previousConfig) {
            $header = [regex]::Escape("[$config]")
            $section = [regex]::Match($settings, "(?mi)^$header[^\r\n]*\r?\n((?:(?!^\[)[\s\S])*)").Groups[1].Value
            if ($section -match '(?m)^Active=[1-9]\d*\s*$') {
                $activeConfigs += $config
                & $rainmeterExe '!DeactivateConfig' $config
            }
        }
        if ($activeConfigs.Count) { Start-Sleep -Milliseconds 700 }
    }

    $oldBridge = Join-Path $legacy '@Resources\Bridge\AmbientMusicBridge.exe'
    $bridgeExe = Join-Path $target '@Resources\Bridge\AmbientMusicBridge.exe'
    $runningBridge = Get-Process -Name AmbientMusicBridge -ErrorAction SilentlyContinue |
        Where-Object { $_.Path -in $oldBridge, $bridgeExe }
    if ($runningBridge) {
        $runningBridge | Stop-Process
        $runningBridge | Wait-Process -Timeout 5 -ErrorAction SilentlyContinue
    }
    if ($migrate) {
        # Both paths are direct children of the resolved skin directory.
        Move-Item -LiteralPath $legacy -Destination $target
        if ($settings -match $legacyHeaders) {
            $backup = "$rainmeterSettings.before-player-rename.bak"
            $current = [IO.File]::ReadAllText($rainmeterSettings)
            Copy-Item -LiteralPath $rainmeterSettings -Destination $backup -Force
            [IO.File]::WriteAllText($rainmeterSettings,
                [regex]::Replace($current, $legacyHeaders, "[$skinName"), [Text.Encoding]::Unicode)
        }
    }

    if ($renameVariant) {
        Move-Item -LiteralPath (Join-Path $target 'AmbientMusic.ini') -Destination (Join-Path $target $skinFile)
    }

    $personalFiles = '@Resources\Fonts.User.inc', '@Resources\Fullscreen.Settings.inc', '@Resources\Performance.Settings.inc', '@Resources\Backgrounds\Custom.inc'
    foreach ($file in (Get-ChildItem -LiteralPath $skinSource -Recurse -File)) {
        $relative = $file.FullName.Substring($skinSource.Length + 1)
        if ($relative -in $personalFiles) { continue }
        $destination = Join-Path $target $relative
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $destination) | Out-Null
        Copy-Item -LiteralPath $file.FullName -Destination $destination -Force
    }
    $resources = Join-Path $target '@Resources'
    $fullscreenSettings = Join-Path $resources 'Fullscreen.Settings.inc'
    if (-not (Test-Path -LiteralPath $fullscreenSettings)) {
        Copy-Item -LiteralPath (Join-Path $resources 'Fullscreen.Settings.example.inc') -Destination $fullscreenSettings
    }
    $performanceSettings = Join-Path $resources 'Performance.Settings.inc'
    if (-not (Test-Path -LiteralPath $performanceSettings)) {
        Copy-Item -LiteralPath (Join-Path $resources 'Performance.Settings.example.inc') -Destination $performanceSettings
    }
    $fontSettings = Join-Path $resources 'Fonts.User.inc'
    if (-not (Test-Path -LiteralPath $fontSettings)) {
        Set-Content -LiteralPath $fontSettings -Value '[Variables]' -Encoding Unicode
    }
    $customBackground = Join-Path $resources 'Backgrounds\Custom.inc'
    if (-not (Test-Path -LiteralPath $customBackground)) {
        Set-Content -LiteralPath $customBackground -Value '; Empty background. See Custom.example.inc.' -Encoding Unicode
    }
    $bridgeDir = Split-Path -Parent $bridgeExe
    New-Item -ItemType Directory -Force -Path $bridgeDir | Out-Null
    if (-not (Test-Path -LiteralPath (Join-Path $bridgeDir 'playlists.json'))) {
        Copy-Item -LiteralPath (Join-Path $bridgeSource 'playlists.json') -Destination $bridgeDir
    }
    Copy-Item -LiteralPath $stagedBridge -Destination $bridgeExe -Force
    if ($rainmeterRunning) {
        if (-not $migrate -and $settings -notmatch $newHeaders -and (Test-Path -LiteralPath $rainmeterSettings)) {
            & $rainmeterExe '!WriteKeyValue' "$skinName\Fullscreen" 'LoadOrder' '-2147483648' $rainmeterSettings
        }
        & $rainmeterExe '!RefreshApp'
        if ($activeConfigs -contains $previousConfig) {
            & $rainmeterExe '!ActivateConfig' $skinName $skinFile
        }
        if ($activeConfigs -contains "$previousConfig\Fullscreen") {
            & $rainmeterExe '!ActivateConfig' "$skinName\Fullscreen" 'Fullscreen.ini'
        }
    }
    return $bridgeExe
} finally {
    if (Test-Path -LiteralPath $stagedBridge) { Remove-Item -LiteralPath $stagedBridge -Force }
}
