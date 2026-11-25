import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import router from './router'
import { i18n, initLocale } from './i18n'

import './assets/fonts.css'
// Oat UI from the pinned npm package (@knadh/oat); bundled by Vite, so it's still self-hosted.
import '@knadh/oat/oat.min.css'
import '@knadh/oat/js/index.js'
import './assets/theme.css'
import './assets/layout.css'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(i18n)

await initLocale()
app.mount('#app')
