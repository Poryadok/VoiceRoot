import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';

/// Durable group/channel count. DM callers omit this footer entirely.
class MessageViewCount extends StatelessWidget {
  const MessageViewCount({super.key, required this.count});
  final int count;

  @override
  Widget build(BuildContext context) => Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      const Icon(Icons.visibility_outlined, size: 14),
      const SizedBox(width: 4),
      Text('$count', style: Theme.of(context).textTheme.labelSmall),
    ],
  );
}

/// List builders prefetch rows outside the viewport. Qualify only positive
/// intersection with the actual scrollable bounds, after layout and on scroll.
class MessageViewportView extends StatefulWidget {
  const MessageViewportView({
    super.key,
    required this.onVisible,
    required this.child,
  });
  final VoidCallback onVisible;
  final Widget child;

  @override
  State<MessageViewportView> createState() => _MessageViewportViewState();
}

class _MessageViewportViewState extends State<MessageViewportView> {
  ScrollableState? _scrollable;
  bool _reported = false;
  bool _scheduled = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final next = Scrollable.maybeOf(context);
    if (next != _scrollable) {
      _scrollable?.position.removeListener(_schedule);
      _scrollable = next;
      _scrollable?.position.addListener(_schedule);
    }
    _schedule();
  }

  void _schedule() {
    if (_reported || _scheduled) return;
    _scheduled = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _scheduled = false;
      if (!mounted || _reported) return;
      final box = context.findRenderObject();
      final Object? viewport = box == null
          ? null
          : RenderAbstractViewport.maybeOf(box);
      if (box is! RenderBox ||
          viewport is! RenderBox ||
          !box.hasSize ||
          !viewport.hasSize) {
        return;
      }
      final bounds = box.localToGlobal(Offset.zero) & box.size;
      final visible = viewport.localToGlobal(Offset.zero) & viewport.size;
      if (bounds.width > 0 && bounds.height > 0 && bounds.overlaps(visible)) {
        _reported = true;
        widget.onVisible();
      }
    });
  }

  @override
  void dispose() {
    _scrollable?.position.removeListener(_schedule);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => widget.child;
}
