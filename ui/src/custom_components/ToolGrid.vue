<!--
  NetMirror 二次开发 · 工具按钮网格
  目录: ui/src/custom_components/ToolGrid.vue

  上游 components/Utilities.vue 的工具按钮循环被整块替换成这个组件，
  原文件只剩一行 <ToolGrid>，侵入最小；视觉（图标 + hover 抬升）在这里维护。
-->
<script setup>
defineProps({
  tools: { type: Array, default: () => [] }
})
const emit = defineEmits(['open'])
</script>

<template>
  <div class="flex flex-wrap gap-2">
    <button
      v-for="tool in tools"
      :key="tool.id"
      @click="emit('open', tool)"
      :title="tool.description"
      class="group inline-flex items-center gap-2 rounded-lg border border-gray-200 dark:border-white/[0.06] bg-white dark:bg-white/[0.03] px-3 py-2 text-[13px] font-medium text-gray-700 dark:text-gray-300 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-primary-300 dark:hover:border-primary-500/40 hover:text-primary-600 dark:hover:text-primary-400 hover:shadow-soft"
    >
      <svg
        class="h-4 w-4 text-gray-400 transition-colors duration-200 group-hover:text-primary-500 dark:group-hover:text-primary-400"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.8" :d="tool.icon"></path>
      </svg>
      {{ tool.label }}
    </button>
  </div>
</template>
