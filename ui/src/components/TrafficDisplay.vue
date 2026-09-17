<script setup>
import { ref, onMounted, onUnmounted, h, toRaw, watch } from 'vue'
import { useMotion } from '@vueuse/motion'
import { useAppStore } from '@/stores/app'
import { useNodeTool } from '@/composables/useNodeTool'
import { formatBytes } from '@/helper/unit'
import { ChartBarIcon, ArrowDownIcon, ArrowUpIcon } from '@heroicons/vue/24/outline'
import VueApexCharts from 'vue3-apexcharts'

const appStore = useAppStore()
const {
  selectedNode,
  selectedNodeName,
  selectedNodeLocation,
  hasSelectedNode,
  hasNodeSession,
  getEventSource
} = useNodeTool()

const interfaces = ref({})
const cardRef = ref()

const { apply } = useMotion(cardRef, {
  initial: { opacity: 0, y: 20 },
  enter: { opacity: 1, y: 0, transition: { duration: 500, delay: 500 } }
})

const handleCache = (e) => {
  const data = JSON.parse(e.data)
  for (const ifaceIndex in data) {
    const iface = data[ifaceIndex]
    const ifaceName = iface.InterfaceName
    if (!interfaces.value.hasOwnProperty(ifaceName)) {
      createGraph(ifaceName)
    }
    const localIface = interfaces.value[ifaceName]
    for (const point of data[ifaceIndex].Caches) {
      localIface.receive = point[1]
      localIface.send = point[2]
      updateSerieByInterface(ifaceName, localIface, new Date(point[0] * 1000))
    }
  }
}

const handleTrafficUpdate = (e) => {
  console.log('Received InterfaceTraffic event:', e.data)
  const data = e.data.split(',')
  const ifaceName = data[0]
  const time = data[1]

  if (!interfaces.value.hasOwnProperty(ifaceName)) {
    console.log('Creating new interface graph for:', ifaceName)
    createGraph(ifaceName)
  }

  const iface = interfaces.value[ifaceName]
  iface.receive = data[2]
  iface.send = data[3]
  console.log('Updated interface data:', ifaceName, {receive: iface.receive, send: iface.send})
  updateSerieByInterface(ifaceName, iface, new Date(time * 1000))
}

