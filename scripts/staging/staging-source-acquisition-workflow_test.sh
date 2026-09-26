#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORKFLOW="$ROOT/.github/workflows/staging-deploy.yml"
TEST_ROOT="$(mktemp -d)"
trap 'rm -rf -- "$TEST_ROOT"' EXIT
SHA=0123456789012345678901234567890123456789
ARCHIVE_ROOT="VoiceRoot-${SHA}"
WORKSPACE_PARENT="$TEST_ROOT/runner-workspace"
WORKSPACE="$WORKSPACE_PARENT/VoiceRoot"
RUNNER_TEMP_DIR="$TEST_ROOT/runner-temp"
FIXTURE_DIR="$TEST_ROOT/fixture"
mkdir -p "$WORKSPACE" "$RUNNER_TEMP_DIR" "$TEST_ROOT/home" "$FIXTURE_DIR/base/bin"
supports_posix_filemode=true
case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*) supports_posix_filemode=false ;;
esac

DOWNLOAD_SCRIPT="$TEST_ROOT/download-source.sh"
awk '
  /^      - name: Download exact staging source archive$/ { step = 1; next }
  step && /^      - / { exit }
  step && /run: &download_staging_source \|/ { run = 1; next }
  run { sub(/^          /, ""); print }
' "$WORKFLOW" >"$DOWNLOAD_SCRIPT"
[ -s "$DOWNLOAD_SCRIPT" ] || { echo 'FAIL: source bootstrap missing from workflow' >&2; exit 1; }
bash -n "$DOWNLOAD_SCRIPT"

printf '*.ignored\n' >"$FIXTURE_DIR/base/.gitignore"
printf 'config.ignored filter=sentinel\n' >"$FIXTURE_DIR/base/.gitattributes"
printf 'tracked despite ignore\n' >"$FIXTURE_DIR/base/config.ignored"
printf '#!/bin/sh\nprintf ok\\n\n' >"$FIXTURE_DIR/base/bin/build.sh"
chmod 755 "$FIXTURE_DIR/base/bin/build.sh"
git -C "$FIXTURE_DIR/base" init -q
git -C "$FIXTURE_DIR/base" -c core.autocrlf=false -c core.filemode="$supports_posix_filemode" add --force --all
if [[ "$supports_posix_filemode" == true ]]; then
  git -C "$FIXTURE_DIR/base" update-index --chmod=+x -- bin/build.sh
fi
TREE_SHA="$(git -C "$FIXTURE_DIR/base" -c core.autocrlf=false write-tree)"
rm -rf -- "$FIXTURE_DIR/base/.git"
printf '{"sha":"%s","commit":{"tree":{"sha":"%s"}}}\n' "$SHA" "$TREE_SHA" >"$FIXTURE_DIR/commit.json"

write_archive() {
  local source_dir="$1" destination="$2" package="$TEST_ROOT/package"
  rm -rf -- "$package"
  mkdir -p "$package/$ARCHIVE_ROOT"
  cp -a -- "$source_dir/." "$package/$ARCHIVE_ROOT/"
  tar -czf "$destination" -C "$package" "$ARCHIVE_ROOT"
}
write_archive "$FIXTURE_DIR/base" "$FIXTURE_DIR/base.tar.gz"
base_mode=755
if [[ "$supports_posix_filemode" != true ]]; then base_mode=644; fi
python3 - "$FIXTURE_DIR/base.tar.gz" "$FIXTURE_DIR/executable.tar.gz" "$ARCHIVE_ROOT/bin/build.sh" "$base_mode" <<'PY'
import sys
import tarfile

with tarfile.open(sys.argv[1], "r:gz") as source, tarfile.open(sys.argv[2], "w:gz") as destination:
    for member in source.getmembers():
        stream = source.extractfile(member) if member.isfile() else None
        if member.name == sys.argv[3]:
            member.mode = int(sys.argv[4], 8)
        destination.addfile(member, stream)
PY
mv "$FIXTURE_DIR/executable.tar.gz" "$FIXTURE_DIR/base.tar.gz"

cat >"$TEST_ROOT/curl" <<'CURL_STUB'
#!/usr/bin/env bash
set -euo pipefail
output=''
url=''
while (($#)); do
  case "$1" in
    --output) output="$2"; shift 2 ;;
    https://*) url="$1"; shift ;;
    *) shift ;;
  esac
