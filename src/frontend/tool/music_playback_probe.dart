// Hosted-only real decoder fixture. This is not an application entrypoint.
import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';
import 'package:flutter/widgets.dart';
import 'package:http/http.dart' as http;
import 'package:web/web.dart' as web;
import 'package:voice_frontend/e2e/e2e_file_crypto.dart';
import 'package:voice_frontend/ui/chat/music_format.dart';
import 'package:voice_frontend/ui/chat/music_playback.dart';
import '../test/support/e2e_attachment_test_fixture.dart';

void require(bool condition, String reason) {
  if (!condition) throw _ProbeFailure(reason);
}

class _ProbeFailure implements Exception {
  const _ProbeFailure(this.code);

  final String code;
}

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final results = <Map<String, Object>>[];
  var stage = 'initialization';
  try {
    final e2e = await E2eAttachmentTestFixture.create();
    for (final name in [
      'track.mp3',
      'track.flac',
      'vorbis.ogg',
      'opus.ogg',
      'flac.ogg',
      'pcm.wav',
      'float.wav',
    ]) {
      stage = 'fetch-$name';
      final response = await http
          .get(Uri.base.resolve('probe-fixtures/$name'))
          .timeout(const Duration(seconds: 10));
      require(response.statusCode == 200, 'fixture_http');
      final original = response.bodyBytes;
      require(hasSupportedMusicSignature(original), 'fixture_admission');
      for (final encrypted in [false, true]) {
        stage = '${encrypted ? 'encrypted' : 'ordinary'}-$name';
        Uint8List input;
        if (encrypted) {
          final payload = await e2e.encryptFileForPeer(plaintext: original);
          require(
            !hasSupportedMusicSignature(payload.ciphertext),
            'ciphertext_format',
          );
          input = await const E2eFileCrypto().decryptBytes(
            ciphertext: payload.ciphertext,
            keyWire: payload.keyWire,
            messageService: e2e.messageService,
            localProfileId: e2e.localProfileId,
            peerProfileId: e2e.peerProfileId,
          );
          require(
            base64Encode(input) == base64Encode(original),
            'decrypt_identity',
          );
          payload.ciphertext.fillRange(0, payload.ciphertext.length, 0);
        } else {
          input = Uint8List.fromList(original);
        }
        final player = PullMusicPlayback();
        try {
          await player.open(input).timeout(const Duration(seconds: 20));
          require(!player.failed && player.playing, 'decoder_open');
          require(
            player.duration.inMilliseconds >= 1800 &&
                player.duration.inMilliseconds <= 2500,
            'decoder_duration',
          );
          player.pause();
          require(!player.playing, 'decoder_pause');
          player.seek(const Duration(milliseconds: 500));
          player.resume();
          await Future<void>.delayed(const Duration(milliseconds: 150));
          require(!player.failed && player.playing, 'decoder_seek_resume');
          if (!name.endsWith('.wav')) {
            require(
              player.title == 'Fixture title' &&
                  player.artist == 'Fixture artist' &&
                  player.album == 'Fixture album',
              'decoder_tags',
            );
          }
          await player.close();
          require(
            player.closed && !player.playing && !player.failed,
            'decoder_dispose',
          );
          results.add({
            'fixture': name,
            'encrypted': encrypted,
            'result': 'pass',
          });
        } finally {
          input.fillRange(0, input.length, 0);
          player.dispose();
        }
      }
      original.fillRange(0, original.length, 0);
    }
    stage = 'unsupported';
    final unsupported = await http.get(
      Uri.base.resolve('probe-fixtures/unsupported.aac'),
    );
    require(
      unsupported.statusCode == 200 &&
          !hasSupportedMusicSignature(unsupported.bodyBytes),
      'unsupported_admission',
    );
    final player = PullMusicPlayback();
    var refused = false;
    try {
      await player.open(unsupported.bodyBytes);
    } on FormatException {
      refused = true;
    } finally {
      player.dispose();
    }
    require(refused, 'unsupported_decoder');
    web.document.documentElement!.setAttribute(
      'data-music-result',
      jsonEncode({'result': 'pass', 'cases': results}),
    );
  } catch (error) {
    // Keep diagnostics to probe-owned codes and exception categories. Never
    // serialize decrypted metadata, keys, arbitrary exceptions or URLs.
    web.document.documentElement!.setAttribute(
      'data-music-result',
      jsonEncode({
        'result': 'fail',
        'stage': stage,
        'completed': results.length,
        'reason': switch (error) {
          _ProbeFailure(:final code) => code,
          TimeoutException() => 'timeout',
          FormatException() => 'format_exception',
          _ => 'unexpected_exception',
        },
      }),
    );
  }
}
