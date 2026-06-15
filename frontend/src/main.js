import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import 'leaflet/dist/leaflet.css'
import './assets/main.css'
import './assets/layout.css'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)

app.mount('#app')
