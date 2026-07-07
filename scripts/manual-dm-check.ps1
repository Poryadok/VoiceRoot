# core manual checklist API smoke (gateway at 18080). Complements browser checklist.
$ErrorActionPreference = 'Stop'
$base = 'http://127.0.0.1:18080'
$tmp = Join-Path $env:TEMP 'voice-manual-check.json'

function Invoke-CurlJson($method, $path, $obj, $token) {
  [IO.File]::WriteAllText($tmp, ($obj | ConvertTo-Json -Compress))
  $args = @('-s', '-w', '%{http_code}', '-X', $method, "$base$path", '-H', 'Content-Type: application/json', '--data-binary', "@$tmp")
  if ($token) { $args += @('-H', "Authorization: Bearer $token") }
  $out = & curl.exe @args
  $code = [int]$out.Substring($out.Length - 3)
  $json = $out.Substring(0, $out.Length - 3)
  return @{ Code = $code; Body = $json }
}

function Invoke-CurlGet($path, $token) {
  $out = & curl.exe -s -w '%{http_code}' -H "Authorization: Bearer $token" "$base$path"
  $code = [int]$out.Substring($out.Length - 3)
  $json = $out.Substring(0, $out.Length - 3)
  return @{ Code = $code; Body = $json }
}

$device = '{"platform":"flutter"}'
$bodyBase = @{
  guest            = $false
  device_info_json = $device
}

# 1. Short password -> 400
$r1 = Invoke-CurlJson POST '/api/v1/auth/register' ($bodyBase + @{
  email    = 'shortpw@test.local'
  password = 'abc'
})
Write-Host "1.validation_short_pw: $($r1.Code)"
if ($r1.Code -ne 400) { throw "expected 400 for short password, got $($r1.Code)" }

$emailA = "manual-a-$(Get-Random)@voice-qa.test"
$emailB = "manual-b-$(Get-Random)@voice-qa.test"
$pw = 'VoiceQaTest1!'

$regA = Invoke-CurlJson POST '/api/v1/auth/register' ($bodyBase + @{ email = $emailA; password = $pw })
$regB = Invoke-CurlJson POST '/api/v1/auth/register' ($bodyBase + @{ email = $emailB; password = $pw })
if ($regA.Code -ne 200) { throw "register A: $($regA.Code) $($regA.Body)" }
if ($regB.Code -ne 200) { throw "register B: $($regB.Code) $($regB.Body)" }

$ja = $regA.Body | ConvertFrom-Json
$jb = $regB.Body | ConvertFrom-Json
$tokenA = $ja.access_token

$q = ($emailB -split '@')[0]
Write-Host "2.users_search: $((Invoke-CurlGet "/api/v1/users/search?q=$q" $tokenA).Code)"
Write-Host "3.friends: $((Invoke-CurlGet '/api/v1/friends' $tokenA).Code)"
Write-Host "4.chats: $((Invoke-CurlGet '/api/v1/chats' $tokenA).Code)"
Write-Host "5.dm_create: $((Invoke-CurlJson POST '/api/v1/chats/dm' @{ other_profile_id = $jb.profile_id } $tokenA).Code)"

for ($i = 1; $i -le 6; $i++) {
  $login = Invoke-CurlJson POST '/api/v1/auth/login' ($bodyBase + @{ email = $emailB; password = $pw })
  Write-Host "6.login_$i`: $($login.Code)"
  if ($login.Code -eq 429) { throw 'rate limited on login' }
  if ($login.Code -ne 200) { throw "login failed: $($login.Code)" }
}

$p = Invoke-CurlGet "/api/v1/users/profiles/$($ja.profile_id)" $tokenA
Write-Host "7.profile: $($p.Code)"
$jp = $p.Body | ConvertFrom-Json
if ($jp.username) {
  Write-Host "   @$(jp.username)#$(jp.discriminator)"
}

Remove-Item -Force $tmp -ErrorAction SilentlyContinue
Write-Host 'OK: API smoke for manual checklist passed'
