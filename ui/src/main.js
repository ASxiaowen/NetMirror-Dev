import './assets/base.css'
// === CUSTOM START: 视觉主题覆盖层 - By ASxiaowen ===
// 理由: 定制样式全部收在 custom_components/theme.css（规则2/规则5），
//       这里只加一行引入，且在 base.css 之后，保证覆盖 Tailwind 工具类。
import './custom_components/theme.css'
// === CUSTOM END: 视觉主题覆盖层 ===
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { setupI18n } from './config/lang.js'
const app = createApp(App)
app.use(setupI18n())
app.use(createPinia())
app.mount('#app')
