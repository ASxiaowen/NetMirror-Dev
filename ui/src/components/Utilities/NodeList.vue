<template>
  <div class="space-y-4">
    <!-- Header with node count and refresh button -->
    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
      <div class="flex items-center gap-2 text-[12px] text-gray-500 dark:text-gray-400 order-2 sm:order-1">
        <svg class="w-3.5 h-3.5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"/>
        </svg>
        <span class="tabular-nums">{{ nodes.length }}</span>
        <span>node{{ nodes.length !== 1 ? 's' : '' }} available</span>
      </div>
      <!-- === CUSTOM START: 顶部操作按钮视觉 - By ASxiaowen === -->
      <!-- 理由: 白底描边 + 图标 + hover 抬升，与全局按钮风格统一 -->
      <button
        @click="testAllLatencies"
        :disabled="loading"
        class="group inline-flex items-center justify-center gap-2 rounded-lg border border-gray-200 dark:border-white/[0.06] bg-white dark:bg-white/[0.03] px-3 py-1.5 text-[13px] font-medium text-gray-600 dark:text-gray-300 shadow-sm transition-all duration-200 order-1 sm:order-2 w-full sm:w-auto hover:-translate-y-px hover:border-primary-300 hover:text-primary-600 dark:hover:text-primary-400 hover:shadow-soft disabled:pointer-events-none disabled:opacity-50"
      >
        <svg
          class="w-3.5 h-3.5 transition-transform duration-500 group-hover:rotate-180"
          :class="{ 'animate-spin': loading }"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
        </svg>
        {{ loading ? 'Testing All Nodes...' : 'Refresh All Nodes' }}
      </button>
      <!-- === CUSTOM END: 顶部操作按钮视觉 === -->
    </div>

    <!-- Node dropdown selector -->
    <div v-if="nodes.length > 0" class="space-y-3">
      <div class="relative">
        <!-- === CUSTOM START: 选择节点标签 - By ASxiaowen === -->
        <!-- 理由: 加一个小号大写标签 + 地球图标，和 SectionTitle 的层级感对齐 -->
        <label class="flex items-center gap-1.5 mb-1.5 text-[10px] font-medium uppercase tracking-wider text-gray-400 dark:text-gray-500">
          <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"/>
          </svg>
          Select node
        </label>
        <!-- === CUSTOM END: 选择节点标签 === -->
        <select
          :value="selectedUrl"
          @change="selectByUrl($event.target.value)"
          class="w-full appearance-none rounded-lg border border-gray-200 dark:border-white/[0.06] bg-white dark:bg-white/[0.03] py-2.5 pl-3 pr-10 text-[13px] font-medium text-gray-900 dark:text-gray-100 shadow-sm transition-all duration-200 hover:border-primary-300 focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/20 cursor-pointer"
        >
          <option value="" disabled>Choose a looking glass node</option>
          <option v-for="node in nodes" :key="node.url" :value="node.url">
            {{ node.name }} ({{ node.location }}) - {{ latencyText(node) }}{{ isCurrentNode(node) ? ' - Current' : '' }}
          </option>
        </select>
        <svg class="w-4 h-4 text-gray-400 pointer-events-none absolute right-3 bottom-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
        </svg>
      </div>

      <!-- === CUSTOM START: 选中节点状态条 - By ASxiaowen === -->
      <!-- 理由: 上游没有这条状态条，切换节点后用户看不到“当前节点/延迟”；
             灰底卡片 + 彩色延迟 pill + ping 涟漪点，与全局卡片风格一致 -->
      <div
        v-if="activeNode"
        class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 rounded-lg border border-gray-200/70 dark:border-white/[0.06] bg-gray-50/60 dark:bg-white/[0.02] p-3 transition-colors duration-200"
      >
        <div class="flex items-center space-x-3 min-w-0">
          <span class="relative flex h-2.5 w-2.5 flex-shrink-0">
            <span
              v-if="latencies[getNodeKey(activeNode)] && latencies[getNodeKey(activeNode)].status !== 'error'"
              class="absolute inline-flex h-full w-full rounded-full opacity-60 animate-ping"
              :class="{
                'bg-emerald-400': latencies[getNodeKey(activeNode)]?.status === 'good',
                'bg-amber-400': latencies[getNodeKey(activeNode)]?.status === 'medium',
                'bg-rose-400': latencies[getNodeKey(activeNode)]?.status === 'high'
              }"
            ></span>
            <span
              class="relative inline-flex h-2.5 w-2.5 rounded-full"
              :class="{
                'bg-emerald-500': latencies[getNodeKey(activeNode)]?.status === 'good',
                'bg-amber-500': latencies[getNodeKey(activeNode)]?.status === 'medium',
                'bg-rose-500': latencies[getNodeKey(activeNode)]?.status === 'high' || latencies[getNodeKey(activeNode)]?.status === 'error',
                'bg-gray-400 animate-pulse': !latencies[getNodeKey(activeNode)]
              }"
            ></span>
          </span>
          <div class="min-w-0">
            <p class="text-[13px] font-semibold text-gray-900 dark:text-white leading-tight truncate">
              {{ activeNode.name }}
              <span v-if="isCurrentNode(activeNode)" class="ml-1 rounded bg-primary-50 dark:bg-primary-500/10 px-1.5 py-0.5 align-middle text-[10px] font-medium text-primary-600 dark:text-primary-400">Current</span>
            </p>
            <p class="mt-0.5 text-[11px] text-gray-500 dark:text-gray-400 leading-tight">
              {{ activeNode.location }}
              <span class="mx-1 text-gray-300 dark:text-gray-600">·</span>
              <span
                class="rounded px-1.5 py-0.5 font-mono text-[11px] tabular-nums"
                :class="{
                  'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-400': latencies[getNodeKey(activeNode)]?.status === 'good',
                  'bg-amber-50 text-amber-700 dark:bg-amber-500/10 dark:text-amber-400': latencies[getNodeKey(activeNode)]?.status === 'medium',
                  'bg-rose-50 text-rose-700 dark:bg-rose-500/10 dark:text-rose-400': latencies[getNodeKey(activeNode)]?.status === 'high' || latencies[getNodeKey(activeNode)]?.status === 'error',
                  'text-gray-500 dark:text-gray-400': !latencies[getNodeKey(activeNode)]
                }">{{ latencyText(activeNode) }}</span>
            </p>
          </div>
        </div>
        <button
          @click="pingSingleNode(activeNode)"
          :disabled="pingStates[getNodeKey(activeNode)]?.isPinging"
          class="inline-flex items-center justify-center gap-1.5 rounded-lg border border-gray-200 dark:border-white/[0.06] bg-white dark:bg-white/[0.03] px-3 py-1.5 text-[12px] font-medium text-gray-600 dark:text-gray-300 shadow-sm transition-all duration-200 flex-shrink-0 hover:-translate-y-px hover:border-primary-300 hover:text-primary-600 dark:hover:text-primary-400 hover:shadow-soft disabled:pointer-events-none disabled:opacity-50"
        >
          <svg
            v-if="pingStates[getNodeKey(activeNode)]?.isPinging"
            class="w-3.5 h-3.5 animate-spin"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
          </svg>
          <svg
            v-else
            class="w-3.5 h-3.5"
            fill="currentColor"
            viewBox="0 0 24 24"
          >
            <path d="M13 10V3L4 14h7v7l9-11h-7z"/>
          </svg>
          <span>{{ pingStates[getNodeKey(activeNode)]?.isPinging ? 'Testing...' : 'Test' }}</span>
        </button>
      </div>
      <!-- === CUSTOM END: 选中节点状态条 === -->
    </div>

    <!-- Empty State -->
    <div v-if="nodes.length === 0 && !loading" class="text-center py-12">
      <div class="mx-auto w-16 h-16 bg-gradient-to-br from-gray-100 to-gray-200 dark:from-gray-700 dark:to-gray-800 rounded-full flex items-center justify-center mb-4">
        <svg class="w-8 h-8 text-gray-400 dark:text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"></path>
        </svg>
      </div>
      <h3 class="text-base font-medium text-gray-900 dark:text-gray-100 mb-2">No nodes available</h3>
      <p class="text-sm text-gray-500 dark:text-gray-400 max-w-sm mx-auto">No looking glass nodes have been configured yet. Please check back later.</p>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useNodesStore } from '@/stores/nodes'
