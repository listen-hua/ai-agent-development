import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import App from './App.vue'
import router from './router'
import { setSessionRecoveryHandler } from './services/api'
import { useAuthStore } from './stores/auth'
import './styles/main.css'

const pinia = createPinia()
const auth = useAuthStore(pinia)

setSessionRecoveryHandler(async () => {
  try {
    await auth.recoverSession()
  } catch (error) {
    if (router.currentRoute.value.name !== 'login') {
      void router.replace({
        name: 'login',
        query: { redirect: router.currentRoute.value.fullPath },
      })
    }
    throw error
  }
})

createApp(App).use(pinia).use(router).use(ElementPlus).mount('#app')
