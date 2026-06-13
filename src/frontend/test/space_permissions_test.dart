import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/space_permissions.dart';

void main() {
  test('hasPermission and setPermission round-trip bitmask', () {
    var mask = 0;
    mask = SpacePermissions.setPermission(mask, SpacePermissions.textChatSendMessages, true);
    expect(SpacePermissions.hasPermission(mask, SpacePermissions.textChatSendMessages), isTrue);
    mask = SpacePermissions.setPermission(mask, SpacePermissions.textChatSendMessages, false);
    expect(SpacePermissions.hasPermission(mask, SpacePermissions.textChatSendMessages), isFalse);
  });

  test('all permission map has 42 entries', () {
    expect(SpacePermissions.all, hasLength(42));
  });
}
