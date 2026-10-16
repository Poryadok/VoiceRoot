#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
command -v ffmpeg >/dev/null || { echo 'Music fixture setup requires ffmpeg' >&2; exit 1; }
cd "$ROOT/src/frontend"
flutter build web --release --no-pub --no-web-resources-cdn --target tool/music_playback_probe.dart
mkdir -p build/web/probe-fixtures
common=(-hide_banner -loglevel error -f lavfi -i 'sine=frequency=440:sample_rate=44100:duration=2' -ac 2 -metadata 'title=Fixture title' -metadata 'artist=Fixture artist' -metadata 'album=Fixture album')
ffmpeg "${common[@]}" -c:a libmp3lame build/web/probe-fixtures/track.mp3
ffmpeg "${common[@]}" -c:a flac build/web/probe-fixtures/track.flac
ffmpeg "${common[@]}" -c:a libvorbis build/web/probe-fixtures/vorbis.ogg
ffmpeg "${common[@]}" -c:a libopus build/web/probe-fixtures/opus.ogg
ffmpeg "${common[@]}" -c:a flac -strict -2 build/web/probe-fixtures/flac.ogg
ffmpeg "${common[@]}" -c:a pcm_s16le build/web/probe-fixtures/pcm.wav
ffmpeg "${common[@]}" -c:a pcm_f32le build/web/probe-fixtures/float.wav
ffmpeg "${common[@]}" -c:a aac build/web/probe-fixtures/unsupported.aac
node "$ROOT/scripts/ci/music-playback-web.mjs" "$ROOT/src/frontend/build/web"
