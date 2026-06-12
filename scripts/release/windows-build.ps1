param(
    [Parameter(Mandatory = $true)]
    [string]$Version,
    [string]$ApiBaseUrl = "http://127.0.0.1:18080",
    [string]$OutputDir = "dist/windows",
    [switch]$SkipInstaller
)

$ErrorActionPreference = "Stop"
$frontend = Join-Path $PSScriptRoot "..\..\src\frontend"
Push-Location $frontend
try {
    flutter pub get
    flutter build windows --release `
        --dart-define=VOICE_APP_VERSION=$Version `
        --dart-define=VOICE_API_BASE_URL=$ApiBaseUrl

    $buildDir = Join-Path $frontend "build\windows\x64\runner\Release"
    $outRoot = Join-Path (Resolve-Path (Join-Path $PSScriptRoot "..\..")) $OutputDir
    New-Item -ItemType Directory -Force -Path $outRoot | Out-Null
    Copy-Item -Path (Join-Path $buildDir "*") -Destination $outRoot -Recurse -Force

    if (-not $SkipInstaller) {
        $iscc = Get-Command iscc.exe -ErrorAction SilentlyContinue
        if ($null -eq $iscc) {
            Write-Warning "Inno Setup (iscc.exe) not found; skipping installer. ZIP output is in $outRoot"
        } else {
            $iss = Join-Path $PSScriptRoot "voice-setup.iss"
            if (Test-Path $iss) {
                & $iscc.Source "/DAppVersion=$Version" "/DBuildDir=$buildDir" $iss
            } else {
                Write-Warning "Missing $iss; copy Release folder manually for distribution."
            }
        }
    }

    Write-Host "Windows release artifacts: $outRoot"
} finally {
    Pop-Location
}
