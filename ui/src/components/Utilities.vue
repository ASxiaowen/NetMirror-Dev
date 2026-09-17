<script setup>
import { ref, computed, defineAsyncComponent, onMounted, h, shallowRef, toRaw } from 'vue'
import { storeToRefs } from 'pinia'
import { useAppStore } from '@/stores/app'
import { useNodesStore } from '@/stores/nodes'
import { useNodeTool } from '@/composables/useNodeTool'
// === CUSTOM START: 工具图标与按钮网格引入 - By ASxiaowen ===
// 理由: 图标路径集中在 custom_components/toolIcons.js，按钮网格抽成 custom_components/ToolGrid.vue，
//       原文件只保留一行 <ToolGrid>，避免在原文件里改动 9 个工具定义。
import { toolIcon } from '@/custom_components/toolIcons'
import ToolGrid from '@/custom_components/ToolGrid.vue'
// === CUSTOM END: 工具图标与按钮网格引入 ===

const appStore = useAppStore()
const nodesStore = useNodesStore()
const {
  selectedNode,
} = useNodeTool()

const config = computed(() => {
  // === CUSTOM START: 配置兜底 - By ASxiaowen ===
  // 理由: 切换节点期间 selectedNode.config 还没到，回落到 effectiveConfig，避免工具区整块消失
  if (selectedNode.value && selectedNode.value.config) {
    return selectedNode.value.config
  }
  return nodesStore.effectiveConfig || appStore.config
  // === CUSTOM END: 配置兜底 ===
})

const toolComponent = shallowRef(null)
const currentTool = ref(null)

const toolComponentShow = computed({
  get() {
    return currentTool.value !== null
  },
  set(newValue) {
    if (!newValue) {
      currentTool.value = null
      toolComponent.value = null
    }
  }
})

