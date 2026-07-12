/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_VOICE_API_BASE: string;
  readonly VITE_STAFF_TOKEN: string;
  readonly VITE_OAUTH_CLIENT_ID: string;
  readonly VITE_OAUTH_DISABLED: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
