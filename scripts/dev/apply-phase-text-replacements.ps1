# Bulk text replacements after phase→feature rename.
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path (Split-Path $PSScriptRoot -Parent) -Parent)

$excludeDirs = @('\.git', '\\gen\\', '\\pb\\', 'node_modules', '\.dart_tool', 'migrations\\')
$extensions = @('*.go', '*.dart', '*.md', '*.yml', '*.yaml', '*.sh', '*.ps1', '*.java', '*.proto', '*.sql', '*.json', 'Makefile', '*.mdc', 'SKILL.md')

$replacements = [ordered]@{
    'TestComposeBotsDailyChatCreateLimit_live' = 'TestComposeBotsDailyChatCreateLimit_live'
    'TestComposeBotsPrivilegedInstall_live' = 'TestComposeBotsPrivilegedInstall_live'
    'TestComposeBotsSlashWhenSearchDown_live' = 'TestComposeBotsSlashWhenSearchDown_live'
    'TestComposeBotsBotCWhenBotDown_live' = 'TestComposeBotsBotCWhenBotDown_live'
    'TestComposeBotsUninstallCleanup_live' = 'TestComposeBotsUninstallCleanup_live'
    'TestComposeBotsPerChatToggle_live' = 'TestComposeBotsPerChatToggle_live'
    'TestComposeBotsOfflineGreyout_live' = 'TestComposeBotsOfflineGreyout_live'
    'TestComposeBotsSlashDeferred_live' = 'TestComposeBotsSlashDeferred_live'
    'TestComposeBotsBotCRoutes_live' = 'TestComposeBotsBotCRoutes_live'
    'TestComposeBotsWebhook_live' = 'TestComposeBotsWebhook_live'
    'TestComposeBotsTimeout_live' = 'TestComposeBotsTimeout_live'
    'TestComposeBotsEphemeral_live' = 'TestComposeBotsEphemeral_live'
    'TestComposeBotsSlash_live' = 'TestComposeBotsSlash_live'
    'TestComposeStoriesExpiryArchive_live' = 'TestComposeStoriesExpiryArchive_live'
    'TestComposeStoriesWhenSocialDown_live' = 'TestComposeStoriesWhenSocialDown_live'
    'TestComposeStories_live' = 'TestComposeStories_live'
    'TestComposeDeepLinksResolveWhenSearchDown_live' = 'TestComposeDeepLinksResolveWhenSearchDown_live'
    'TestComposeDeepLinks_live' = 'TestComposeDeepLinks_live'
    'TestComposeE2E_EnableRejectedWhenPeerMissingPreKey_live' = 'TestComposeE2E_EnableRejectedWhenPeerMissingPreKey_live'
    'TestComposeE2E_GroupEnableRejected_live' = 'TestComposeE2E_GroupEnableRejected_live'
    'TestComposeE2EKeyBackup_OversizedRejected_live' = 'TestComposeE2EKeyBackup_OversizedRejected_live'
    'TestComposeE2EKeyBackup_live' = 'TestComposeE2EKeyBackup_live'
    'TestComposeE2EOptOut_live' = 'TestComposeE2EOptOut_live'
    'TestComposeE2EEdit_live' = 'TestComposeE2EEdit_live'
    'TestComposeE2EDM_live' = 'TestComposeE2EDM_live'
    'TestComposeModeration_live' = 'TestComposeModeration_live'
    'TestComposeProfileFriendIsolation_live' = 'TestComposeProfileFriendIsolation_live'
    'TestComposeSubscriptionWiring_yaml' = 'TestComposeSubscriptionWiring_yaml'
    'TestComposeBilling_live' = 'TestComposeBilling_live'
    'TestComposePrivacyActions_live' = 'TestComposePrivacyActions_live'
    'TestComposePrivacyFoF_live' = 'TestComposePrivacyFoF_live'
    'TestComposePhoneSync_live' = 'TestComposePhoneSync_live'
    'TestComposeTrust_live' = 'TestComposeTrust_live'
    'TestComposeDMRealtime_live' = 'TestComposeDMRealtime_live'
    'TestComposeWiring_yaml' = 'TestComposeWiring_yaml'
    'TestStagingBotsWebhook_live' = 'TestStagingBotsWebhook_live'
    'E2EKeyBackupJdbcIntegrationTest' = 'E2EKeyBackupJdbcIntegrationTest'
    'E2EKeyBackupIntegrationTest' = 'E2EKeyBackupIntegrationTest'
    'ProfilesVerificationIntegrationTest' = 'ProfilesVerificationIntegrationTest'
    'compose-migrate-e2e' = 'compose-migrate-e2e'
    'compose-migrate-all.sh e2e' = 'compose-migrate-all.sh e2e'
    'manual-dm-check.ps1' = 'manual-dm-check.ps1'
    'search_db_verification.sql.snippet' = 'search_db_verification.sql.snippet'
    'transcode_profiles_verification_test.go' = 'transcode_profiles_verification_test.go'
    'transcode_profiles_verification.go' = 'transcode_profiles_verification.go'
    'rest_transcoding_integration_test.go' = 'rest_transcoding_integration_test.go'
    'compose_bots_slash_live_test.go' = 'compose_bots_slash_live_test.go'
    'staging_bots_webhook_live_test.go' = 'staging_bots_webhook_live_test.go'
    'compose_moderation_live_test.go' = 'compose_moderation_live_test.go'
    'compose_deeplinks_live_test.go' = 'compose_deeplinks_live_test.go'
    'compose_phone_sync_live_test.go' = 'compose_phone_sync_live_test.go'
    'deeplink_web_chrome_test.dart' = 'deeplink_web_chrome_test.dart'
    'deeplink_web_test.dart' = 'deeplink_web_test.dart'
    'deeplink_invite_e2e_live_test.dart' = 'deeplink_invite_e2e_live_test.dart'
    'bots_slash_e2e_live_test.dart' = 'bots_slash_e2e_live_test.dart'
    'e2e_key_backup_live_test.dart' = 'e2e_key_backup_live_test.dart'
    'moderation_e2e_live_test.dart' = 'moderation_e2e_live_test.dart'
    'billing_e2e_live_test.dart' = 'billing_e2e_live_test.dart'
    'search_e2e_live_test.dart' = 'search_e2e_live_test.dart'
    'matchmaking_e2e_live_test.dart' = 'matchmaking_e2e_live_test.dart'
    'spaces_creation_e2e_live_test.dart' = 'spaces_creation_e2e_live_test.dart'
    'groups_e2e_live_test.dart' = 'groups_e2e_live_test.dart'
    'voice_call_signaling_e2e_live_test.dart' = 'voice_call_signaling_e2e_live_test.dart'
    'friends_e2e_live_test.dart' = 'friends_e2e_live_test.dart'
    'auth_logout_e2e_live_test.dart' = 'auth_logout_e2e_live_test.dart'
    'dm_two_users_e2e_live_test.dart' = 'dm_two_users_e2e_live_test.dart'
    'apns_e2e_live_test.dart' = 'apns_e2e_live_test.dart'
    'voip_e2e_live_test.dart' = 'voip_e2e_live_test.dart'
    'core gRPC' = 'core gRPC'
    'Full app stack' = 'Full app stack'
    'app stack' = 'app stack'
    'core' = 'core'
    'phases 0–10' = 'full app stack'
    'full app stack' = 'full app stack'
    'bots' = 'bots'
    'app stack6' = 'Bots'
    'phase 6–8' = 'push notifications'
    'push notifications' = 'push notifications'
}

# Path renames from TSV
$mapFile = Join-Path $PSScriptRoot 'phase-rename-map.tsv'
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

function Should-Skip($path) {
    foreach ($ex in $excludeDirs) {
        if ($path -match $ex) { return $true }
    }
    if ($path -match '\\lib\\gen\\') { return $true }
    if ($path -match '\.pb\.go$') { return $true }
    if ($path -match 'project\.pbxproj$') { return $true }
    return $false
}

$files = Get-ChildItem -Recurse -File -Include $extensions | Where-Object { -not (Should-Skip $_.FullName) }
$changed = 0
foreach ($file in $files) {
    $content = [System.IO.File]::ReadAllText($file.FullName)
    $original = $content
    foreach ($kv in $replacements.GetEnumerator()) {
        $content = $content.Replace($kv.Key, $kv.Value)
    }
    if ($content -ne $original) {
        [System.IO.File]::WriteAllText($file.FullName, $content)
        $changed++
    }
}
Write-Host "Updated $changed files"
