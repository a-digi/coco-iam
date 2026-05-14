/// <reference types="vite/client" />

// Typed environment variables exposed to the client bundle.
// Every var read via import.meta.env.VITE_* needs an entry here
// so TypeScript knows the shape. Match the keys in .env /
// .env.example.
interface ImportMetaEnv {
  readonly VITE_API_BASE_URL: string;
  readonly VITE_FRONTEND_URL: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
