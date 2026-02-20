/// <reference types="vite/client" />

declare module 'vue3-emoji-picker/css'

interface ImportMetaEnv {
  readonly VITE_API_URL: string
  readonly VITE_WS_URL: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
