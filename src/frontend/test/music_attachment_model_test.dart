import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/messages_client.dart';

void main() {
  test('Music metadata survives history and refreshed attachment decoding', () {
    final attachment = MessageAttachment.fromJson({
      'file_id': 'file', 'type': 'music', 'title': 'Title',
      'artist': 'Artist', 'album': 'Album', 'duration_seconds': 12,
    });
    expect(attachment.isMusic, isTrue);
    expect(attachment.musicTitle, 'Title');
    expect(attachment.musicArtist, 'Artist');
    expect(attachment.musicAlbum, 'Album');
    expect(attachment.durationSeconds, 12);
    expect(attachment.toJson()['title'], 'Title');
    final cleared = MessageAttachment.fromJson({'file_id': 'file', 'type': 'music'});
    expect(cleared.musicTitle, isNull);
    expect(cleared.durationSeconds, isNull);
  });

  test('Encrypted Music never serializes locally decoded tags in public JSON', () {
    const attachment = MessageAttachment(
      fileId: 'file', type: 'music', e2eKeyWire: 'wrapped-secret',
      musicTitle: 'Private title', musicArtist: 'Private artist',
      musicAlbum: 'Private album', durationSeconds: 12,
    );
    final wire = attachment.toJson();
    for (final key in ['title', 'artist', 'album', 'duration_seconds']) {
      expect(wire.containsKey(key), isFalse);
    }
    expect(wire['e2e_key_wire'], 'wrapped-secret');
  });
}
