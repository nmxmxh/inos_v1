import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';
import viteCompression from 'vite-plugin-compression';

export default defineConfig({
  plugins: [
    react(),
    // Gzip compression for static assets
    viteCompression({
      algorithm: 'gzip',
      ext: '.gz',
      threshold: 10240, // Only compress if > 10kb
      deleteOriginFile: false,
    }),
  ],
  server: {
    port: 5173,
    headers: {
      // Disable WASM caching in development
      'Cross-Origin-Opener-Policy': 'same-origin',
      'Cross-Origin-Embedder-Policy': 'require-corp',
    },
  },
  build: {
    target: 'esnext', // Modern browsers
    cssCodeSplit: true, // Ensure CSS is split
    sourcemap: false,
    modulePreload: {
      polyfill: true,
    },
    // Ensure WASM files are not cached in production builds
    rollupOptions: {
      output: {
        // Add hash to WASM files for cache busting
        assetFileNames: 'assets/[name].[hash][extname]',
        manualChunks: {
          'vendor-react': ['react', 'react-dom'],
          'vendor-ui': ['framer-motion'],
          'vendor-utils': ['zustand'],
        },
      },
    },
  },
  optimizeDeps: {
    exclude: ['kernel.wasm'],
  },
});
