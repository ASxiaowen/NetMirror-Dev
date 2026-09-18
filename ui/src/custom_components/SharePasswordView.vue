<!--
  NetMirror 二次开发 · 临时链接的密码页
  目录: ui/src/custom_components/SharePasswordView.vue

  访客打开 /t/<id> 后看到的界面：先告诉他们这条链接给了什么（节点、可用功能、
  剩余有效期、备注），再要求输入对方单独发来的临时密码。

  为什么要先展示信息再要密码：
    链接与密码是分开两个渠道发的，对方可能同时收到多条链接。先把「这条链接
    绑的是哪台机器、能做什么」摆出来，能让他们立刻确认没拿错，而不是输完
    密码才发现进的是另一个机房。

  视觉沿用项目语言：浅灰底 + 光晕 + .bg-grid + .lg-card，与登录页同一套。
-->
<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useShare, toolLabel, formatDuration } from './useShare'
import { setShareToken } from './authState'

const props = defineProps({
  /** 链接标识（URL 里 /t/<id> 的 id） */
  linkId: { type: String, required: true },
  /** 由 RootShell 查好的链接信息 */
  info: { type: Object, default: () => ({}) }
})

const emit = defineEmits(['success', 'invalid'])

const { redeemLink } = useShare()

const password = ref('')
const passRef = ref(null)
const loading = ref(false)
const error = ref('')
const attempts = ref(0)

/** 剩余有效期文案，随 info.expiresAt 计算 */
const leftText = computed(() => {
  const left = props.info?.leftSecs
  if (left === undefined || left === null) {
    const exp = props.info?.expiresAt
    if (!exp) return ''
    return formatDuration(Math.max(0, exp - Math.floor(Date.now() / 1000)))
  }
  return formatDuration(left)
})

/** 工具白名单转成标签，最多展示 6 个，多的收成「等 N 项」 */
const toolLabels = computed(() => {
  const list = props.info?.tools || []
  if (list.includes('*')) return ['全部功能']
  return list.map(toolLabel)
})
const shownTools = computed(() => toolLabels.value.slice(0, 6))
const moreTools = computed(() => Math.max(0, toolLabels.value.length - 6))

/**
 * 被授权的节点名列表。
 * 单节点时与 nodeName 等价；多节点时逐台列出 —— 对方可以在输密码前
 * 就确认「这条链接覆盖的正是我要测的那几台」。
 */
const nodeNames = computed(() => {
  const list = props.info?.nodes || []
  return list.map((n) => n.name || n.id || n.url).filter(Boolean)
})

const submit = async () => {
  if (loading.value) return
  error.value = ''
  loading.value = true
  try {
    const r = await redeemLink(props.linkId, password.value)
    setShareToken(r.token, r.scope, props.linkId)
    emit('success')
  } catch (e) {
    const code = e?.payload?.code || e?.code || ''
    // 链接本身失效：交给外壳换成「链接不可用」卡片，这里不再留一个输密码的框
    if (['LINK_NOT_FOUND', 'LINK_REVOKED', 'LINK_EXPIRED'].includes(code)) {
      emit('invalid', e?.payload?.error || e?.message || '该临时链接不可用', code)
      return
    }
    error.value = e?.payload?.error || e?.message || '临时密码不正确'
    attempts.value += 1
    password.value = ''
    await nextTick()
    passRef.value?.focus()
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  passRef.value?.focus()
})
</script>

