param(
  [string]$CodexHome = $(if ($env:CODEX_HOME) { $env:CODEX_HOME } else { Join-Path $HOME ".codex" })
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..\..")
$sourceSkills = Join-Path $repoRoot ".agent\codex\skills"
$targetSkills = Join-Path $CodexHome "skills"

if (!(Test-Path $sourceSkills)) {
  throw "Project skills not found: $sourceSkills"
}

New-Item -ItemType Directory -Force $targetSkills | Out-Null

Get-ChildItem -Directory $sourceSkills | ForEach-Object {
  $target = Join-Path $targetSkills $_.Name
  if (Test-Path $target) {
    Remove-Item -Recurse -Force $target
  }
  Copy-Item -Recurse -Force $_.FullName $target
  Write-Host "Installed skill: $($_.Name)"
}

Write-Host ""
Write-Host "Skills installed to $targetSkills"
Write-Host "MCP examples: .agent\codex\mcp.example.toml"
