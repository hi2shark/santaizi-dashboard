<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ServerRecord } from '@santaizi/api'
import { formatPercent, stateValue } from '../../utils/host'
import { formatCompactBytes } from '../../domain/nazhuaServerView'
import DonutChart from '../charts/DonutChart.vue'

const props = defineProps<{ server: ServerRecord }>()
const { t } = useI18n()

const items = computed(() => {
  const state = props.server.state || {}
  const memUsed = stateValue(state, 'MemUsed', 'mem_used')
  const memTotal = stateValue(state, 'MemTotal', 'mem_total')
  const diskUsed = stateValue(state, 'DiskUsed', 'disk_used')
  const diskTotal = stateValue(state, 'DiskTotal', 'disk_total')
  return [
    { key: 'cpu', label: 'CPU', percent: Number(formatPercent(stateValue(state, 'CPU', 'Cpu', 'cpu')).replace('%', '')), value: '', content: `${Number(props.server.host?.CPU || 0) || '—'}C` },
    { key: 'mem', label: t('nazhua.memory'), percent: Number(formatPercent(memUsed, memTotal || undefined).replace('%', '')), value: formatCompactBytes(memUsed), content: `${t('nazhua.memory')}${formatCompactBytes(memTotal)}` },
    { key: 'disk', label: t('nazhua.disk'), percent: Number(formatPercent(diskUsed, diskTotal || undefined).replace('%', '')), value: formatCompactBytes(diskUsed), content: `${t('nazhua.disk')}${formatCompactBytes(diskTotal)}` },
  ]
})

const stats = computed(() => {
  const state = props.server.state || {}
  const seconds = Math.max(0, stateValue(state, 'Uptime', 'uptime'))
  const uptime = seconds >= 86_400
    ? { value: String(Math.floor(seconds / 86_400)), unit: t('day') }
    : seconds >= 3_600
      ? { value: String(Math.floor(seconds / 3_600)), unit: 'h' }
      : { value: String(Math.floor(seconds / 60)), unit: 'm' }
  const transfer = stateValue(state, 'NetInTransfer', 'net_in_transfer') + stateValue(state, 'NetOutTransfer', 'net_out_transfer')
  return [
    { key: 'duration', value: uptime.value, unit: uptime.unit, label: t('nazhua.uptime') },
    { key: 'traffic', ...splitCompact(transfer, 2), label: t('trafficBidirectionalQuota') },
    { key: 'in', ...splitCompact(stateValue(state, 'NetInSpeed', 'net_in_speed'), 1), label: t('nazhua.download') },
    { key: 'out', ...splitCompact(stateValue(state, 'NetOutSpeed', 'net_out_speed'), 1), label: t('nazhua.upload') },
  ]
})

function splitCompact(value: number, decimals: number) {
  const text = formatCompactBytes(value, decimals)
  const matched = text.match(/^([0-9.]+)(.*)$/)
  return { value: matched?.[1] || text, unit: matched?.[2] || '' }
}
</script>

<template>
  <section class="nazhua-detail-status">
    <div class="nazhua-detail-status__metrics">
      <div v-for="item in items" :key="item.key" class="nazhua-detail-status__metric">
        <DonutChart :label="item.label" :percent="item.percent" :value="item.value" :color="item.key === 'mem' ? 'green' : item.key === 'disk' ? 'cyan' : 'blue'" />
        <span>{{ item.content }}</span>
      </div>
    </div>
    <div class="nazhua-detail-status__stats">
      <span v-for="item in stats" :key="item.key" :class="item.key">
        <strong>{{ item.value }}<em>{{ item.unit }}</em></strong>
        <small>{{ item.label }}</small>
      </span>
    </div>
  </section>
</template>
