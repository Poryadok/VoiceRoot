import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/voice_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/settings/voice_input_settings.dart';
import 'package:voice_frontend/state/call_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/ui/a11y/voice_shortcuts.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

class _FixedVoiceInputSettings extends VoiceInputSettingsNotifier {
  _FixedVoiceInputSettings(this.fixed);
  final VoiceInputSettings fixed;

  @override
  VoiceInputSettings build() => fixed;
}

void main() {
  testWidgets('in-app PTT keybind hold/release while focused', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://127.0.0.1:18080'),
        ),
        voiceInputSettingsProvider.overrideWith(
          () => _FixedVoiceInputSettings(
            const VoiceInputSettings(mode: VoiceInputMode.ptt),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);

    container.read(callControllerProvider.notifier).state = const CallState(
      phase: CallPhase.active,
      session: VoiceCallSession(
        roomId: 'room-1',
        livekitRoomName: 'lk-room',
        chatId: 'chat-1',
        initiatorProfileId: 'me',
        calleeProfileId: 'peer',
        mediaKind: VoiceCallMediaKind.audio,
        status: VoiceCallStatus.active,
      ),
    );

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const VoiceShortcuts(child: SizedBox.expand()),
        ),
      ),
    );
    await tester.pump();

    await tester.sendKeyDownEvent(LogicalKeyboardKey.backquote);
    await tester.pump();
    expect(container.read(callControllerProvider).isPttHeld, isTrue);

    await tester.sendKeyUpEvent(LogicalKeyboardKey.backquote);
    await tester.pump();
    expect(container.read(callControllerProvider).isPttHeld, isFalse);
  });

  testWidgets('unrelated key does not hold PTT', (tester) async {
    final container = ProviderContainer(
      overrides: [
        ...voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
        voiceInputSettingsProvider.overrideWith(
          () => _FixedVoiceInputSettings(
            const VoiceInputSettings(mode: VoiceInputMode.ptt),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: MaterialApp(
          theme: voiceTestTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const VoiceShortcuts(child: SizedBox.expand()),
        ),
      ),
    );
    await tester.pump();

    await tester.sendKeyDownEvent(LogicalKeyboardKey.keyA);
    await tester.pump();
    expect(container.read(callControllerProvider).isPttHeld, isFalse);
  });
}
