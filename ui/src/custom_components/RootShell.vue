<!--
  NetMirror 二次开发 · 应用外壳（多态）
  目录: ui/src/custom_components/RootShell.vue

  为什么需要外壳而不是改 App.vue：
    「打开面板先登录」这件事本质是路由/入口层的职责，塞进 App.vue 会让这个
    400 行的业务组件再多出一个正交的关注点。这里用一层外壳包裹 App.vue，
    App.vue 保持零改动（规范第 1 条）。

  状态：
    loading        启动校验中
    login          登录门已启用且无有效身份 → 账号 + 密码
    share-locked   打开了 /t/<id>，但还没输入临时密码
    app            已登录 / 未启用登录门 / 临时链接受限模式（以 banner + 锁定节点区分）
    invalid        临时链接不可用（不存在 / 过期 / 被吊销）

  两条身份链路互不干扰：
    · 自己人 → 账号+密码 → user 令牌（localStorage）
    · 访客   → 链接+临时密码 → share 令牌（sessionStorage，按链接 id 分键）
-->
<script setup>
import { ref, computed, onMounted } from 'vue'
import App from '@/App.vue'
import LoginView from './LoginView.vue'
import SharePasswordView from './SharePasswordView.vue'
import ShareBanner from './ShareBanner.vue'
import ShareAdminDialog from './ShareAdminDialog.vue'
import { useAuth } from './useAuth'
import { useShare } from './useShare'
import { onAuthExpired, ErrCode } from './apiClient'
import {
  authState,
  isShareMode,
  isLoggedIn,
  clearAuth,
  setShareToken,
  loadStoredShareToken,
  parseShareLinkFromUrl
} from './authState'

const { fetchConfig, bootstrapIdentity, verify, logout, error: authError } = useAuth()
const { linkInfo } = useShare()

/** loading | login | share-locked | app | invalid */
const state = ref('loading')
/** invalid 状态的说明文案与细分原因 */
const invalidReason = ref('')
const invalidCode = ref('')
/** share-locked 状态：当前链接标识与已查到的链接信息 */
const shareLinkId = ref('')
const shareInfo = ref({})

const showShareAdmin = ref(false)

// === CUSTOM START: 登录页也遵循已保存的主题 - By ASxiaowen ===
// 理由: 主题类原本只在 stores/app.js 初始化（即 App.vue 首次使用该 store）时写入
//       documentElement；登录页/密码页/失效页都不渲染 App.vue，所以会一直停在
//       浅色，与用户已选的主题不一致。这里在外壳入口补一次同样的逻辑。
//       只读 localStorage，不写回，避免和 store 形成两处真相。
function applyStoredTheme() {
  try {
    const dark = (localStorage.getItem('theme') || 'light') === 'dark'
    document.documentElement.classList.toggle('dark', dark)
    document.body.classList.toggle('dark', dark)
    document.getElementById('app')?.classList.toggle('dark', dark)
  } catch (e) {
    // localStorage 不可用时忽略，按浅色渲染
  }
}
applyStoredTheme()
// === CUSTOM END: 登录页也遵循已保存的主题 ===

const shareMode = computed(() => isShareMode())
const loggedIn = computed(() => isLoggedIn())
/** 只在「登录门已启用 + 已登录」时提供临时链接入口（访客不该看到） */
const canManageShares = computed(
  () => state.value === 'app' && loggedIn.value && authState.enabled
)

/** 把某条链接标成不可用并切到提示卡片 */
const markInvalid = (reason, code) => {
  invalidReason.value = reason || '该临时链接无效或已被吊销。'
  invalidCode.value = code || 'LINK_INVALID'
  clearAuth()
  state.value = 'invalid'
}

/**
 * 处理 URL 上的临时链接：先看本次会话是否已兑换过，避免刷新后重复输密码。
 * @returns {Promise<boolean>} 是否已进入 app 状态
 */
