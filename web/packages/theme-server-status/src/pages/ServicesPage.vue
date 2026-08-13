<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { listPublicServices, type ResourceRecord } from '@santaizi/api'
import { AppEmpty } from '@santaizi/ui'

const { t } = useI18n()
const services = ref<ResourceRecord[]>([])
const loading = ref(false)
const failed = ref(false)
function percent(up: unknown, down: unknown) {
  const a = Number(up || 0)
  const b = Number(down || 0)
  return a + b ? 100 * a / (a + b) : 0
}
async function load() {
  loading.value = true
  try {
    services.value = (await listPublicServices()).data
    failed.value = false
  } catch {
    failed.value = true
  } finally {
    loading.value = false
  }
}
onMounted(load)
</script>

<template>
  <div class="status-container">
    <section class="status-panel service-panel">
      <header class="group-title"><span>{{ t('statusServices') }}</span><small>{{ t('serviceAvailability') }}</small></header>
      <div v-if="services.length" class="service-list">
        <article v-for="service in services" :key="String(service.id || service.name)">
          <div class="service-title">
            <div>
              <i class="live-dot" :class="percent(service.current_up, service.current_down) > 95 ? 'online' : 'offline'"></i>
              <strong>{{ service.name || service.monitor_name }}</strong>
            </div>
            <b>{{ percent(service.current_up, service.current_down).toFixed(2) }}%</b>
          </div>
          <div class="availability-days">
            <i
              v-for="(up, index) in (service.up as unknown[] || [])"
              :key="index"
              :class="percent(up, (service.down as unknown[] || [])[index]) > 95 ? 'good' : percent(up, (service.down as unknown[] || [])[index]) > 80 ? 'warn' : 'down'"
              :title="`${percent(up, (service.down as unknown[] || [])[index]).toFixed(2)}%`"
            />
          </div>
          <small>{{ t('averageLatency') }}: {{ service.avg_delay || service.delay || '—' }} ms</small>
        </article>
      </div>
      <div v-else class="empty-status status-page-empty">
        <AppEmpty
          :tone="failed ? 'danger' : 'default'"
          :icon="failed ? 'ri-error-warning-line' : 'ri-heart-pulse-line'"
          :title="failed ? t('loadFailed') : ''"
          :description="t(failed ? 'requestFailed' : loading ? 'loading' : 'noData')"
        />
        <el-button v-if="failed" type="primary" @click="load">
          <i class="ri-refresh-line"></i>{{ t('refresh') }}
        </el-button>
      </div>
    </section>
  </div>
</template>
