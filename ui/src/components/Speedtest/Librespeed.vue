<script setup>
import { ref, onMounted, toRaw, computed } from 'vue'
import { useMotion } from '@vueuse/motion'
import { useAppStore } from '@/stores/app'
import { useNodesStore } from '@/stores/nodes'
import { useNodeTool } from '@/composables/useNodeTool'
import { PlayIcon, StopIcon, ArrowDownIcon, ArrowUpIcon, ChartBarIcon } from '@heroicons/vue/24/outline'
import VueApexCharts from 'vue3-apexcharts'
// === CUSTOM START: 测速 URL 携带访问令牌 - By ASxiaowen ===
// 理由: 见 startTest 中 sessionUrl 处的说明；令牌附加逻辑集中在 authState。
import { appendToken } from '@/custom_components/authState'
// === CUSTOM END: 测速 URL 携带访问令牌 ===

const appStore = useAppStore()
const containerRef = ref()

const {
  selectedNode,
} = useNodeTool()

// 获取当前会话ID和基础URL
const currentSessionId = computed(() => {
  if (selectedNode.value) {
    // 使用选定节点的session
    const nodesStore = useNodesStore()
    return nodesStore.selectedNodeSession
  }
  // 回退到当前节点的session
  return appStore.sessionId
})

const baseUrl = computed(() => {
  if (selectedNode.value) {
    return selectedNode.value.url
  }
  return ''
})

const { apply } = useMotion(containerRef, {
  initial: { opacity: 0, y: 20 },
  enter: { opacity: 1, y: 0, transition: { type: 'spring', stiffness: 100, damping: 15 } }
})

let workerInstance = null
let workerTimer = null
const working = ref(false)
const uploadText = ref('0')
const downloadText = ref('0')
const chartDownloadRef = ref()
const chartUploadRef = ref()

const gaugeValue = (value) => {
  const val = parseFloat(value)
  if (val < 1) return val * 100
  if (val < 10) return val * 10
  return Math.min(val, 1000)
}

const downloadGauge = computed(() => gaugeValue(downloadText.value))
const uploadGauge = computed(() => gaugeValue(uploadText.value))

const baseChartOptions = {
  chart: {
    height: 120,
    foreColor: '#6b7280',
    animations: {
      enabled: true,
      easing: 'linear',
      dynamicAnimation: {
        speed: 300
      }
    },
    zoom: {
      enabled: false
    },
    toolbar: {
      show: false
    },
    background: 'transparent'
  },
  tooltip: {
    enabled: false
  },
  xaxis: {
    type: 'category',
    categories: [''],
    labels: {
      show: false
    },
    axisBorder: {
      show: false
    },
    axisTicks: {
      show: false
    }
  },
  yaxis: {
    min: 0,
    labels: {
      show: true,
      // === CUSTOM START: 纵轴单位格式 - By ASxiaowen ===
      // 理由: 上游默认格式带多位小数且无单位，这里统一取整并补 Mbps
      formatter: (value) => Math.round(value) + 'Mbps',
      // === CUSTOM END: 纵轴单位格式 ===
      style: {
        fontSize: '11px'
      }
    },
    axisBorder: {
      show: false
    },
    axisTicks: {
      show: false
    }
  },
  dataLabels: {
    enabled: false
  },
  markers: {
    size: 0
  },
  stroke: {
    curve: 'smooth',
    width: 3
  },
  fill: {
    type: 'gradient',
    gradient: {
      shadeIntensity: 1,
      opacityFrom: 0.7,
      opacityTo: 0.1,
      stops: [0, 90, 100]
    }
  },
  grid: {
    show: false
  }
}

const baseSeries = {
  type: 'area',
  name: 'Speed',
  data: []
}

const charts = ref({
  download: {
    ref: null,
    options: { 
      ...baseChartOptions,
      colors: ['#0ea5e9'],
      fill: {
        ...baseChartOptions.fill,
        gradient: {
          ...baseChartOptions.fill.gradient,
          colorStops: [
            { offset: 0, color: '#0ea5e9', opacity: 0.8 },
            { offset: 100, color: '#0ea5e9', opacity: 0.1 }
          ]
        }
      }
    },
    series: [{ ...baseSeries, name: 'Download' }],
    data: [],
    categories: []
  },
  upload: {
    ref: null,
    options: { 
      ...baseChartOptions,
      colors: ['#3b82f6'],
      fill: {
        ...baseChartOptions.fill,
        gradient: {
          ...baseChartOptions.fill.gradient,
          colorStops: [
            { offset: 0, color: '#3b82f6', opacity: 0.8 },
            { offset: 100, color: '#3b82f6', opacity: 0.1 }
          ]
        }
      }
    },
    series: [{ ...baseSeries, name: 'Upload' }],
    data: [],
    categories: []
  }
})

