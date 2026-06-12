import 'package:connectivity_plus/connectivity_plus.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/state/connectivity_providers.dart';

void main() {
  test('connectivityResultsAreOffline treats none as offline', () {
    expect(connectivityResultsAreOffline([ConnectivityResult.none]), isTrue);
    expect(connectivityResultsAreOffline(const []), isTrue);
    expect(
      connectivityResultsAreOffline([ConnectivityResult.wifi]),
      isFalse,
    );
    expect(
      connectivityResultsAreOffline([
        ConnectivityResult.wifi,
        ConnectivityResult.mobile,
      ]),
      isFalse,
    );
  });
}
