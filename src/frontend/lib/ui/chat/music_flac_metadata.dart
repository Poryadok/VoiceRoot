import 'dart:convert';
import 'dart:typed_data';

import 'music_format.dart';

/// Returns the first usable, bounded Music tag from decoder and container
/// metadata candidates, preserving their supplied precedence.
String? firstUsableMusicTag(Iterable<String?> candidates) {
  for (final value in candidates) {
    if (value == null ||
        value.length > 1024 ||
        RegExp(r'[\x00-\x1f\x7f]').hasMatch(value)) {
      continue;
    }
    final clean = value.trim();
    if (clean.isNotEmpty) return clean;
  }
  return null;
}

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

/// Reads the terminal Ogg-FLAC granule when STREAMINFO has no total sample
/// count. Ogg-FLAC granules count interchannel samples; malformed, truncated,
/// chained, and multiplexed streams return null.
Duration? unknownTotalOggFlacDuration(Uint8List bytes) {
  if (bytes.length < 79 || bytes.length > musicMaxStoredBytes) {
    return null;
  }

  var offset = 0;
  var expectedSequence = 0;
  var streamSerial = -1;
  var packetContinues = false;
  var headerPacketsRemaining = 0;
  int? sampleRate;
  int? terminalGranule;
  while (offset < bytes.length) {
    if (bytes.length - offset < 27 ||
        bytes[offset] != 0x4f ||
        bytes[offset + 1] != 0x67 ||
        bytes[offset + 2] != 0x67 ||
        bytes[offset + 3] != 0x53 ||
        bytes[offset + 4] != 0) {
      return null;
    }

    final flags = bytes[offset + 5];
    if ((flags & ~0x07) != 0) return null;
    final isBeginning = (flags & 0x02) != 0;
    final isEnd = (flags & 0x04) != 0;
    final isContinued = (flags & 0x01) != 0;
    if (isContinued != packetContinues) return null;
    final serial = _oggLittleEndian32(bytes, offset + 14);
    final sequence = _oggLittleEndian32(bytes, offset + 18);
    final segmentCount = bytes[offset + 26];
    final segmentStart = offset + 27;
    if (segmentStart > bytes.length - segmentCount) return null;
    final bodyStart = segmentStart + segmentCount;
    var bodyLength = 0;
    var hasPacketEnd = false;
    var completedPackets = 0;
    for (var i = segmentStart; i < bodyStart; i++) {
      if (bytes[i] < 255) {
        hasPacketEnd = true;
        completedPackets++;
      }
      bodyLength += bytes[i];
    }
    if (bodyLength > bytes.length - bodyStart) return null;
    final pageEnd = bodyStart + bodyLength;

    if (expectedSequence == 0) {
      // RFC 9639 requires the Ogg-FLAC mapping packet to occupy the first
      // page by itself (a 51-byte packet in a 79-byte page).
      if (!isBeginning ||
          isEnd ||
          segmentCount != 1 ||
          bytes[segmentStart] != 51 ||
          bodyLength != 51 ||
          (headerPacketsRemaining = _oggFlacHeaderPacketCount(
                bytes,
                bodyStart,
              )) <
              0) {
        return null;
      }
      final streamInfo = bodyStart + 17;
      final packedOffset = streamInfo + 10;
      sampleRate =
          (bytes[packedOffset] << 12) |
          (bytes[packedOffset + 1] << 4) |
          (bytes[packedOffset + 2] >> 4);
      final totalSamples =
          ((bytes[packedOffset + 3] & 0x0f) << 32) |
          (bytes[packedOffset + 4] << 24) |
          (bytes[packedOffset + 5] << 16) |
          (bytes[packedOffset + 6] << 8) |
          bytes[packedOffset + 7];
      if (sampleRate <= 0 || sampleRate > 655350 || totalSamples != 0) {
        return null;
      }
      streamSerial = serial;
    } else if (isBeginning || serial != streamSerial) {
      // A second BOS or serial makes a chained/multiplexed duration ambiguous.
      return null;
    }
    final granule = _oggGranule(bytes, offset + 6);
    if (granule == null) return null;
    final isMappingPage = expectedSequence == 0;
    final isHeaderPage = !isMappingPage && headerPacketsRemaining > 0;
    if (isHeaderPage) {
      // The mapping's count excludes its own packet. Header packets may span
      // pages, but the page completing the final one cannot also start audio.
      if (granule != 0 || completedPackets > headerPacketsRemaining) {
        return null;
      }
      if (completedPackets == headerPacketsRemaining && completedPackets > 0) {
        var completions = 0;
        var lastCompletion = bodyStart;
        for (var i = segmentStart; i < bodyStart; i++) {
          if (bytes[i] < 255) {
            completions++;
            if (completions == completedPackets) lastCompletion = i;
          }
        }
        if (lastCompletion != bodyStart - 1) return null;
      }
      headerPacketsRemaining -= completedPackets;
    } else if ((granule == -1 && hasPacketEnd) ||
        (!hasPacketEnd && granule != -1)) {
      return null;
    }
    if (isMappingPage && granule != 0) return null;
    if (sequence != expectedSequence) return null;

    final expectedCrc = _oggLittleEndian32(bytes, offset + 22);
    if (_oggPageCrc(bytes, offset, pageEnd) != expectedCrc) return null;

    if (isEnd) {
      if (headerPacketsRemaining != 0 ||
          granule <= 0 ||
          pageEnd != bytes.length) {
        return null;
      }
      terminalGranule = granule;
    } else if (terminalGranule != null) {
      return null;
    }

    if (segmentCount > 0) packetContinues = bytes[bodyStart - 1] == 255;
    if (isEnd && packetContinues) return null;

    offset = pageEnd;
    expectedSequence++;
  }

  final samples = terminalGranule;
  final rate = sampleRate;
  if (samples == null || rate == null || samples > rate * 2147483647) {
    return null;
  }
  final milliseconds = (samples * 1000 / rate).ceil();
  if (milliseconds <= 0 || milliseconds > 2147483647000) return null;
  return Duration(milliseconds: milliseconds);
}

