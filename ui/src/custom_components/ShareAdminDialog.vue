<!--
  NetMirror 二次开发 · 临时链接弹窗（主面板入口）
  目录: ui/src/custom_components/ShareAdminDialog.vue

  由 RootShell 在「已登录」状态下挂一个右下角入口按钮打开（不改 AdminPanel.vue，避免动上游文件）。

  本文件只负责「弹窗外壳」：遮罩、卡片、标题、关闭按钮。
  表单与列表的实现在 SharePanel.vue，那份实现同时被管理页（AdminExtras.vue）复用，
  避免同一套逻辑维护两份。
-->

<script setup>
import SharePanel from './SharePanel.vue'

const emit = defineEmits(['close'])
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-[60] flex items-start justify-center overflow-y-auto p-4 sm:p-8">
      <div class="absolute inset-0 bg-black/40 backdrop-blur-sm" @click="emit('close')"></div>

      <div class="lg-card relative z-10 my-auto w-full max-w-2xl p-5 sm:p-6">
        <!-- 头部 -->
        <div class="mb-4 flex items-start justify-between gap-4">
          <div class="min-w-0">
            <h2 class="text-[15px] font-semibold text-gray-900 dark:text-gray-100">临时测试链接</h2>
            <p class="mt-0.5 text-[12px] text-gray-500 dark:text-gray-400">
              生成一条限定节点与功能的链接，同时得到一个临时密码。两者<b>分开发送</b>，对方打开链接后输入密码才能使用；到期自动失效，也可随时吊销。
            </p>
          </div>
          <button
            class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-gray-100 text-gray-500 transition-colors hover:bg-gray-200 dark:bg-white/[0.05] dark:text-gray-400 dark:hover:bg-white/[0.1]"
            @click="emit('close')"
          >
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
            </svg>
          </button>
        </div>

        <SharePanel :show-cancel="true" @close="emit('close')" />
      </div>
    </div>
  </Teleport>
</template>
