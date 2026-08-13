<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getAdminSummary } from '@santaizi/api'
import { notifyAPIError } from '@/composables/notify'

const { t, te } = useI18n()
const loading = ref(false)
const summary = ref<Record<string, unknown>>({})
const cards = computed(() => [
  ['ri-server-line', 'totalServers', Number(summary.value.total_servers || 0), 'blue'],
  ['ri-pulse-line', 'onlineServers', Number(summary.value.online_servers || 0), 'green'],
  ['ri-alarm-warning-line', 'activeIncidents', Number(summary.value.active_incidents || 0), 'amber'],
  ['ri-radar-line', 'activeCollectors', Number(summary.value.active_collectors || 0), 'violet'],
])

async function load() {
  loading.value = true
  try { summary.value = await getAdminSummary() }
  catch (error) { notifyAPIError(error, t as never, te) }
  finally { loading.value = false }
}
onMounted(load)
</script>

<template>
  <div class="page-head">
    <h1>{{ t('overview') }}</h1>
    <el-button @click="load"><i class="ri-refresh-line"></i>{{ t('refresh') }}</el-button>
  </div>
  <div v-loading="loading" class="metric-grid">
    <article v-for="card in cards" :key="String(card[1])" class="surface metric-card">
      <span class="metric-icon" :class="String(card[3])"><i :class="String(card[0])"></i></span>
      <div><p>{{ t(String(card[1])) }}</p><strong>{{ card[2] }}</strong></div>
    </article>
  </div>
  <div class="dashboard-grid">
    <section class="surface dashboard-panel">
      <div class="section-title"><div><h2>{{ t('reliabilitySummary') }}</h2></div><RouterLink to="/telemetry">{{ t('details') }} <i class="ri-arrow-right-line"></i></RouterLink></div>
      <div class="reliability-grid">
        <div><span>{{ t('pendingEvents') }}</span><strong>{{ Number(summary.telemetry_pending || 0).toLocaleString() }}</strong></div>
        <div><span>{{ t('dataLoss') }}</span><strong>{{ Number(summary.data_loss || 0).toLocaleString() }}</strong></div>
        <div><span>{{ t('alerts') }}</span><strong>{{ Number(summary.telemetry_alerts || 0).toLocaleString() }}</strong></div>
      </div>
    </section>
    <section class="surface dashboard-panel">
      <div class="section-title"><div><h2>{{ t('connectionSummary') }}</h2></div><RouterLink to="/connections">{{ t('details') }} <i class="ri-arrow-right-line"></i></RouterLink></div>
      <div class="connection-grid">
        <div><span>{{ t('collectorsOnline') }}</span><strong>{{ Number(summary.active_collectors || 0).toLocaleString() }}</strong></div>
        <div><span>{{ t('collectorsOffline') }}</span><strong>{{ Number(summary.collectors_offline || 0).toLocaleString() }}</strong></div>
        <div><span>{{ t('pathsConnected') }}</span><strong>{{ Number(summary.paths_connected || 0).toLocaleString() }}</strong></div>
        <div><span>{{ t('pathsAssigned') }}</span><strong>{{ Number(summary.paths_assigned || 0).toLocaleString() }}</strong></div>
      </div>
    </section>
    <section class="surface dashboard-panel dashboard-span">
      <div class="section-title"><div><h2>{{ t('quickActions') }}</h2></div></div>
      <div class="quick-grid">
        <RouterLink to="/servers?create=1"><i class="ri-add-circle-line"></i><span>{{ t('createServer') }}</span></RouterLink>
        <RouterLink to="/services?create=1"><i class="ri-heart-pulse-line"></i><span>{{ t('createMonitor') }}</span></RouterLink>
        <RouterLink to="/telemetry?create=1"><i class="ri-radar-line"></i><span>{{ t('createCollector') }}</span></RouterLink>
        <RouterLink to="/api-tokens"><i class="ri-key-2-line"></i><span>{{ t('issueToken') }}</span></RouterLink>
      </div>
    </section>
  </div>
</template>
