<!--
  NetMirror 二次开发 · 临时链接受限模式提示条
  目录: ui/src/custom_components/ShareBanner.vue

  访客通过 /t/<token> 进入时，页首固定一条提示：
  说明当前处于受限模式、能用哪些功能、还剩多久。倒计时归零时通知外层。-->

<script setup>
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { authState } from './authState'
import { formatDuration, toolLabel } from './useShare'

const emit = defineEmits(['expired'])

const props = defineProps({
  /** 是否可折叠（移动端可以收起，减少遮挡） */
  collapsible: { type: Boolean, default: true }
})

const collapsed = ref(false)
/** 剩余秒数，由定时器刷新 */
const left = ref(0)

const scope = computed(() => authState.scope || {})
const tools = computed(() => scope.value.tools || [])

/** 展示用工具名；'*' 表示全部 */
const toolText = computed(() => {
  if (tools.value.includes('*')) return '全部功能'
  return tools.value.map(toolLabel).join('、')
})

/**
 * 展示用节点名。
 *
 * 这里刻意从 nodes 数组推导，而不是读 nodeId：多节点令牌只有 nodes 数组，
 * nodeId/nodeUrl 是留给旧令牌的兼容字段（多节点时为空），只读它们会让
 * 横幅显示成「节点 —」。
 */
const nodeText = computed(() => {
  const list = scope.value.nodes || []
  if (list.length > 1) {
    return (list[0].name || list[0].url || '—') + ' 等 ' + list.length + ' 个节点'
  }
  if (list.length === 1) {
    return list[0].name || list[0].url || '—'
  }
  // 旧令牌没有 nodes 数组，退回兼容字段
  return scope.value.nodeName || scope.value.nodeId || scope.value.nodeUrl || '—'
})
const countdown = computed(() => formatDuration(left.value))
/** 剩余不足 10 分钟时改成警示配色 */
const urgent = computed(() => left.value > 0 && left.value <= 600)

let timer = null

const tick = () => {
  const exp = scope.value.exp || 0
  left.value = exp ? Math.max(0, exp - Math.floor(Date.now() / 1000)) : 0
  if (exp && left.value <= 0) {
    emit('expired')
  }
}

onMounted(() => {
  tick()
  timer = setInterval(tick, 1000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div
    class="sticky top-0 z-40 border-b border-amber-200/70 bg-amber-50/90 backdrop-blur-md dark:border-amber-500/20 dark:bg-amber-500/[0.08]"
  >
    <div class="mx-auto flex max-w-6xl flex-wrap items-center gap-x-3 gap-y-1 px-4 py-2 text-[12px]">
      <span
        class="inline-flex flex-shrink-0 items-center gap-1.5 rounded-md bg-amber-100 px-2 py-0.5 font-medium text-amber-800 ring-1 ring-inset ring-amber-200/80 dark:bg-amber-500/15 dark:text-amber-300 dark:ring-amber-500/25"
      >
        <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          ></path>
        </svg>
        临时链接模式
      </span>

      <span class="text-amber-900/90 dark:text-amber-200/80">
        节点 <span class="font-mono font-medium">{{ nodeText }}</span>
        <template v-if="scope.note">
          · 备注 <span class="font-medium">{{ scope.note }}</span>
        </template>
      </span>

      <span
        v-if="!collapsed"
        class="min-w-0 flex-1 truncate text-amber-900/70 dark:text-amber-200/60"
        :title="toolText"
      >
        可用功能：{{ toolText }}
      </span>

      <span class="ml-auto flex flex-shrink-0 items-center gap-2">
        <span
          class="rounded-md px-2 py-0.5 font-mono tabular-nums ring-1 ring-inset"
          :class="
            urgent
              ? 'bg-red-100 text-red-700 ring-red-200 dark:bg-red-500/15 dark:text-red-300 dark:ring-red-500/25'
              : 'bg-white/70 text-amber-800 ring-amber-200/70 dark:bg-white/[0.04] dark:text-amber-300 dark:ring-amber-500/20'
          "
        >
          剩余 {{ countdown }}
        </span>
        <button
          v-if="collapsible"
          type="button"
          class="rounded-md px-1.5 py-0.5 text-amber-700/80 transition-colors hover:bg-amber-100/70 dark:text-amber-300/70 dark:hover:bg-white/[0.06]"
          :title="collapsed ? '展开详情' : '收起详情'"
          @click="collapsed = !collapsed"
        >
          {{ collapsed ? '展开' : '收起' }}
        </button>
      </span>
    </div>
  </div>
</template>
