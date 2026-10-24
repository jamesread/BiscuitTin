import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// The dev server proxies the ConnectRPC API to a running biscuittin with web
// control enabled, on $PORT if set.
const backend = `http://localhost:${process.env.PORT || 8137}`

export default defineConfig({
	plugins: [vue()],
	// Embedded in the Go binary by desktop/internal/webcontrol, which can only
	// embed files in its own directory.
	build: {
		outDir: '../desktop/internal/webcontrol/webdist',
		emptyOutDir: true,
	},
	resolve: {
		dedupe: ['vue'],
	},
	server: {
		proxy: {
			'/biscuittin.v1.RemoteService': { target: backend, changeOrigin: true },
		},
	},
})
