<!--
  NetMirror 二次开发 · 区块标题组件
  目录: ui/src/custom_components/SectionTitle.vue

  新增组件，不改动上游任何文件；由 App.vue 直接 import 使用。
-->
<script setup>
defineProps({
  title: { type: String, default: '' },
  hint: { type: String, default: '' }
})
</script>

<template>
  <div class="mb-4 flex items-center gap-2.5">
    <span class="h-4 w-[3px] rounded-full bg-gradient-to-b from-primary-500 to-sky-400 shrink-0"></span>
    <h2 class="text-[15px] font-semibold tracking-tight text-gray-900 dark:text-gray-100">{{ title }}</h2>
    <span v-if="hint" class="ml-auto text-[11px] tabular-nums text-gray-400 dark:text-gray-500">{{ hint }}</span>
    <slot />
  </div>
</template>
