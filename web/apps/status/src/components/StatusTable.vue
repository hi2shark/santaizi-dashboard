<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ServerRecord } from '@santaizi/api'

const props = defineProps<{ title?: string; servers: readonly ServerRecord[] }>()
const { t, locale } = useI18n()
const expanded = ref<number[]>([])
const rows = computed(() => [...props.servers].sort((a, b) => b.display_index - a.display_index))
function value(object: Record<string, unknown> | undefined, ...keys: string[]) { for (const key of keys) if (object?.[key] !== undefined) return object[key]; return 0 }
function bytes(input: unknown) { let n = Number(input || 0); const units = ['B', 'KB', 'MB', 'GB', 'TB']; let i = 0; while (n >= 1024 && i < 4) { n /= 1024; i++ } return `${n.toFixed(i ? 1 : 0)} ${units[i]}` }
function percent(current: unknown, total?: unknown) { const n = Number(current || 0); return `${Math.max(0, Math.min(100, total ? 100 * n / Number(total) : n)).toFixed(1)}%` }
function uptime(input: unknown) { const total = Math.max(0, Number(input || 0)); const days = Math.floor(total / 86400); const hours = Math.floor(total % 86400 / 3600); const minutes = Math.floor(total % 3600 / 60); const parts = days ? [[days,'day'],[hours,'hour']] : hours ? [[hours,'hour'],[minutes,'minute']] : [[minutes,'minute']]; return parts.map(([value,unit]) => new Intl.NumberFormat(locale.value,{style:'unit',unit:String(unit),unitDisplay:'short'}).format(Number(value))).join(' ') }
function publicSummary(note: Record<string, unknown> | undefined) { const presentation = note?.presentation as Record<string, unknown> | undefined; return String(presentation?.slogan || presentation?.locationLabel || '') }
function toggle(id: number) { expanded.value = expanded.value.includes(id) ? expanded.value.filter(value => value !== id) : [...expanded.value, id] }
</script>
<template>
  <section class="status-panel">
    <header v-if="title" class="group-title"><span>{{title}}</span><small>{{servers.length}} {{t('servers')}}</small></header>
    <div class="status-table" role="table">
      <div class="status-row status-head" role="row"><span>{{t('status')}}</span><span>{{t('name')}}</span><span>{{t('location')}}</span><span>{{t('cpu')}}</span><span>{{t('memory')}}</span><span>{{t('disk')}}</span><span>{{t('networkSpeed')}}</span><span>{{t('traffic')}}</span></div>
      <template v-for="row in rows" :key="row.id">
        <button class="status-row server-row" type="button" role="row" :aria-expanded="expanded.includes(row.id)" @click="toggle(row.id)">
          <span><i class="live-dot" :class="row.online?'online':'offline'"></i><em>{{t(row.online?'online':'offline')}}</em></span>
          <span class="server-title"><strong>{{row.name}}</strong><small>{{publicSummary(row.public_note)}}</small></span>
          <span>{{value(row.host,'CountryCode','country_code')||'—'}}</span>
          <span><b>{{percent(value(row.state,'CPU','cpu'))}}</b><i class="bar"><i :style="{width:percent(value(row.state,'CPU','cpu'))}"/></i></span>
          <span><b>{{percent(value(row.state,'MemUsed','mem_used'),value(row.host,'MemTotal','mem_total'))}}</b><i class="bar"><i :style="{width:percent(value(row.state,'MemUsed','mem_used'),value(row.host,'MemTotal','mem_total'))}"/></i></span>
          <span><b>{{percent(value(row.state,'DiskUsed','disk_used'),value(row.host,'DiskTotal','disk_total'))}}</b><i class="bar"><i :style="{width:percent(value(row.state,'DiskUsed','disk_used'),value(row.host,'DiskTotal','disk_total'))}"/></i></span>
          <span class="network-rate"><small><i class="ri-arrow-up-line"></i>{{bytes(value(row.state,'NetOutSpeed','net_out_speed'))}}/s</small><small><i class="ri-arrow-down-line"></i>{{bytes(value(row.state,'NetInSpeed','net_in_speed'))}}/s</small></span>
          <span>{{bytes(Number(value(row.state,'NetInTransfer','net_in_transfer'))+Number(value(row.state,'NetOutTransfer','net_out_transfer')))}}</span>
        </button>
        <Transition name="expand"><div v-if="expanded.includes(row.id)" class="server-detail"><dl>
          <div><dt>{{t('host')}}</dt><dd>{{t(row.telemetry?.host||'unknown')}}</dd></div>
          <div><dt>{{t('connectivity')}}</dt><dd>{{t(row.telemetry?.connectivity||'unknown')}}</dd></div>
          <div><dt>{{t('availability')}}</dt><dd>{{row.telemetry?.available===null?t('unknown'):t(row.telemetry?.available?'available':'unavailable')}}</dd></div>
          <div><dt>{{t('coverage')}}</dt><dd>{{t(row.telemetry?.coverage||'unknown')}}</dd></div>
          <div><dt>{{t('platform')}}</dt><dd>{{value(row.host,'Platform','platform')||'—'}}</dd></div>
          <div><dt>{{t('version')}}</dt><dd>{{value(row.host,'PlatformVersion','platform_version')||'—'}}</dd></div>
          <div><dt>{{t('uptime')}}</dt><dd>{{uptime(value(row.state,'Uptime','uptime'))}}</dd></div>
        </dl></div></Transition>
      </template>
    </div>
  </section>
</template>
