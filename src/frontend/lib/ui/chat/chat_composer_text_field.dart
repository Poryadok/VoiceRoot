import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// Multiline chat composer: Enter sends, Ctrl/Shift+Enter inserts newline.
class ChatComposerTextField extends StatelessWidget {
  const ChatComposerTextField({
    super.key,
    required this.controller,
    required this.focusNode,
    required this.decoration,
    this.onSend,
    this.onChanged,
    this.readOnly = false,
    this.maxLines = 6,
  });

  final TextEditingController controller;
  final FocusNode focusNode;
  final InputDecoration decoration;
  final VoidCallback? onSend;
  final ValueChanged<String>? onChanged;
  final bool readOnly;
  final int maxLines;

  KeyEventResult _onKey(FocusNode node, KeyEvent event) {
    if (readOnly || onSend == null) {
      return KeyEventResult.ignored;
    }
    if (event is! KeyDownEvent) {
      return KeyEventResult.ignored;
    }
    if (event.logicalKey != LogicalKeyboardKey.enter) {
      return KeyEventResult.ignored;
    }
    final keyboard = HardwareKeyboard.instance;
    if (keyboard.isControlPressed ||
        keyboard.isShiftPressed ||
        keyboard.isAltPressed ||
        keyboard.isMetaPressed) {
      final value = controller.value;
      final selection = value.selection;
      final text = value.text;
      final start = selection.start >= 0 ? selection.start : text.length;
      final end = selection.end >= 0 ? selection.end : text.length;
      final newText = text.replaceRange(start, end, '\n');
      controller.value = value.copyWith(
        text: newText,
        selection: TextSelection.collapsed(offset: start + 1),
        composing: TextRange.empty,
      );
      return KeyEventResult.handled;
    }
    onSend!();
    return KeyEventResult.handled;
  }

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: controller,
      focusNode: focusNode,
      decoration: decoration,
      readOnly: readOnly,
      minLines: 1,
      maxLines: maxLines,
      keyboardType: TextInputType.multiline,
      textInputAction: TextInputAction.newline,
      onChanged: onChanged,
      onKeyEvent: _onKey,
    );
  }
}
