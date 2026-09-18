import './assets/base.css'
// === CUSTOM START: 视觉主题覆盖层 - By ASxiaowen ===
// 理由: 定制样式全部收在 custom_components/theme.css（规则2/规则5），
//       这里只加一行引入，且在 base.css 之后，保证覆盖 Tailwind 工具类。
import './custom_components/theme.css'
// === CUSTOM END: 视觉主题覆盖层 ===
import { createApp } from 'vue'
import { createPinia } from 'pinia'
// === CUSTOM START: 应用外壳（登录门 / 临时链接） - By ASxiaowen ===
// 理由: 「先登录再使用」属于入口层职责，用 RootShell 包裹原 App.vue，
//       App.vue 保持零改动（规范第 1 条）。实现见 custom_components/RootShell.vue。
import RootShell from './custom_components/RootShell.vue'
// === CUSTOM END: 应用外壳 ===
import { setupI18n } from './config/lang.js'
const app = createApp(RootShell)
app.use(setupI18n())
app.use(createPinia())
app.mount('#app')
