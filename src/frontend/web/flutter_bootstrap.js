{{flutter_js}}
{{flutter_build_config}}

// runApp completion is not a rendered-frame signal. The early HTML controller
// retains its panel until the engine emits flutter-first-frame.
(async () => {
  try {
    await _flutter.loader.load({
      onEntrypointLoaded: async engineInitializer => {
        try {
          window.voiceStartup.stage('entrypoint');
          const appRunner = await engineInitializer.initializeEngine();
          window.voiceStartup.stage('engine');
          await appRunner.runApp();
          window.voiceStartup.stage('run-app');
        } catch {
          window.voiceStartup.fail();
        }
      }
    });
  } catch {
    window.voiceStartup.fail();
  }
})();
