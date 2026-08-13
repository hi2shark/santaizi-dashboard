<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { NazhuaServerView } from '../../domain/nazhuaServerView'
import { formatCompactBytes } from '../../domain/nazhuaServerView'
import { formatSpeed, stateValue } from '../../utils/host'

defineProps<{ servers: NazhuaServerView[] }>()
const { t } = useI18n()

function hostText(server: NazhuaServerView, ...keys: string[]) {
  for (const key of keys) {
    const value = server.source.host?.[key]
    if (value !== undefined && value !== null && String(value) !== '') return String(value)
  }
  return '—'
}
</script>

<template>
  <div class="nazhua-status-table">
    <div class="nazhua-status-table__head">
      <span>{{ t('nazhua.status') }}</span><span>{{ t('nazhua.name') }}</span><span>{{ t('nazhua.location') }}</span>
      <span>{{ t('nazhua.platform') }}</span><span>{{ t('nazhua.arch') }}</span><span>{{ t('nazhua.uptime') }}</span>
      <span>{{ t('nazhua.netSpeed') }}</span><span>{{ t('nazhua.cycleTransfer') }}</span><span>{{ t('load') }}</span>
      <span>CPU</span><span>{{ t('nazhua.memory') }}</span><span>{{ t('nazhua.disk') }}</span><span>{{ t('nazhua.billing') }}</span>
    </div>
    <RouterLink v-for="server in servers" :key="server.id" :to="{ name: 'public-detail', params: { serverId: String(server.id) } }" class="nazhua-status-table__row" :class="{ offline: !server.online }">
      <span><i :class="server.online ? 'ri-checkbox-circle-fill online' : 'ri-indeterminate-circle-fill offline'"></i></span>
      <span>{{ server.name }}</span>
      <span>{{ server.flagCode.toUpperCase() || 'UN' }}</span>
      <span>{{ hostText(server, 'Platform', 'platform') }}</span>
      <span>{{ server.spec || hostText(server, 'Arch', 'arch') }}</span>
      <span>{{ server.uptime }}</span>
      <span>{{ formatSpeed(server.speedIn) }} / {{ formatSpeed(server.speedOut) }}</span>
      <span>{{ formatCompactBytes(server.trafficBytes, 1) }}</span>
      <span>{{ stateValue(server.source.state, 'Load1', 'load1').toFixed(2) }}</span>
      <span>{{ server.cpuPercent.toFixed(1) }}%</span>
      <span>{{ server.memoryPercent.toFixed(1) }}%</span>
      <span>{{ server.diskPercent.toFixed(1) }}%</span>
      <span>{{ server.billing || '—' }}</span>
    </RouterLink>
  </div>
</template>
