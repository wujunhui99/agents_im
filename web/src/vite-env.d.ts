/// <reference types="vite/client" />
/// <reference types="vitest/globals" />

declare module 'node:fs' {
  export function readFileSync(path: string, encoding: string): string;
}
