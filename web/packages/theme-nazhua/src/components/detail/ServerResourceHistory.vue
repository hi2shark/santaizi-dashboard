<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getPublicMetrics, type PublicMetricPoint, type ServerRecord } from '@santaizi/api'
import { formatSpeed } from '@santaizi/theme-server-status'
import { AppEmpty } from '@santaizi/ui'
import { percentOf, toNazhuaServerView } from '../../domain/nazhuaServerView'
import ResourceHistoryChart, { type HistorySeries, type HistoryUnit } from './ResourceHistoryChart.vue'

const COLORS = {
  cpu: '#4e90ff',
  memory: '#27c975',
  disk: '#22d3ee',
  process: '#f5b199',
  netIn: '#f5b199',
  netOut: '#89c3eb',
  tcp: '#89c3eb',
  udp: '#4e90ff',
}

type HistoryCard = {
  key: string
  title: string
  unit: HistoryUnit
  summary?: string
  detail?: string
  metrics?: Array<{ key: string; label: string; value: string; color: string }>
  series: HistorySeries[]
}

const props = defineProps<{ server: ServerRecord }>()
const { t } = useI18n()
const loading = ref(true)
const failed = ref(false)
const empty = ref(false)
const points = ref<PublicMetricPoint[]>([])
let requestSeq = 0

const view = computed(() => toNazhuaServerView(props.server))

function formatPercent(value: number) {
  return `${Number(value).toFixed(1)}%`
}

function seriesOf(rows: PublicMetricPoint[], name: string, color: string, pick: (row: PublicMetricPoint) => number): HistorySeries {
  return {
    name,
    color,
    data: rows.map(row => [row.window_start || '', pick(row)]),
  }
}

const cards = computed<HistoryCard[]>(() => {
  const rows = points.value
  const current = view.value
  const memTotal = current.memoryTotal
  const diskTotal = current.diskTotal
  const memUnit: HistoryUnit = memTotal > 0 ? 'percent' : 'bytes'
  const diskUnit: HistoryUnit = diskTotal > 0 ? 'percent' : 'bytes'
  return [
    {
      key: 'cpu',
      title: t('nazhua.historyCpu'),
      unit: 'percent',
      summary: formatPercent(current.cpuPercent),
      series: [seriesOf(rows, 'CPU', COLORS.cpu, row => Number(row.cpu || 0))],
    },
    {
      key: 'memory',
      title: t('nazhua.historyMemory'),
      unit: memUnit,
      summary: formatPercent(current.memoryPercent),
      detail: current.memoryText,
      series: [seriesOf(rows, t('nazhua.memory'), COLORS.memory, row => (
        memTotal > 0 ? percentOf(Number(row.mem_used || 0), memTotal) : Number(row.mem_used || 0)
      ))],
    },
    {
      key: 'disk',
      title: t('nazhua.historyDisk'),
      unit: diskUnit,
      summary: formatPercent(current.diskPercent),
      detail: current.diskText,
      series: [seriesOf(rows, t('nazhua.disk'), COLORS.disk, row => (
        diskTotal > 0 ? percentOf(Number(row.disk_used || 0), diskTotal) : Number(row.disk_used || 0)
      ))],
    },
    {
      key: 'process',
      title: t('nazhua.historyProcess'),
      unit: 'count',
      summary: String(Math.round(current.processCount)),
      series: [seriesOf(rows, t('nazhua.processCount'), COLORS.process, row => Number(row.process_count || 0))],
    },
    {
      key: 'net',
      title: t('nazhua.historyNet'),
      unit: 'speed',
      metrics: [
        { key: 'in', label: t('nazhua.download'), value: formatSpeed(current.speedIn), color: COLORS.netIn },
        { key: 'out', label: t('nazhua.upload'), value: formatSpeed(current.speedOut), color: COLORS.netOut },
      ],
      series: [
        seriesOf(rows, t('nazhua.download'), COLORS.netIn, row => Number(row.net_in_speed || 0)),
        seriesOf(rows, t('nazhua.upload'), COLORS.netOut, row => Number(row.net_out_speed || 0)),
      ],
    },
    {
      key: 'conn',
      title: t('nazhua.historyConn'),
      unit: 'count',
      metrics: [
        { key: 'tcp', label: t('nazhua.tcpConn'), value: String(Math.round(current.tcpConnCount)), color: COLORS.tcp },
        { key: 'udp', label: t('nazhua.udpConn'), value: String(Math.round(current.udpConnCount)), color: COLORS.udp },
      ],
      series: [
        seriesOf(rows, t('nazhua.tcpConn'), COLORS.tcp, row => Number(row.tcp_conn_count || 0)),
        seriesOf(rows, t('nazhua.udpConn'), COLORS.udp, row => Number(row.udp_conn_count || 0)),
      ],
    },
  ]
})

async function load() {
  if (!props.server.id) return
  const seq = ++requestSeq
  loading.value = true
  failed.value = false
  empty.value = false
  try {
    const result = await getPublicMetrics(props.server.id, { resolution: '1m', hours: 24 })
    if (seq !== requestSeq) return
    points.value = result.data || []
    empty.value = !points.value.length
  } catch {
    if (seq !== requestSeq) return
    failed.value = true
    points.value = []
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

watch(() => props.server.id, load, { immediate: true })
</script>

<template>
  <section class="nazhua-history">
    <h2>{{ t('nazhua.resourceHistory') }}</h2>
    <div v-if="failed || empty || loading" class="nazhua-monitor__empty">
      <AppEmpty
        :tone="failed ? 'danger' : 'default'"
        :icon="failed ? 'ri-error-warning-line' : loading ? 'ri-loader-4-line' : 'ri-line-chart-line'"
        :title="failed ? t('nazhua.loadFailed') : ''"
        :description="t(failed ? 'nazhua.requestFailed' : loading ? 'nazhua.loading' : 'nazhua.noData')"
      />
      <button v-if="failed" type="button" @click="load"><i class="ri-refresh-line"></i>{{ t('nazhua.refresh') }}</button>
    </div>
    <div v-else class="nazhua-history__grid">
      <article v-for="card in cards" :key="card.key" class="nazhua-history__card">
        <header>
          <strong>{{ card.title }}</strong>
          <div v-if="card.metrics" class="nazhua-history__metrics">
            <span
              v-for="metric in card.metrics"
              :key="metric.key"
              class="nazhua-history__metric"
              :style="{ '--metric-color': metric.color }"
            >
              <em></em>
              <span>{{ metric.label }}</span>
              <b>{{ metric.value }}</b>
            </span>
          </div>
          <div v-else class="nazhua-history__summary">
            <span>{{ card.summary }}</span>
            <small v-if="card.detail">{{ card.detail }}</small>
          </div>
        </header>
        <ResourceHistoryChart :series="card.series" :unit="card.unit" />
      </article>
    </div>
  </section>
</template>
