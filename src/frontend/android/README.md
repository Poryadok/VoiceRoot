# Android release builds

## Signing

1. Create a release keystore (once):

   ```text
   keytool -genkey -v -keystore voice-release.jks -keyalg RSA -keysize 2048 -validity 10000 -alias voice
   ```

2. Copy `key.properties.example` to `key.properties` (gitignored) and fill paths/passwords.

3. Build:

   ```text
   cd src/frontend
   flutter build appbundle --release --dart-define=VOICE_API_BASE_URL=https://api.example.com
   ```

Without `key.properties`, release builds use the debug keystore for local smoke only.

## Firebase

Place `google-services.json` from the Firebase console in `app/`. Do not commit production credentials to a public repo.
