import '../settings/app_icon_preference.dart';

const supported = false;

Future<void> apply(AppIconPreference icon) async {
  throw UnsupportedError('Runtime app icons are unavailable on this host');
}
