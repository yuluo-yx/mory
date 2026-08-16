param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("amd64", "arm64")]
    [string]$Architecture
)

$ErrorActionPreference = "Stop"
$Repository = Split-Path -Parent $PSScriptRoot
$Version = (Get-Content (Join-Path $Repository "package.json") -Raw | ConvertFrom-Json).version
$Project = Join-Path $Repository "cmd/mory-windows"
$Destination = Join-Path $Repository "dist/windows"
$WailsVersion = "v2.13.0"
$PublicArchitecture = if ($Architecture -eq "amd64") { "x64" } else { "arm64" }

New-Item -ItemType Directory -Force -Path $Destination | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $Project "build") | Out-Null
# 复用现有 Mory 图标；Wails 会从 PNG 生成带多种尺寸的 Windows ICO 与资源对象。
Copy-Item (Join-Path $Repository "build/icon.png") (Join-Path $Project "build/appicon.png") -Force
Push-Location $Project
try {
    go run "github.com/wailsapp/wails/v2/cmd/wails@$WailsVersion" build `
        -platform "windows/$Architecture" `
        -webview2 embed `
        -skipbindings `
        -trimpath `
        -nsis

    $Binary = Join-Path $Project "build/bin/Mory.exe"
    $Installer = Join-Path $Project "build/bin/Mory-$Architecture-installer.exe"
    if (-not (Test-Path $Binary)) { throw "Wails portable binary was not generated: $Binary" }
    if (-not (Test-Path $Installer)) { throw "Wails NSIS installer was not generated: $Installer" }

    Copy-Item $Binary (Join-Path $Destination "Mory-Portable-$Version-$PublicArchitecture.exe") -Force
    Copy-Item $Installer (Join-Path $Destination "Mory-Setup-$Version-$PublicArchitecture.exe") -Force
}
finally {
    Pop-Location
}