// Generate unique chart IDs
const generateChartIds = () => {
  for (const chartId in charts.value) {
    charts.value[chartId].options.chart.id = 
      'chart-librespeed-' + chartId + '-' + (Math.random() + 1).toString(36).substring(2)
  }
}

const errorMessage = ref('')

const startOrStopSpeedtest = (force = false) => {
  if (workerInstance != null) {
    workerInstance.postMessage('abort')
    clearInterval(workerTimer)
    workerInstance.terminate()
    workerInstance = null

    if (force) {
      uploadText.value = '0'
      downloadText.value = '0'
      // Reset charts
      charts.value.download.data = []
      charts.value.download.categories = []
      charts.value.upload.data = []
      charts.value.upload.categories = []
    }
    working.value = false
    return
  }

  // 验证 session 是否有效
  if (!currentSessionId.value) {
    errorMessage.value = 'Session not ready, please wait...'
    setTimeout(() => { errorMessage.value = '' }, 3000)
    return
  }

  errorMessage.value = ''
  working.value = true
  uploadText.value = '0'
  downloadText.value = '0'

  // Reset chart data
  charts.value.download.data = []
  charts.value.download.categories = []
  charts.value.upload.data = []
  charts.value.upload.categories = []

  // Worker 必须从本地加载（同源限制），不能从远程节点加载
  const workerUrl = `${window.location.origin}/speedtest_worker.js`

  try {
    workerInstance = new Worker(workerUrl)
  } catch (err) {
    console.error('Failed to create speedtest worker:', err)
    errorMessage.value = 'Failed to load speedtest worker'
    working.value = false
    setTimeout(() => { errorMessage.value = '' }, 3000)
    return
  }

  workerInstance.onerror = (err) => {
    console.error('Worker error:', err)
    errorMessage.value = 'Speedtest worker error'
    working.value = false
    if (workerTimer) clearInterval(workerTimer)
    workerInstance = null
    setTimeout(() => { errorMessage.value = '' }, 3000)
  }

  workerInstance.onmessage = (e) => {
    const nowPointName = new Date().toLocaleTimeString()
    const data = JSON.parse(e.data)
    const status = data.testState

    if (status >= 4) {
      return startOrStopSpeedtest(false)
    }

    if (status === 1 && data.dlStatus === 0) {
      downloadText.value = '0'
      uploadText.value = '0'
    }

    if (data.ulStatus) {
      uploadText.value = data.ulStatus
      charts.value.upload.data.push(parseFloat(data.ulStatus))
      charts.value.upload.categories.push(nowPointName)

      if (chartUploadRef.value) {
        chartUploadRef.value.updateOptions({
          xaxis: {
            categories: toRaw(charts.value.upload.categories)
          }
        })
        chartUploadRef.value.updateSeries([{
          name: 'Upload',
          data: toRaw(charts.value.upload.data)
        }])
      }
      return
    }

    if (data.dlStatus) {
      downloadText.value = data.dlStatus
      charts.value.download.data.push(parseFloat(data.dlStatus))
      charts.value.download.categories.push(nowPointName)

      if (chartDownloadRef.value) {
        chartDownloadRef.value.updateOptions({
          xaxis: {
            categories: toRaw(charts.value.download.categories)
          }
        })
        chartDownloadRef.value.updateSeries([{
          name: 'Download',
          data: toRaw(charts.value.download.data)
        }])
      }
    }
  }

  // 构造 session URL - 使用绝对路径
  // === CUSTOM START: 测速 URL 携带访问令牌 - By ASxiaowen ===
  // 理由: speedtest worker 用 XHR 直连节点，无法复用 apiClient 的请求头注入；
  //       下行是 GET、上行是带 Content-Encoding 的 POST，令牌只能走查询参数。
  //       后端 ExtractToken 支持 ?token=，因此这里统一附加。
  const rawSessionUrl = baseUrl.value
    ? `${baseUrl.value}/session/${currentSessionId.value}`
    : `${window.location.origin}/session/${currentSessionId.value}`
  const sessionUrl = appendToken(rawSessionUrl, null)
  // === CUSTOM END: 测速 URL 携带访问令牌 ===

  workerInstance.postMessage(
    'start ' + JSON.stringify({
      test_order: 'D_U',
      url_dl: `${sessionUrl}/speedtest/download`,
      url_ul: `${sessionUrl}/speedtest/upload`,
      url_ping: `${sessionUrl}/speedtest/upload`,
      url_getIp: '',  // 禁用 IP 查询，后端没有这个接口
      getIp_ispInfo: false
    })
  )

  workerTimer = setInterval(() => {
    workerInstance.postMessage('status')
  }, 200)
}

onMounted(() => {
  apply()
  generateChartIds()
})
</script>

