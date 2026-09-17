<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { useMotion } from '@vueuse/motion'
import { useNodeTool } from '@/composables/useNodeTool'
import { useNodesStore } from '@/stores/nodes'
import FileSpeedtest from '@/components/Speedtest/FileSpeedtest.vue'
import Librespeed from '@/components/Speedtest/Librespeed.vue'

const nodesStore = useNodesStore()
const cardRef = ref()

const {
  selectedNode,
  selectedNodeName,
  selectedNodeLocation,
  // === CUSTOM START: 会话状态透出 - By ASxiaowen ===
  // 理由: 连接中给提示，避免看起来像空白
  sessionStatus
  // === CUSTOM END: 会话状态透出 ===
} = useNodeTool()

// === CUSTOM START: 配置兜底 - By ASxiaowen ===
// 理由: 切换节点期间 selectedNode.config 还没到，回落到 effectiveConfig，避免测速区整块消失
const currentConfig = computed(() => {
  if (selectedNode.value && selectedNode.value.config) {
    return selectedNode.value.config
  }
  return nodesStore.effectiveConfig || {}
})
// === CUSTOM END: 配置兜底 ===

const availableTests = computed(() => {
  const tests = []
  const config = currentConfig.value
  if (config?.feature_librespeed) {
    tests.push({ id: 'librespeed', name: 'Librespeed' })
  }
  if (config?.feature_filespeedtest) {
    tests.push({ id: 'filespeedtest', name: 'File-based Test' })
  }
  return tests
})

const activeTest = ref(null)

// 监听availableTests变化，自动设置第一个可用测试
watch(availableTests, (newTests) => {
  if (newTests.length > 0 && !activeTest.value) {
    activeTest.value = newTests[0].id
  }
}, { immediate: true })

const { apply } = useMotion(cardRef, {
  initial: { opacity: 0, y: 20 },
  enter: { opacity: 1, y: 0, transition: { duration: 500, delay: 400 } }
})

onMounted(() => {
  if (availableTests.value.length > 0) {
    apply()
  }
})
</script>

<template>
  <div 
    v-if="availableTests.length > 0"
    ref="cardRef" 
  >
    <div class="space-y-6">
    <!-- === CUSTOM START: 测速子标签分段控件 - By ASxiaowen === -->
    <!-- 理由: 原文件这里是下划线式 tab，改成容器内分段控件，视觉与 .lg-card 一致 -->
      <div v-if="availableTests.length > 1" class="flex justify-center">
        <div class="inline-flex gap-1 rounded-lg border border-gray-200/70 dark:border-white/[0.06] bg-gray-50/70 dark:bg-white/[0.02] p-1">
          <button
            v-for="test in availableTests"
            :key="test.id"
            @click="activeTest = test.id"
            class="rounded-md px-3.5 py-1.5 text-[13px] font-medium transition-all duration-200 focus:outline-none"
            :class="[
              activeTest === test.id
                ? 'bg-white dark:bg-white/[0.08] text-primary-600 dark:text-primary-400 shadow-sm ring-1 ring-gray-200/70 dark:ring-white/[0.06]'
                : 'text-gray-500 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200'
            ]"
          >
            {{ test.name }}
          </button>
        </div>
      </div>
      <!-- === CUSTOM END: 测速子标签分段控件 === -->

      <!-- === CUSTOM START: 连接中提示 - By ASxiaowen === -->
      <!-- 理由: 切换节点时给个明确提示，避免用户以为测速区挂了 -->
      <p v-if="sessionStatus === 'connecting'" class="text-center text-[12px] text-gray-400 dark:text-gray-500">
        正在连接节点，测速将在连接完成后可用…
      </p>
      <!-- === CUSTOM END: 连接中提示 === -->

      <!-- Conditionally rendered speed test components -->
      <div>
        <Librespeed v-if="activeTest === 'librespeed'" />
        <FileSpeedtest v-if="activeTest === 'filespeedtest'" />
      </div>
    </div>
  </div>
  
  <!-- No node selected state -->
  <div v-else class="text-center py-8 text-gray-500 dark:text-gray-400">
    Please select a node to run speed tests.
  </div>
</template>
