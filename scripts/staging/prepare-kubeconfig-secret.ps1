# Build base64 for GitHub secret STAGING_KUBECONFIG (Environment: staging).
# Replaces cluster server URL so a GitHub-hosted runner can reach k3s (not 127.0.0.1 / 0.0.0.0).
#
# Usage:
#   $env:STAGING_KUBE_API_SERVER = 'https://95.31.10.177:6443'
#   .\scripts\staging\prepare-kubeconfig-secret.ps1 $env:USERPROFILE\.kube\staging-config
#
param(
    [Parameter(Mandatory = $true)]
    [string] $KubeconfigPath,
    [string] $StagingKubeApiServer = $(if ($env:STAGING_KUBE_API_SERVER) { $env:STAGING_KUBE_API_SERVER } else { 'https://95.31.10.177:6443' })
)
$ErrorActionPreference = 'Stop'
$lines = Get-Content -LiteralPath $KubeconfigPath
$out = foreach ($line in $lines) {
    if ($line -match '^\s*server:\s*https://') {
        $line -replace '(^\s*server:\s*)https://\S+', "`$1$StagingKubeApiServer"
    } else {
        $line
    }
}
$utf8NoBom = New-Object System.Text.UTF8Encoding $false
$bytes = $utf8NoBom.GetBytes(($out -join "`n") + "`n")
[Convert]::ToBase64String($bytes)
