<!--
  NetMirror 二次开发 · Node Management 管理页扩展
  目录: ui/src/custom_components/AdminExtras.vue

  挂载方式：在 ui/src/components/Admin.vue 的「已认证」区域插入一行 <AdminExtras />，
  并用 CUSTOM START/END 包裹（规范第 2 条入口隔离 / 第 3 条显式标记）。
  本文件是唯一实现处，上游文件只多两行。

  含两块控制：
    ① 登录凭据 —— 改面板访问的账号与密码，改完立即生效、无需重启服务
    ② 临时链接 —— 生成「链接 + 临时密码」发给客户/同事自测（复用 SharePanel，
                   与主面板右下角那个弹窗共用同一份实现，避免两处逻辑漂移）

  视觉：沿用本页既有的毛玻璃卡片风格（bg-white/90 + backdrop-blur + rounded-2xl），
       深浅主题都跟随 Admin.vue。
-->

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useCredentials } from './useCredentials'
import SharePanel from './SharePanel.vue'

const { loading, saving, error, info, load, save } = useCredentials()

/** 当前密码是否明文显示 */
const showPwd = ref(false)
const notice = ref('')
const saveError = ref('')

const form = ref({
  username: '',
  currentPassword: '',
  password: '',
  confirm: ''
})

/** 至少要改动一项，且必须填当前密码 */
const willChangeUser = computed(
  () => form.value.username.trim() !== '' && form.value.username.trim() !== info.value.username
)
const willChangePass = computed(() => !!form.value.password)

const canSubmit = computed(
  () => !saving.value && !!form.value.currentPassword && (willChangeUser.value || willChangePass.value)
)

const sourceText = computed(() =>
  info.value.source === 'runtime'
    ? '当前值由本页保存（服务器上的 custom_auth.json）'
    : '当前值来自环境变量 PANEL_USER / PANEL_PASSWORD'
)

const resetForm = () => {
  form.value.username = info.value.username
  form.value.currentPassword = ''
  form.value.password = ''
  form.value.confirm = ''
}

const submit = async () => {
  saveError.value = ''
  notice.value = ''

  if (!form.value.currentPassword) {
    saveError.value = '请输入当前密码以确认身份'
    return
  }
  const min = info.value.minPassword || 8
  if (form.value.password && form.value.password.length < min) {
    saveError.value = `新密码至少 ${min} 位`
    return
  }
  if (form.value.password && form.value.password !== form.value.confirm) {
    saveError.value = '两次输入的新密码不一致'
    return
  }

  try {
    const d = await save({
      currentPassword: form.value.currentPassword,
      username: form.value.username.trim(),
      password: form.value.password
    })
    const labels = (d?.changed || []).map((x) => (x === 'username' ? '账号' : '密码'))
    notice.value = labels.length
      ? `已保存并立即生效（${labels.join(' + ')}）。`
      : '内容与当前一致，未做修改。'
    // 清空全部敏感字段，再回读一次确保界面显示的是真正生效的值
    await load()
    resetForm()
  } catch (e) {
    saveError.value = e?.message || '保存失败'
  }
}

onMounted(async () => {
  try {
    await load()
  } catch (e) {
    /* error 已由 useCredentials 填好，界面直接展示 */
  }
  resetForm()
})
</script>

