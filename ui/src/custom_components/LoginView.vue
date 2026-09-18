<!--
  NetMirror 二次开发 · 登录页
  目录: ui/src/custom_components/LoginView.vue

  视觉沿用项目既有语言：浅灰底 + 顶部光晕 + 细网格（.bg-grid）+ .lg-card 卡片，
  与主界面保持同一套设计变量，不做第二套风格。
-->
<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { useAuth } from './useAuth'

const emit = defineEmits(['success'])

const { login, loading, error, config } = useAuth()

const username = ref('')
const password = ref('')
const userRef = ref(null)
const passRef = ref(null)
/** 连续失败次数，达到阈值后给出更明确的引导（减少反复试错的挫败感） */
const attempts = ref(0)

const submit = async () => {
  if (loading.value) return
  const ok = await login(username.value.trim(), password.value)
  if (ok) {
    attempts.value = 0
    emit('success')
    return
  }
  attempts.value += 1
  // 只清密码，保留账号 —— 手误多半出在密码，重输账号很烦
  password.value = ''
  await nextTick()
  passRef.value?.focus()
}

onMounted(() => {
  // 有账号时直接聚焦密码框，省一次点击
  if (username.value) passRef.value?.focus()
  else userRef.value?.focus()
})
</script>

<template>
  <div
    class="relative flex min-h-screen items-center justify-center overflow-hidden bg-[#f6f7f9] px-4 dark:bg-[#0a0b0f]"
    style="min-height: 100vh; min-height: 100dvh"
  >
    <!-- 背景装饰：与主界面 App.vue 的装饰层保持一致 -->
    <div class="pointer-events-none fixed inset-0 z-0 overflow-hidden" aria-hidden="true">
      <div
        class="absolute -top-48 left-1/2 h-[460px] w-[900px] -translate-x-1/2 rounded-full bg-gradient-to-br from-primary-300/35 via-sky-200/20 to-transparent blur-3xl dark:from-primary-500/12 dark:via-sky-500/6"
      ></div>
      <div class="absolute inset-0 bg-grid opacity-60 dark:opacity-[0.18]"></div>
    </div>

    <div class="relative z-10 w-full max-w-sm">
      <div class="lg-card p-6 md:p-8 lg-rise">
        <!-- 品牌区 -->
        <div class="mb-6 flex items-center gap-3">
          <div
            class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-primary-500 to-primary-600 shadow-glow"
          >
            <svg class="h-5 w-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.8"
                d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
              ></path>
            </svg>
          </div>
          <div class="min-w-0">
            <h1 class="text-[17px] font-semibold leading-tight text-gray-900 dark:text-gray-100">
              访问受限
            </h1>
            <p class="mt-0.5 text-[12px] text-gray-500 dark:text-gray-400">
              请登录以继续使用面板
            </p>
          </div>
        </div>

        <form @submit.prevent="submit" class="space-y-3">
          <div>
            <label
              for="nm-panel-username"
              class="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-gray-400 dark:text-gray-500"
            >
              账号
            </label>
            <input
              id="nm-panel-username"
              ref="userRef"
              v-model="username"
              type="text"
              autocomplete="username"
              autocapitalize="off"
              autocorrect="off"
              spellcheck="false"
              :disabled="loading"
              placeholder="请输入账号"
              class="w-full rounded-lg border border-gray-200 bg-white px-3 py-2.5 text-[13px] text-gray-800 outline-none transition-colors placeholder:text-gray-400 focus:border-primary-400 focus:ring-2 focus:ring-primary-100 disabled:opacity-60 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-gray-100 dark:placeholder:text-gray-600 dark:focus:border-primary-500/50 dark:focus:ring-primary-500/10"
            />
          </div>

          <div>
            <label
              for="nm-panel-password"
              class="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-gray-400 dark:text-gray-500"
            >
              密码
            </label>
            <input
              id="nm-panel-password"
              ref="passRef"
              v-model="password"
              type="password"
              autocomplete="current-password"
              :disabled="loading"
              placeholder="请输入密码"
              class="w-full rounded-lg border border-gray-200 bg-white px-3 py-2.5 font-mono text-[13px] text-gray-800 outline-none transition-colors placeholder:font-sans placeholder:text-gray-400 focus:border-primary-400 focus:ring-2 focus:ring-primary-100 disabled:opacity-60 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-gray-100 dark:placeholder:text-gray-600 dark:focus:border-primary-500/50 dark:focus:ring-primary-500/10"
            />
          </div>

          <!-- 错误提示：不换行抖动，只做颜色与文案，避免过度动效 -->
          <p
            v-if="error"
            class="flex items-start gap-1.5 rounded-md bg-red-50 px-2.5 py-2 text-[12px] text-red-700 ring-1 ring-inset ring-red-100 dark:bg-red-500/10 dark:text-red-300 dark:ring-red-500/20"
          >
            <svg class="mt-px h-3.5 w-3.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"
              ></path>
            </svg>
            <span>{{ error }}</span>
          </p>

          <button
            type="submit"
            :disabled="loading || !username || !password"
            class="flex w-full items-center justify-center gap-2 rounded-lg bg-gradient-to-b from-primary-500 to-primary-600 px-4 py-2.5 text-[13px] font-medium text-white shadow-soft transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lift disabled:translate-y-0 disabled:opacity-50 disabled:shadow-none"
          >
            <div
              v-if="loading"
              class="h-3.5 w-3.5 animate-spin rounded-full border-2 border-white/40 border-t-white"
            ></div>
            <span>{{ loading ? '验证中…' : '登录' }}</span>
          </button>
        </form>

        <!-- 辅助信息：有效期 + 临时链接说明 -->
        <div
          class="mt-5 space-y-1.5 border-t border-gray-200/70 pt-4 text-[11px] leading-relaxed text-gray-400 dark:border-white/[0.06] dark:text-gray-500"
        >
          <p>登录状态有效期约 {{ Math.round((config.tokenTtlHours || 168) / 24) }} 天。</p>
          <p v-if="attempts >= 3">
            账号由 PANEL_USER、密码由 PANEL_PASSWORD 环境变量设定，遗忘请联系管理员重置。
          </p>
          <p>如果收到的是临时测试链接，直接打开该链接并输入对方给你的临时密码即可，无需在此登录。</p>
        </div>
      </div>
    </div>
  </div>
</template>
