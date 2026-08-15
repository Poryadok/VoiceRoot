import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// Multiline chat composer: Enter sends, Ctrl/Shift+Enter inserts newline.
class ChatComposerTextField extends StatefulWidget {
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

  @override
  State<ChatComposerTextField> createState() => _ChatComposerTextFieldState();
}

class _ChatComposerTextFieldState extends State<ChatComposerTextField> {
  @override
  void initState() {
    super.initState();
    widget.focusNode.onKeyEvent = _onKey;
  }

  @override
  void didUpdateWidget(covariant ChatComposerTextField oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.focusNode != widget.focusNode) {
      oldWidget.focusNode.onKeyEvent = null;
      widget.focusNode.onKeyEvent = _onKey;
    }
  }

  @override
  void dispose() {
    widget.focusNode.onKeyEvent = null;
    super.dispose();
  }

  KeyEventResult _onKey(FocusNode node, KeyEvent event) {
    if (widget.readOnly || widget.onSend == null) {
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
      final value = widget.controller.value;
      final selection = value.selection;
      final text = value.text;
      final start = selection.start >= 0 ? selection.start : text.length;
      final end = selection.end >= 0 ? selection.end : text.length;
      final newText = text.replaceRange(start, end, '\n');
      widget.controller.value = value.copyWith(
        text: newText,
        selection: TextSelection.collapsed(offset: start + 1),
        composing: TextRange.empty,
      );
      return KeyEventResult.handled;
    }
    widget.onSend!();
    return KeyEventResult.handled;
  }

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: widget.controller,
      focusNode: widget.focusNode,
      decoration: widget.decoration,
      readOnly: widget.readOnly,
      minLines: 1,
      maxLines: widget.maxLines,
      keyboardType: TextInputType.multiline,
      textInputAction: TextInputAction.newline,
      onChanged: widget.onChanged,
    );
  }
}
