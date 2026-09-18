<!--
  NetMirror 二次开发 · 临时链接面板（可嵌入）
  目录: ui/src/custom_components/SharePanel.vue

  这份实现被两处复用，避免同一套逻辑维护两份：
    · ShareAdminDialog.vue —— 主面板右下角入口打开的弹窗（showCancel = true）
    · AdminExtras.vue      —— Node Management 管理页里的内嵌区块

  两件凭证的分工（界面必须讲清楚，否则管理员会把密码一起发出去，等于白设）：
    · 链接   —— 不是秘密，可以重发、可以从列表里再次复制
    · 密码   —— 是秘密，只在生成那一刻返回一次，之后任何接口都不回显
  所以这两行各有独立的复制按钮，并明确提示要「分开发送」。
-->

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useNodesStore } from '@/stores/nodes'
import { useShare, TOOL_CATALOG, TTL_PRESETS, DEFAULT_TOOLS, formatDuration } from './useShare'

const props = defineProps({
  /** 是否显示「取消」按钮 —— 弹窗模式需要，内嵌模式不需要 */
  showCancel: { type: Boolean, default: false },
  /** 是否在挂载时自动拉取列表 */
  autoLoad: { type: Boolean, default: true }
})

const emit = defineEmits(['close', 'created'])

const nodesStore = useNodesStore()
const { createShare, listShares, revokeShare } = useShare()

const tab = ref('create') // create | list

// ---- 创建表单 ----
/**
 * 用户显式勾选的节点 url 集合（多选）。
 *
 * 为什么是数组 + 复选框，而不是原先那个单选 <select>：
 * 除了「一条链接可以测多台机器」这个需求本身，原生 <select> 还有个坑 ——
 * 模型值匹配不上任何 option 时，**浏览器仍会照常显示第一个 option**，
 * 于是出现自相矛盾的界面：下拉里明明显示着节点，点生成却报「请先选择节点」
 * （节点列表比弹窗晚加载时极易触发）。复选框不会：没勾就是没勾，一眼可见。
 *
 * 存 url 而不是下标/对象：节点列表刷新后对象会换新实例，url 是稳定主键。
 */
const selectedNodeUrls = ref([])
/** 是否已经补过默认勾选 —— 用户手动清空后不再自动补回来 */
const nodeDefaultApplied = ref(false)
const note = ref('')
const ttl = ref(24 * 3600)
const selectedTools = ref([...DEFAULT_TOOLS])

const creating = ref(false)
const createError = ref('')
/** 生成成功后的结果：链接与一次性密码 */
const created = ref(null)
/** 哪一项刚被复制：'' | 'url' | 'password' */
const copied = ref('')

/** 刚生成的链接覆盖了几个节点（用于结果区文案） */
const createdNodeCount = computed(() => created.value?.record?.nodeCount || 0)

/** 刚生成的链接覆盖的节点名，逗号分隔；多台时便于发出去前复核 */
const createdNodeNames = computed(() => {
  const list = created.value?.record?.nodes || []
  return list.map((n) => n.name || n.id || n.url).filter(Boolean).join('、')
})

// ---- 列表 ----
const shares = ref([])
const listLoading = ref(false)
const listError = ref('')
const revokingId = ref('')
/** 列表里刚复制过的记录 id */
const listCopiedId = ref('')

const nodes = computed(() => nodesStore.nodes || [])

const selectedToolCount = computed(() => selectedTools.value.length)

/** 节点选择项：url 为唯一键，名称做展示 */
const nodeOptions = computed(() =>
  nodes.value.map((n) => ({
    url: String(n.url || '').replace(/\/+$/, ''),
    name: n.name || n.url,
    location: n.location || ''
  }))
)

/**
 * 归一化后的选中节点 url：按节点列表顺序排列，并自动丢弃已失效的 url
 * （节点从列表里被删掉、或地址改了）。提交与计数都以它为准，
 * 和界面显示的勾选状态严格一致。
 */
const checkedNodeUrls = computed(() =>
  nodeOptions.value.filter((n) => selectedNodeUrls.value.includes(n.url)).map((n) => n.url)
)

