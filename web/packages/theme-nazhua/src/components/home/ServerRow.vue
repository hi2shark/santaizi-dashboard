<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { NazhuaServerView } from '../../domain/nazhuaServerView'
import { formatCompactBytes } from '../../domain/nazhuaServerView'
import { formatSpeed } from '../../utils/host'

defineProps<{ server: NazhuaServerView }>()
const { t } = useI18n()
</script>

<template>
  <RouterLink :to="{ name: 'public-detail', params: { serverId: String(server.id) } }" class="nazhua-row" :class="{ offline: !server.online }">
    <div class="nazhua-row__name">
      <span v-if="server.flagClass" :class="server.flagClass" class="nazhua-flag" />
      <span v-else class="nazhua-flag-fallback"><i class="ri-global-line"></i></span>
      <div><strong>{{ server.name }}</strong><small>{{ server.slogan || server.spec }}</small></div>
      <i :class="server.online ? 'ri-checkbox-circle-fill online' : 'ri-indeterminate-circle-fill offline'"></i>
    </div>
    <div class="nazhua-row__metric"><small>CPU</small>{{ server.cpuPercent.toFixed(1) }}%</div>
    <div class="nazhua-row__metric"><small>{{ t('nazhua.memory') }}</small>{{ server.memoryPercent.toFixed(1) }}%</div>
    <div class="nazhua-row__metric"><small>{{ t('nazhua.disk') }}</small>{{ server.diskPercent.toFixed(1) }}%</div>
    <div class="nazhua-row__metric"><small>{{ t('nazhua.download') }}</small>{{ formatSpeed(server.speedIn) }}</div>
    <div class="nazhua-row__metric"><small>{{ t('nazhua.upload') }}</small>{{ formatSpeed(server.speedOut) }}</div>
    <div class="nazhua-row__metric"><small>{{ t('nazhua.cycleTransfer') }}</small>{{ formatCompactBytes(server.trafficBytes, 1) }}</div>
  </RouterLink>
</template>
