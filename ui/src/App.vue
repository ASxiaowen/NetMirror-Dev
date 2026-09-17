<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { list as langList, setI18nLanguage, loadLocaleMessages } from './config/lang.js'
import { useAppStore } from './stores/app'
import { useNodesStore } from './stores/nodes'
import { useNodeTool } from '@/composables/useNodeTool'
import LoadingCard from '@/components/Loading.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'
import LanguageSelector from '@/components/LanguageSelector.vue'
import Toast from '@/components/Toast.vue'
import AdminPanel from '@/components/Admin.vue'
import UtilitiesCard from '@/components/Utilities.vue'
import SpeedtestCard from '@/components/Speedtest.vue'
import TrafficCard from '@/components/TrafficDisplay.vue'
import NodeListCard from '@/components/Utilities/NodeList.vue'
// === CUSTOM START: 区块标题组件 - By ASxiaowen ===
// 理由: SectionTitle 是新增组件，按规则2 放在 custom_components/，原 components/ 目录不动。
import SectionTitle from '@/custom_components/SectionTitle.vue'
// === CUSTOM END: 区块标题组件 ===

const appStore = useAppStore()
const nodesStore = useNodesStore()
const { selectedNode } = useNodeTool()
const adminMode = ref(false)
const showFab = ref(false)

// === CUSTOM START: 节点会话状态透出 - By ASxiaowen ===
// 理由: 上游 App.vue 没有节点连接状态，切换节点失败时整页空白且无提示。
//       这里直接从 nodesStore 取状态（数据源：custom_components/useNodeSession.js），
//       用于顶部状态条与重试按钮；不重复调用 useNodeTool() 以免多注册一次清理钩子。
const sessionStatus = computed(() => nodesStore.sessionStatus)
const sessionError = computed(() => nodesStore.sessionError)
const retryNode = () => {
  if (selectedNode.value) nodesStore.selectNode(selectedNode.value)
}
// === CUSTOM END: 节点会话状态透出 ===

// Use store theme and language
const isDark = computed(() => appStore.theme === 'dark')
const currentLangCode = computed(() => appStore.language)

// === CUSTOM START: 服务器信息 chips - By ASxiaowen ===
// 理由: 上游这里是一行纯文本，切换节点拿不到 config 时整行消失；
//       改成 chips 结构并对 ASN 缺失显式标注“未知”，同时回落到 effectiveConfig。
const infoItems = computed(() => {
  const c = (selectedNode.value && selectedNode.value.config)
    ? selectedNode.value.config
    : (appStore.config || nodesStore.effectiveConfig)
  if (!c) return []
  const items = []
  if (c.public_ipv4) items.push({ label: 'IP', value: c.public_ipv4 })
  const asn = c.bgp || c.asn
  // ASN 拿不到时显式标出来，避免看起来像漏渲染
  items.push({ label: 'ASN', value: asn || '未知', muted: !asn })
  if (c.public_ipv6) items.push({ label: 'IPv6', value: c.public_ipv6 })
  if (c.my_ip) items.push({ label: 'Your IP', value: c.my_ip })
  return items
})
// === CUSTOM END: 服务器信息 chips ===

const currentLang = computed(() => {
  for (const lang of langList) {
    if (lang.value === currentLangCode.value) {
      return lang
    }
  }
  return null
})

const handleLangChange = async (newLang) => {
  appStore.setLanguage(newLang)
  await loadLocaleMessages(newLang)
  setI18nLanguage(newLang)
}

const toggleTheme = () => {
  const newTheme = appStore.theme === 'dark' ? 'light' : 'dark'
  appStore.setTheme(newTheme)
  applyThemeClass(newTheme === 'dark')
}

const applyThemeClass = (dark) => {
  if (dark) {
    document.documentElement.classList.add('dark')
    document.body.classList.add('dark')
    document.getElementById('app')?.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
    document.body.classList.remove('dark')
    document.getElementById('app')?.classList.remove('dark')
  }
}

const handleScroll = () => {
  showFab.value = window.scrollY > 200
}

