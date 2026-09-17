<script setup>
import { ref, onMounted } from 'vue'
import { useMotion } from '@vueuse/motion'

const cardRef = ref()
const { apply } = useMotion(cardRef, {
  initial: { opacity: 0, y: 20 },
  enter: { opacity: 1, y: 0, transition: { duration: 500 } }
})

onMounted(() => {
  apply()
})
</script>

<template>
  <!-- === CUSTOM START: 连接卡片视觉 - By ASxiaowen === -->
  <!-- 理由: 改用 .lg-card + 渐变图标方块 + 1.5px 进度条，与全局卡片风格统一；
          样式在 custom_components/theme.css，原文件这里是纯文字 spinner -->
  <div ref="cardRef" class="max-w-md mx-auto px-4 pt-16">
    <div class="lg-card p-8 text-center">
      <div class="mx-auto mb-6 flex h-14 w-14 items-center justify-center rounded-xl bg-gradient-to-br from-primary-500 to-sky-400 shadow-soft">
        <div class="w-6 h-6 border-[3px] border-white border-t-transparent rounded-full animate-spin"></div>
      </div>
      <h3 class="text-[15px] font-semibold tracking-tight text-gray-900 dark:text-white mb-1.5">Connecting...</h3>
      <p class="text-[12px] text-gray-500 dark:text-gray-400 mb-6">Establishing connection to the server</p>

      <!-- Progress bar -->
      <div class="w-full bg-gray-100 dark:bg-white/[0.06] rounded-full h-1.5 overflow-hidden">
        <div class="h-1.5 rounded-full bg-gradient-to-r from-primary-500 to-sky-400 animate-pulse" style="width: 100%"></div>
      </div>
    </div>
  </div>
  <!-- === CUSTOM END: 连接卡片视觉 === -->
</template>