int _oggFlacHeaderPacketCount(Uint8List bytes, int offset) {
  const prefix = [0x7f, 0x46, 0x4c, 0x41, 0x43, 0x01, 0x00];
  for (var i = 0; i < prefix.length; i++) {
    if (bytes[offset + i] != prefix[i]) return -1;
  }
  final headerPacketCount = (bytes[offset + 7] << 8) | bytes[offset + 8];
  // Bound parser state and reject layouts too unusual for duration fallback.
  // RFC 9639 uses zero to mean the number of following headers is unknown;
  // without that boundary this fallback cannot distinguish headers from audio.
  if (headerPacketCount == 0 || headerPacketCount > 1024) return -1;
  // The FLAC marker and the mandatory 34-byte STREAMINFO block follow.
  final hasStreamInfo =
      bytes[offset + 9] == 0x66 &&
      bytes[offset + 10] == 0x4c &&
      bytes[offset + 11] == 0x61 &&
      bytes[offset + 12] == 0x43 &&
      (bytes[offset + 13] & 0x7f) == 0 &&
      bytes[offset + 14] == 0 &&
      bytes[offset + 15] == 0 &&
      bytes[offset + 16] == 34;
  return hasStreamInfo ? headerPacketCount : -1;
}

int _oggLittleEndian32(Uint8List bytes, int offset) =>
    bytes[offset] +
    bytes[offset + 1] * 0x100 +
    bytes[offset + 2] * 0x10000 +
    bytes[offset + 3] * 0x1000000;

int? _oggGranule(Uint8List bytes, int offset) {
  final low = _oggLittleEndian32(bytes, offset);
  final high = _oggLittleEndian32(bytes, offset + 4);
  if (high == 0xffffffff && low == 0xffffffff) return -1;
  // Keep conversion exact in Flutter web, where integers use IEEE-754 doubles.
  // Ogg granules are signed; reject other negative values and values beyond
  // the largest exactly representable integer before combining the words.
  if (high >= 0x80000000 || high > 0x1fffff) return null;
  return high * 0x100000000 + low;
}

int _oggPageCrc(Uint8List bytes, int start, int end) {
  var crc = 0;
  for (var offset = start; offset < end; offset++) {
    final byte = offset >= start + 22 && offset < start + 26
        ? 0
        : bytes[offset];
    crc =
        (((crc << 8) & 0xffffffff) ^
            _oggCrcTable[((crc >> 24) ^ byte) & 0xff]) &
        0xffffffff;
  }
  return crc;
}

final List<int> _oggCrcTable = List<int>.generate(256, (index) {
  var crc = index << 24;
  for (var bit = 0; bit < 8; bit++) {
    crc = (crc & 0x80000000) != 0
        ? ((crc << 1) ^ 0x04c11db7) & 0xffffffff
        : (crc << 1) & 0xffffffff;
  }
  return crc;
}, growable: false);

Map<String, String>? _parseCommentBlock(Uint8List bytes, int start, int end) {
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