const tools = ref([
  {
    id: 'ping',
    label: 'Ping',
    description: 'IPv4 connectivity test',
    color: 'from-primary-500 to-primary-600',
    featureFlag: 'feature_ping',
    componentNode: defineAsyncComponent({
      loader: () => import('./Utilities/Ping.vue'),
      delay: 200,
      timeout: 10000,
      loadingComponent: () => h('div', { class: 'flex items-center justify-center p-8' }, [
        h('div', { class: 'w-8 h-8 border-4 border-primary-500 border-t-transparent rounded-full animate-spin' })
      ]),
      errorComponent: () => h('div', { class: 'text-red-500 p-4' }, 'Failed to load component')
    })
  },
  {
    id: 'ping6',
    label: 'Ping IPv6',
    description: 'IPv6 connectivity test',
    color: 'from-primary-600 to-blue-600',
    featureFlag: 'feature_ping',
    componentNode: defineAsyncComponent({
      loader: () => import('./Utilities/Ping6.vue'),
      delay: 200,
      timeout: 10000,
      loadingComponent: () => h('div', { class: 'flex items-center justify-center p-8' }, [
        h('div', { class: 'w-8 h-8 border-4 border-primary-500 border-t-transparent rounded-full animate-spin' })
      ]),
      errorComponent: () => h('div', { class: 'text-red-500 p-4' }, 'Failed to load component')
    })
  },
  {
    id: 'mtr',
    label: 'MTR',
    description: 'Network path analysis',
    color: 'from-blue-500 to-sky-500',
    featureFlag: 'feature_mtr',
    componentNode: defineAsyncComponent({
      loader: () => import('./Utilities/MTR.vue'),
      delay: 200,
      timeout: 10000,
      loadingComponent: () => h('div', { class: 'flex items-center justify-center p-8' }, [
        h('div', { class: 'w-8 h-8 border-4 border-primary-500 border-t-transparent rounded-full animate-spin' })
      ]),
      errorComponent: () => h('div', { class: 'text-red-500 p-4' }, 'Failed to load component')
    })
  },
  {
    id: 'mtr6',
    label: 'MTR IPv6',
    description: 'IPv6 path analysis',
    color: 'from-sky-500 to-cyan-500',
    featureFlag: 'feature_mtr',
    componentNode: defineAsyncComponent({
      loader: () => import('./Utilities/MTR6.vue'),
      delay: 200,
      timeout: 10000,
      loadingComponent: () => h('div', { class: 'flex items-center justify-center p-8' }, [
        h('div', { class: 'w-8 h-8 border-4 border-primary-500 border-t-transparent rounded-full animate-spin' })
      ]),
      errorComponent: () => h('div', { class: 'text-red-500 p-4' }, 'Failed to load component')
    })
  },
  {
    id: 'traceroute',
    label: 'Traceroute',
    description: 'Route path discovery',
    color: 'from-purple-500 to-pink-500',
    featureFlag: 'feature_traceroute',
    componentNode: defineAsyncComponent({
      loader: () => import('./Utilities/Traceroute.vue'),
      delay: 200,
      timeout: 10000,
      loadingComponent: () => h('div', { class: 'flex items-center justify-center p-8' }, [
        h('div', { class: 'w-8 h-8 border-4 border-primary-500 border-t-transparent rounded-full animate-spin' })
      ]),
      errorComponent: () => h('div', { class: 'text-red-500 p-4' }, 'Failed to load component')
    })
  },
  {
    id: 'traceroute6',
    label: 'Traceroute IPv6',
    description: 'IPv6 route discovery',
    color: 'from-pink-500 to-rose-500',
    featureFlag: 'feature_traceroute',
    componentNode: defineAsyncComponent({
      loader: () => import('./Utilities/Traceroute6.vue'),
      delay: 200,
      timeout: 10000,
      loadingComponent: () => h('div', { class: 'flex items-center justify-center p-8' }, [
        h('div', { class: 'w-8 h-8 border-4 border-primary-500 border-t-transparent rounded-full animate-spin' })
      ]),
      errorComponent: () => h('div', { class: 'text-red-500 p-4' }, 'Failed to load component')
    })
  },
  {
    id: 'iperf3',
    label: 'IPerf3',
    description: 'Bandwidth measurement',
    color: 'from-green-500 to-emerald-500',
    featureFlag: 'feature_iperf3',
    componentNode: defineAsyncComponent({
      loader: () => import('./Utilities/IPerf3.vue'),
      delay: 200,
      timeout: 10000,
      loadingComponent: () => h('div', { class: 'flex items-center justify-center p-8' }, [
        h('div', { class: 'w-8 h-8 border-4 border-primary-500 border-t-transparent rounded-full animate-spin' })
      ]),
      errorComponent: () => h('div', { class: 'text-red-500 p-4' }, 'Failed to load component')
    })
  },
  {
    id: 'speedtest-net',
    label: 'Speedtest.net',
    description: 'Official Speedtest CLI',
    color: 'from-orange-500 to-amber-500',
    featureFlag: 'feature_speedtest_dot_net',
    componentNode: defineAsyncComponent({
      loader: () => import('./Utilities/SpeedtestNet.vue'),
      delay: 200,
      timeout: 10000,
      loadingComponent: () => h('div', { class: 'flex items-center justify-center p-8' }, [
        h('div', { class: 'w-8 h-8 border-4 border-primary-500 border-t-transparent rounded-full animate-spin' })
      ]),
      errorComponent: () => h('div', { class: 'text-red-500 p-4' }, 'Failed to load component')
    })
  },
  {
    id: 'shell',
    label: 'Shell',
    description: 'Interactive command line',
    color: 'from-gray-600 to-gray-700',
    featureFlag: 'feature_shell',
    componentNode: defineAsyncComponent({
      loader: () => import('./Utilities/Shell.vue'),
      delay: 200,
      timeout: 10000,
      loadingComponent: () => h('div', { class: 'flex items-center justify-center p-8' }, [
        h('div', { class: 'w-8 h-8 border-4 border-primary-500 border-t-transparent rounded-full animate-spin' })
      ]),
      errorComponent: () => h('div', { class: 'text-red-500 p-4' }, 'Failed to load component')
    })
  }
])

