import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import router from './router'
import { i18n } from './i18n'

import './assets/fonts.css'
import './vendor/oat/oat.css'
import './vendor/oat/js/index.js'
import './assets/layout.css'
import './assets/index.css'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(i18n)

app.mount('#app')
