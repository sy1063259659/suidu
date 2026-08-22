import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createDiscreteApi } from 'naive-ui'

import App from './App.vue'
import './style.css'

const app = createApp(App)
app.use(createPinia())
app.provide('discrete-api', createDiscreteApi(['message']))
app.mount('#app')

