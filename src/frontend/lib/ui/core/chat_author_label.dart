import 'package:flutter/material.dart';

import 'premium_badge.dart';
import 'verified_badge.dart';

/// Display name with optional premium ★ and verification badges.
class ChatAuthorLabel extends StatelessWidget {
  const ChatAuthorLabel({
    super.key,
    required this.displayName,
    this.isPremium = false,
    this.verificationType = 'none',
    this.style,
    this.premiumBadgeSemanticLabel,
    this.verifiedBadgeSemanticLabel,
  });

  final String displayName;
  final bool isPremium;
  final String verificationType;
  final TextStyle? style;
  final String? premiumBadgeSemanticLabel;
  final String? verifiedBadgeSemanticLabel;

  @override
  Widget build(BuildContext context) {
    final textStyle = style ?? Theme.of(context).textTheme.bodyLarge;
    final showVerified = showsVerifiedBadge(verificationType);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Flexible(
          child: Text(
            displayName,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: textStyle,
          ),
        ),
        if (showVerified) ...[
          const SizedBox(width: 4),
          VerifiedBadge(
            verificationType: verificationType,
            semanticLabel: verifiedBadgeSemanticLabel,
          ),
        ],
        if (isPremium) ...[
          const SizedBox(width: 4),
          PremiumBadge(semanticLabel: premiumBadgeSemanticLabel),
        ],
      ],
    );
  }
}
