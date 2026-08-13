<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PublicNoteView } from '../domain/publicNoteView'
import type { ServerStatusView } from '../domain/serverStatusView'
import MetricBar from './MetricBar.vue'

const props = defineProps<{
  server: ServerStatusView
  showAvailability: boolean
  showGroup?: boolean
}>()

const { t, te, locale } = useI18n()

const showDetails = computed(() => props.server.hasSpecs || (props.showAvailability && props.server.available !== null))

function localizeCycle(cycleLabel: string) {
  if (!cycleLabel) return ''
  return te(cycleLabel) ? t(cycleLabel) : cycleLabel
}

function billingText(view: PublicNoteView) {
  const { amountKind, amountValue, cycleLabel } = view.bill
  const cycle = localizeCycle(cycleLabel)
  if (amountKind === 'metered') return cycle ? `${t('everyCycle', { cycle })} ${t('meteredBilling')}` : t('meteredBilling')
  if (amountKind === 'free') return t('freeBilling')
  if (amountKind === 'priced') {
    if (cycleLabel === 'cycleOnetime') return amountValue
    return cycle ? `${amountValue} · ${t('cyclePay', { cycle })}` : amountValue
  }
  return ''
}

function remainingText(view: PublicNoteView) {
  const { remainingKind, remainingDays } = view.bill
  if (remainingKind === 'infinity') return t('foreverValid')
  if (remainingKind === 'expired') return t('expired')
  if (remainingKind === 'days' && remainingDays !== null) return t('remainingDays', { n: remainingDays })
  return ''
}

function trafficText(view: PublicNoteView) {
  if (!view.bill.trafficVol) return ''
  const typeKey = view.bill.trafficType === '1'
    ? 'trafficOneWayOut'
    : view.bill.trafficType === '3'
      ? 'trafficOneWayMax'
      : 'trafficBidirectionalQuota'
  return `${t(typeKey)} ${view.bill.trafficVol}`
}

function planTagLabel(tag: string) {
  if (tag === '__dual_stack__') return t('dualStack')
  if (tag === '__ipv4_only__') return t('ipv4Only')
  if (tag === '__ipv6_only__') return t('ipv6Only')
  return tag
}

function uptime(input: number) {
  const total = Math.max(0, input)
  const days = Math.floor(total / 86400)
  const hours = Math.floor(total % 86400 / 3600)
  const minutes = Math.floor(total % 3600 / 60)
  const parts = days ? [[days, 'day'], [hours, 'hour']] : hours ? [[hours, 'hour'], [minutes, 'minute']] : [[minutes, 'minute']]
  return parts.map(([value, unit]) => new Intl.NumberFormat(locale.value, {
    style: 'unit',
    unit: String(unit),
    unitDisplay: 'short',
  }).format(Number(value))).join(' ')
}

function bootLabel(seconds: number) {
  if (!seconds) return ''
  const ms = seconds > 1e12 ? seconds : seconds * 1000
  return new Date(ms).toLocaleString(locale.value)
}

function availabilityLabel(value: boolean | null) {
  if (value === true) return t('available')
  if (value === false) return t('unavailable')
  return t('unknown')
}

function memDetail(metric: ServerStatusView['memory']) {
  return metric.total > 0 ? `${metric.usedLabel} / ${metric.totalLabel}` : ''
}

const hasSummary = computed(() => Boolean(
  props.server.uptimeSeconds
  || props.server.hasLoad
  || props.server.platform
  || props.server.arch
  || props.server.virtualization,
))

</script>

