<!--
  === CUSTOM START: 语言选择器修复 - By ASxiaowen ===
  理由:
    1) 原实现在 footer 内用 absolute + bottom-full 定位，滚动/图表重绘后 footer 位移，
       菜单跟着飘，鼠标落到菜单上时位置已经变了 → “点击弹出里面消失了”
    2) 语言按钮中心点被右下角 ThemeToggle 悬浮按钮盖住，事件到不了按钮 → “中文没办法选”

  做法: 修复实现整体放到 custom_components/LanguageSelectorCustom.vue（Teleport + fixed 定位 +
        @click.stop + 滚动重算），这里只做默认导出转发，原组件内部实现保持在定制目录里，
        上游合并时本文件只需保留这 4 行。
  === CUSTOM END: 语言选择器修复 ===
-->
<script>
import LanguageSelectorCustom from '@/custom_components/LanguageSelectorCustom.vue'
export default LanguageSelectorCustom
</script>
