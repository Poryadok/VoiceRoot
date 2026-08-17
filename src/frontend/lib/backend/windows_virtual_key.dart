import 'package:flutter/services.dart';

/// Win32 virtual-key code for a Flutter logical key (global PTT hook).
int? windowsVkCodeForLogicalKey(LogicalKeyboardKey key) {
  if (key == LogicalKeyboardKey.backquote) return 0xC0; // VK_OEM_3
  if (key == LogicalKeyboardKey.space) return 0x20;
  if (key == LogicalKeyboardKey.tab) return 0x09;
  if (key.keyId >= LogicalKeyboardKey.keyA.keyId &&
      key.keyId <= LogicalKeyboardKey.keyZ.keyId) {
    return 0x41 + (key.keyId - LogicalKeyboardKey.keyA.keyId);
  }
  if (key.keyId >= LogicalKeyboardKey.digit0.keyId &&
      key.keyId <= LogicalKeyboardKey.digit9.keyId) {
    return 0x30 + (key.keyId - LogicalKeyboardKey.digit0.keyId);
  }
  if (key.keyId >= LogicalKeyboardKey.f1.keyId &&
      key.keyId <= LogicalKeyboardKey.f24.keyId) {
    return 0x70 + (key.keyId - LogicalKeyboardKey.f1.keyId);
  }
  return null;
}