<template>
  <article class="ss-card" :class="{ 'is-offline': !server.online }">
    <header class="ss-card__head">
      <span class="status-dot" :class="server.online ? 'online' : 'offline'"></span>
      <span
        v-if="server.flagCode"
        class="server-flag"
        :class="`fi fi-${server.flagCode}`"
        aria-hidden="true"
      />
      <span v-else class="server-flag server-flag--empty" aria-hidden="true"><i class="ri-global-line"></i></span>
      <div class="ss-card__identity">
        <strong>{{ server.name }}</strong>
        <small v-if="server.slogan">{{ server.slogan }}</small>
      </div>
      <span v-if="showGroup !== false && server.group" class="ss-chip">{{ server.group }}</span>
      <span v-if="server.location" class="ss-chip ss-chip--muted">{{ server.location }}</span>
      <em class="ss-online">{{ t(server.online ? 'online' : 'offline') }}</em>
    </header>

    <div class="ss-card__metrics">
      <MetricBar :label="t('cpu')" :percent="server.cpu.percent" />
      <MetricBar :label="t('memory')" :percent="server.memory.percent" :detail="memDetail(server.memory)" />
      <MetricBar :label="t('disk')" :percent="server.disk.percent" :detail="memDetail(server.disk)" />
      <MetricBar v-if="server.gpu" :label="t('metric_gpu')" :percent="server.gpu.percent" />
    </div>

    <div class="ss-card__net">
      <span><i class="ri-arrow-down-line"></i>{{ server.speedInLabel }}</span>
      <span><i class="ri-arrow-up-line"></i>{{ server.speedOutLabel }}</span>
      <span><i class="ri-exchange-line"></i>{{ server.transferTotalLabel }}</span>
    </div>

    <div v-if="server.cycles.length" class="ss-card__cycles">
      <div v-for="cycle in server.cycles" :key="`${server.id}-${cycle.policyId}-${cycle.name}`" class="ss-cycle">
        <div class="ss-metric__meta">
          <span>{{ cycle.name || t('cycleTransfer') }}</span>
          <em>{{ cycle.usedLabel }} / {{ cycle.quotaLabel }}</em>
        </div>
        <div class="ss-metric-bar" role="meter" :aria-valuenow="cycle.usagePercent" aria-valuemin="0" aria-valuemax="100">
          <span class="ss-metric-bar__fill" :style="{ width: `${cycle.usagePercent}%` }"></span>
          <span class="ss-metric-bar__label">{{ cycle.usagePercent.toFixed(1) }}%</span>
        </div>
      </div>
    </div>

    <div v-if="server.publicNote.hasBillMeta || server.publicNote.hasPlanMeta || server.publicNote.hasBuy" class="ss-card__tags">
      <span v-if="billingText(server.publicNote)" class="meta-tag meta-tag--billing">{{ billingText(server.publicNote) }}</span>
      <span
        v-if="remainingText(server.publicNote)"
        class="meta-tag"
        :class="{
          'meta-tag--success': server.publicNote.bill.remainingTone === 'success',
          'meta-tag--warning': server.publicNote.bill.remainingTone === 'warning',
          'meta-tag--danger': server.publicNote.bill.remainingTone === 'danger',
        }"
      >{{ remainingText(server.publicNote) }}</span>
      <span v-if="server.publicNote.bill.bandwidth" class="meta-tag">{{ server.publicNote.bill.bandwidth }}</span>
      <span v-if="trafficText(server.publicNote)" class="meta-tag">{{ trafficText(server.publicNote) }}</span>
      <span v-for="tag in server.publicNote.planTags" :key="`${server.id}-${tag}`" class="meta-tag meta-tag--plan">{{ planTagLabel(tag) }}</span>
      <a
        v-if="server.publicNote.hasBuy"
        class="buy-link"
        :href="server.orderLink"
        target="_blank"
        rel="noopener noreferrer"
      >
        <i :class="server.publicNote.presentation.buyBtnIcon || 'ri-shopping-bag-3-line'"></i>
        {{ server.publicNote.presentation.buyBtnText || t('purchase') }}
      </a>
    </div>

    <dl v-if="hasSummary" class="ss-card__summary">
      <div v-if="server.uptimeSeconds"><dt>{{ t('uptime') }}</dt><dd>{{ uptime(server.uptimeSeconds) }}</dd></div>
      <div v-if="server.hasLoad"><dt>{{ t('load') }}</dt><dd>{{ server.load1.toFixed(2) }} / {{ server.load5.toFixed(2) }} / {{ server.load15.toFixed(2) }}</dd></div>
      <div v-if="server.platform"><dt>{{ t('platform') }}</dt><dd>{{ server.platform }}{{ server.platformVersion ? ` ${server.platformVersion}` : '' }}</dd></div>
      <div v-if="server.arch"><dt>{{ t('arch') }}</dt><dd>{{ server.arch }}</dd></div>
      <div v-if="server.virtualization"><dt>{{ t('virtualization') }}</dt><dd>{{ server.virtualization }}</dd></div>
    </dl>

    <details v-if="showDetails" class="ss-card__specs">
      <summary class="icon-text"><i class="ri-information-line"></i>{{ t('moreSpecs') }}</summary>
      <dl>
        <div v-if="showAvailability && server.available !== null">
          <dt>{{ t('availability') }}</dt>
          <dd>{{ availabilityLabel(server.available) }}</dd>
        </div>
        <div v-if="server.cpuModels.length">
          <dt>{{ t('cpu') }}</dt>
          <dd>{{ server.cpuModels.join(', ') }}</dd>
        </div>
        <div v-if="server.gpuNames.length">
          <dt>{{ t('metric_gpu') }}</dt>
          <dd>{{ server.gpuNames.join(', ') }}</dd>
        </div>
        <div v-if="server.swap">
          <dt>{{ t('metric_swap') }}</dt>
          <dd>{{ server.swap.usedLabel }} / {{ server.swap.totalLabel }}</dd>
        </div>
        <div v-if="server.tcp !== null">
          <dt>{{ t('metric_tcp_conn_count') }}</dt>
          <dd>{{ server.tcp }}</dd>
        </div>
        <div v-if="server.udp !== null">
          <dt>{{ t('metric_udp_conn_count') }}</dt>
          <dd>{{ server.udp }}</dd>
        </div>
        <div v-if="server.processes !== null">
          <dt>{{ t('metric_process_count') }}</dt>
          <dd>{{ server.processes }}</dd>
        </div>
        <div v-if="server.temperatures.length">
          <dt>{{ t('capability_temperature') }}</dt>
          <dd>{{ server.temperatures.map((row) => `${row.name || t('metric_temperature_max')} ${row.value.toFixed(1)}°C`).join(', ') }}</dd>
        </div>
        <div v-if="server.agentVersion">
          <dt>{{ t('version') }}</dt>
          <dd>{{ server.agentVersion }}</dd>
        </div>
        <div v-if="server.bootTime">
          <dt>{{ t('bootTime') }}</dt>
          <dd>{{ bootLabel(server.bootTime) }}</dd>
        </div>
      </dl>
    </details>
  </article>
</template>
