import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations_en.dart';
import 'package:voice_frontend/ui/privacy/privacy_action_errors.dart';

void main() {
  test('privacyActionErrorMessage maps backend denial strings', () {
    final l10n = AppLocalizationsEn();
    expect(
      privacyActionErrorMessage(l10n, 'call blocked by recipient privacy settings'),
      l10n.privacyDeniedCall,
    );
    expect(
      privacyActionErrorMessage(l10n, 'invite blocked by recipient privacy settings'),
      l10n.privacyDeniedInvite,
    );
    expect(
      privacyActionErrorMessage(
        l10n,
        'file attachment blocked by recipient privacy settings',
      ),
      l10n.privacyDeniedFile,
    );
    expect(
      privacyActionErrorMessage(
        l10n,
        'voice message blocked by recipient privacy settings',
      ),
      l10n.privacyDeniedVoice,
    );
    expect(
      privacyActionErrorMessage(l10n, 'unknown upstream'),
      'unknown upstream',
    );
  });
}
