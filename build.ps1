# Builds dist\recast\recast.exe.
#   .\build.ps1             build
#   .\build.ps1 -Icon       also redraw build\appicon.png (and the .ico from it)
#   .\build.ps1 -FFmpeg     also copy ffmpeg.exe + ffprobe.exe from PATH next to the exe
#   .\build.ps1 -NoTest     skip the conversion tests
#   .\build.ps1 -Release    also pack release\recast-<ver>-win64.zip (with ffmpeg)
#                           and recast-<ver>-win64-lite.zip (without) for GitHub
param([switch]$Icon, [switch]$FFmpeg, [switch]$NoTest, [switch]$Release)
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

if ($Icon -or -not (Test-Path build\appicon.png)) {
    go run ./tools/mkicon
    if ($LASTEXITCODE) { throw 'mkicon failed' }
    Remove-Item build\windows\icon.ico -ErrorAction SilentlyContinue
}

if (-not $NoTest) {
    go test -count=1 .
    if ($LASTEXITCODE) { throw 'tests failed' }
}

wails build -clean -trimpath -ldflags '-s -w'
if ($LASTEXITCODE) { throw 'wails build failed' }

$dist = 'dist\recast'
New-Item -ItemType Directory -Force $dist | Out-Null
# A running exe cannot be overwritten but can be renamed: the open window keeps
# working and the next launch gets the new build.
Remove-Item "$dist\recast.old.exe" -ErrorAction SilentlyContinue
if (Test-Path "$dist\recast.old.exe") { throw 'an older recast.old.exe is still running - close it first' }
if (Test-Path "$dist\recast.exe") { Rename-Item "$dist\recast.exe" recast.old.exe }
Copy-Item build\bin\recast.exe $dist -Force
if ($FFmpeg) {
    $ff = (Get-Command ffmpeg -ErrorAction Stop).Source
    Copy-Item $ff, (Join-Path (Split-Path $ff) 'ffprobe.exe') $dist -Force
    # gyan.dev builds are GPL: the license travels with the binaries
    $lic = Join-Path (Split-Path (Split-Path $ff)) 'LICENSE'
    if (Test-Path $lic) { Copy-Item $lic "$dist\ffmpeg-LICENSE.txt" -Force }
}
Get-ChildItem $dist -File | Select-Object Name, @{n = 'MB'; e = { [math]::Round($_.Length / 1MB, 1) } }

if ($Release) {
    # Packed from a clean folder, never from dist: dist\recast\data holds this
    # machine's settings and WebView cache.
    $ver = (Get-Content wails.json -Raw | ConvertFrom-Json).info.productVersion
    $ff = (Get-Command ffmpeg -ErrorAction Stop).Source
    $ffDir = Split-Path $ff
    New-Item -ItemType Directory -Force release | Out-Null
    foreach ($full in $true, $false) {
        $name = "recast-$ver-win64" + $(if ($full) { '' } else { '-lite' })
        $stage = Join-Path release $name
        Remove-Item $stage, "release\$name.zip" -Recurse -Force -ErrorAction SilentlyContinue
        New-Item -ItemType Directory $stage | Out-Null
        Copy-Item build\bin\recast.exe, README.txt, LICENSE $stage
        if ($full) {
            Copy-Item $ff, (Join-Path $ffDir 'ffprobe.exe') $stage
            $lic = Join-Path (Split-Path $ffDir) 'LICENSE'
            if (Test-Path $lic) { Copy-Item $lic "$stage\ffmpeg-LICENSE.txt" }
            # the build's own README names the exact source commit (GPL)
            $rd = Join-Path (Split-Path $ffDir) 'README.txt'
            if (Test-Path $rd) { Copy-Item $rd "$stage\ffmpeg-README.txt" }
        }
        # the zip opens to one folder named like the archive
        tar.exe -a -c -f "release\$name.zip" -C release $name
        if ($LASTEXITCODE) { throw "zip $name failed" }
        Remove-Item $stage -Recurse -Force
    }
    Get-ChildItem release -Filter *.zip | Select-Object Name, @{n = 'MB'; e = { [math]::Round($_.Length / 1MB, 1) } }
}
