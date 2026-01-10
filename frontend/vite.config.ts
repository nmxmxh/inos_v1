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
    // Custom plugin to serve .br files correctly in dev mode
    {
      name: 'serve-brotli-wasm',
      configureServer(server) {
        server.middlewares.use((req, res, next) => {
          const url = (req as any).url || '';
          if (url.endsWith('.wasm.br')) {
            res.setHeader('Content-Type', 'application/wasm');
            res.setHeader('Content-Encoding', 'br');
            res.setHeader('Cache-Control', 'no-cache, no-store, must-revalidate');
          }
          // Enforce COOP/COEP for all responses in dev
          res.setHeader('Cross-Origin-Opener-Policy', 'same-origin');
          res.setHeader('Cross-Origin-Embedder-Policy', 'require-corp');
          next();
        });
      },
    },
  ],
  server: {
    port: 5173,
    host: true, // Allow external access
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
          'vendor-react': ['react', 'react-dom', 'react-router-dom'],
          'vendor-three': ['three', '@react-three/fiber', '@react-three/drei'],
          'vendor-d3': ['d3'],
          'vendor-ui': ['framer-motion', 'styled-components'],
          'vendor-utils': ['zustand', 'react-use'],
        },
      },
    },
  },
  optimizeDeps: {
    exclude: ['kernel.wasm'],
  },
});
