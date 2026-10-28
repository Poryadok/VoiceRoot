import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;
import 'package:url_launcher/url_launcher.dart';

import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';

const _requestTimeout = Duration(seconds: 6);
const _maxHtmlBytes = 256 * 1024;
const _maxImageBytes = 1024 * 1024;
const _maxTitleLength = 160;
const _maxDescriptionLength = 320;

final http.Client _defaultClient = http.Client();

/// Returns the first inline HTTPS URL eligible for an OG preview.
String? linkPreviewUrlFromContent(String content) {
  for (final match in RegExp(
    r'''https://[^\s<>"']+''',
    caseSensitive: false,
  ).allMatches(content)) {
    final candidate = match.group(0)!.replaceFirst(RegExp(r'[.,!?;:)}]+$'), '');
    final uri = _safeHttpsUri(candidate);
    if (uri != null) return uri.toString();
  }
  return null;
}

/// Fetches and displays an inline URL preview for TEXT messages.
///
/// The request is client-side, HTTPS-only, bounded, and does not follow
/// redirects. Remote HTML is only parsed for text fields and is never rendered.
class LinkPreviewCard extends StatefulWidget {
  const LinkPreviewCard({super.key, required this.url, this.client});

  final String url;
  final http.Client? client;

  @override
  State<LinkPreviewCard> createState() => _LinkPreviewCardState();
}

class _LinkPreviewCardState extends State<LinkPreviewCard> {
  late Future<_LinkMetadata?> _metadata;

  @override
  void initState() {
    super.initState();
    _metadata = _load();
  }

