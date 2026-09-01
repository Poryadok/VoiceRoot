import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/state/message_requests_providers.dart';

void main() {
  test('isMessageRequestsFolderSelected recognizes virtual folder id', () {
    expect(isMessageRequestsFolderSelected(kVirtualMessageRequestsFolderId), isTrue);
    expect(isMessageRequestsFolderSelected('folder-1'), isFalse);
    expect(isMessageRequestsFolderSelected(null), isFalse);
  });

  test('MessageRequestsSummary visibility follows pending count', () {
    expect(
      const MessageRequestsSummary(pendingCount: 0, unreadCount: 0).isVisible,
      isFalse,
    );
    expect(
      const MessageRequestsSummary(pendingCount: 1, unreadCount: 0).isVisible,
      isTrue,
    );
  });
}
