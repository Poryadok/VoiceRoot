import 'dart:math' as math;
import 'dart:typed_data';
import 'dart:ui' as ui;

/// Max inline preview size for E2E images in chat (matches [_AttachmentPreview]).
const int kE2eImageThumbMaxWidth = 220;
const int kE2eImageThumbMaxHeight = 160;

/// Downscales [imageBytes] for inline chat preview after E2E decrypt.
///
/// Returns PNG bytes at most [maxWidth]×[maxHeight], preserving aspect ratio.
/// Returns null when the input is not a decodable image.
Future<Uint8List?> resizeImageBytesForThumb(
  Uint8List imageBytes, {
  int maxWidth = kE2eImageThumbMaxWidth,
  int maxHeight = kE2eImageThumbMaxHeight,
}) async {
  if (imageBytes.isEmpty) return null;
  try {
    final codec = await ui.instantiateImageCodec(imageBytes);
    final frame = await codec.getNextFrame();
    final image = frame.image;
    final width = image.width;
    final height = image.height;
    if (width <= 0 || height <= 0) {
      image.dispose();
      return null;
    }

    final scale = math.min(maxWidth / width, maxHeight / height);
    if (scale >= 1.0) {
      final png = await image.toByteData(format: ui.ImageByteFormat.png);
      image.dispose();
      return png?.buffer.asUint8List();
    }

    final targetWidth = math.max(1, (width * scale).round());
    final targetHeight = math.max(1, (height * scale).round());
    image.dispose();

    final thumbCodec = await ui.instantiateImageCodec(
      imageBytes,
      targetWidth: targetWidth,
      targetHeight: targetHeight,
    );
    final thumbFrame = await thumbCodec.getNextFrame();
    final thumbImage = thumbFrame.image;
    final png = await thumbImage.toByteData(format: ui.ImageByteFormat.png);
    thumbImage.dispose();
    return png?.buffer.asUint8List();
  } on Object {
    return null;
  }
}