const checkedCount = computed(() => checkedNodeUrls.value.length)

const allNodesChecked = computed(
  () => nodeOptions.value.length > 0 && checkedCount.value === nodeOptions.value.length
)

const toggleNode = (url) => {
  const i = selectedNodeUrls.value.indexOf(url)
  if (i >= 0) selectedNodeUrls.value.splice(i, 1)
  else selectedNodeUrls.value.push(url)
}

const checkAllNodes = () => {
  selectedNodeUrls.value = nodeOptions.value.map((n) => n.url)
}

const clearNodes = () => {
  selectedNodeUrls.value = []
}

/**
 * 节点列表就绪后补一次默认勾选：只勾「当前正在浏览的那个节点」。
 *
 * 刻意不默认全选 —— 只想分享一台时，全选意味着得先手动取消掉其余几台，
 * 一旦漏看就会把本来不该给的机器一起发出去。少给比多给安全。
 */
watch(nodeOptions, (list) => {
  if (!list.length) return
  if (nodeDefaultApplied.value) return
  nodeDefaultApplied.value = true
  const cur = nodesStore.currentNode
  const curUrl = cur ? String(cur.url || '').replace(/\/+$/, '') : ''
  const pick = list.find((n) => n.url === curUrl) || list[0]
  selectedNodeUrls.value = [pick.url]
}, { immediate: true })


const toggleTool = (id) => {
  const i = selectedTools.value.indexOf(id)
  if (i >= 0) selectedTools.value.splice(i, 1)
  else selectedTools.value.push(id)
}

const submit = async () => {
  createError.value = ''
  created.value = null
  copied.value = ''

  // 先区分「还没加载完」与「没选」—— 否则节点列表晚到时，用户会看到
  // 「请先选择节点」，而界面上其实已经勾着节点，纯属误导。
  if (nodeOptions.value.length === 0) {
    createError.value = '节点列表尚未加载完成，请稍候重试'
    return
  }
  if (checkedCount.value === 0) {
    createError.value = '请至少勾选一个要分享的节点'
    return
  }
  if (selectedTools.value.length === 0) {
    createError.value = '请至少勾选一个允许使用的功能'
    return
  }

  // 按勾选顺序提交完整节点信息（id/name/location 一并带上）：
  // 受限模式下的节点来自令牌 scope，不查 /nodes —— 少了这些字段，
  // 访客侧就只能看到一串裸地址。
  const picked = nodeOptions.value.filter((n) => checkedNodeUrls.value.includes(n.url))
  creating.value = true
  try {
    const d = await createShare({
      nodes: picked.map((n) => ({ id: n.name, name: n.name, url: n.url, location: n.location })),
      note: note.value.trim(),
      tools: [...selectedTools.value],
      ttlSeconds: ttl.value
    })
    created.value = d
    emit('created', d)
    // 顺手刷新列表，方便直接看到记录
    await refreshList()
  } catch (e) {
    createError.value = e?.message || '生成失败'
  } finally {
    creating.value = false
  }
}

/** 复制指定文本；非安全上下文下 clipboard 不可用则退化为选中 */
const copyText = async (text, which) => {
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    copied.value = which
    setTimeout(() => {
      if (copied.value === which) copied.value = ''
    }, 2000)
  } catch (e) {
    copied.value = ''
  }
}

const copyListUrl = async (url, id) => {
  if (!url) return
  try {
    await navigator.clipboard.writeText(url)
    listCopiedId.value = id
    setTimeout(() => {
      if (listCopiedId.value === id) listCopiedId.value = ''
    }, 2000)
  } catch (e) {
    listCopiedId.value = ''
  }
}

const refreshList = async () => {
  listLoading.value = true
  listError.value = ''
  try {
    shares.value = await listShares()
  } catch (e) {
    listError.value = e?.message || '读取失败'
  } finally {
    listLoading.value = false
  }
}

const doRevoke = async (id) => {
  revokingId.value = id
  try {
    await revokeShare(id)
    await refreshList()
  } catch (e) {
    listError.value = e?.message || '吊销失败'
  } finally {
    revokingId.value = ''
  }
}