// === CUSTOM START: 工具图标注入 - By ASxiaowen ===
// 理由: 上游工具定义不含 icon 字段，这里统一注入，图标路径全部维护在 toolIcons.js，
//       上游改动工具列表时不需要合并任何图标相关代码。
tools.value.forEach((t) => {
  t.icon = toolIcon(t.id)
})
// === CUSTOM END: 工具图标注入 ===

// 过滤可用的工具
const availableTools = computed(() => {
  return tools.value.filter(tool => {
    return config.value && config.value[tool.featureFlag]
  })
})

const loadComponentOnDemand = (tool) => {
  if (!toolComponent.value || toolComponent.value !== tool.componentNode) {
    toolComponent.value = tool.componentNode
  }
}

const openTool = (tool) => {
  currentTool.value = tool
  // Only load component when actually needed
  loadComponentOnDemand(tool)
}

const closeTool = () => {
  toolComponentShow.value = false
}
</script>

<template>
  <div>
    <div>
      <!-- === CUSTOM START: 工具按钮网格 - By ASxiaowen === -->
      <!-- 理由: 按钮视觉（图标 + hover 抬升）抽到 custom_components/ToolGrid.vue，此处替换原按钮循环 -->
      <ToolGrid :tools="availableTools" @open="openTool" />
      <!-- === CUSTOM END: 工具按钮网格 === -->
    </div>
  </div>

  <!-- Tool Drawer -->
  <Teleport to="body">
    <Transition
      enter-active-class="transition-opacity duration-300"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition-opacity duration-300"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div v-if="toolComponentShow" class="fixed inset-0 z-50 overflow-hidden">
        <!-- Backdrop -->
        <div class="absolute inset-0 bg-black/30 backdrop-blur-sm" @click="closeTool"></div>
        
        <!-- Drawer -->
        <Transition
          enter-active-class="transition-transform duration-500 ease-[cubic-bezier(0.16,1,0.3,1)]"
          enter-from-class="translate-x-full"
          enter-to-class="translate-x-0"
          leave-active-class="transition-transform duration-300 ease-in-out"
          leave-from-class="translate-x-0"
          leave-to-class="translate-x-full"
        >
          <div v-if="toolComponentShow" class="absolute right-0 top-0 h-full w-full max-w-3xl bg-white/95 dark:bg-gray-900/95 backdrop-blur-lg shadow-2xl flex flex-col border-l border-primary-200/50 dark:border-primary-700/50">
            <!-- Header -->
            <div class="flex items-center justify-between p-4 border-b border-primary-200/30 dark:border-primary-700/30 flex-shrink-0 bg-primary-50/50 dark:bg-gray-800/50">
              <div class="flex items-center space-x-4">
                <div v-if="currentTool" class="flex items-center justify-center w-10 h-10 rounded-xl" :class="`bg-gradient-to-br ${currentTool.color}`">
                  <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>
                  </svg>
                </div>
                <div>
                  <h2 class="text-xl font-semibold text-gray-900 dark:text-gray-100">{{ currentTool?.label }}</h2>
                  <p class="text-sm text-gray-600 dark:text-gray-300">{{ currentTool?.description }}</p>
                </div>
              </div>
              <button
                @click="closeTool"
                class="flex items-center justify-center w-10 h-10 rounded-full bg-primary-100 hover:bg-primary-200 dark:bg-gray-700 dark:hover:bg-gray-600 transition-colors duration-200"
              >
                <svg class="w-5 h-5 text-gray-600 dark:text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                </svg>
              </button>
            </div>
            
            <!-- Content -->
            <div class="flex-1 overflow-y-auto p-6 bg-primary-25 dark:bg-gray-850">
              <component :is="toolComponent" @closed="closeTool" />
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.animate-slide-up {
  animation: slideUp 0.4s ease-out;
}

@keyframes slideUp {
  from {
    transform: translateY(20px);
    opacity: 0;
  }
  to {
    transform: translateY(0);
    opacity: 1;
  }
}
</style>