<template>
  <div
    class="relative flex min-h-screen items-center justify-center overflow-hidden bg-[#f6f7f9] px-4 py-8 dark:bg-[#0a0b0f]"
    style="min-height: 100vh; min-height: 100dvh"
  >
    <div class="pointer-events-none fixed inset-0 z-0 overflow-hidden" aria-hidden="true">
      <div
        class="absolute -top-48 left-1/2 h-[460px] w-[900px] -translate-x-1/2 rounded-full bg-gradient-to-br from-primary-300/35 via-sky-200/20 to-transparent blur-3xl dark:from-primary-500/12 dark:via-sky-500/6"
      ></div>
      <div class="absolute inset-0 bg-grid opacity-60 dark:opacity-[0.18]"></div>
    </div>

    <div class="relative z-10 w-full max-w-md">
      <div class="lg-card p-6 md:p-8 lg-rise">
        <!-- 品牌区 -->
        <div class="mb-5 flex items-center gap-3">
          <div
            class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-primary-500 to-primary-600 shadow-glow"
          >
            <svg class="h-5 w-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.8"
                d="M13.828 10.172a4 4 0 010 5.656l-3 3a4 4 0 01-5.656-5.656l1.5-1.5m7.328-2.672a4 4 0 00-5.656 0l-1.5 1.5m8.985 1.172l1.5-1.5a4 4 0 00-5.657-5.657l-3 3a4 4 0 000 5.657"
              ></path>
            </svg>
          </div>
          <div class="min-w-0">
            <h1 class="text-[17px] font-semibold leading-tight text-gray-900 dark:text-gray-100">
              临时测试链接
            </h1>
            <p class="mt-0.5 text-[12px] text-gray-500 dark:text-gray-400">
              请输入对方发来的临时密码以继续
            </p>
          </div>
        </div>

        <!-- 这条链接给了什么 -->
        <div
          class="mb-5 space-y-2.5 rounded-lg border border-gray-200/70 bg-gray-50/60 p-3.5 dark:border-white/[0.06] dark:bg-white/[0.02]"
        >
          <div class="flex items-baseline gap-2 text-[12px]">
            <span class="w-16 flex-shrink-0 text-gray-400 dark:text-gray-500">测试节点</span>
            <span class="min-w-0 font-medium text-gray-800 dark:text-gray-200">
              <template v-if="nodeNames.length">{{ nodeNames.join('、') }}</template>
              <template v-else>{{ info.nodeName || '—' }}</template>
              <span v-if="nodeNames.length > 1" class="ml-1 font-normal text-gray-400 dark:text-gray-500">
                （共 {{ nodeNames.length }} 台，进入后可切换）
              </span>
            </span>
          </div>
          <div v-if="info.note" class="flex items-baseline gap-2 text-[12px]">
            <span class="w-16 flex-shrink-0 text-gray-400 dark:text-gray-500">用途备注</span>
            <span class="min-w-0 text-gray-700 dark:text-gray-300">{{ info.note }}</span>
          </div>
          <div class="flex items-baseline gap-2 text-[12px]">
            <span class="w-16 flex-shrink-0 text-gray-400 dark:text-gray-500">可用功能</span>
            <span class="min-w-0 text-gray-700 dark:text-gray-300">
              <template v-if="shownTools.length">
                {{ shownTools.join('、')
                }}<template v-if="moreTools"> 等 {{ toolLabels.length }} 项</template>
              </template>
              <template v-else>—</template>
            </span>
          </div>
          <div v-if="leftText" class="flex items-baseline gap-2 text-[12px]">
            <span class="w-16 flex-shrink-0 text-gray-400 dark:text-gray-500">剩余有效</span>
            <span class="min-w-0 text-gray-700 dark:text-gray-300">{{ leftText }}</span>
          </div>
        </div>

        <form @submit.prevent="submit" class="space-y-3">
          <div>
            <label
              for="nm-share-password"
              class="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-gray-400 dark:text-gray-500"
            >
              临时密码
            </label>
            <input
              id="nm-share-password"
              ref="passRef"
              v-model="password"
              type="text"
              autocomplete="off"
              autocapitalize="off"
              autocorrect="off"
              spellcheck="false"
              :disabled="loading"
              placeholder="例如 a1b2-c3d4-e5f6-7890"
              class="w-full rounded-lg border border-gray-200 bg-white px-3 py-2.5 text-center font-mono text-[14px] tracking-wide text-gray-800 outline-none transition-colors placeholder:text-[12px] placeholder:tracking-normal placeholder:text-gray-400 focus:border-primary-400 focus:ring-2 focus:ring-primary-100 disabled:opacity-60 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-gray-100 dark:placeholder:text-gray-600 dark:focus:border-primary-500/50 dark:focus:ring-primary-500/10"
            />
          </div>

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
            :disabled="loading || !password"
            class="flex w-full items-center justify-center gap-2 rounded-lg bg-gradient-to-b from-primary-500 to-primary-600 px-4 py-2.5 text-[13px] font-medium text-white shadow-soft transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lift disabled:translate-y-0 disabled:opacity-50 disabled:shadow-none"
          >
            <div
              v-if="loading"
              class="h-3.5 w-3.5 animate-spin rounded-full border-2 border-white/40 border-t-white"
            ></div>
            <span>{{ loading ? '验证中…' : '进入' }}</span>
          </button>
        </form>

        <div
          class="mt-5 space-y-1.5 border-t border-gray-200/70 pt-4 text-[11px] leading-relaxed text-gray-400 dark:border-white/[0.06] dark:text-gray-500"
        >
          <p>临时密码由发送方的面板生成，与链接分开送达；大小写、横杠都不影响输入。</p>
          <p v-if="attempts >= 3">连续输错会被临时锁定，请先向发送方确认密码。</p>
          <p>忘记密码或链接已过期时，需要请发送方重新生成一条链接。</p>
        </div>
      </div>
    </div>
  </div>
</template>
