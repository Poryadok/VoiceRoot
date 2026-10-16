import 'dart:convert';
import 'dart:typed_data';

/// Reads only bounded TITLE, ARTIST, and ALBUM Vorbis comments from native FLAC.
/// Audio frames and all bytes passed to the decoder remain untouched.
Map<String, String> nativeFlacMusicTags(Uint8List bytes) {
  const empty = <String, String>{};
  if (bytes.length < 8 ||
      bytes[0] != 0x66 ||
      bytes[1] != 0x4c ||
      bytes[2] != 0x61 ||
      bytes[3] != 0x43) {
    return empty;
  }

  var offset = 4;
  var blockCount = 0;
  var sawStreamInfo = false;
  var sawComments = false;
  Map<String, String>? tags;
  while (offset <= bytes.length - 4 && blockCount < 1024) {
    blockCount++;
    final header = bytes[offset];
    final type = header & 0x7f;
    final isLast = (header & 0x80) != 0;
    final blockLength =
        (bytes[offset + 1] << 16) |
        (bytes[offset + 2] << 8) |
        bytes[offset + 3];
    final dataStart = offset + 4;
    if (type == 0x7f || (sawStreamInfo && type == 0)) return empty;
    if (blockLength > bytes.length - dataStart) return empty;
    final dataEnd = dataStart + blockLength;

    if (!sawStreamInfo) {
      if (type != 0 || blockLength != 34) return empty;
      sawStreamInfo = true;
    } else if (type == 4) {
      // RFC 9639 permits only one native-FLAC Vorbis-comment block.
      if (sawComments) return empty;
      sawComments = true;
      tags = _parseCommentBlock(bytes, dataStart, dataEnd);
      if (tags == null) return empty;
    }

    offset = dataEnd;
    if (isLast) {
      return sawStreamInfo ? (tags ?? empty) : empty;
    }
  }
  return empty;
}

Map<String, String>? _parseCommentBlock(
  Uint8List bytes,
  int start,
  int end,
) {
  var offset = start;
  if (end - offset < 4) return null;
  final vendorLength = _littleEndian32(bytes, offset);
  offset += 4;
  if (vendorLength > end - offset) return null;
  offset += vendorLength;
  if (end - offset < 4) return null;

  final fieldCount = _littleEndian32(bytes, offset);
  offset += 4;
  // Every field needs at least its four-byte length. Bound work even for a
  // malicious block whose advertised count is much larger than its contents.
  if (fieldCount > 4096 || fieldCount > (end - offset) ~/ 4) return null;

  const wanted = {'title', 'artist', 'album'};
  final counts = <String, int>{};
  final values = <String, String>{};
  for (var i = 0; i < fieldCount; i++) {
    if (end - offset < 4) return null;
    final fieldLength = _littleEndian32(bytes, offset);
    offset += 4;
    if (fieldLength > end - offset) return null;
    final fieldEnd = offset + fieldLength;

    var equals = offset;
    while (equals < fieldEnd && bytes[equals] != 0x3d) {
      final c = bytes[equals];
      if (c < 0x20 || c > 0x7e) return null;
      equals++;
    }
    if (equals < fieldEnd && equals > offset && equals - offset <= 16) {
      final key = String.fromCharCodes(
        Uint8List.sublistView(bytes, offset, equals),
      ).toLowerCase();
      if (wanted.contains(key)) {
        counts.update(key, (count) => count + 1, ifAbsent: () => 1);
        final valueStart = equals + 1;
        final valueLength = fieldEnd - valueStart;
        if (valueLength <= 4096) {
          try {
            values[key] = utf8.decode(
              Uint8List.sublistView(bytes, valueStart, fieldEnd),
              allowMalformed: false,
            );
          } on FormatException {
            values.remove(key);
          }
        } else {
          values.remove(key);
        }
      }
    }
    offset = fieldEnd;
  }
  if (offset != end) return null;

  return {
    for (final key in wanted)
      if (counts[key] == 1 && values.containsKey(key)) key: values[key]!,
  };
}

int _littleEndian32(Uint8List bytes, int offset) =>
    bytes[offset] +
    bytes[offset + 1] * 0x100 +
    bytes[offset + 2] * 0x10000 +
    bytes[offset + 3] * 0x1000000;