import { storeToRefs } from 'pinia'

const nodesStore = useNodesStore()
const { 
  nodes, 
  selectedNode, 
  currentNode, 
  latencies, 
  loading, 
  pingStates 
} = storeToRefs(nodesStore)

let latencyInterval = null

// Use store methods
const { 
  getNodeKey, 
  isCurrentNode, 
  getStatusText, 
  fetchNodes, 
  testAllLatencies, 
  pingSingleNode,
  selectNode 
} = nodesStore

// Dropdown value: falls back to the local node so the selector always shows something meaningful
const selectedUrl = computed(() => selectedNode.value?.url || currentNode.value?.url || '')
const activeNode = computed(() => {
  const url = selectedUrl.value
  return nodes.value.find((n) => n.url === url) || null
})

const selectByUrl = (url) => {
  const node = nodes.value.find((n) => n.url === url)
  if (node) selectNode(node)
}

const latencyText = (node) => {
  const l = latencies.value[getNodeKey(node)]
  if (!l) return 'Testing...'
  if (l.status === 'error') return 'Offline'
  return `${l.latency}ms`
}

onMounted(() => {
  fetchNodes()

  // Refresh latencies every 5 minutes (300,000 ms) to reduce system load.
  latencyInterval = setInterval(() => {
    testAllLatencies()
  }, 300000)
})

onUnmounted(() => {
  if (latencyInterval) {
    clearInterval(latencyInterval)
  }
})
</script>
