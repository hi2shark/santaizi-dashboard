<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { getPublicNetwork } from '@santaizi/api'
import { AppEmpty } from '@santaizi/ui'
import { useInjectedStatusStore } from '@santaizi/status-core'

echarts.use([LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

const { t } = useI18n()
const store = useInjectedStatusStore()
const selected = ref(0)
const node = ref<HTMLElement>()
const loading = ref(false)
const failed = ref(false)
const empty = ref(false)
let chart: echarts.EChartsType | undefined
let requestSeq = 0

const serverOptions = computed(() =>
  store.servers
    .filter((server) => Number.isFinite(server.id) && server.id > 0)
    .map((server) => ({ id: server.id, name: server.name || `#${server.id}` })),
)

const showChart = computed(() => Boolean(selected.value) && !failed.value && !empty.value && !loading.value)
const showPlaceholder = computed(() => !showChart.value)

const placeholderIcon = computed(() => {
  if (failed.value) return 'ri-error-warning-line'
  if (loading.value) return 'ri-loader-4-line'
  if (!serverOptions.value.length) return 'ri-server-line'
  return 'ri-line-chart-line'
})

const placeholderTitle = computed(() => (failed.value ? t('loadFailed') : ''))

const placeholderDesc = computed(() => {
  if (failed.value) return t('requestFailed')
  if (loading.value) return t('loading')
  return t('noData')
})

async function render() {
  const id = selected.value
  if (!id) {
    failed.value = false
    empty.value = !serverOptions.value.length
    chart?.dispose()
    chart = undefined
    return
  }

  const seq = ++requestSeq
  loading.value = true
  failed.value = false
  empty.value = false
  try {
    const rows = (await getPublicNetwork(id)).data
    if (seq !== requestSeq) return
    if (!rows.length) {
      empty.value = true
      chart?.dispose()
      chart = undefined
      return
    }
    loading.value = false
    await nextTick()
    if (!node.value || seq !== requestSeq) return
    chart?.dispose()
    chart = echarts.init(node.value)
    const root = document.documentElement
    const isDark = root.classList.contains('dark') || root.dataset.theme === 'dark'
    const muted = getComputedStyle(root).getPropertyValue('--sz-text-muted').trim() || (isDark ? '#a8b2c3' : '#667085')
    const border = getComputedStyle(root).getPropertyValue('--sz-border').trim() || (isDark ? '#263449' : '#e4e9f2')
    chart.setOption({
      backgroundColor: 'transparent',
      tooltip: { trigger: 'axis' },
      legend: {
        top: 8,
        textStyle: { color: muted },
      },
      grid: { left: 48, right: 24, top: 56, bottom: 36 },
      xAxis: {
        type: 'time',
        axisLabel: { color: muted },
        axisLine: { lineStyle: { color: border } },
      },
      yAxis: {
        type: 'value',
        name: 'ms',
        nameTextStyle: { color: muted },
        axisLabel: { color: muted },
        splitLine: { lineStyle: { color: border } },
      },
      series: rows.map((row, index) => ({
        name: String(row.monitor_name || `${t('statusNetwork')} ${index + 1}`),
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: ((row.created_at as unknown[]) || []).map((time, i) => [
          time,
          ((row.avg_delay as unknown[]) || [])[i],
        ]),
      })),
    })
  } catch {
    if (seq !== requestSeq) return
    failed.value = true
    chart?.dispose()
    chart = undefined
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

function onResize() {
  chart?.resize()
}

watch(selected, render)
watch(() => store.servers.map((server) => server.id).join(','), () => {
  if (!selected.value && serverOptions.value[0]) {
    selected.value = serverOptions.value[0].id
    return
  }
  if (selected.value && !serverOptions.value.some((server) => server.id === selected.value)) {
    selected.value = serverOptions.value[0]?.id || 0
  }
})

onMounted(async () => {
  if (!store.servers.length && !store.loading) await store.load()
  selected.value = serverOptions.value[0]?.id || 0
  window.addEventListener('resize', onResize)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  chart?.dispose()
})
</script>

<template>
  <div class="status-container">
    <section class="status-panel network-panel">
      <header class="network-header">
        <div class="network-header-title">
          <h1>{{ t('statusNetwork') }}</h1>
        </div>
        <el-select
          v-model="selected"
          class="network-server-select"
          filterable
          :disabled="!serverOptions.length || loading"
          :placeholder="t('name')"
        >
          <el-option
            v-for="server in serverOptions"
            :key="server.id"
            :label="server.name"
            :value="server.id"
          />
        </el-select>
      </header>

      <div class="network-body">
        <div v-show="showChart" ref="node" class="network-chart" />
        <div v-if="showPlaceholder" class="empty-status network-empty">
          <AppEmpty
            :tone="failed ? 'danger' : 'default'"
            :icon="placeholderIcon"
            :title="placeholderTitle"
            :description="placeholderDesc"
          />
          <el-button v-if="failed" type="primary" @click="render">
            <i class="ri-refresh-line"></i>{{ t('refresh') }}
          </el-button>
        </div>
      </div>
    </section>
  </div>
</template>
