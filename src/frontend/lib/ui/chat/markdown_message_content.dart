import 'package:flutter/material.dart';
import 'package:flutter_highlight/flutter_highlight.dart';
import 'package:highlight/highlight.dart' show highlight;
import 'package:url_launcher/url_launcher.dart';

import '../../theme/voice_colors.dart';

/// Renders the text-chat markdown subset for message bubbles.
class MarkdownMessageContent extends StatefulWidget {
  const MarkdownMessageContent({super.key, required this.content});

  final String content;

  @override
  State<MarkdownMessageContent> createState() => _MarkdownMessageContentState();
}

class _MarkdownMessageContentState extends State<MarkdownMessageContent> {
  final Set<String> _revealedSpoilers = {};
  bool _revealAllSpoilers = false;

  @override
  Widget build(BuildContext context) {
    try {
      return _buildContent(context);
    } catch (_) {
      return Text(widget.content);
    }
  }

  Widget _buildContent(BuildContext context) {
    final theme = Theme.of(context);
    final voice = VoiceColors.of(context);
    final blocks = _splitFencedBlocks(widget.content);
    final children = <Widget>[];

    for (var i = 0; i < blocks.length; i++) {
      final block = blocks[i];
      if (block.isCode) {
        children.add(_CodeBlock(language: block.lang, code: block.text));
        continue;
      }
      final lines = block.text.split('\n');
      for (final line in lines) {
        if (line.trim().isEmpty) {
          children.add(const SizedBox(height: 4));
          continue;
        }
        final trimmed = line.trimLeft();
        if (trimmed.startsWith('>')) {
          final quote = trimmed.replaceFirst(RegExp(r'^>\s?'), '');
          children.add(
            Padding(
              padding: const EdgeInsets.only(left: 8, top: 2, bottom: 2),
              child: DecoratedBox(
                decoration: BoxDecoration(
                  border: Border(
                    left: BorderSide(color: voice.profileAccent, width: 3),
                  ),
                ),
                child: Padding(
                  padding: const EdgeInsets.only(left: 8),
                  child: _inlineRich(
                    context,
                    quote,
                    theme.textTheme.bodyMedium,
                  ),
                ),
              ),
            ),
          );
          continue;
        }
        final header = RegExp(r'^(#{1,3})\s+(.+)$').firstMatch(trimmed);
        if (header != null) {
          final level = header.group(1)!.length;
          final style = switch (level) {
            1 => theme.textTheme.titleLarge,
            2 => theme.textTheme.titleMedium,
            _ => theme.textTheme.titleSmall,
          };
          children.add(
            Padding(
              padding: const EdgeInsets.only(top: 4, bottom: 2),
              child: _inlineRich(context, header.group(2)!, style),
            ),
          );
          continue;
        }
        final bullet = RegExp(r'^-\s+(.+)$').firstMatch(trimmed);
        if (bullet != null) {
          children.add(
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text('• '),
                Expanded(
                  child: _inlineRich(
                    context,
                    bullet.group(1)!,
                    theme.textTheme.bodyMedium,
                  ),
                ),
              ],
            ),
          );
          continue;
        }
        final numbered = RegExp(r'^\d+\.\s+(.+)$').firstMatch(trimmed);
        if (numbered != null) {
          final prefix = trimmed.substring(0, trimmed.indexOf(' ') + 1);
          children.add(
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(prefix),
                Expanded(
                  child: _inlineRich(
                    context,
                    numbered.group(1)!,
                    theme.textTheme.bodyMedium,
                  ),
                ),
              ],
            ),
          );
          continue;
        }
        children.add(_inlineRich(context, line, theme.textTheme.bodyMedium));
      }
    }

    if (children.isEmpty) {
      return const SizedBox.shrink();
    }
    final body = children.length == 1
        ? children.first
        : Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: children,
          );
    return GestureDetector(
      behavior: HitTestBehavior.translucent,
      onTap: () {
        if (!_revealAllSpoilers) {
          setState(() => _revealAllSpoilers = true);
        }
      },
      child: body,
    );
  }

  Widget _inlineRich(BuildContext context, String text, TextStyle? base) {
    final spans = _parseInline(context, text, base);
    if (spans.isEmpty) {
      return Text(text, style: base);
    }
    return RichText(text: TextSpan(style: base, children: spans));
  }

  List<InlineSpan> _parseInline(
    BuildContext context,
    String text,
    TextStyle? base,
  ) {
    final spans = <InlineSpan>[];
    var i = 0;

    while (i < text.length) {
      final slice = text.substring(i);

      final link = RegExp(
        r'^\[([^\]]+)\]\(((?:[^()]|\([^()]*\))*)\)',
      ).firstMatch(slice);
      if (link != null) {
        final label = link.group(1)!;
        final url = link.group(2)!;
        spans.add(_linkSpan(context, label, url, base));
        i += link.end;
        continue;
      }

      final autoUrl = RegExp(r'^(https?://[^\s]+)').firstMatch(slice);
      if (autoUrl != null) {
        final url = autoUrl.group(1)!;
        spans.add(_linkSpan(context, url, url, base));
        i += autoUrl.end;
        continue;
      }

      final spoiler = RegExp(r'^\|\|([^|]+)\|\|').firstMatch(slice);
      if (spoiler != null) {
        final inner = spoiler.group(1)!;
        final revealed = _revealAllSpoilers || _revealedSpoilers.contains(inner);
        spans.add(
          WidgetSpan(
            alignment: PlaceholderAlignment.middle,
            child: revealed
                ? Text(inner, style: base)
                : Container(
                    padding: const EdgeInsets.symmetric(horizontal: 4),
                    color: Theme.of(
                      context,
                    ).colorScheme.surfaceContainerHighest,
                    child: Text('spoiler', style: base),
                  ),
          ),
        );
        i += spoiler.end;
        continue;
      }

      final inlineCode = RegExp(r'^`([^`]+)`').firstMatch(slice);
      if (inlineCode != null) {
        spans.add(
          TextSpan(
            text: inlineCode.group(1),
            style: base?.copyWith(
              fontFamily: 'monospace',
              backgroundColor: Theme.of(
                context,
              ).colorScheme.surfaceContainerHighest,
            ),
          ),
        );
        i += inlineCode.end;
        continue;
      }

      final bold = RegExp(r'^\*\*([^*]+)\*\*').firstMatch(slice);
      if (bold != null) {
        spans.add(
          TextSpan(
            text: bold.group(1),
            style: base?.copyWith(fontWeight: FontWeight.bold),
          ),
        );
        i += bold.end;
        continue;
      }

      final italic = RegExp(r'^\*([^*]+)\*').firstMatch(slice);
      if (italic != null) {
        spans.add(
          TextSpan(
            text: italic.group(1),
            style: base?.copyWith(fontStyle: FontStyle.italic),
          ),
        );
        i += italic.end;
        continue;
      }

      final underline = RegExp(r'^__([^_]+)__').firstMatch(slice);
      if (underline != null) {
        spans.add(
          TextSpan(
            text: underline.group(1),
            style: base?.copyWith(decoration: TextDecoration.underline),
          ),
        );
        i += underline.end;
        continue;
      }

      final strike = RegExp(r'^~~([^~]+)~~').firstMatch(slice);
      if (strike != null) {
        spans.add(
          TextSpan(
            text: strike.group(1),
            style: base?.copyWith(decoration: TextDecoration.lineThrough),
          ),
        );
        i += strike.end;
        continue;
      }

      final nextSpecial = _indexOfSpecial(slice);
      if (nextSpecial < 0) {
        _appendPlainWithAutolinks(context, spans, slice, base);
        break;
      }
      if (nextSpecial > 0) {
        _appendPlainWithAutolinks(
          context,
          spans,
          slice.substring(0, nextSpecial),
          base,
        );
      }
      i += nextSpecial == 0 ? 1 : nextSpecial;
    }

    return spans;
  }

  int _indexOfSpecial(String slice) {
    final re = RegExp(r'[\[\*`~_|]');
    final m = re.firstMatch(slice);
    if (m == null) {
      return -1;
    }
    return m.start;
  }

  void _appendPlainWithAutolinks(
    BuildContext context,
    List<InlineSpan> spans,
    String text,
    TextStyle? base,
  ) {
    if (text.isEmpty) {
      return;
    }
    final re = RegExp(r'https?://[^\s]+');
    var start = 0;
    for (final m in re.allMatches(text)) {
      if (m.start > start) {
        spans.add(TextSpan(text: text.substring(start, m.start), style: base));
      }
      spans.add(_linkSpan(context, m.group(0)!, m.group(0)!, base));
      start = m.end;
    }
    if (start < text.length) {
      spans.add(TextSpan(text: text.substring(start), style: base));
    }
  }

  InlineSpan _linkSpan(
    BuildContext context,
    String label,
    String url,
    TextStyle? base,
  ) {
    final uri = Uri.tryParse(url);
    final safe =
        uri != null && (uri.scheme == 'http' || uri.scheme == 'https');
    final voice = VoiceColors.of(context);
    final style = base?.copyWith(
      color: voice.profileAccent,
      decoration: TextDecoration.underline,
    );
    if (!safe) {
      return TextSpan(text: label, style: style);
    }
    return WidgetSpan(
      alignment: PlaceholderAlignment.baseline,
      baseline: TextBaseline.alphabetic,
      child: GestureDetector(
        key: const Key('markdown_link'),
        onTap: () => launchUrl(uri, mode: LaunchMode.externalApplication),
        child: Text(label, style: style),
      ),
    );
  }
}