const scrollToTop = () => {
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

const toggleAdminMode = () => {
  adminMode.value = !adminMode.value
  if (adminMode.value) window.scrollTo({ top: 0 })
}

const goBackToMain = () => {
  adminMode.value = false
}

// 设置页面标题（favicon由后端处理）
const updatePageInfo = () => {
  if (!appStore.config) return
  const title = appStore.config.app_title || appStore.config.location || 'Network Diagnostic Tools'
  if (document.title === 'Looking glass server') {
    document.title = title
  }
}

onMounted(async () => {
  await appStore.initialize()
  await loadLocaleMessages(appStore.language)
  setI18nLanguage(appStore.language)
  updatePageInfo()
  applyThemeClass(appStore.theme === 'dark')

  window.addEventListener('scroll', handleScroll, { passive: true })
})

watch(() => appStore.config, () => {
  updatePageInfo()
}, { deep: true })

onUnmounted(() => {
  window.removeEventListener('scroll', handleScroll)
})
</script>

<template>
  <div class="min-h-screen bg-[#f6f7f9] dark:bg-[#0a0b0f] transition-colors duration-300" style="min-height: 100vh; min-height: 100dvh;">
    <!-- === CUSTOM START: 背景装饰层 - By ASxiaowen === -->
    <!-- 理由: 顶部光晕 + 细网格，纯装饰，样式定义在 custom_components/theme.css (.bg-grid) -->
    <div class="pointer-events-none fixed inset-0 z-0 overflow-hidden" aria-hidden="true">
      <div class="absolute -top-48 left-1/2 h-[460px] w-[900px] -translate-x-1/2 rounded-full bg-gradient-to-br from-primary-300/35 via-sky-200/20 to-transparent blur-3xl dark:from-primary-500/12 dark:via-sky-500/6"></div>
      <div class="absolute inset-0 bg-grid opacity-60 dark:opacity-[0.18]"></div>
      <div class="absolute inset-x-0 top-0 h-64 bg-gradient-to-b from-white/70 to-transparent dark:from-white/[0.03]"></div>
    </div>
    <!-- === CUSTOM END: 背景装饰层 === -->

    <!-- Main container -->
    <div class="relative z-10 min-h-screen">
      <!-- Main content area -->
      <main class="pb-8">
        <!-- Admin Panel -->
        <AdminPanel v-if="adminMode" @back="goBackToMain" />

        <!-- Single scrolling page -->
        <template v-else>
          <LoadingCard v-if="appStore.connecting" />
          <template v-else>
            <!-- === CUSTOM START: 单页滚动布局与卡片视觉 - By ASxiaowen === -->
            <!-- 理由: 把节点/工具/测速/流量四个区块统一成 .lg-card 卡片（custom_components/theme.css），
                  并加 SectionTitle 与 .lg-rise 入场动画；原文件这里是各自独立的卡片结构。 -->
            <div class="max-w-6xl mx-auto space-y-5 md:space-y-6 px-4 pt-5 md:pt-8">
              <!-- Nodes + Server Info (merged into one card) -->
              <section id="section-nodes" data-section class="scroll-mt-24 lg-rise" style="animation-delay: .02s">
                <div class="lg-card p-4 md:p-6">
                  <SectionTitle title="Looking Glass Nodes" />
                  <NodeListCard />

                  <div id="section-info" data-section class="scroll-mt-24 mt-5 pt-4 border-t border-gray-200/70 dark:border-white/[0.06]">
                    <div class="flex flex-wrap gap-2">
                      <span
                        v-for="item in infoItems"
                        :key="item.label"
                        class="inline-flex items-center gap-1.5 rounded-md bg-gray-50 dark:bg-white/[0.04] px-2.5 py-1 ring-1 ring-inset ring-gray-200/70 dark:ring-white/[0.06]"
                      >
                        <span class="text-[10px] font-medium uppercase tracking-wider text-gray-400 dark:text-gray-500">{{ item.label }}</span>
                        <span
                          class="font-mono text-[12px] tabular-nums"
                          :class="item.muted ? 'text-gray-400 dark:text-gray-500' : 'text-gray-700 dark:text-gray-200'"
                        >{{ item.value }}</span>
                      </span>
                    </div>

                    <!-- === CUSTOM START: 节点连接状态提示 - By ASxiaowen === -->
                    <!-- 理由: 修复“切换节点后整页空白且无任何反馈”。connecting / error 两态 + 重试按钮，
                           状态数据来自 custom_components/useNodeSession.js -->
                    <div v-if="sessionStatus === 'connecting'" class="mt-3 inline-flex items-center gap-2 rounded-md bg-primary-50 dark:bg-primary-500/10 px-2.5 py-1.5 text-[12px] text-primary-700 dark:text-primary-300 ring-1 ring-inset ring-primary-100 dark:ring-primary-500/20">
                      <div class="w-3 h-3 border-2 border-primary-500 border-t-transparent rounded-full animate-spin"></div>
                      正在连接节点，工具与测速将在连接完成后可用…
                    </div>
                    <div v-else-if="sessionStatus === 'error'" class="mt-3 flex flex-wrap items-center justify-between gap-3 rounded-md bg-red-50 dark:bg-red-500/10 px-2.5 py-1.5 text-[12px] text-red-700 dark:text-red-300 ring-1 ring-inset ring-red-100 dark:ring-red-500/20">
                      <span>节点连接失败：{{ sessionError }}（该节点的工具/测速不可用）</span>
                      <button
                        @click="retryNode"
                        class="flex-shrink-0 rounded border border-red-200 dark:border-red-500/30 bg-white/70 dark:bg-white/[0.04] px-2 py-0.5 font-medium hover:bg-white dark:hover:bg-white/[0.08] transition-colors">
                        重试
                      </button>
                    </div>
                    <!-- === CUSTOM END: 节点连接状态提示 === -->
                  </div>
                </div>
              </section>

              <!-- Network Tools -->
              <section id="section-tools" data-section class="scroll-mt-24 lg-rise" style="animation-delay: .08s">
                <div class="lg-card p-4 md:p-6">
                  <SectionTitle :title="$t('network_tools')" />
                  <UtilitiesCard />
                </div>
              </section>

              <!-- Speed Test -->
              <section id="section-speedtest" data-section class="scroll-mt-24 lg-rise" style="animation-delay: .14s">
                <div class="lg-card p-4 md:p-6">
                  <SectionTitle :title="$t('server_speedtest')" />
                  <SpeedtestCard />
                </div>
              </section>

              <!-- Traffic Monitor -->
              <section v-if="appStore.config.feature_iface_traffic" id="section-traffic" data-section class="scroll-mt-24 lg-rise" style="animation-delay: .2s">
                <div class="lg-card p-4 md:p-6">
                  <SectionTitle :title="$t('server_bandwidth_graph')" />
                  <TrafficCard />
                </div>
              </section>
            </div>
            <!-- === CUSTOM END: 单页滚动布局与卡片视觉 === -->
          </template>
        </template>
      </main>

      <!-- Footer -->
      <!-- === CUSTOM START: 页脚避让右下角悬浮按钮 - By ASxiaowen === -->
      <!-- 理由: 修复“语言选择器点不动”——语言按钮中心被右下角 FAB 盖住，
             elementFromPoint 命中的是 FAB。这里给 footer 右侧留出 sm:pr-28 的避让区。 -->
      <footer class="pb-12 px-4 sm:pr-28">
        <div class="max-w-6xl mx-auto border-t border-gray-200/70 dark:border-white/[0.06] pt-5">
          <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
            <p class="text-[12px] text-gray-500 dark:text-gray-500">
              Powered by
              <a
                href="https://github.com/X-Zero-L/als"
                target="_blank"
                class="font-medium text-gray-700 dark:text-gray-300 underline-offset-4 hover:text-primary-600 dark:hover:text-primary-400 hover:underline transition-colors duration-200"
              >
                WIKIHOST Opensource - ALS (Github)
              </a>
            </p>
            <div class="w-44">
              <LanguageSelector :current-lang="currentLangCode" :lang-list="langList" @change="handleLangChange" />
            </div>
          </div>
          <p v-if="appStore.memoryUsage" class="text-[11px] text-gray-400 dark:text-gray-600 mt-3 tabular-nums">
            {{ $t('memory_usage') }}: {{ appStore.memoryUsage }}
          </p>
        </div>
      </footer>
      <!-- === CUSTOM END: 页脚避让右下角悬浮按钮 === -->
    </div>

    <!-- === CUSTOM START: 悬浮按钮视觉 - By ASxiaowen === -->
    <!-- 理由: 毛玻璃 + 阴影工具类（.shadow-soft/.shadow-lift 定义在 custom_components/theme.css），
           原文件这里是实心背景无 blur。 -->
    <!-- Admin Button (Always Visible) -->
    <button
      @click="toggleAdminMode"
      class="fixed bottom-8 right-8 z-50 w-11 h-11 flex items-center justify-center rounded-full border border-gray-200/80 dark:border-white/[0.08] bg-white/80 dark:bg-gray-800/70 text-gray-500 dark:text-gray-400 shadow-soft backdrop-blur-md transition-all duration-200 hover:-translate-y-0.5 hover:text-primary-600 dark:hover:text-primary-400 hover:shadow-lift"
      :class="adminMode ? 'ring-2 ring-primary-500/60 text-primary-600 dark:text-primary-400' : ''"
      title="Admin Panel"
    >
      <svg class="w-[18px] h-[18px]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path>
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
      </svg>
    </button>

    <!-- Floating Action Button Group (scroll to top, theme) -->
    <div class="fixed bottom-8 right-24 z-50">
      <transition
        enter-active-class="transition-all duration-300 ease-out"
        enter-from-class="opacity-0 translate-y-4"
        enter-to-class="opacity-100 translate-y-0"
        leave-active-class="transition-all duration-200 ease-in"
        leave-from-class="opacity-100 translate-y-0"
        leave-to-class="opacity-0 translate-y-4"
      >
        <div v-if="showFab" class="flex items-center space-x-2">
          <div class="w-11 h-11 flex items-center justify-center rounded-full border border-gray-200/80 dark:border-white/[0.08] bg-white/80 dark:bg-gray-800/70 shadow-soft backdrop-blur-md">
            <ThemeToggle :is-dark="isDark" @toggle="toggleTheme" />
          </div>
          <button @click="scrollToTop" class="w-11 h-11 flex items-center justify-center rounded-full border border-gray-200/80 dark:border-white/[0.08] bg-white/80 dark:bg-gray-800/70 text-gray-500 dark:text-gray-400 shadow-soft backdrop-blur-md transition-all duration-200 hover:-translate-y-0.5 hover:text-primary-600 dark:hover:text-primary-400 hover:shadow-lift">
            <svg class="w-[18px] h-[18px]" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7"></path></svg>
          </button>
        </div>
      </transition>
    </div>

    <!-- === CUSTOM END: 悬浮按钮视觉 === -->

    <!-- Toast Notifications -->
    <Toast />
  </div>
</template>

<style>
@import 'tailwindcss/base';
@import 'tailwindcss/components';
@import 'tailwindcss/utilities';

/* ---------- 基础排版 ---------- */
html {
  scroll-behavior: smooth;
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}

body {
  margin: 0;
  padding: 0;
  line-height: 1.6;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

/* Hide horizontal scrollbar on the sticky nav (mobile) */
.scrollbar-hide {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
.scrollbar-hide::-webkit-scrollbar {
  display: none;
}

@keyframes fadeIn {
  0% { opacity: 0; transform: translateY(20px); }
  100% { opacity: 1; transform: translateY(0); }
}

@keyframes slideUp {
  0% { opacity: 0; transform: translateY(30px); }
  100% { opacity: 1; transform: translateY(0); }
}

@keyframes scaleIn {
  0% { opacity: 0; transform: scale(0.9); }
  100% { opacity: 1; transform: scale(1); }
}

@keyframes pulse-slow {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 0.8; }
}

.animate-fade-in {
  animation: fadeIn 0.8s ease-out;
}

.animate-slide-up {
  animation: slideUp 0.6s ease-out both;
}

.animate-scale-in {
  animation: scaleIn 0.5s ease-out;
}

.animate-pulse-slow {
  animation: pulse-slow 4s ease-in-out infinite;
}

/* Custom scrollbar */
::-webkit-scrollbar {
  width: 8px;
}

::-webkit-scrollbar-track {
  background: transparent;
}

::-webkit-scrollbar-thumb {
  background: rgba(59, 130, 246, 0.3);
  border-radius: 4px;
}

::-webkit-scrollbar-thumb:hover {
  background: rgba(59, 130, 246, 0.5);
}

.dark ::-webkit-scrollbar-thumb {
  background: rgba(147, 197, 253, 0.3);
}

.dark ::-webkit-scrollbar-thumb:hover {
  background: rgba(147, 197, 253, 0.5);
}

.glass-effect {
  background: rgba(255, 255, 255, 0.6);
  backdrop-filter: blur(12px) saturate(180%);
  -webkit-backdrop-filter: blur(12px) saturate(180%);
}

.dark .glass-effect {
  background: rgba(31, 41, 55, 0.6);
}
</style>
