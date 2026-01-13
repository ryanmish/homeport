import path from "path"
import { defineConfig, type LibraryFormats } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig(() => {
  // Check if we're building the VS Code share component
  const isVSCodeBuild = process.env.VSCODE_BUILD === 'true'

  return {
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        "@": path.resolve(__dirname, "./src"),
      },
    },
    server: {
      proxy: {
        '/api': 'http://localhost:8080',
      },
    },
    define: isVSCodeBuild ? {
      'process.env.NODE_ENV': JSON.stringify('production'),
    } : undefined,
    build: isVSCodeBuild ? {
      // VS Code share component build
      lib: {
        entry: path.resolve(__dirname, 'src/vscode-share.tsx'),
        name: 'HomeportShare',
        fileName: () => 'vscode-share.js',
        formats: ['iife'] as LibraryFormats[],
      },
      outDir: 'dist/vscode',
      emptyOutDir: true,
      cssCodeSplit: false,
      rollupOptions: {
        output: {
          assetFileNames: 'vscode-share.[ext]',
        },
      },
    } : undefined,
  }
})