class _FencedBlock {
  _FencedBlock({required this.text, this.lang, this.isCode = false});

  final String text;
  final String? lang;
  final bool isCode;
}

List<_FencedBlock> _splitFencedBlocks(String content) {
  final re = RegExp(r'```([\w-]*)\n?([\s\S]*?)```', multiLine: true);
  final out = <_FencedBlock>[];
  var start = 0;
  for (final m in re.allMatches(content)) {
    if (m.start > start) {
      out.add(_FencedBlock(text: content.substring(start, m.start)));
    }
    out.add(
      _FencedBlock(
        text: m.group(2) ?? '',
        lang: m.group(1)?.isEmpty == true ? null : m.group(1),
        isCode: true,
      ),
    );
    start = m.end;
  }
  if (start < content.length) {
    out.add(_FencedBlock(text: content.substring(start)));
  }
  if (out.isEmpty) {
    out.add(_FencedBlock(text: content));
  }
  return out;
}

class _CodeBlock extends StatelessWidget {
  const _CodeBlock({required this.code, this.language});

  final String code;
  final String? language;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    Map<String, TextStyle>? highlightTheme;
    try {
      highlight.parse(code, language: language);
      highlightTheme = {
        'root': TextStyle(
          color: theme.colorScheme.onSurface,
          fontFamily: 'monospace',
          fontSize: 13,
        ),
      };
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: 4),
        child: HighlightView(
          code,
          language: language ?? 'plaintext',
          theme: highlightTheme,
          padding: const EdgeInsets.all(8),
          textStyle: const TextStyle(fontFamily: 'monospace', fontSize: 13),
        ),
      );
    } catch (_) {
      return Container(
        width: double.infinity,
        padding: const EdgeInsets.all(8),
        color: theme.colorScheme.surfaceContainerHighest,
        child: Text(
          code.trimRight(),
          style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
        ),
      );
    }
  }
}
