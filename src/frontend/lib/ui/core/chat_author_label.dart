import 'package:flutter/material.dart';

import 'premium_badge.dart';

/// Display name with optional premium ★ badge.
class ChatAuthorLabel extends StatelessWidget {
  const ChatAuthorLabel({
    super.key,
    required this.displayName,
    this.isPremium = false,
    this.style,
    this.premiumBadgeSemanticLabel,
  });

  final String displayName;
  final bool isPremium;
  final TextStyle? style;
  final String? premiumBadgeSemanticLabel;

  @override
  Widget build(BuildContext context) {
    final textStyle = style ?? Theme.of(context).textTheme.bodyLarge;
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
        if (isPremium) ...[
          const SizedBox(width: 4),
          PremiumBadge(semanticLabel: premiumBadgeSemanticLabel),
        ],
      ],
    );
  }
}
