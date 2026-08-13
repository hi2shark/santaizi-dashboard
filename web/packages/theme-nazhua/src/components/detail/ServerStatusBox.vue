<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ServerRecord } from '@santaizi/api'
import { formatCompactBytes, toNazhuaServerView } from '../../domain/nazhuaServerView'
import DonutChart from '../charts/DonutChart.vue'

const props = defineProps<{ server: ServerRecord }>()
const { t } = useI18n()
const view = computed(() => toNazhuaServerView(props.server))

const stats = computed(() => {
  const seconds = Math.max(0, Math.floor(view.value.uptimeSeconds))
  const uptime = seconds >= 86_400
    ? { value: String(Math.floor(seconds / 86_400)), unit: t('day') }
    : seconds >= 3_600
      ? { value: String(Math.floor(seconds / 3_600)), unit: 'h' }
      : { value: String(Math.floor(seconds / 60)), unit: 'm' }
  return [
    { key: 'duration', value: uptime.value, unit: uptime.unit, label: t('nazhua.uptime') },
    { key: 'traffic', ...splitCompact(view.value.trafficBytes, 2), label: view.value.cycle?.quotaBytes ? t('nazhua.remainingTraffic') : t('trafficBidirectionalQuota') },
    { key: 'in', ...splitCompact(view.value.speedIn, 1), label: t('nazhua.download') },
    { key: 'out', ...splitCompact(view.value.speedOut, 1), label: t('nazhua.upload') },
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
      <div class="nazhua-detail-status__metric">
        <DonutChart label="CPU" :percent="view.cpuPercent" :value="`${view.cpuPercent.toFixed(1)}%`" :caption="view.cpuCaption" color="blue" />
      </div>
      <div class="nazhua-detail-status__metric">
        <DonutChart :label="t('nazhua.memory')" :percent="view.memoryPercent" :value="view.memoryValue" :caption="view.memoryCaption" color="green" />
      </div>
      <div v-if="view.swapTotal > 0" class="nazhua-detail-status__metric">
        <DonutChart :label="t('nazhua.swap')" :percent="view.swapPercent" :value="formatCompactBytes(view.swapUsed)" :caption="view.swapCaption" color="orange" />
      </div>
      <div class="nazhua-detail-status__metric">
        <DonutChart :label="t('nazhua.disk')" :percent="view.diskPercent" :value="view.diskValue" :caption="view.diskCaption" color="cyan" />
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
