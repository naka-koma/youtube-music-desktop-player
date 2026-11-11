param(
    [string]$OutputDirectory = (Join-Path $PSScriptRoot '..\Bridge\dist')
)

$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$output = (New-Item -ItemType Directory -Path $OutputDirectory -Force).FullName
$assetName = 'youtube-music-desktop-player-windows-x64.zip'
$bridgeName = 'YouTubeMusicDesktopPlayerBridge.exe'
$bridge = Join-Path ([IO.Path]::GetTempPath()) ('ytm-build-' + [guid]::NewGuid().ToString('N') + '.exe')
$archive = Join-Path $output ([guid]::NewGuid().ToString('N') + '.tmp')

try {
    Push-Location (Join-Path $projectRoot 'Bridge')
    try {
        $target = & go env GOOS GOARCH
        if ($LASTEXITCODE -ne 0 -or ($target -join '/') -ne 'windows/amd64') {
            throw 'The release build requires a windows/amd64 Go target.'
        }
        & go build -trimpath -ldflags '-H=windowsgui' -o $bridge .
        if ($LASTEXITCODE -ne 0) { throw 'Go bridge build failed.' }
    } finally { Pop-Location }

    # Only ship tracked skin files and the files required by setup.bat.
    $files = & git -C $projectRoot -c core.quotePath=false ls-files -- Rainmeter Bridge/playlists.json Scripts/Setup.ps1 Scripts/Install-PlayerSkin.ps1 setup.bat README.md README.ja.md
    if ($LASTEXITCODE -ne 0 -or -not $files) { throw 'Could not enumerate the release files.' }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [IO.Compression.ZipFile]::Open($archive, [IO.Compression.ZipArchiveMode]::Create)
    try {
        foreach ($file in $files) {
            [IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, (Join-Path $projectRoot $file), $file) | Out-Null
        }
        [IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $bridge, "Bridge/bin/$bridgeName") | Out-Null
    } finally { $zip.Dispose() }

    $asset = Join-Path $output $assetName
    Move-Item -LiteralPath $archive -Destination $asset -Force
    $hash = (Get-FileHash -LiteralPath $asset -Algorithm SHA256).Hash.ToLowerInvariant()
    [IO.File]::WriteAllText((Join-Path $output 'SHA256SUMS.txt'), "$hash  $assetName`n", [Text.Encoding]::ASCII)
    Write-Output $asset
} finally {
    foreach ($temporaryFile in $bridge, $archive) {
        if (Test-Path -LiteralPath $temporaryFile) { Remove-Item -LiteralPath $temporaryFile }
    }
}
