import {defineConfig} from 'vite';
import vue from '@vitejs/plugin-vue';

// The Vite project lives under web/, while the Express runtime serves the
// repository-level dist/ directory. Keeping the proxy here makes browser
// development use the same /api contract as the production client.
export default defineConfig({
    root: 'web',
    plugins: [vue()],
    server: {
        host: '0.0.0.0',
        port: 5173,
        proxy: {
            '/api': {
                target: 'http://localhost:4178',
                changeOrigin: true
            }
        }
    },
    build: {
        outDir: '../dist',
        emptyOutDir: true,
        manifest: true,
        assetsDir: 'assets',
        sourcemap: false,
        rollupOptions: {
            output: {
                entryFileNames: 'assets/[name]-[hash].js',
                chunkFileNames: 'assets/[name]-[hash].js',
                assetFileNames: 'assets/[name]-[hash][extname]'
            }
        }
    }
});
