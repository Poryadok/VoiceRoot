import 'dart:js_interop';

@JS('voiceMusicReady')
external JSPromise<JSAny?> get _ready;

/// The index installs this promise before Flutter starts. Await the WASM
/// instance before the package calls its synchronous native exports.
Future<void> musicModuleReady() async {
  await _ready.toDart;
}