  @override
  void didUpdateWidget(covariant LinkPreviewCard oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.url != widget.url || oldWidget.client != widget.client) {
      _metadata = _load();
    }
  }

  Future<_LinkMetadata?> _load() =>
      _fetchLinkMetadata(widget.url, client: widget.client ?? _defaultClient);

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final uri = Uri.tryParse(widget.url);
    final domain = uri?.host ?? widget.url;
    final colors = VoiceColors.of(context);
    return FutureBuilder<_LinkMetadata?>(
      key: const Key('link_preview_card'),
      future: _metadata,
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) {
          return _PreviewFrame(
            child: Row(
              key: const Key('link_preview_loading'),
              children: [
                SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: colors.profileAccent,
                  ),
                ),
                const SizedBox(width: 8),
                Text(l10n.chatLinkPreviewLoading),
              ],
            ),
          );
        }
        final metadata = snapshot.data;
        if (metadata == null) {
          return _PreviewFrame(
            child: Row(
              children: [
                Expanded(
                  child: Column(
                    key: const Key('link_preview_unavailable'),
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        domain,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: Theme.of(context).textTheme.labelMedium,
                      ),
                      Text(l10n.chatLinkPreviewUnavailable),
                    ],
                  ),
                ),
                TextButton(
                  key: const Key('link_preview_retry'),
                  onPressed: () => setState(() {
                    _metadata = _load();
                  }),
                  child: Text(l10n.chatLinkPreviewRetry),
                ),
              ],
            ),
          );
        }
        return Semantics(
          link: true,
          label: '${metadata.title}, ${metadata.domain}',
          child: InkWell(
            key: const Key('link_preview_ready'),
            onTap: () => launchUrl(
              Uri.parse(widget.url),
              mode: LaunchMode.externalApplication,
            ),
            child: _PreviewFrame(
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          metadata.title,
                          key: const Key('link_preview_title'),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                          style: Theme.of(context).textTheme.titleSmall,
                        ),
                        if (metadata.description case final description?)
                          Padding(
                            padding: const EdgeInsets.only(top: 4),
                            child: Text(
                              description,
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text(
                            metadata.domain,
                            key: const Key('link_preview_domain'),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: Theme.of(context).textTheme.labelSmall
                                ?.copyWith(color: colors.textSecondary),
                          ),
                        ),
                      ],
                    ),
                  ),
                  if (metadata.imageBytes case final imageBytes?) ...[
                    const SizedBox(width: 8),
                    ClipRRect(
                      borderRadius: BorderRadius.circular(4),
                      child: Image.memory(
                        imageBytes,
                        key: const Key('link_preview_image'),
                        width: 64,
                        height: 64,
                        cacheWidth: 128,
                        cacheHeight: 128,
                        fit: BoxFit.cover,
                        errorBuilder: (_, _, _) => const SizedBox.shrink(),
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}

class _PreviewFrame extends StatelessWidget {
  const _PreviewFrame({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final colors = VoiceColors.of(context);
    return Container(
      constraints: const BoxConstraints(maxWidth: 360),
      margin: const EdgeInsets.only(top: 6),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: colors.surface,
        border: Border(left: BorderSide(color: colors.profileAccent, width: 3)),
        borderRadius: BorderRadius.circular(4),
      ),
      child: child,
    );
  }
}

/// Failures return null so the card can keep the original URL and show a
/// recoverable unavailable state.
Future<_LinkMetadata?> _fetchLinkMetadata(
  String rawUrl, {
  required http.Client client,
}) async {
  final uri = _safeHttpsUri(rawUrl);
  if (uri == null) return null;
  final bytes = await _fetchHttpsBytes(
    uri,
    client: client,
    contentType: 'text/html',
    maxBytes: _maxHtmlBytes,
  );
  if (bytes == null) return null;
  final metadata = _parseMetadata(
    uri,
    utf8.decode(bytes, allowMalformed: true),
  );
  final imageUri = metadata.imageUrl == null
      ? null
      : _safeHttpsUri(metadata.imageUrl!);
  final imageBytes = imageUri == null
      ? null
      : await _fetchHttpsBytes(
          imageUri,
          client: client,
          contentType: 'image/',
          maxBytes: _maxImageBytes,
        );
  return _LinkMetadata(
    title: metadata.title,
    domain: metadata.domain,
    description: metadata.description,
    imageBytes: imageBytes,
  );
}

/// Metadata and thumbnails share HTTPS-only, redirect-denying bounded reads.
Future<Uint8List?> _fetchHttpsBytes(
  Uri uri, {
  required http.Client client,
  required String contentType,
  required int maxBytes,
}) async {
  try {
    final request = http.Request('GET', uri)
      ..followRedirects = false
      ..maxRedirects = 0
      ..headers['Accept'] = contentType == 'image/' ? 'image/*' : contentType;
    final response = await client.send(request).timeout(_requestTimeout);
    if (response.statusCode != 200 ||
        !(response.headers['content-type']?.toLowerCase().startsWith(
              contentType,
            ) ??
            false)) {
      await response.stream.listen(null).cancel();
      return null;
    }
    final bytes = BytesBuilder(copy: false);
    await for (final chunk in response.stream.timeout(_requestTimeout)) {
      if (bytes.length + chunk.length > maxBytes) return null;
      bytes.add(chunk);
    }
    return bytes.takeBytes();
  } catch (_) {
    return null;
  }
}

Uri? _safeHttpsUri(String rawUrl) {
  final uri = Uri.tryParse(rawUrl);
  if (uri == null ||
      uri.scheme.toLowerCase() != 'https' ||
      uri.host.isEmpty ||
      uri.userInfo.isNotEmpty ||
      uri.host.toLowerCase() == 'localhost' ||
      uri.host.toLowerCase().endsWith('.localhost') ||
      uri.host.toLowerCase().endsWith('.local') ||
      uri.host.toLowerCase().endsWith('.internal') ||
      !RegExp(r'^[a-z0-9.-]+$', caseSensitive: false).hasMatch(uri.host)) {
    return null;
  }
  return uri;
}

_LinkMetadata _parseMetadata(Uri uri, String html) {
  final meta = <String, String>{};
  for (final tag in RegExp(
    r'<meta\b[^>]*>',
    caseSensitive: false,
  ).allMatches(html)) {
    final attrs = _htmlAttributes(tag.group(0)!);
    final property = (attrs['property'] ?? attrs['name'] ?? '').toLowerCase();
    final content = attrs['content'];
    if (content != null && content.isNotEmpty) {
      meta.putIfAbsent(property, () => content);
    }
  }
  final titleTag = RegExp(
    r'<title\b[^>]*>([\s\S]*?)</title\s*>',
    caseSensitive: false,
  ).firstMatch(html)?.group(1);
  final title = _cleanHtmlText(meta['og:title'] ?? meta['twitter:title'] ?? '')
      .ifEmpty(() => _cleanHtmlText(titleTag ?? ''))
      .ifEmpty(() => uri.host)
      .truncate(_maxTitleLength);
  final description = _cleanHtmlText(
    meta['og:description'] ?? meta['description'] ?? '',
  ).truncate(_maxDescriptionLength);
  final imageValue = meta['og:image'] ?? meta['twitter:image'];
  final imageUri = imageValue == null ? null : uri.resolve(imageValue);
  return _LinkMetadata(
    title: title,
    description: description.isEmpty ? null : description,
    domain: uri.host,
    imageUrl: imageUri != null && _safeHttpsUri(imageUri.toString()) != null
        ? imageUri.toString()
        : null,
  );
}

Map<String, String> _htmlAttributes(String tag) {
  final attributes = <String, String>{};
  final pattern = RegExp(
    r'''([\w:-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))''',
  );
  for (final match in pattern.allMatches(tag)) {
    attributes[match.group(1)!.toLowerCase()] = _decodeHtmlEntities(
      match.group(2) ?? match.group(3) ?? match.group(4) ?? '',
    );
  }
  return attributes;
}

String _cleanHtmlText(String input) => _decodeHtmlEntities(
  input
      .replaceAll(RegExp(r'<[^>]*>'), ' ')
      .replaceAll(RegExp(r'[\u0000-\u001f\u007f]'), ' '),
).replaceAll(RegExp(r'<[^>]*>'), ' ').replaceAll(RegExp(r'\s+'), ' ').trim();

String _decodeHtmlEntities(String input) => input.replaceAllMapped(
  RegExp(r'&(#x[0-9a-f]+|#\d+|amp|quot|apos|#39|lt|gt);', caseSensitive: false),
  (match) {
    final entity = match.group(1)!;
    if (entity.startsWith('#x')) {
      return String.fromCharCode(
        int.tryParse(entity.substring(2), radix: 16) ?? 0xfffd,
      );
    }
    if (entity.startsWith('#')) {
      return String.fromCharCode(int.tryParse(entity.substring(1)) ?? 0xfffd);
    }
    return switch (entity.toLowerCase()) {
      'amp' => '&',
      'quot' => '"',
      'apos' || '#39' => "'",
      'lt' => '<',
      'gt' => '>',
      _ => match.group(0)!,
    };
  },
);

class _LinkMetadata {
  const _LinkMetadata({
    required this.title,
    required this.domain,
    this.description,
    this.imageUrl,
    this.imageBytes,
  });

  final String title;
  final String domain;
  final String? description;
  final String? imageUrl;
  final Uint8List? imageBytes;
}

extension on String {
  String ifEmpty(String Function() fallback) => isEmpty ? fallback() : this;

  String truncate(int maxLength) =>
      length <= maxLength ? this : '${substring(0, maxLength - 1)}…';
}