<template>
  <div class="space-y-6">
    <!-- ============ ① 登录凭据 ============ -->
    <div
      class="bg-white/90 dark:bg-gray-800/90 backdrop-blur-lg rounded-2xl shadow-lg border border-primary-200/30 dark:border-primary-700/30 p-6 animate-slide-up"
      style="animation-delay: 0.42s;"
    >
      <div class="flex items-start justify-between gap-4 mb-6">
        <div class="flex items-center space-x-3">
          <div class="w-10 h-10 bg-gradient-to-br from-blue-500 to-blue-600 rounded-xl shadow-lg shadow-blue-500/25 flex items-center justify-center flex-shrink-0">
            <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"></path>
            </svg>
          </div>
          <div>
            <h3 class="text-lg font-bold text-gray-900 dark:text-gray-100">登录凭据</h3>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              修改面板访问的账号与密码 · 改完立即生效，无需重启服务
            </p>
          </div>
        </div>
      </div>

      <div v-if="loading" class="py-8 text-center text-sm text-gray-400 dark:text-gray-500">读取中…</div>

      <template v-else>
        <div class="grid gap-4 sm:grid-cols-2 max-w-2xl">
          <!-- 账号 -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              账号
            </label>
            <input
              v-model="form.username"
              type="text"
              autocomplete="off"
              class="w-full px-4 py-2.5 border border-gray-300/50 dark:border-gray-600/50 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white/80 dark:bg-gray-700/80 backdrop-blur-sm text-gray-900 dark:text-gray-100 transition-all duration-200 placeholder-gray-400 dark:placeholder-gray-500 text-sm"
              placeholder="登录账号"
            />
          </div>

          <!-- 当前密码（必填，用于确认身份） -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              当前密码<span class="ml-1 text-xs font-normal text-red-500">必填</span>
            </label>
            <div class="relative">
              <input
                v-model="form.currentPassword"
                type="password"
                autocomplete="current-password"
                class="w-full px-4 py-2.5 pr-11 border border-gray-300/50 dark:border-gray-600/50 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white/80 dark:bg-gray-700/80 backdrop-blur-sm text-gray-900 dark:text-gray-100 transition-all duration-200 placeholder-gray-400 dark:placeholder-gray-500 text-sm"
                placeholder="确认身份用"
              />
            </div>
          </div>

          <!-- 新密码 -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              新密码<span class="ml-1 text-xs font-normal text-gray-400 dark:text-gray-500">留空则不修改</span>
            </label>
            <input
              v-model="form.password"
              type="password"
              autocomplete="new-password"
              class="w-full px-4 py-2.5 border border-gray-300/50 dark:border-gray-600/50 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white/80 dark:bg-gray-700/80 backdrop-blur-sm text-gray-900 dark:text-gray-100 transition-all duration-200 placeholder-gray-400 dark:placeholder-gray-500 text-sm"
              :placeholder="`至少 ${info.minPassword || 8} 位`"
            />
          </div>

          <!-- 确认新密码 -->
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
              确认新密码
            </label>
            <input
              v-model="form.confirm"
              type="password"
              autocomplete="new-password"
              :disabled="!form.password"
              class="w-full px-4 py-2.5 border border-gray-300/50 dark:border-gray-600/50 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-primary-500 bg-white/80 dark:bg-gray-700/80 backdrop-blur-sm text-gray-900 dark:text-gray-100 transition-all duration-200 placeholder-gray-400 dark:placeholder-gray-500 text-sm disabled:opacity-50"
              placeholder="再输一次"
            />
          </div>
        </div>

        <!-- 当前生效值（只读回显，便于确认自己没记错） -->
        <div class="mt-5 rounded-xl bg-gray-50/80 dark:bg-gray-700/40 border border-gray-200/60 dark:border-gray-600/40 px-4 py-3">
          <div class="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-600 dark:text-gray-400">
            <span class="font-medium text-gray-700 dark:text-gray-300">当前生效</span>
            <span class="font-mono text-gray-800 dark:text-gray-200">{{ info.username || '—' }}</span>
            <span class="text-gray-300 dark:text-gray-600">/</span>
            <span class="font-mono text-gray-800 dark:text-gray-200">
              {{ showPwd ? (info.password || '—') : '•'.repeat(Math.min((info.password || '').length || 8, 16)) }}
            </span>
            <button
              type="button"
              class="ml-1 inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-gray-500 hover:bg-gray-200/70 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-white/[0.08] dark:hover:text-gray-200 transition-colors"
              @click="showPwd = !showPwd"
            >
              <svg v-if="showPwd" class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                      d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.878 9.878L3 3m6.878 6.878L21 21"></path>
              </svg>
              <svg v-else class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                      d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"></path>
              </svg>
              {{ showPwd ? '隐藏' : '显示' }}
            </button>
          </div>
          <p class="mt-1.5 text-[11px] leading-relaxed text-gray-400 dark:text-gray-500">
            {{ sourceText }}<template v-if="info.filePath">（{{ info.filePath }}）</template>。
            删除该文件即可恢复为环境变量里的初始值。
          </p>
        </div>

        <!-- 提示 -->
        <p
          v-if="saveError"
          class="mt-4 rounded-xl bg-red-50 px-3 py-2 text-sm text-red-700 ring-1 ring-inset ring-red-100 dark:bg-red-500/10 dark:text-red-300 dark:ring-red-500/20"
        >
          {{ saveError }}
        </p>
        <p
          v-else-if="notice"
          class="mt-4 rounded-xl bg-emerald-50 px-3 py-2 text-sm text-emerald-700 ring-1 ring-inset ring-emerald-100 dark:bg-emerald-500/10 dark:text-emerald-300 dark:ring-emerald-500/20"
        >
          {{ notice }}
        </p>

        <div class="mt-5 flex flex-wrap items-center justify-between gap-3">
          <p class="text-[11px] leading-relaxed text-gray-400 dark:text-gray-500 max-w-lg">
            改密码不会让已签发的登录令牌立即失效 —— 它们在到期前仍然有效。需要立刻踢下线时，
            请一并在服务器上重启服务或更换 AUTH_SECRET。
          </p>
          <button
            :disabled="!canSubmit"
            class="inline-flex items-center px-5 py-2.5 bg-gradient-to-r from-blue-600 to-blue-700 text-white rounded-xl hover:from-blue-700 hover:to-blue-800 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200 transform hover:scale-105 shadow-lg shadow-blue-500/25 text-sm font-medium"
            @click="submit"
          >
            <svg v-if="saving" class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <svg v-else class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
            </svg>
            {{ saving ? '保存中…' : '保存' }}
          </button>
        </div>
      </template>
    </div>

    <!-- ============ ② 临时测试链接 ============ -->
    <div
      class="bg-white/90 dark:bg-gray-800/90 backdrop-blur-lg rounded-2xl shadow-lg border border-purple-200/30 dark:border-purple-700/30 p-6 animate-slide-up"
      style="animation-delay: 0.47s;"
    >
      <div class="flex items-center space-x-3 mb-6">
        <div class="w-10 h-10 bg-gradient-to-r from-purple-500 to-purple-600 rounded-xl shadow-lg shadow-purple-500/25 flex items-center justify-center flex-shrink-0">
          <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"></path>
          </svg>
        </div>
        <div>
          <h3 class="text-lg font-bold text-gray-900 dark:text-gray-100">临时测试链接</h3>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            生成「链接 + 临时密码」，限定节点与可用功能，到期自动失效，也可随时吊销
          </p>
        </div>
      </div>

      <SharePanel />
    </div>
  </div>
</template>
