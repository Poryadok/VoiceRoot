import 'package:flutter/material.dart';

/// Global keys for onboarding coach-mark anchors (docs/features/onboarding.md).
abstract final class OnboardingAnchorKeys {
  static final saveAccountStep = GlobalKey(
    debugLabel: 'onboarding_save_account_step',
  );
  static final chatsNav = GlobalKey(debugLabel: 'onboarding_chats_nav');
  static final spaces = GlobalKey(debugLabel: 'onboarding_spaces');
  static final matchmaking = GlobalKey(
    debugLabel: 'onboarding_matchmaking',
  );
}
