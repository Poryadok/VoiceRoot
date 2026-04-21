# Apply deploy/gateway/ingress.yaml with substitutions (same as Staging deploy workflow).
# Requires: kubectl configured for the target cluster.
#
# Usage:
#   .\scripts\gateway\apply-ingress.ps1 -IngressHost 'voice.tastytest.online' [-Namespace voice-staging] [-TlsSecretName voice-gateway-tls]
#
param(
    [Parameter(Mandatory = $true)]
    [string] $IngressHost,
    [string] $Namespace = 'voice-staging',
    [string] $TlsSecretName = 'voice-gateway-tls'
)
$ErrorActionPreference = 'Stop'
$root = Resolve-Path (Join-Path $PSScriptRoot '..\..')
$manifestPath = Join-Path $root.Path 'deploy\gateway\ingress.yaml'
if (-not (Test-Path -LiteralPath $manifestPath)) { throw "Not found: $manifestPath" }
$raw = Get-Content -LiteralPath $manifestPath -Raw
$raw = $raw.Replace('__K_NAMESPACE__', $Namespace).Replace('__INGRESS_HOST__', $IngressHost).Replace('__TLS_SECRET_NAME__', $TlsSecretName)
$raw | kubectl apply -f -