done
[[ -n "$output" && -n "$url" ]]
case "$url" in
  */commits/*) cp -- "$FIXTURE_COMMIT" "$output" ;;
  *codeload.github.com/Poryadok/VoiceRoot/tar.gz/*) cp -- "$FIXTURE_ARCHIVE" "$output" ;;
  *) exit 22 ;;
esac
CURL_STUB
chmod +x "$TEST_ROOT/curl"
git config --file "$TEST_ROOT/gitconfig" filter.sentinel.clean 'touch "$SENTINEL"; cat'

run_case() {
  local name="$1" expected="$2" archive="$3"
  rm -rf -- "$WORKSPACE"
  mkdir -p "$WORKSPACE"
  printf 'stale\n' >"$WORKSPACE/stale-marker"
  local output rc
  set +e
  output="$(PATH="$TEST_ROOT:$PATH" GIT_CONFIG_GLOBAL="$TEST_ROOT/gitconfig" \
    SENTINEL="$TEST_ROOT/filter-fired" FIXTURE_COMMIT="$FIXTURE_DIR/commit.json" \
    FIXTURE_ARCHIVE="$archive" STAGING_SOURCE_SHA="$SHA" GITHUB_WORKSPACE="$WORKSPACE" \
    RUNNER_WORKSPACE="$WORKSPACE_PARENT" RUNNER_TEMP="$RUNNER_TEMP_DIR" HOME="$TEST_ROOT/home" \
    bash "$DOWNLOAD_SCRIPT" 2>&1)"
  rc=$?
  set -e
  if [[ "$expected" == PASS ]]; then
    [[ "$rc" == 0 && "$output" == *'STAGING_SOURCE_ARCHIVE=PASS'* ]] || {
      printf 'FAIL: %s should pass, rc=%s output=%s\n' "$name" "$rc" "$output" >&2
      return 1
    }
  else
    [[ "$rc" != 0 && "$output" == *"STAGING_SOURCE_ARCHIVE=$expected"* ]] || {
      printf 'FAIL: %s should fail as %s, rc=%s output=%s\n' "$name" "$expected" "$rc" "$output" >&2
      return 1
    }
    [[ -f "$WORKSPACE/stale-marker" ]] || { echo "FAIL: $name cleaned workspace before verification" >&2; return 1; }
  fi
}

run_case valid-content-and-executable-mode PASS "$FIXTURE_DIR/base.tar.gz"
[[ ! -e "$TEST_ROOT/filter-fired" ]] || { echo 'FAIL: source clean filter was executed' >&2; exit 1; }
[[ -f "$WORKSPACE/config.ignored" ]] || { echo 'FAIL: tracked ignored file was not copied' >&2; exit 1; }
if [[ "$supports_posix_filemode" == true ]]; then
  [[ -x "$WORKSPACE/bin/build.sh" ]] || { echo 'FAIL: executable mode was not preserved' >&2; exit 1; }
else
  echo 'SKIP: executable-mode fixtures require a POSIX runner'
fi
[[ ! -e "$WORKSPACE/stale-marker" ]] || { echo 'FAIL: stale workspace content was not removed' >&2; exit 1; }

mkdir -p "$FIXTURE_DIR/content/bin" "$FIXTURE_DIR/mode/bin" "$FIXTURE_DIR/symlink/bin"
cp -a "$FIXTURE_DIR/base/." "$FIXTURE_DIR/content/"
cp -a "$FIXTURE_DIR/base/." "$FIXTURE_DIR/mode/"
cp -a "$FIXTURE_DIR/base/." "$FIXTURE_DIR/symlink/"
printf 'changed\n' >"$FIXTURE_DIR/content/config.ignored"
write_archive "$FIXTURE_DIR/content" "$FIXTURE_DIR/content.tar.gz"
run_case content-mismatch FAIL_TREE_MISMATCH "$FIXTURE_DIR/content.tar.gz"
python3 - "$FIXTURE_DIR/base.tar.gz" "$FIXTURE_DIR/mode.tar.gz" "$ARCHIVE_ROOT/bin/build.sh" <<'PY'
import sys
import tarfile

with tarfile.open(sys.argv[1], "r:gz") as source, tarfile.open(sys.argv[2], "w:gz") as destination:
    for member in source.getmembers():
        stream = source.extractfile(member) if member.isfile() else None
        if member.name == sys.argv[3]:
            member.mode = 0o644
        destination.addfile(member, stream)
PY
if [[ "$supports_posix_filemode" == true ]]; then
  run_case executable-mode-mismatch FAIL_TREE_MISMATCH "$FIXTURE_DIR/mode.tar.gz"
else
  echo 'SKIP: executable-mode mismatch fixture requires a POSIX runner'
fi
python3 - "$FIXTURE_DIR/symlink.tar.gz" "$ARCHIVE_ROOT" <<'PY'
import sys
import tarfile

with tarfile.open(sys.argv[1], "w:gz") as archive:
    info = tarfile.TarInfo(f"{sys.argv[2]}/bin/build.sh")
    info.type = tarfile.SYMTYPE
    info.linkname = "../../outside"
    archive.addfile(info)
PY
run_case symlink-rejected FAIL_ARCHIVE_TYPE "$FIXTURE_DIR/symlink.tar.gz"

python3 - "$FIXTURE_DIR/traversal.tar.gz" "$ARCHIVE_ROOT" <<'PY'
import io
import sys
import tarfile

with tarfile.open(sys.argv[1], "w:gz") as archive:
    info = tarfile.TarInfo(f"{sys.argv[2]}/../escaped")
    content = b"must not escape"
    info.size = len(content)
    archive.addfile(info, io.BytesIO(content))
PY
run_case traversal-rejected FAIL_ARCHIVE_PATH "$FIXTURE_DIR/traversal.tar.gz"
[[ ! -e "$WORKSPACE_PARENT/escaped" ]] || { echo 'FAIL: traversal fixture escaped workspace' >&2; exit 1; }

echo 'staging source archive workflow fixtures passed'