const createGraph = (interfaceName) => {
  const theRef = ref()
  interfaces.value[interfaceName] = {
    ref: null,
    theComponent: null,
    traffic: {
      receive: null,
      send: null
    },
    lines: [[], []],
    categories: [],
    receive: 0,
    send: 0,
    lastReceive: 0,
    lastSend: 0,
    chartOptions: {
      chart: {
        id: 'interface-' + interfaceName + '-chart',
        foreColor: '#6b7280',
        animations: {
          enabled: true,
          easing: 'linear',
          dynamicAnimation: {
            speed: 1000
          },
          animateGradually: {
            enabled: true,
            delay: 300
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
        theme: 'dark'
      },
      xaxis: {
        range: 10,
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
        labels: {
          formatter: (value) => {
            return formatBytes(value, 1, true)
          },
          style: {
            fontSize: '12px'
          }
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
        width: 2
      },
      fill: {
        type: 'gradient',
        gradient: {
          shadeIntensity: 1,
          opacityFrom: 0.6,
          opacityTo: 0.1,
          stops: [0, 90, 100]
        }
      },
      grid: {
        borderColor: '#374151',
        strokeDashArray: 3
      },
      colors: ['#22c55e', '#3b82f6']
    },
    series: [
      {
        type: 'area',
        name: 'Receive',
        data: []
      },
      {
        type: 'area',
        name: 'Send',
        data: []
      }
    ]
  }
  
  interfaces.value[interfaceName].theComponent = () =>
    h(VueApexCharts, {
      ref: theRef,
      type: 'area',
      options: interfaces.value[interfaceName].chartOptions,
      series: interfaces.value[interfaceName].series,
      height: '200px'
    })
  interfaces.value[interfaceName].ref = theRef
}

const updateSerieByInterface = (interfaceName, iface, date = null) => {
  let nowPointName
  if (date === null) {
    date = new Date()
  }

  nowPointName = date.getHours().toString().padStart(2, '0') +
    ':' + date.getMinutes().toString().padStart(2, '0') +
    ':' + date.getSeconds().toString().padStart(2, '0')

  const categories = iface.categories
  const receiveDatas = iface.lines[0]
  const sendDatas = iface.lines[1]

  if (iface.lastReceive === 0) {
    iface.lastReceive = iface.receive
  }

  if (iface.lastSend === 0) {
    iface.lastSend = iface.send
  }

  const receive = iface.receive - iface.lastReceive
  const send = iface.send - iface.lastSend
  iface.lastReceive = iface.receive
  iface.lastSend = iface.send
  iface.traffic.receive = receive
  iface.traffic.send = send
  
  receiveDatas.push(receive)
  sendDatas.push(send)
  categories.push(nowPointName)
  
  if (receiveDatas.length > 30) {
    interfaces.value[interfaceName].categories = categories.slice(-10)
    interfaces.value[interfaceName].lines[0] = receiveDatas.slice(-10)
    interfaces.value[interfaceName].lines[1] = sendDatas.slice(-10)
  }
  
  const finalCategories = categories.slice(0)
  const finalReceiveDatas = receiveDatas.slice(0)
  const finalSendDatas = sendDatas.slice(0)
  
  if (!iface.ref) return
  
  iface.ref.updateOptions({
    xaxis: {
      categories: toRaw(finalCategories)
    }
  })

  iface.ref.updateSeries([
    {
      name: 'Receive',
      data: toRaw(finalReceiveDatas)
    },
    {
      name: 'Send',
      data: toRaw(finalSendDatas)
    }
  ])
}

// 设置事件监听器
const setupEventListeners = () => {
  console.log('=== TrafficDisplay setupEventListeners ===')
  console.log('selectedNode:', selectedNode.value)
  console.log('hasSelectedNode:', hasSelectedNode.value)
  console.log('hasNodeSession:', hasNodeSession.value)
  
  try {
    cleanupEventListeners()
    
    const source = getEventSource()
    console.log('Successfully got event source:', source)
    console.log('Event source URL:', source.url)
    console.log('Event source readyState:', source.readyState)
    
    source.addEventListener('InterfaceCache', handleCache)
    source.addEventListener('InterfaceTraffic', handleTrafficUpdate)
    
    console.log('Event listeners added successfully')
    console.log('Current interfaces count:', Object.keys(interfaces.value).length)
    
  } catch (error) {
    console.error('ERROR in setupEventListeners:', error)
    console.error('Error message:', error.message)
    console.error('Error stack:', error.stack)
  }
}

// 清理事件监听器
const cleanupEventListeners = () => {
  const source = getEventSource()
  source.removeEventListener('InterfaceCache', handleCache)
  source.removeEventListener('InterfaceTraffic', handleTrafficUpdate)
}

// 监听节点session状态变化
watch(() => hasNodeSession.value, (hasSession) => {
  console.log('=== TrafficDisplay: Node session status changed ===')
  console.log('Has session:', hasSession)
  
  if (hasSession) {
    console.log('Session established, setting up traffic listeners...')
    setupEventListeners()
  }
})

// 监听节点变化，重新设置事件监听
watch(() => selectedNode.value, (newNode, oldNode) => {
  console.log('=== TrafficDisplay: Node changed ===')
  console.log('Old node:', oldNode?.name)
  console.log('New node:', newNode?.name)
  
  if (newNode !== oldNode) {
    console.log('Cleaning up old listeners...')
    // 清理旧的监听器
    try {
      cleanupEventListeners()
    } catch (error) {
      console.warn('Error cleaning up listeners:', error)
    }
    
    // 清空现有的接口数据
    interfaces.value = {}
    console.log('Cleared interfaces data')
    
    // 新的监听器会在hasNodeSession变化时设置
  }
})

onMounted(() => {
  console.log('TrafficDisplay mounted, current selectedNode:', selectedNode.value)
  console.log('Current interfaces before setup:', interfaces.value)
  setupEventListeners()
  apply()
})

onUnmounted(() => {
  cleanupEventListeners()
})
</script>

<template>
  <div ref="cardRef">
    <div>
      <div v-if="Object.keys(interfaces).length === 0" class="text-center py-12">
        <div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-xl bg-gray-50 dark:bg-white/[0.03] ring-1 ring-inset ring-gray-200/70 dark:ring-white/[0.06]">
          <ChartBarIcon class="w-7 h-7 text-gray-300 dark:text-gray-600" />
        </div>
        <h3 class="text-[13px] font-medium text-gray-700 dark:text-gray-300 mb-1">No Traffic Data</h3>
        <p class="text-[12px] text-gray-400 dark:text-gray-500">Waiting for network interface data...</p>
      </div>
      
      <!-- === CUSTOM START: 接口流量卡视觉 - By ASxiaowen === -->
      <!-- 理由: 圆角卡片 + 阴影、Active 状态 pill + ping 圆点、20px 等宽数字；
             原文件是平铺无卡片的列表；卡片阴影类在 custom_components/theme.css -->
      <div v-else :class="Object.keys(interfaces).length === 1 ? 'block' : 'grid grid-cols-1 xl:grid-cols-2 gap-5'">
        <div 
          v-for="(interfaceData, interfaceName) in interfaces" 
          :key="interfaceName"
          class="rounded-xl border border-gray-200/80 dark:border-white/[0.06] bg-white dark:bg-white/[0.02] p-4 md:p-5 shadow-card transition-all duration-300 hover:shadow-soft dark:hover:border-white/[0.1]"
          :class="Object.keys(interfaces).length === 1 ? 'mx-auto max-w-lg' : ''"
        >
          <div class="flex items-center justify-between mb-4">
            <h3 class="font-mono text-[13px] font-semibold text-gray-900 dark:text-white">{{ interfaceName }}</h3>
            <span class="inline-flex items-center gap-1.5 rounded-md bg-emerald-50 dark:bg-emerald-500/10 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:text-emerald-400">
              <span class="relative flex h-1.5 w-1.5">
                <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-70"></span>
                <span class="relative inline-flex h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
              </span>
              Active
            </span>
          </div>
          
          <!-- Stats -->
          <div :class="Object.keys(interfaces).length === 1 ? 'flex justify-center gap-16 mb-6' : 'grid grid-cols-2 gap-4 mb-6'">
            <div class="text-center">
              <div class="flex items-center justify-center space-x-2 mb-2">
                <span class="flex h-6 w-6 items-center justify-center rounded-md bg-emerald-50 dark:bg-emerald-500/10">
                  <ArrowDownIcon class="w-3.5 h-3.5 text-emerald-600 dark:text-emerald-400" />
                </span>
                <span class="text-[11px] font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {{ $t('server_bandwidth_graph_receive') }}
                </span>
              </div>
              <div class="space-y-1">
                <p class="text-[20px] font-semibold leading-none tabular-nums text-emerald-600 dark:text-emerald-400">
                  {{ formatBytes(interfaceData.traffic?.receive || 0, 1, true) }}
                </p>
                <p class="text-[11px] tabular-nums text-gray-400 dark:text-gray-500">
                  Total: {{ formatBytes(interfaceData.receive || 0) }}
                </p>
              </div>
            </div>
            
            <div class="text-center">
              <div class="flex items-center justify-center space-x-2 mb-2">
                <span class="flex h-6 w-6 items-center justify-center rounded-md bg-blue-50 dark:bg-blue-500/10">
                  <ArrowUpIcon class="w-3.5 h-3.5 text-blue-600 dark:text-blue-400" />
                </span>
                <span class="text-[11px] font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {{ $t('server_bandwidth_graph_sended') }}
                </span>
              </div>
              <div class="space-y-1">
                <p class="text-[20px] font-semibold leading-none tabular-nums text-blue-600 dark:text-blue-400">
                  {{ formatBytes(interfaceData.traffic?.send || 0, 1, true) }}
                </p>
                <p class="text-[11px] tabular-nums text-gray-400 dark:text-gray-500">
                  Total: {{ formatBytes(interfaceData.send || 0) }}
                </p>
              </div>
            </div>
          </div>
          
          <!-- Chart -->
          <div>
            <component :is="interfaceData.theComponent" />
          </div>
        </div>
      </div>
      <!-- === CUSTOM END: 接口流量卡视觉 === -->
    </div>
  </div>
</template>

