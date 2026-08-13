<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { getPublicMetrics, type PublicMetricPoint } from '@santaizi/api'
import { AppEmpty } from '@santaizi/ui'
import { formatCompactBytes } from '../../domain/nazhuaServerView'

echarts.use([LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

type Tab = 'cpu' | 'memory' | 'disk' | 'net'

const props = defineProps<{ serverId: number }>()
const { t } = useI18n()
const node = ref<HTMLElement>()
const loading = ref(false)
const failed = ref(false)
const empty = ref(false)
const tab = ref<Tab>('cpu')
const points = ref<PublicMetricPoint[]>([])
let chart: echarts.ECharts | undefined
let requestSeq = 0

const tabs: Array<{ key: Tab; label: string }> = [
  { key: 'cpu', label: 'nazhua.historyCpu' },
  { key: 'memory', label: 'nazhua.historyMemory' },
  { key: 'disk', label: 'nazhua.historyDisk' },
  { key: 'net', label: 'nazhua.historyNet' },
]

function seriesFor(kind: Tab, rows: PublicMetricPoint[]) {
  if (kind === 'cpu') {
    return [{
      name: 'CPU',
      type: 'line' as const,
      smooth: true,
      showSymbol: false,
      data: rows.map(row => [row.window_start, Number(row.cpu || 0)]),
    }]
  }
  if (kind === 'memory') {
    return [{
      name: t('nazhua.memory'),
      type: 'line' as const,
      smooth: true,
      showSymbol: false,
      data: rows.map(row => [row.window_start, Number(row.mem_used || 0)]),
    }]
  }
  if (kind === 'disk') {
    return [{
      name: t('nazhua.disk'),
      type: 'line' as const,
      smooth: true,
      showSymbol: false,
      data: rows.map(row => [row.window_start, Number(row.disk_used || 0)]),
    }]
  }
  return [
    {
      name: t('nazhua.download'),
      type: 'line' as const,
      smooth: true,
      showSymbol: false,
      data: rows.map(row => [row.window_start, Number(row.net_in_speed || 0)]),
    },
    {
      name: t('nazhua.upload'),
      type: 'line' as const,
      smooth: true,
      showSymbol: false,
      data: rows.map(row => [row.window_start, Number(row.net_out_speed || 0)]),
    },
  ]
}

function yName(kind: Tab) {
  if (kind === 'cpu') return '%'
  if (kind === 'net') return 'B/s'
  return 'B'
}

function formatAxis(kind: Tab, value: number) {
  if (kind === 'cpu') return `${Number(value).toFixed(1)}%`
  return formatCompactBytes(Number(value), 1) + (kind === 'net' ? '/s' : '')
}

function draw() {
  if (!node.value || empty.value || failed.value) return
  chart?.dispose()
  chart = echarts.init(node.value)
  const kind = tab.value
  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      valueFormatter: (value: unknown) => formatAxis(kind, Number(value || 0)),
    },
    legend: { top: 8 },
    grid: { left: 56, right: 24, top: 56, bottom: 36 },
    xAxis: { type: 'time' },
    yAxis: {
      type: 'value',
      name: yName(kind),
      axisLabel: { formatter: (value: number) => formatAxis(kind, value) },
    },
    series: seriesFor(kind, points.value),
  })
}

async function load() {
  if (!props.serverId) return
  const seq = ++requestSeq
  loading.value = true
  failed.value = false
  empty.value = false
  try {
    const result = await getPublicMetrics(props.serverId, { resolution: '1m', hours: 24 })
    if (seq !== requestSeq) return
    points.value = result.data || []
    if (!points.value.length) {
      empty.value = true
      chart?.dispose()
      chart = undefined
      return
    }
    loading.value = false
    await nextTick()
    if (seq !== requestSeq) return
    draw()
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

watch(() => props.serverId, load, { immediate: true })
watch(tab, async () => {
  if (empty.value || failed.value || loading.value || !points.value.length) return
  await nextTick()
  draw()
})
onMounted(() => window.addEventListener('resize', onResize))
onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  chart?.dispose()
})
</script>

<template>
  <section class="nazhua-history">
    <header class="nazhua-history__head">
      <h2>{{ t('nazhua.resourceHistory') }}</h2>
      <el-button-group>
        <el-button
          v-for="item in tabs"
          :key="item.key"
          :type="tab === item.key ? 'primary' : 'default'"
          @click="tab = item.key"
        >{{ t(item.label) }}</el-button>
      </el-button-group>
    </header>
    <div v-show="!failed && !empty && !loading" ref="node" class="nazhua-history__chart" />
    <div v-if="failed || empty || loading" class="nazhua-monitor__empty">
      <AppEmpty
        :tone="failed ? 'danger' : 'default'"
        :icon="failed ? 'ri-error-warning-line' : loading ? 'ri-loader-4-line' : 'ri-line-chart-line'"
        :title="failed ? t('nazhua.loadFailed') : ''"
        :description="t(failed ? 'nazhua.requestFailed' : loading ? 'nazhua.loading' : 'nazhua.noData')"
      />
      <button v-if="failed" type="button" @click="load"><i class="ri-refresh-line"></i>{{ t('nazhua.refresh') }}</button>
    </div>
  </section>
</template>
