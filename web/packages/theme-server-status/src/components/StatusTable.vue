<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ServerRecord } from '@santaizi/api'
import {
  buildPublicNoteView,
  decodeOrderLink,
  flagCode,
  publicLocation,
  publicSubtitle,
  type PublicNoteView,
} from '../domain/publicNoteView'

const props = defineProps<{ title?: string; servers: readonly ServerRecord[] }>()
const { t, te, locale } = useI18n()
const expanded = ref<number[]>([])
const rows = computed(() => [...props.servers].sort((a, b) => b.display_index - a.display_index))
const noteViews = computed(() => {
  const map = new Map<number, PublicNoteView>()
  for (const row of rows.value) map.set(row.id, buildPublicNoteView(row.public_note))
  return map
})
function noteView(row: ServerRecord) {
  return noteViews.value.get(row.id) || buildPublicNoteView(row.public_note)
}

function value(object: Record<string, unknown> | undefined, ...keys: string[]) {
  for (const key of keys) if (object?.[key] !== undefined) return object[key]
  return 0
}
function bytes(input: unknown) {
  let n = Number(input || 0)
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (n >= 1024 && i < 4) {
    n /= 1024
    i++
  }
  return `${n.toFixed(i ? 1 : 0)} ${units[i]}`
}
function percent(current: unknown, total?: unknown) {
  const n = Number(current || 0)
  return `${Math.max(0, Math.min(100, total ? 100 * n / Number(total) : n)).toFixed(1)}%`
}
function uptime(input: unknown) {
  const total = Math.max(0, Number(input || 0))
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
function toggle(id: number) {
  expanded.value = expanded.value.includes(id) ? expanded.value.filter((value) => value !== id) : [...expanded.value, id]
}

function country(row: ServerRecord) {
  return String(value(row.host, 'CountryCode', 'country_code') || '')
}

function availabilityKnown(row: ServerRecord) {
  return row.telemetry?.available === true || row.telemetry?.available === false
}

function availabilityLabel(row: ServerRecord) {
  if (!availabilityKnown(row)) return t('unknown')
  return t(row.telemetry!.available ? 'available' : 'unavailable')
}

function localizeCycle(cycleLabel: string) {
  if (!cycleLabel) return ''
  return te(cycleLabel) ? t(cycleLabel) : cycleLabel
}

function billingText(view: PublicNoteView) {
  const { amountKind, amountValue, cycleLabel } = view.bill
  const cycle = localizeCycle(cycleLabel)
  if (amountKind === 'metered') {
    return cycle ? `${t('everyCycle', { cycle })} ${t('meteredBilling')}` : t('meteredBilling')
  }
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

</script>

<template>
  <section class="status-panel">
    <header v-if="title" class="group-title">
      <span>{{ title }}</span>
      <small>{{ servers.length }} {{ t('servers') }}</small>
    </header>
    <div class="status-table" role="table">
      <div class="status-row status-head" role="row">
        <span>{{ t('status') }}</span>
        <span>{{ t('name') }}</span>
        <span>{{ t('location') }}</span>
        <span>{{ t('cpu') }}</span>
        <span>{{ t('memory') }}</span>
        <span>{{ t('disk') }}</span>
        <span>{{ t('networkSpeed') }}</span>
        <span>{{ t('traffic') }}</span>
      </div>
      <template v-for="(row, index) in rows" :key="row.id > 0 ? row.id : `server-${index}`">
        <button
          class="status-row server-row"
          type="button"
          role="row"
          :aria-expanded="expanded.includes(row.id)"
          @click="toggle(row.id)"
        >
          <span>
            <i class="live-dot" :class="row.online ? 'online' : 'offline'"></i>
            <em>{{ t(row.online ? 'online' : 'offline') }}</em>
          </span>
          <span class="server-title">
            <strong>
              <span
                v-if="flagCode(row.public_note, country(row))"
                class="server-flag"
                :class="`fi fi-${flagCode(row.public_note, country(row))}`"
                aria-hidden="true"
              />
              <span v-else class="server-flag server-flag--empty" aria-hidden="true"><i class="ri-global-line"></i></span>
              {{ row.name }}
            </strong>
            <small v-if="publicSubtitle(row.public_note)">{{ publicSubtitle(row.public_note) }}</small>
          </span>
          <span>{{ publicLocation(row.public_note, country(row)) || '—' }}</span>
          <span
            class="metric-bar"
            role="meter"
            :aria-valuenow="Number(percent(value(row.state, 'CPU', 'cpu')).replace('%', ''))"
            aria-valuemin="0"
            aria-valuemax="100"
          >
            <span class="metric-bar__fill" :style="{ width: percent(value(row.state, 'CPU', 'cpu')) }"></span>
            <span class="metric-bar__label">{{ percent(value(row.state, 'CPU', 'cpu')) }}</span>
          </span>
          <span
            class="metric-bar"
            role="meter"
            :aria-valuenow="Number(percent(value(row.state, 'MemUsed', 'mem_used'), value(row.host, 'MemTotal', 'mem_total')).replace('%', ''))"
            aria-valuemin="0"
            aria-valuemax="100"
          >
            <span
              class="metric-bar__fill"
              :style="{ width: percent(value(row.state, 'MemUsed', 'mem_used'), value(row.host, 'MemTotal', 'mem_total')) }"
            ></span>
            <span class="metric-bar__label">{{ percent(value(row.state, 'MemUsed', 'mem_used'), value(row.host, 'MemTotal', 'mem_total')) }}</span>
          </span>
          <span
            class="metric-bar"
            role="meter"
            :aria-valuenow="Number(percent(value(row.state, 'DiskUsed', 'disk_used'), value(row.host, 'DiskTotal', 'disk_total')).replace('%', ''))"
            aria-valuemin="0"
            aria-valuemax="100"
          >
            <span
              class="metric-bar__fill"
              :style="{ width: percent(value(row.state, 'DiskUsed', 'disk_used'), value(row.host, 'DiskTotal', 'disk_total')) }"
            ></span>
            <span class="metric-bar__label">{{ percent(value(row.state, 'DiskUsed', 'disk_used'), value(row.host, 'DiskTotal', 'disk_total')) }}</span>
          </span>
          <span class="network-rate">
            <small><i class="ri-arrow-up-line"></i>{{ bytes(value(row.state, 'NetOutSpeed', 'net_out_speed')) }}/s</small>
            <small><i class="ri-arrow-down-line"></i>{{ bytes(value(row.state, 'NetInSpeed', 'net_in_speed')) }}/s</small>
          </span>
          <span>{{ bytes(Number(value(row.state, 'NetInTransfer', 'net_in_transfer')) + Number(value(row.state, 'NetOutTransfer', 'net_out_transfer'))) }}</span>
        </button>

        <Transition name="expand">
          <div v-if="expanded.includes(row.id)" class="server-detail">
            <dl>
              <div>
                <dt>{{ t('availability') }}</dt>
                <dd>{{ availabilityLabel(row) }}</dd>
              </div>
              <div>
                <dt>{{ t('platform') }}</dt>
                <dd>{{ value(row.host, 'Platform', 'platform') || '—' }}</dd>
              </div>
              <div>
                <dt>{{ t('version') }}</dt>
                <dd>{{ value(row.host, 'PlatformVersion', 'platform_version') || '—' }}</dd>
              </div>
              <div>
                <dt>{{ t('uptime') }}</dt>
                <dd>{{ uptime(value(row.state, 'Uptime', 'uptime')) }}</dd>
              </div>
              <template v-if="noteView(row).hasDetail">
                <div v-if="billingText(noteView(row))">
                  <dt>{{ t('billingFee') }}</dt>
                  <dd>{{ billingText(noteView(row)) }}</dd>
                </div>
                <div v-if="remainingText(noteView(row))">
                  <dt>{{ t('remaining') }}</dt>
                  <dd>{{ remainingText(noteView(row)) }}</dd>
                </div>
                <div v-if="noteView(row).bill.bandwidth">
                  <dt>{{ t('bandwidth') }}</dt>
                  <dd>{{ noteView(row).bill.bandwidth }}</dd>
                </div>
                <div v-if="trafficText(noteView(row))">
                  <dt>{{ t('trafficVolume') }}</dt>
                  <dd>{{ trafficText(noteView(row)) }}</dd>
                </div>
              </template>
            </dl>
            <div v-if="noteView(row).planTags.length" class="detail-tags">
              <span
                v-for="tag in noteView(row).planTags"
                :key="`detail-${row.id}-${tag}`"
                class="meta-tag meta-tag--plan"
              >
                {{ planTagLabel(tag) }}
              </span>
            </div>
            <a
              v-if="noteView(row).hasBuy"
              class="buy-link"
              :href="decodeOrderLink(noteView(row).presentation.orderLink)"
              target="_blank"
              rel="noopener noreferrer"
              @click.stop
            >
              <i :class="noteView(row).presentation.buyBtnIcon || 'ri-shopping-bag-3-line'"></i>
              {{ noteView(row).presentation.buyBtnText || t('purchase') }}
            </a>
          </div>
        </Transition>
      </template>
    </div>
  </section>
</template>
