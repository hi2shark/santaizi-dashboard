<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { DataZoomComponent, GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { AppEmpty } from '@santaizi/ui'
import { loadNetworkHistory } from '../domain/networkHistoryCache'

echarts.use([LineChart, DataZoomComponent, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

const props = defineProps<{ serverId: number; hideHeading?: boolean }>()
const { t } = useI18n()
const node = ref<HTMLElement>()
const loading = ref(false)
const failed = ref(false)
const empty = ref(false)
let chart: echarts.ECharts | undefined
let requestSeq = 0
let resizeObserver: ResizeObserver | undefined

async function render() {
  if (!props.serverId) return
  const seq = ++requestSeq
  loading.value = true
  failed.value = false
  empty.value = false
  try {
    const rows = await loadNetworkHistory(props.serverId)
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
    observeChart()
    const root = document.documentElement
    const isDark = root.classList.contains('dark') || root.dataset.theme === 'dark'
    const muted = getComputedStyle(root).getPropertyValue('--svc-muted').trim()
      || getComputedStyle(root).getPropertyValue('--sz-text-muted').trim()
      || (isDark ? '#a8b2c3' : '#667085')
    const border = getComputedStyle(root).getPropertyValue('--svc-border').trim()
      || getComputedStyle(root).getPropertyValue('--sz-border').trim()
      || (isDark ? '#263449' : '#e4e9f2')
    const narrow = (node.value.clientWidth || 0) < 520
    chart.setOption({
      backgroundColor: 'transparent',
      tooltip: { trigger: 'axis' },
      legend: { top: 4, textStyle: { color: muted, fontSize: 12 } },
      grid: { left: narrow ? 36 : 48, right: narrow ? 12 : 16, top: 40, bottom: 48 },
      dataZoom: [{ type: 'inside' }, { type: 'slider', height: 18, bottom: 8 }],
      xAxis: { type: 'time', axisLabel: { color: muted }, axisLine: { lineStyle: { color: border } } },
      yAxis: { type: 'value', name: 'ms', nameTextStyle: { color: muted }, axisLabel: { color: muted }, splitLine: { lineStyle: { color: border } } },
      series: rows.map((row, index) => ({
        name: String(row.monitor_name || `${t('nazhua.networkMonitor')} ${index + 1}`),
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: ((row.created_at as unknown[]) || []).map((time, i) => [time, ((row.avg_delay as unknown[]) || [])[i]]),
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

function observeChart() {
  resizeObserver?.disconnect()
  if (node.value && typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(() => { chart?.resize() })
    resizeObserver.observe(node.value)
  }
}

watch(() => props.serverId, render)
onMounted(() => {
  window.addEventListener('resize', onResize)
  observeChart()
  void render()
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  resizeObserver?.disconnect()
  chart?.dispose()
})
</script>

<template>
  <section class="ss-latency">
    <h3 v-if="!hideHeading">{{ t('nazhua.networkMonitor') }}</h3>
    <div v-show="!failed && !empty && !loading" ref="node" class="ss-latency__chart" />
    <div v-if="failed || empty || loading" class="ss-latency__empty">
      <AppEmpty
        :tone="failed ? 'danger' : 'default'"
        :icon="failed ? 'ri-error-warning-line' : loading ? 'ri-loader-4-line' : 'ri-line-chart-line'"
        :title="failed ? t('loadFailed') : ''"
        :description="t(failed ? 'requestFailed' : loading ? 'loading' : 'noData')"
      />
      <button v-if="failed" type="button" @click="render">
        <i class="ri-refresh-line"></i>{{ t('refresh') }}
      </button>
    </div>
  </section>
</template>
