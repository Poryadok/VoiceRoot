import 'app_shell_icon_stub.dart'
    if (dart.library.html) 'app_shell_icon_web.dart'
    as implementation;

import '../settings/app_icon_preference.dart';

bool get supportsWebAppShellIcon => implementation.supported;

Future<void> applyWebAppShellIcon(AppIconPreference icon) =>
    implementation.apply(icon);