const bootShareLink = async (linkId) => {
  shareLinkId.value = linkId

  // 1) 本次会话已兑换过 → 用令牌换回作用域。
  //    作用域（绑定节点、工具白名单）是从令牌载荷里读出来的，不依赖链接信息接口。
  const cached = loadStoredShareToken(linkId)
  if (cached) {
    setShareToken(cached, null, linkId)
    const r = await verify()
    if (r.valid && r.scope) {
      // 链接信息只用于展示；取不到也不该拦住进入，后续请求会给出准确错误
      try {
        shareInfo.value = await linkInfo(linkId)
      } catch (e) {
        shareInfo.value = {}
      }
      setShareToken(cached, r.scope, linkId)
      state.value = 'app'
      return true
    }
    // 缓存的令牌已失效（过期/被吊销），清掉后回落到输密码
    clearAuth()
  }

  // 2) 查链接信息（公开接口，不需要密码），顺带确认链接是否还存在/已过期
  try {
    shareInfo.value = await linkInfo(linkId)
    state.value = 'share-locked'
    return false
  } catch (e) {
    markInvalid(
      e?.payload?.error || e?.message || '该临时链接不可用',
      e?.payload?.code || 'LINK_INVALID'
    )
    return false
  }
}

/** 启动流程：探测配置 → 处理 URL 上的临时链接 → 回落到本地登录令牌 */
const boot = async () => {
  const linkId = parseShareLinkFromUrl()

  await fetchConfig()

  // 临时链接优先于既有登录态：打开分享链接是明确的意图，不该被上一个管理员的
  // 登录状态覆盖（否则访客会以管理员身份看到整站，权限模型就废了）。
  if (linkId) {
    await bootShareLink(linkId)
    authState.checked = true
    return
  }

  // 登录门未启用：不做任何拦截，按原行为直接进主界面
  if (!authState.enabled) {
    state.value = 'app'
    authState.checked = true
    return
  }

  const identity = await bootstrapIdentity()
  authState.checked = true
  state.value = identity === 'user' ? 'app' : 'login'
}

const onLoginSuccess = () => {
  state.value = 'app'
  invalidCode.value = ''
  invalidReason.value = ''
}

/** 临时密码兑换成功 */
const onRedeemSuccess = () => {
  state.value = 'app'
  invalidCode.value = ''
  invalidReason.value = ''
}

/** 密码页发现链接本身已失效（不存在/过期/被吊销） */
const onRedeemInvalid = (reason, code) => {
  markInvalid(reason, code)
}

const doLogout = async () => {
  await logout()
  state.value = 'login'
}

/** 临时链接在倒计时归零时触发 */
const onLinkExpired = () => {
  if (isShareMode()) {
    markInvalid('该临时链接已过期，请向发送方索取新的链接', 'LINK_EXPIRED')
  }
}

onMounted(() => {
  // 令牌在任意请求上失效（过期/被吊销）时的统一兜底
  onAuthExpired((code) => {
    if (state.value !== 'app') return
    if (isShareMode()) {
      markInvalid(
        code === ErrCode.FORBIDDEN
          ? '该临时链接无权访问该功能'
          : '该临时链接已失效，请向发送方索取新的链接',
        code || ErrCode.TOKEN_EXPIRED
      )
      return
    }
    clearAuth()
    state.value = 'login'
  })

  boot()
})
</script>

