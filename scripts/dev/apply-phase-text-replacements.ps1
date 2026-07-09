# Bulk text replacements after phase→feature rename.
# Usage: .\scripts\dev\apply-phase-text-replacements.ps1 [-Paths 'src','scripts/dev/ping-bot','deploy/prod']
param(
    [string[]]$Paths = @('src', 'scripts/dev/ping-bot', 'deploy/prod', 'deploy/observability')
)

$ErrorActionPreference = 'Stop'
$RepoRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
Set-Location $RepoRoot

$extensions = @('*.go', '*.dart', '*.md', '*.yml', '*.yaml', '*.sh', '*.ps1', '*.java', '*.proto', '*.sql', '*.json', 'Makefile', '*.mdc', 'SKILL.md', '*.js')

# Longer keys first (ordered hashtable insertion order matters for .NET Replace chain).
$replacements = [ordered]@{
    'app stack–3' = 'early stack (auth, DM, voice) — docs/PLAN.md'
    'app stack6' = 'bots (docs/features/bots.md)'
    'app stack7' = 'stories (docs/features/stories.md)'
    'app stack8' = 'deep-links/platforms (docs/features/deep-links.md)'
    'app stack5' = 'encryption (docs/features/encryption.md)'
    'app stack4' = 'moderation (docs/features/reports.md)'
    'app stack3' = 'multi-profile/verification (docs/features/multi-profile.md)'
    'app stack2' = 'subscription (docs/features/subscription.md)'
    'app stack1' = 'privacy/trust (docs/features/privacy.md)'
    'app stack0' = 'roles/threads (docs/features/roles.md)'
    'Phase 18' = 'deep-links (docs/features/deep-links.md)'
    'Phase 17' = 'stories (docs/features/stories.md)'
    'Phase 16' = 'bots (docs/features/bots.md)'
    'Phase 15' = 'encryption (docs/features/encryption.md)'
    'Phase 14' = 'moderation (docs/features/reports.md)'
    'Phase 13' = 'verification (docs/features/verification.md)'
    'Phase 12' = 'subscription (docs/features/subscription.md)'
    'Phase 11' = 'privacy (docs/features/privacy.md)'
    'Phase 10' = 'threads/roles (docs/features/roles.md)'
    'Phase 9' = 'search (docs/features/search.md)'
    'Phase 8' = 'platforms (docs/features/platforms.md)'
    'Phase-8' = 'platforms (docs/features/platforms.md)'
    'Phase 7' = 'matchmaking (docs/features/matchmaking.md)'
    'Phase 6' = 'notifications (docs/features/notifications.md)'
    'Phase 5' = 'roles/spaces (docs/features/roles.md)'
    'Phase 4' = 'groups (docs/features/text-chat.md)'
    'Phase 3' = 'file-storage (docs/features/file-storage.md)'
    'Phase 0' = 'platforms (docs/PLAN.md)'
    'PLAN.md phase 14' = 'docs/features/reports.md'
    'phase13EventsRecorder' = 'profilesVerificationEventsRecorder'
}

# Lowercase "phase N" in test names/fixtures (PowerShell hashtables are case-insensitive — separate list).
$lowercasePhaseReplacements = @(
    @('phase 18 deg', 'deep-links deg'),
    @('phase 18', 'deep-links'),
    @('phase 17 ', 'stories '),
    @('phase 14 ', 'moderation '),
    @('phase 13 ', 'verification '),
    @('phase 12 ', 'subscription '),
    @('phase 11 live', 'privacy live'),
    @('phase 11 ', 'privacy '),
    @('phase 5', 'roles/spaces'),
    @('"phase 17"', '"stories live"')
)

# Path renames from TSV (legacy batch — idempotent if already renamed).
$mapFile = Join-Path $PSScriptRoot 'phase-rename-map.tsv'
if (Test-Path $mapFile) {
    Get-Content $mapFile -Encoding UTF8 | ForEach-Object {
        if ($_ -match '^#' -or -not $_) { return }
        $p = $_ -split "`t"
        if ($p.Count -ge 3 -and $p[2] -eq 'RENAME' -and $p[0] -and $p[1]) {
            $replacements[$p[0].Replace('\', '/')] = $p[1].Replace('\', '/')
            $oldBase = [System.IO.Path]::GetFileName($p[0])
            $newBase = [System.IO.Path]::GetFileName($p[1])
            if ($oldBase -ne $newBase) { $replacements[$oldBase] = $newBase }
        }
    }
}

function Should-Skip($path) {
    if ($path -match '\\pb\\') { return $true }
    if ($path -match '\\lib\\gen\\') { return $true }
    if ($path -match '\.pb\.go$') { return $true }
    if ($path -match 'project\.pbxproj$') { return $true }
    if ($path -match '\\migrations\\') { return $true }
    return $false
}

$files = @()
foreach ($rel in $Paths) {
    $abs = Join-Path $RepoRoot $rel
    if (-not (Test-Path $abs)) { continue }
    $files += Get-ChildItem -Path $abs -Recurse -File -Include $extensions -ErrorAction SilentlyContinue |
        Where-Object { -not (Should-Skip $_.FullName) }
}

$changed = 0
foreach ($file in $files) {
    $content = [System.IO.File]::ReadAllText($file.FullName)
    $original = $content
    foreach ($kv in $replacements.GetEnumerator()) {
        $content = $content.Replace($kv.Key, $kv.Value)
    }
    foreach ($pair in $lowercasePhaseReplacements) {
        $content = $content.Replace($pair[0], $pair[1])
    }
    if ($content -ne $original) {
        [System.IO.File]::WriteAllText($file.FullName, $content)
        $changed++
        Write-Host "updated $($file.FullName.Substring($RepoRoot.Length + 1))"
    }
}
Write-Host "Updated $changed files"
