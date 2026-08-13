<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { NazhuaServerView } from '../../domain/nazhuaServerView'
import { formatCompactBytes } from '../../domain/nazhuaServerView'
import { formatSpeed } from '../../utils/host'
import OsLogo from '../common/OsLogo.vue'

defineProps<{ servers: NazhuaServerView[] }>()
const { t } = useI18n()
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
      <span class="nazhua-status-table__flag">
        <span v-if="server.flagClass" :class="server.flagClass" class="nazhua-flag" />
        {{ server.flagCode.toUpperCase() || 'UN' }}
      </span>
      <span class="nazhua-status-table__os"><OsLogo :platform="server.platform" />{{ server.platform || '—' }}</span>
      <span>{{ server.spec || server.arch || '—' }}</span>
      <span>{{ server.uptime }}</span>
      <span>{{ formatSpeed(server.speedIn) }} / {{ formatSpeed(server.speedOut) }}</span>
      <span>{{ formatCompactBytes(server.trafficBytes, 1) }}</span>
      <span>{{ server.load1.toFixed(2) }}</span>
      <span>{{ server.cpuPercent.toFixed(1) }}%{{ server.cpuCaption ? ` ${server.cpuCaption}` : '' }}</span>
      <span>{{ server.memoryCaption }}</span>
      <span>{{ server.diskCaption }}</span>
      <span>{{ server.billing || '—' }}</span>
    </RouterLink>
  </div>
</template>
