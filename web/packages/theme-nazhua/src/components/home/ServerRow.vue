<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { NazhuaServerView } from '../../domain/nazhuaServerView'
import { formatCompactBytes } from '../../domain/nazhuaServerView'
import { formatSpeed } from '../../utils/host'
import OsLogo from '../common/OsLogo.vue'

defineProps<{ server: NazhuaServerView }>()
const { t } = useI18n()
</script>

<template>
  <RouterLink :to="{ name: 'public-detail', params: { serverId: String(server.id) } }" class="nazhua-row" :class="{ offline: !server.online }">
    <div class="nazhua-row__name">
      <span v-if="server.flagClass" :class="server.flagClass" class="nazhua-flag" aria-hidden="true" />
      <span v-else class="nazhua-flag-fallback" aria-hidden="true"><i class="ri-global-line"></i></span>
      <OsLogo :platform="server.platform" />
      <strong>{{ server.name }}</strong>
      <small>{{ server.slogan || server.spec }}</small>
    </div>
    <div class="nazhua-row__metric"><small>CPU</small>{{ server.cpuPercent.toFixed(1) }}%{{ server.cpuCaption ? ` ${server.cpuCaption}` : '' }}</div>
    <div class="nazhua-row__metric"><small>{{ t('nazhua.memory') }}</small>{{ server.memoryCaption }}</div>
    <div class="nazhua-row__metric"><small>{{ t('nazhua.disk') }}</small>{{ server.diskCaption }}</div>
    <div class="nazhua-row__metric"><small>{{ t('nazhua.download') }}</small>{{ formatSpeed(server.speedIn) }}</div>
    <div class="nazhua-row__metric"><small>{{ t('nazhua.upload') }}</small>{{ formatSpeed(server.speedOut) }}</div>
    <div class="nazhua-row__metric"><small>{{ t('nazhua.cycleTransfer') }}</small>{{ formatCompactBytes(server.trafficBytes, 1) }}</div>
  </RouterLink>
</template>
