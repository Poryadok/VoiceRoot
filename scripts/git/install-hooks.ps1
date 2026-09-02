# Install Voice git safety hooks (repo + optional Cursor beforeShellExecution).
# Run from repo root: .\scripts\git\install-hooks.ps1

param(
    [switch]$SkipCursor
)

$ErrorActionPreference = 'Stop'
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$GitHooksDir = Join-Path $RepoRoot '.git\hooks'
$SourceHooksDir = Join-Path $PSScriptRoot 'hooks'

if (-not (Test-Path $GitHooksDir)) {
    throw "Not a git repository: $GitHooksDir missing"
}

foreach ($name in @('pre-rebase', 'pre-push')) {
    $src = Join-Path $SourceHooksDir $name
    $dst = Join-Path $GitHooksDir $name
    Copy-Item -LiteralPath $src -Destination $dst -Force
    Write-Host "Installed git hook: $dst"
}

if ($SkipCursor) {
    Write-Host 'Skipped Cursor hook (-SkipCursor).'
    exit 0
}

$CursorHooksDir = Join-Path $env:USERPROFILE '.cursor\hooks'
$CursorHookDst = Join-Path $CursorHooksDir 'block-history-rewrite.js'
$CursorHookSrc = Join-Path $PSScriptRoot 'cursor-hook-block-history-rewrite.js'
$HooksJson = Join-Path $env:USERPROFILE '.cursor\hooks.json'

New-Item -ItemType Directory -Force -Path $CursorHooksDir | Out-Null
Copy-Item -LiteralPath $CursorHookSrc -Destination $CursorHookDst -Force
Write-Host "Installed Cursor hook: $CursorHookDst"

$hookPathForJson = ($CursorHookDst -replace '\\', '/')
$entry = @{
    command = "node `"$hookPathForJson`""
    matcher = '\bgit(\.exe)?\s'
    failClosed = $true
}

if (Test-Path $HooksJson) {
    $config = Get-Content -Raw -Encoding UTF8 $HooksJson | ConvertFrom-Json
    if (-not $config.hooks) {
        $config | Add-Member -NotePropertyName hooks -NotePropertyValue ([pscustomobject]@{})
    }
    if (-not $config.hooks.beforeShellExecution) {
        $config.hooks | Add-Member -NotePropertyName beforeShellExecution -NotePropertyValue @()
    }
    $existing = @($config.hooks.beforeShellExecution)
    $filtered = $existing | Where-Object { $_.command -notlike '*block-history-rewrite.js*' }
    $config.hooks.beforeShellExecution = @($filtered) + @($entry)
    $config | ConvertTo-Json -Depth 10 | Set-Content -Encoding UTF8 $HooksJson
    Write-Host "Updated Cursor hooks.json: $HooksJson"
} else {
    $config = @{
        version = 1
        hooks = @{
            beforeShellExecution = @($entry)
        }
    }
    $config | ConvertTo-Json -Depth 10 | Set-Content -Encoding UTF8 $HooksJson
    Write-Host "Created Cursor hooks.json: $HooksJson"
}

Write-Host 'Done. Self-test: node scripts/git/selftest-history-rewrite.js'