const fmtTime = (ts) => {
  if (!ts) return '—'
  const d = new Date(ts * 1000)
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

const statusMeta = (s) => {
  if (s === 'active') return { text: '有效', cls: 'bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-500/10 dark:text-emerald-300 dark:ring-emerald-500/20' }
  if (s === 'expired') return { text: '已过期', cls: 'bg-gray-100 text-gray-500 ring-gray-200 dark:bg-white/[0.04] dark:text-gray-400 dark:ring-white/[0.06]' }
  return { text: '已吊销', cls: 'bg-red-50 text-red-600 ring-red-200 dark:bg-red-500/10 dark:text-red-300 dark:ring-red-500/20' }
}

defineExpose({ refreshList })

onMounted(() => {
  if (props.autoLoad) refreshList()
})
</script>

<template>
  <div>
    <!-- 页签 -->
    <div class="mb-4 flex gap-1 rounded-lg bg-gray-100/80 p-1 dark:bg-white/[0.04]">
      <button
        v-for="t in [{ k: 'create', n: '生成链接与密码' }, { k: 'list', n: '已生成' }]"
        :key="t.k"
        class="flex-1 rounded-md px-3 py-1.5 text-[12px] font-medium transition-colors"
        :class="
          tab === t.k
            ? 'bg-white text-gray-900 shadow-card dark:bg-white/[0.08] dark:text-gray-100'
            : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'
        "
        @click="tab = t.k; t.k === 'list' && refreshList()"
      >
        {{ t.n }}<span v-if="t.k === 'list' && shares.length"> ({{ shares.length }})</span>
      </button>
    </div>

    <!-- ============ 生成 ============ -->
    <div v-if="tab === 'create'" class="space-y-4">
      <!-- 绑定节点：多选。一条链接可覆盖多台机器，访客在它们之间自由切换，
           工具白名单对所有节点共用一套（不按节点区分权限）。 -->
      <div>
        <div class="mb-1.5 flex items-baseline justify-between gap-2">
          <label class="text-[11px] font-medium uppercase tracking-wider text-gray-400 dark:text-gray-500">
            绑定节点
          </label>
          <div v-if="nodeOptions.length" class="flex items-center gap-2 text-[11px]">
            <span class="tabular-nums text-gray-400 dark:text-gray-500">
              已选 {{ checkedCount }} / {{ nodeOptions.length }}
            </span>
            <button
              type="button"
              :disabled="allNodesChecked"
              class="font-medium text-primary-600 transition-colors hover:underline disabled:cursor-default disabled:text-gray-300 disabled:no-underline dark:text-primary-400 dark:disabled:text-gray-600"
              @click="checkAllNodes"
            >
              全选
            </button>
            <button
              type="button"
              :disabled="checkedCount === 0"
              class="font-medium text-gray-500 transition-colors hover:underline disabled:cursor-default disabled:text-gray-300 disabled:no-underline dark:text-gray-400 dark:disabled:text-gray-600"
              @click="clearNodes"
            >
              清空
            </button>
          </div>
        </div>

        <div
          v-if="nodeOptions.length === 0"
          class="rounded-lg border border-amber-200 bg-amber-50/60 px-3 py-2 text-[12px] text-amber-700 dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-300"
        >
          暂无可用节点，请确认节点列表已加载。
        </div>
        <!-- 节点多时限高滚动，避免把「有效期 / 功能」挤出视野 -->
        <div
          v-else
          class="max-h-44 space-y-0.5 overflow-y-auto rounded-lg border border-gray-200/80 p-1.5 dark:border-white/[0.08]"
        >
          <button
            v-for="n in nodeOptions"
            :key="n.url"
            type="button"
            data-nm-node
            class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[12px] transition-colors"
            :class="
              checkedNodeUrls.includes(n.url)
                ? 'bg-primary-50 text-primary-700 dark:bg-primary-500/15 dark:text-primary-300'
                : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-white/[0.05]'
            "
            @click="toggleNode(n.url)"
          >
            <span
              class="flex h-3.5 w-3.5 flex-shrink-0 items-center justify-center rounded-[4px] border transition-colors"
              :class="
                checkedNodeUrls.includes(n.url)
                  ? 'border-primary-500 bg-primary-500 text-white'
                  : 'border-gray-300 dark:border-white/20'
              "
            >
              <svg v-if="checkedNodeUrls.includes(n.url)" class="h-2.5 w-2.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7"></path>
              </svg>
            </span>
            <span class="min-w-0 flex-1 truncate">
              <span class="font-medium">{{ n.name }}</span>
              <span v-if="n.location" class="text-gray-400 dark:text-gray-500"> · {{ n.location }}</span>
            </span>
          </button>
        </div>
        <p class="mt-1 text-[11px] text-gray-400 dark:text-gray-500">
          勾选的机器访客都能连；未勾选的连不上（后端按请求 Host 校验归属）。
        </p>
      </div>

      <div>
        <label class="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-gray-400 dark:text-gray-500">
          备注（发给谁）
        </label>
        <input
          v-model="note"
          maxlength="40"
          placeholder="例如：张三 / 客户A 排障"
          class="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-[13px] text-gray-800 outline-none transition-colors placeholder:text-gray-400 focus:border-primary-400 focus:ring-2 focus:ring-primary-100 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-gray-100 dark:placeholder:text-gray-600 dark:focus:border-primary-500/50 dark:focus:ring-primary-500/10"
        />
      </div>

      <div>
        <label class="mb-1.5 block text-[11px] font-medium uppercase tracking-wider text-gray-400 dark:text-gray-500">
          有效期
        </label>
        <div class="flex flex-wrap gap-1.5">
          <button
            v-for="p in TTL_PRESETS"
            :key="p.value"
            class="rounded-md px-2.5 py-1 text-[12px] font-medium ring-1 ring-inset transition-colors"
            :class="
              ttl === p.value
                ? 'bg-primary-50 text-primary-700 ring-primary-200 dark:bg-primary-500/15 dark:text-primary-300 dark:ring-primary-500/25'
                : 'bg-white text-gray-600 ring-gray-200 hover:bg-gray-50 dark:bg-white/[0.03] dark:text-gray-300 dark:ring-white/[0.08] dark:hover:bg-white/[0.06]'
            "
            @click="ttl = p.value"
          >
            {{ p.label }}
          </button>
        </div>
      </div>

      <div>
        <div class="mb-1.5 flex items-baseline justify-between">
          <label class="text-[11px] font-medium uppercase tracking-wider text-gray-400 dark:text-gray-500">
            允许使用的功能
          </label>
          <span class="text-[11px] text-gray-400 dark:text-gray-500">已选 {{ selectedToolCount }} 项</span>
        </div>
        <div class="grid grid-cols-2 gap-1.5 sm:grid-cols-3">
          <button
            v-for="t in TOOL_CATALOG"
            :key="t.id"
            type="button"
            :title="t.desc"
            class="flex items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-[12px] ring-1 ring-inset transition-colors"
            :class="
              selectedTools.includes(t.id)
                ? 'bg-primary-50 text-primary-700 ring-primary-200 dark:bg-primary-500/15 dark:text-primary-300 dark:ring-primary-500/25'
                : 'bg-white text-gray-600 ring-gray-200 hover:bg-gray-50 dark:bg-white/[0.03] dark:text-gray-300 dark:ring-white/[0.08] dark:hover:bg-white/[0.06]'
            "
            @click="toggleTool(t.id)"
          >
            <span
              class="flex h-3.5 w-3.5 flex-shrink-0 items-center justify-center rounded-[4px] border transition-colors"
              :class="
                selectedTools.includes(t.id)
                  ? 'border-primary-500 bg-primary-500 text-white'
                  : 'border-gray-300 dark:border-white/20'
              "
            >
              <svg v-if="selectedTools.includes(t.id)" class="h-2.5 w-2.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7"></path>
              </svg>
            </span>
            <span class="truncate">{{ t.label }}</span>
          </button>
        </div>
        <p class="mt-1.5 text-[11px] text-gray-400 dark:text-gray-500">
          未勾选的功能对该链接不可用，越权访问会被后端拒绝（403）。
        </p>
      </div>

      <p
        v-if="createError"
        class="rounded-md bg-red-50 px-2.5 py-2 text-[12px] text-red-700 ring-1 ring-inset ring-red-100 dark:bg-red-500/10 dark:text-red-300 dark:ring-red-500/20"
      >
        {{ createError }}
      </p>

      <div class="flex items-center justify-end gap-2">
        <button
          v-if="showCancel"
          class="rounded-lg px-3 py-2 text-[13px] font-medium text-gray-500 transition-colors hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-white/[0.06]"
          @click="emit('close')"
        >
          取消
        </button>
        <button
          :disabled="creating"
          class="inline-flex items-center gap-2 rounded-lg bg-gradient-to-b from-primary-500 to-primary-600 px-4 py-2 text-[13px] font-medium text-white shadow-soft transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lift disabled:translate-y-0 disabled:opacity-50 disabled:shadow-none"
          @click="submit"
        >
          <div v-if="creating" class="h-3.5 w-3.5 animate-spin rounded-full border-2 border-white/40 border-t-white"></div>
          {{ creating ? '生成中…' : '生成链接与密码' }}
        </button>
      </div>

      <!-- 生成结果：链接与密码分两行，各带独立复制按钮 -->
      <div
        v-if="created"
        class="space-y-3 rounded-lg border border-emerald-200 bg-emerald-50/70 p-3 dark:border-emerald-500/20 dark:bg-emerald-500/[0.08]"
      >
        <div class="flex items-center gap-1.5 text-[12px] font-medium text-emerald-800 dark:text-emerald-300">
          <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
          </svg>
          已生成，有效期 {{ formatDuration(ttl) }}
          <span v-if="createdNodeCount > 1" class="font-normal">· 覆盖 {{ createdNodeCount }} 个节点</span>
        </div>

        <!-- 绑定了多台时把机器列出来：管理员发出去之前要能一眼核对自己是不是多勾了 -->
        <p v-if="createdNodeNames" class="text-[11px] leading-relaxed text-emerald-700/90 dark:text-emerald-400/90">
          可访问：{{ createdNodeNames }}
        </p>

        <!-- 链接：不是秘密，可重发 -->
        <div>
          <div class="mb-1 flex items-baseline justify-between gap-2">
            <span class="text-[11px] font-medium uppercase tracking-wider text-emerald-700/80 dark:text-emerald-400/80">
              链接（可随时从「已生成」里再复制）
            </span>
          </div>
          <div class="flex items-center gap-2">
            <input
              :value="created.url"
              readonly
              class="min-w-0 flex-1 rounded-md border border-emerald-200/80 bg-white px-2.5 py-1.5 font-mono text-[11px] text-gray-700 dark:border-emerald-500/20 dark:bg-black/20 dark:text-gray-200"
              @focus="$event.target.select()"
            />
            <button
              class="flex-shrink-0 rounded-md bg-emerald-600 px-2.5 py-1.5 text-[12px] font-medium text-white transition-colors hover:bg-emerald-700"
              @click="copyText(created.url, 'url')"
            >
              {{ copied === 'url' ? '已复制' : '复制' }}
            </button>
          </div>
        </div>

        <!-- 密码：秘密，只显示这一次 -->
        <div>
          <div class="mb-1 flex items-baseline justify-between gap-2">
            <span class="text-[11px] font-medium uppercase tracking-wider text-emerald-700/80 dark:text-emerald-400/80">
              临时密码（关闭后不再显示）
            </span>
          </div>
          <div class="flex items-center gap-2">
            <input
              :value="created.password"
              readonly
              class="min-w-0 flex-1 rounded-md border border-emerald-300/80 bg-white px-2.5 py-1.5 text-center font-mono text-[13px] tracking-wide text-gray-800 dark:border-emerald-500/25 dark:bg-black/20 dark:text-gray-100"
              @focus="$event.target.select()"
            />
            <button
              class="flex-shrink-0 rounded-md bg-emerald-600 px-2.5 py-1.5 text-[12px] font-medium text-white transition-colors hover:bg-emerald-700"
              @click="copyText(created.password, 'password')"
            >
              {{ copied === 'password' ? '已复制' : '复制' }}
            </button>
          </div>
          <p class="mt-1.5 flex items-start gap-1.5 text-[11px] leading-relaxed text-amber-700 dark:text-amber-400">
            <svg class="mt-px h-3 w-3 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                    d="M12 9v2m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"></path>
            </svg>
            <span>密码只在此刻显示一次，请先复制保存。把链接和密码<b>分两条消息</b>发给对方 —— 只要链接被转发出去，没有密码也进不来。</span>
          </p>
        </div>
      </div>
    </div>

    <!-- ============ 列表 ============ -->
    <div v-else class="space-y-2">
      <div v-if="listLoading" class="py-8 text-center text-[12px] text-gray-400">读取中…</div>
      <div
        v-else-if="listError"
        class="rounded-md bg-red-50 px-2.5 py-2 text-[12px] text-red-700 dark:bg-red-500/10 dark:text-red-300"
      >
        {{ listError }}
      </div>
      <div v-else-if="shares.length === 0" class="py-8 text-center text-[12px] text-gray-400">
        还没有生成过临时链接
      </div>

      <div
        v-for="s in shares"
        v-else
        :key="s.id"
        class="flex flex-wrap items-center gap-2 rounded-lg border border-gray-200/80 bg-white p-2.5 text-[12px] dark:border-white/[0.06] dark:bg-white/[0.02]"
      >
        <span
          class="flex-shrink-0 rounded-md px-1.5 py-0.5 text-[11px] ring-1 ring-inset"
          :class="statusMeta(s.status).cls"
        >
          {{ statusMeta(s.status).text }}
        </span>
        <span class="min-w-0 flex-1">
          <!-- nodeName 是后端给的概括（单节点=名字；多节点=「A 等 3 个节点」），
               完整清单放在 title 里，鼠标悬停可核对具体是哪几台。 -->
          <span
            class="font-medium text-gray-800 dark:text-gray-200"
            :title="(s.nodes || []).map(n => n.name || n.id || n.url).join('\n')"
          >{{ s.nodeName || s.nodeId || s.nodeUrl }}</span>
          <span v-if="s.nodeCount > 1" class="text-gray-400 dark:text-gray-500"> · {{ s.nodeCount }} 台</span>
          <span v-if="s.note" class="text-gray-500 dark:text-gray-400"> · {{ s.note }}</span>
        </span>
        <span class="font-mono text-[11px] tabular-nums text-gray-400 dark:text-gray-500" :title="`创建 ${fmtTime(s.createdAt)} / 过期 ${fmtTime(s.expiresAt)}`">
          {{ fmtTime(s.expiresAt) }} 过期
        </span>
        <span class="text-[11px] text-gray-400 dark:text-gray-500">用 {{ s.useCount }} 次</span>

        <!-- 链接可重取；密码不可重取，所以这里只给链接的复制入口 -->
        <button
          v-if="s.status === 'active'"
          class="flex-shrink-0 rounded-md px-2 py-1 text-[11px] font-medium text-gray-600 ring-1 ring-inset ring-gray-200 transition-colors hover:bg-gray-50 dark:text-gray-300 dark:ring-white/[0.08] dark:hover:bg-white/[0.06]"
          @click="copyListUrl(s.url, s.id)"
        >
          {{ listCopiedId === s.id ? '已复制' : '复制链接' }}
        </button>
        <button
          v-if="s.status === 'active'"
          :disabled="revokingId === s.id"
          class="flex-shrink-0 rounded-md px-2 py-1 text-[11px] font-medium text-red-600 ring-1 ring-inset ring-red-200 transition-colors hover:bg-red-50 disabled:opacity-50 dark:text-red-400 dark:ring-red-500/25 dark:hover:bg-red-500/10"
          @click="doRevoke(s.id)"
        >
          {{ revokingId === s.id ? '吊销中…' : '吊销' }}
        </button>
      </div>

      <p
        v-if="shares.length"
        class="pt-1 text-[11px] leading-relaxed text-gray-400 dark:text-gray-500"
      >
        临时密码不会保存在服务器上，列表里无法再查看。忘记密码时请吊销这条并重新生成一条。
      </p>
    </div>
  </div>
</template>
