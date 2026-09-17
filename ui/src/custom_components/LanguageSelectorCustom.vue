<!--
  NetMirror 二次开发 · 语言选择器（修复版）
  目录: ui/src/custom_components/LanguageSelectorCustom.vue

  修复的两个问题：
    1. 原实现在 footer 内用 absolute + bottom-full 定位，页面滚动 / 图表重绘后
       footer 位移，菜单跟着飘，鼠标点下去时菜单位置已经变了 → “点击弹出里面消失了”
    2. 语言按钮中心点被右下角 ThemeToggle 悬浮按钮盖住 → `document.elementFromPoint`
       命中的是 FAB，事件到不了语言按钮 → “中文没办法选”

  做法：菜单 Teleport 到 body + 用按钮 getBoundingClientRect() 算 fixed 坐标，
        滚动/ resize 时重算，按钮滚出视口就关闭；按钮加 @click.stop 消除开关竞态。

  由 components/LanguageSelector.vue 转发引用，上游文件内部实现不动。
-->
<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'

const props = defineProps({
  currentLang: String,
  langList: Array,
  showLabel: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['change'])

const isOpen = ref(false)
const wrapperRef = ref()
const btnRef = ref()
const menuRef = ref()
const pos = ref({ top: 0, left: 0 })

const currentLangLabel = computed(() => {
  const lang = props.langList.find(l => l.value === props.currentLang)
  return lang?.label || 'English'
})

// 菜单挂到 body 用 fixed 定位：页面滚动/图表重绘导致 footer 位移时，菜单不会跟着乱跑
const updatePos = () => {
  const btn = btnRef.value
  if (!btn) return
  const r = btn.getBoundingClientRect()
  const mh = menuRef.value?.offsetHeight || 90
  const mw = menuRef.value?.offsetWidth || 176
  let top = r.top - mh - 8
  if (top < 8) top = r.bottom + 8
  let left = r.left
  const vw = window.innerWidth
  if (left + mw > vw - 8) left = Math.max(8, vw - mw - 8)
  pos.value = { top, left }
}

const open = async () => {
  isOpen.value = true
  await nextTick()
  updatePos()
}

const toggle = () => {
  if (isOpen.value) {
    isOpen.value = false
  } else {
    open()
  }
}

const selectLanguage = (langValue) => {
  emit('change', langValue)
  isOpen.value = false
}

// Close dropdown when clicking outside
const closeDropdown = (event) => {
  const t = event.target
  if (wrapperRef.value?.contains(t)) return
  if (menuRef.value?.contains(t)) return
  isOpen.value = false
}

const onScrollOrResize = () => {
  if (!isOpen.value) return
  const r = btnRef.value?.getBoundingClientRect()
  // 按钮滚出视口就关掉，避免菜单飘在半空
  if (!r || r.bottom < 0 || r.top > window.innerHeight) {
    isOpen.value = false
    return
  }
  updatePos()
}

onMounted(() => {
  document.addEventListener('click', closeDropdown)
  window.addEventListener('scroll', onScrollOrResize, true)
  window.addEventListener('resize', onScrollOrResize)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', closeDropdown)
  window.removeEventListener('scroll', onScrollOrResize, true)
  window.removeEventListener('resize', onScrollOrResize)
})
</script>

<template>
  <div ref="wrapperRef" class="relative">
    <button
      ref="btnRef"
      @click.stop="toggle"
      class="inline-flex items-center justify-center transition-all duration-200"
      :class="showLabel ? 'space-x-2 rounded-lg border border-gray-200/80 dark:border-white/[0.06] bg-white dark:bg-white/[0.03] px-3 py-2 shadow-sm hover:border-primary-300 dark:hover:border-primary-500/40' : 'w-full h-full hover:bg-gray-100 dark:hover:bg-gray-600 rounded-xl'"
      title="Language"
    >
      <!-- Globe Icon -->
      <svg class="w-[18px] h-[18px] text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9-3-9m-9 9a9 9 0 019-9" />
      </svg>
      <span v-if="showLabel" class="text-[13px] font-medium text-gray-700 dark:text-gray-200">{{ currentLangLabel }}</span>
      <!-- Chevron Down Icon -->
      <svg
        v-if="showLabel"
        class="w-3.5 h-3.5 text-gray-400 transition-transform duration-200"
        :class="{ 'rotate-180': isOpen }"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
      </svg>
    </button>

    <Teleport to="body">
      <Transition
        enter-active-class="transition-all duration-150 ease-out"
        enter-from-class="opacity-0 scale-95"
        enter-to-class="opacity-100 scale-100"
        leave-active-class="transition-all duration-100 ease-in"
        leave-from-class="opacity-100 scale-100"
        leave-to-class="opacity-0 scale-95"
      >
        <div
          v-if="isOpen"
          ref="menuRef"
          :style="{ top: pos.top + 'px', left: pos.left + 'px' }"
          class="fixed z-[60] w-44 overflow-hidden rounded-lg border border-gray-200/80 dark:border-white/[0.08] bg-white dark:bg-[#14161d] shadow-lift py-1.5"
        >
          <button
            v-for="lang in langList"
            :key="lang.value"
            @click.stop="selectLanguage(lang.value)"
            class="w-full px-3.5 py-2 text-left text-[13px] text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-white/[0.04] transition-colors duration-150"
            :class="{ 'bg-primary-50 dark:bg-primary-500/10 text-primary-600 dark:text-primary-400 font-medium': currentLang === lang.value }"
          >
            {{ lang.label }}
          </button>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
