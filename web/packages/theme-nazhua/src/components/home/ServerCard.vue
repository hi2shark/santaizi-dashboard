<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { NazhuaServerView } from '../../domain/nazhuaServerView'
import { formatCompactBytes } from '../../domain/nazhuaServerView'
import DonutChart from '../charts/DonutChart.vue'
import OsLogo from '../common/OsLogo.vue'

const props = defineProps<{ server: NazhuaServerView }>()
const { t, te } = useI18n()

const trafficLabel = computed(() => {
  if (props.server.cycle?.direction === 'out' || props.server.cycle?.direction === 'outbound') return t('nazhua.outboundTraffic')
  if (props.server.cycle && props.server.cycle.quotaBytes > 0) return t('nazhua.remainingTraffic')
  if (props.server.publicNote.bill.trafficType === '1') return t('trafficOneWayOut')
  if (props.server.publicNote.bill.trafficType === '3') return t('trafficOneWayMax')
  return t('trafficBidirectionalQuota')
})

const billingText = computed(() => {
  const bill = props.server.publicNote.bill
  if (bill.amountKind === 'free') return t('freeBilling')
  if (bill.amountKind === 'metered') return t('meteredBilling')
  if (!bill.amountValue) return ''
  const cycle = bill.cycleLabel && te(bill.cycleLabel) ? t(bill.cycleLabel) : bill.cycleLabel
  return cycle ? `${bill.amountValue}/${cycle}` : bill.amountValue
})

const uptime = computed(() => {
  const seconds = Math.max(0, Math.floor(props.server.uptimeSeconds))
  if (seconds >= 86_400) return { value: String(Math.floor(seconds / 86_400)), unit: t('day') }
  if (seconds >= 3_600) return { value: String(Math.floor(seconds / 3_600)), unit: 'h' }
  return { value: String(Math.floor(seconds / 60)), unit: 'm' }
})

function splitCompact(value: number, decimals: number) {
  const text = formatCompactBytes(value, decimals)
  const matched = text.match(/^([0-9.]+)(.*)$/)
  return { value: matched?.[1] || text, unit: matched?.[2] || '' }
}

const traffic = computed(() => splitCompact(props.server.trafficBytes, 2))
const inSpeed = computed(() => splitCompact(props.server.speedIn, 1))
const outSpeed = computed(() => splitCompact(props.server.speedOut, 1))
const showFoot = computed(() => props.server.publicNote.planTags.length > 0 || Boolean(props.server.orderLink || billingText.value))

function planTag(tag: string) {
  if (tag === '__dual_stack__') return t('dualStack')
  if (tag === '__ipv4_only__') return t('ipv4Only')
  if (tag === '__ipv6_only__') return t('ipv6Only')
  return tag
}
</script>

<template>
  <article class="nazhua-card" :class="{ offline: !server.online }">
    <RouterLink :to="{ name: 'public-detail', params: { serverId: String(server.id) } }" class="nazhua-card__head" :aria-label="server.name">
      <span v-if="server.flagClass" :class="server.flagClass" class="nazhua-flag" />
      <span v-else class="nazhua-flag-fallback"><i class="ri-global-line"></i></span>
      <strong>{{ server.name }}</strong>
      <span v-if="!server.online" class="nazhua-offline-label"><i class="ri-wifi-off-line"></i>{{ t('nazhua.offline') }}</span>
      <span v-else class="nazhua-card__spec">
        <OsLogo :platform="server.platform" />
        {{ server.spec || '—' }}
      </span>
    </RouterLink>
    <RouterLink :to="{ name: 'public-detail', params: { serverId: String(server.id) } }" class="nazhua-card__main" :aria-label="server.name">
      <div class="nazhua-card__metrics">
        <DonutChart label="CPU" :percent="server.cpuPercent" :value="`${server.cpuPercent.toFixed(1)}%`" :caption="server.cpuCaption" color="blue" />
        <DonutChart :label="t('nazhua.memory')" :percent="server.memoryPercent" :value="server.memoryValue" :caption="server.memoryCaption" color="green" />
        <DonutChart :label="t('nazhua.disk')" :percent="server.diskPercent" :value="server.diskValue" :caption="server.diskCaption" color="cyan" />
      </div>
      <div class="nazhua-card__stats">
        <span><strong>{{ uptime.value }}<em>{{ uptime.unit }}</em></strong><small>{{ t('nazhua.uptime') }}</small></span>
        <span class="traffic"><strong>{{ traffic.value }}<em>{{ traffic.unit }}</em></strong><small>{{ trafficLabel }}</small></span>
        <span class="in"><strong>{{ inSpeed.value }}<em>{{ inSpeed.unit }}</em></strong><small>{{ t('nazhua.download') }}</small></span>
        <span class="out"><strong>{{ outSpeed.value }}<em>{{ outSpeed.unit }}</em></strong><small>{{ t('nazhua.upload') }}</small></span>
      </div>
    </RouterLink>
    <footer v-if="showFoot" class="nazhua-card__foot">
      <div class="nazhua-card__tags">
        <span v-for="tag in server.publicNote.planTags" :key="tag">{{ planTag(tag) }}</span>
      </div>
      <a v-if="server.orderLink" :href="server.orderLink" target="_blank" rel="noopener noreferrer" class="nazhua-card__buy">
        <i class="ri-shopping-bag-3-line"></i>{{ server.publicNote.presentation.buyBtnText || t('purchase') }}
      </a>
      <strong v-else-if="billingText" class="nazhua-card__billing">{{ billingText }}</strong>
    </footer>
  </article>
</template>