<template>
  <div ref="containerRef" class="space-y-5 max-w-4xl mx-auto">
    <!-- Results Display -->
    <!-- === CUSTOM START: 上下行结果卡视觉 - By ASxiaowen === -->
    <!-- 理由: 圆角卡片 + 顶部高光模糊点 + 渐变图标方块 + 26px 等宽数字，
           原文件是两块朴素面板；卡片阴影类在 custom_components/theme.css -->
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <!-- Download -->
      <div class="group relative overflow-hidden rounded-xl border border-gray-200/80 dark:border-white/[0.06] bg-white dark:bg-white/[0.02] p-4 shadow-card transition-all duration-300 hover:shadow-soft dark:hover:border-white/[0.1]">
        <div class="pointer-events-none absolute -right-8 -top-8 h-24 w-24 rounded-full bg-primary-400/10 blur-2xl"></div>
        <div class="flex items-center space-x-3 mb-3">
          <div class="flex items-center justify-center w-9 h-9 rounded-lg bg-gradient-to-br from-primary-500 to-sky-400 text-white shadow-sm flex-shrink-0">
            <ArrowDownIcon class="w-[18px] h-[18px]" />
          </div>
          <div class="min-w-0">
            <h4 class="text-[13px] font-medium text-gray-500 dark:text-gray-400">下行</h4>
            <p class="flex items-baseline gap-1">
              <span class="text-[26px] font-semibold leading-none tabular-nums tracking-tight text-primary-600 dark:text-primary-400">{{ downloadText }}</span>
              <span class="text-[11px] font-medium text-gray-400 dark:text-gray-500">Mbps</span>
            </p>
          </div>
        </div>
        <VueApexCharts
          type="area"
          ref="chartDownloadRef"
          :options="charts.download.options"
          :series="charts.download.series"
        />
      </div>

      <!-- Upload -->
      <div class="group relative overflow-hidden rounded-xl border border-gray-200/80 dark:border-white/[0.06] bg-white dark:bg-white/[0.02] p-4 shadow-card transition-all duration-300 hover:shadow-soft dark:hover:border-white/[0.1]">
        <div class="pointer-events-none absolute -right-8 -top-8 h-24 w-24 rounded-full bg-blue-400/10 blur-2xl"></div>
        <div class="flex items-center space-x-3 mb-3">
          <div class="flex items-center justify-center w-9 h-9 rounded-lg bg-gradient-to-br from-blue-500 to-indigo-400 text-white shadow-sm flex-shrink-0">
            <ArrowUpIcon class="w-[18px] h-[18px]" />
          </div>
          <div class="min-w-0">
            <h4 class="text-[13px] font-medium text-gray-500 dark:text-gray-400">上行</h4>
            <p class="flex items-baseline gap-1">
              <span class="text-[26px] font-semibold leading-none tabular-nums tracking-tight text-blue-600 dark:text-blue-400">{{ uploadText }}</span>
              <span class="text-[11px] font-medium text-gray-400 dark:text-gray-500">Mbps</span>
            </p>
          </div>
        </div>
        <VueApexCharts
          type="area"
          ref="chartUploadRef"
          :options="charts.upload.options"
          :series="charts.upload.series"
        />
      </div>
    </div>
    <!-- === CUSTOM END: 上下行结果卡视觉 === -->

    <!-- Error Message -->
    <div v-if="errorMessage" class="text-center">
      <p class="text-red-500 dark:text-red-400 text-sm">{{ errorMessage }}</p>
    </div>

    <!-- Control Button -->
    <!-- === CUSTOM START: 开始/停止按钮视觉 - By ASxiaowen === -->
    <!-- 理由: 白色描边按钮 + hover 抬升，与全局卡片风格统一 -->
    <div class="text-center">
      <button
        @click="startOrStopSpeedtest"
        :disabled="!currentSessionId && !working"
        class="inline-flex items-center px-5 py-2 rounded-lg text-[13px] font-medium transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
        :class="working
          ? 'bg-red-500 hover:bg-red-600 text-white shadow-soft'
          : 'border border-gray-200 dark:border-white/[0.08] bg-white dark:bg-white/[0.03] text-gray-700 dark:text-gray-200 shadow-sm hover:-translate-y-0.5 hover:border-primary-300 dark:hover:border-primary-500/40 hover:text-primary-600 dark:hover:text-primary-400 hover:shadow-soft'"
      >
        <component :is="working ? StopIcon : PlayIcon" class="w-4 h-4 mr-2" />
        <span v-if="working" class="flex items-center">
          <div class="w-3.5 h-3.5 border-2 border-white border-t-transparent rounded-full animate-spin mr-2"></div>
          Stop
        </span>
        <span v-else>开始测试</span>
      </button>
    </div>
    <!-- === CUSTOM END: 开始/停止按钮视觉 === -->
  </div>
</template>
