import 'dart:typed_data';

const musicMaxStoredBytes = 10 * 1024 * 1024;

/// Shared Music admission formats: MP3, native FLAC, Ogg audio and RIFF WAVE.
/// This is bounded signature admission, not a replacement for decoder validation.
/// In particular an ID3 prefix alone must not route AAC into a native fallback.
bool hasSupportedMusicSignature(Uint8List bytes) {
  if (bytes.isEmpty || bytes.length > musicMaxStoredBytes) return false;
  bool at(int offset, List<int> signature) =>
      offset >= 0 &&
      offset + signature.length <= bytes.length &&
      List.generate(
        signature.length,
        (i) => bytes[offset + i] == signature[i],
      ).every((same) => same);
  if (at(0, const [0x66, 0x4c, 0x61, 0x43])) return true; // fLaC
  if (at(0, const [0x52, 0x49, 0x46, 0x46]) &&
      at(8, const [0x57, 0x41, 0x56, 0x45])) {
    final view = ByteData.sublistView(bytes);
    var offset = 12;
    // WAVE can contain compressed codecs too. Admit only the same PCM/float
    // forms as File's probe, with a bounded header scan before decoding.
    while (offset + 8 <= bytes.length && offset < 64 * 1024) {
      final length = view.getUint32(offset + 4, Endian.little);
      final body = offset + 8;
      if (length > bytes.length - body) return false;
      if (at(offset, const [0x66, 0x6d, 0x74, 0x20])) {
        if (length < 16) return false;
        var codec = view.getUint16(body, Endian.little);
        final bits = view.getUint16(body + 14, Endian.little);
        if (codec == 0xfffe) {
          if (length < 40 ||
              view.getUint16(body + 16, Endian.little) < 22 ||
              !at(body + 26, const [
                0,
                0,
                0,
                0,
                0x10,
                0,
                0x80,
                0,
                0,
                0xaa,
                0,
                0x38,
                0x9b,
                0x71,
              ])) {
            return false;
          }
          codec = view.getUint16(body + 24, Endian.little);
        }
        return (codec == 1 && const [8, 16, 24, 32].contains(bits)) ||
            (codec == 3 && const [32, 64].contains(bits));
      }
      offset = body + length + (length & 1);
    }
    return false;
  }
  if (at(0, const [0x4f, 0x67, 0x67, 0x53]) && bytes.length >= 28) {
    if (bytes[4] != 0 || bytes[26] == 0) return false;
    final payload = 27 + bytes[26];
    return at(payload, const [
          0x4f,
          0x70,
          0x75,
          0x73,
          0x48,
          0x65,
          0x61,
          0x64,
        ]) ||
        at(payload, const [1, 0x76, 0x6f, 0x72, 0x62, 0x69, 0x73]) ||
        at(payload, const [0x7f, 0x46, 0x4c, 0x41, 0x43]);
  }
  var offset = 0;
  if (at(0, const [0x49, 0x44, 0x33])) {
    if (bytes.length < 10 || bytes[3] < 2 || bytes[3] > 4) return false;
    var tagSize = 0;
    for (var i = 6; i < 10; i++) {
      if (bytes[i] >= 128) return false;
      tagSize = (tagSize << 7) | bytes[i];
    }
    offset = 10 + tagSize;
    if (bytes[3] == 4 && (bytes[5] & 0x10) != 0) offset += 10;
  }
  if (offset + 4 > bytes.length) return false;
  final a = bytes[offset + 1];
  final b = bytes[offset + 2];
  return bytes[offset] == 0xff &&
      (a & 0xe0) == 0xe0 &&
      (a & 0x18) != 0x08 && // reserved MPEG version
      (a & 0x06) == 0x02 && // Layer III only (excludes ADTS AAC)
      (b >> 4) != 0 &&
      (b >> 4) != 15 &&
      (b & 0x0c) != 0x0c;
}
