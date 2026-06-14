/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_VOICE_API_BASE: string;
  readonly VITE_STAFF_TOKEN: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