<template>
  <!-- 启动校验 -->
  <div
    v-if="state === 'loading'"
    class="flex min-h-screen items-center justify-center bg-[#f6f7f9] dark:bg-[#0a0b0f]"
    style="min-height: 100vh; min-height: 100dvh"
  >
    <div class="flex flex-col items-center gap-3">
      <div class="h-7 w-7 animate-spin rounded-full border-[3px] border-primary-500 border-t-transparent"></div>
      <p class="text-[12px] text-gray-400 dark:text-gray-500">正在校验访问权限…</p>
    </div>
  </div>

  <!-- 登录（账号 + 密码） -->
  <LoginView v-else-if="state === 'login'" @success="onLoginSuccess" />

  <!-- 临时链接：输入临时密码 -->
  <SharePasswordView
    v-else-if="state === 'share-locked'"
    :link-id="shareLinkId"
    :info="shareInfo"
    @success="onRedeemSuccess"
    @invalid="onRedeemInvalid"
  />

  <!-- 临时链接不可用 -->
  <div
    v-else-if="state === 'invalid'"
    class="relative flex min-h-screen items-center justify-center overflow-hidden bg-[#f6f7f9] px-4 dark:bg-[#0a0b0f]"
    style="min-height: 100vh; min-height: 100dvh"
  >
    <div class="pointer-events-none fixed inset-0 z-0 overflow-hidden" aria-hidden="true">
      <div class="absolute inset-0 bg-grid opacity-60 dark:opacity-[0.18]"></div>
    </div>

    <div class="lg-card relative z-10 w-full max-w-sm p-6 text-center md:p-8 lg-rise">
      <div
        class="mx-auto mb-4 flex h-11 w-11 items-center justify-center rounded-full bg-amber-50 text-amber-600 ring-1 ring-inset ring-amber-200 dark:bg-amber-500/10 dark:text-amber-400 dark:ring-amber-500/20"
      >
        <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.8"
            d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          ></path>
        </svg>
      </div>
      <h1 class="text-[16px] font-semibold text-gray-900 dark:text-gray-100">
        {{ invalidCode === 'LINK_EXPIRED' || invalidCode === 'TOKEN_EXPIRED' ? '链接已过期' : '链接不可用' }}
      </h1>
      <p class="mt-2 text-[12px] leading-relaxed text-gray-500 dark:text-gray-400">
        {{ invalidReason || '该临时链接无效或已被吊销。' }}
      </p>
      <a
        href="/"
        class="mt-5 inline-flex items-center justify-center rounded-lg bg-gradient-to-b from-primary-500 to-primary-600 px-4 py-2 text-[13px] font-medium text-white shadow-soft transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lift"
      >
        前往主页登录
      </a>
    </div>
  </div>

  <!-- 主应用（含临时链接受限模式） -->
  <template v-else>
    <div :class="{ 'nm-restricted-mode': shareMode }">
      <ShareBanner v-if="shareMode" @expired="onLinkExpired" />
      <App />
    </div>

    <!-- 临时链接管理入口：仅在登录后可用的左下角悬浮按钮（不动 App.vue / Admin.vue） -->
    <template v-if="canManageShares">
      <button
        class="fixed bottom-8 left-8 z-50 flex h-11 items-center gap-2 rounded-full border border-gray-200/80 bg-white/80 px-4 text-[12px] font-medium text-gray-600 shadow-soft backdrop-blur-md transition-all duration-200 hover:-translate-y-0.5 hover:text-primary-600 hover:shadow-lift dark:border-white/[0.08] dark:bg-gray-800/70 dark:text-gray-300 dark:hover:text-primary-400"
        title="生成临时测试链接"
        @click="showShareAdmin = true"
      >
        <svg class="h-[15px] w-[15px]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.8"
            d="M13.828 10.172a4 4 0 010 5.656l-3 3a4 4 0 01-5.656-5.656l1.5-1.5m7.328-2.672a4 4 0 00-5.656 0l-1.5 1.5m8.985 1.172l1.5-1.5a4 4 0 00-5.657-5.657l-3 3a4 4 0 000 5.657"
          ></path>
        </svg>
        临时链接
      </button>

      <button
        class="fixed bottom-8 left-40 z-50 flex h-11 items-center gap-2 rounded-full border border-gray-200/80 bg-white/80 px-3.5 text-[12px] font-medium text-gray-500 shadow-soft backdrop-blur-md transition-all duration-200 hover:-translate-y-0.5 hover:text-red-500 hover:shadow-lift dark:border-white/[0.08] dark:bg-gray-800/70 dark:text-gray-400 dark:hover:text-red-400"
        title="退出登录"
        @click="doLogout"
      >
        <svg class="h-[15px] w-[15px]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.8"
            d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
          ></path>
        </svg>
        退出
      </button>

      <ShareAdminDialog v-if="showShareAdmin" @close="showShareAdmin = false" />
    </template>
  </template>
</template>
