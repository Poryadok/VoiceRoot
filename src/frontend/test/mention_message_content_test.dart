import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/messages_client.dart';
import 'package:voice_frontend/backend/users_client.dart';
import 'package:voice_frontend/state/social_providers.dart';
import 'package:voice_frontend/ui/chat/markdown_message_content.dart';
import 'package:voice_frontend/ui/chat/mention_message_content.dart';

import 'support/voice_test_theme.dart';

const _userProfileId = '11111111-1111-1111-1111-111111111111';

void main() {
  testWidgets('user mention is blue underlined link to profile', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          profileProvider(_userProfileId).overrideWith(
            (ref) async => const VoiceProfile(
              id: _userProfileId,
              accountId: 'acc-1',
              username: 'alice',
              discriminator: '0001',
              displayName: 'Alice',
            ),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          home: Scaffold(
            body: MentionMessageContent(
              content: 'hi @$_userProfileId',
              mentions: const [
                MessageMention(type: 'user', targetId: _userProfileId),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('mention_user_link_$_userProfileId')), findsOneWidget);

    final rich = tester.widget<RichText>(find.byType(RichText).first);
    final root = rich.text as TextSpan;
    final mentionSpan = root.children!.singleWhere(
      (span) => span is TextSpan && span.text?.contains('@alice') == true,
    ) as TextSpan;
    expect(mentionSpan.style?.decoration, TextDecoration.underline);
    expect(mentionSpan.recognizer, isA<TapGestureRecognizer>());
  });

  testWidgets('broadcast mention is styled but not a profile link', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: Scaffold(
          body: MentionMessageContent(
            content: 'ping @everyone',
            mentions: const [MessageMention(type: 'everyone')],
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('mention_user_link_everyone')), findsNothing);
    expect(find.byKey(const Key('mention_broadcast_everyone')), findsOneWidget);

    final rich = tester.widget<RichText>(find.byType(RichText).first);
    final root = rich.text as TextSpan;
    final mentionSpan = root.children!.singleWhere(
      (span) => span is TextSpan && span.text == '@everyone',
    ) as TextSpan;
    expect(mentionSpan.recognizer, isNull);
  });

  testWidgets('plain content without mentions uses markdown path', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: Scaffold(
          body: MentionMessageContent(
            content: 'hello **world**',
            mentions: const [],
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(MarkdownMessageContent), findsOneWidget);
  });

  testWidgets('@here broadcast mention is highlighted', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        home: Scaffold(
          body: MentionMessageContent(
            content: 'heads up @here',
            mentions: const [MessageMention(type: 'here')],
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('mention_broadcast_here')), findsOneWidget);
    final rich = tester.widget<RichText>(find.byType(RichText).first);
    expect(rich.text.toPlainText(), contains('@here'));
  });
}
