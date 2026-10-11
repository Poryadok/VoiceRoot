// ignore_for_file: avoid_web_libraries_in_flutter, deprecated_member_use

import 'dart:html' as html;

import 'package:flutter/services.dart';

import '../settings/app_icon_preference.dart';

const supported = true;

String? _previousObjectUrl;

Future<void> apply(AppIconPreference icon) async {
  final data = await rootBundle.load('assets/app_icons/${icon.id}.png');
  final bytes = data.buffer.asUint8List(data.offsetInBytes, data.lengthInBytes);
  final objectUrl = html.Url.createObjectUrlFromBlob(
    html.Blob(<Object>[bytes], 'image/png'),
  );
  final links = html.document.querySelectorAll('link[rel~="icon"]');
  final html.LinkElement link;
  if (links.isEmpty) {
    link = html.LinkElement()..rel = 'icon';
    html.document.head!.append(link);
  } else {
    link = links.first as html.LinkElement;
  }
  link.href = objectUrl;
  final oldUrl = _previousObjectUrl;
  _previousObjectUrl = objectUrl;
  if (oldUrl != null) html.Url.revokeObjectUrl(oldUrl);
}
