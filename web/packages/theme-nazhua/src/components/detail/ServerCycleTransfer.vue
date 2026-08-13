<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CycleTransfer, ServerRecord } from '@santaizi/api'
import { listPublicCycleTransfer } from '@santaizi/api'
import { AppEmpty } from '@santaizi/ui'
import { formatCompactBytes, percentOf } from '../../domain/nazhuaServerView'
import { formatDateTime } from '../../utils/host'

const props = defineProps<{ server: ServerRecord }>()
const { t, locale } = useI18n()
const rows = ref<CycleTransfer[]>([])
const loading = ref(false)
const failed = ref(false)

async function load() {
  loading.value = true
  failed.value = false
  try {
    const result = await listPublicCycleTransfer(props.server.id)
    rows.value = result.data || []
  } catch {
    failed.value = true
    rows.value = []
  } finally {
    loading.value = false
  }
}

const hasRows = computed(() => rows.value.length > 0)

function statusKey(status?: string) {
  if (!status || status === 'normal' || status === 'ok') return 'ok'
  if (status === 'warning' || status === 'critical' || status === 'exceeded') return status
  return 'ok'
}

function usage(row: CycleTransfer) {
  const used = Number(row.used_bytes || 0)
  const quota = Number(row.quota_bytes || 0)
  return quota > 0 ? percentOf(used, quota) : Number(row.usage_percent || 0)
}

onMounted(load)
watch(() => props.server.id, load)
</script>

<template>
  <section class="nazhua-cycle-transfer">
    <header class="nazhua-cycle-transfer__head">
      <h2>{{ t('nazhua.cycleTransfer') }}</h2>
      <button type="button" @click="load"><i class="ri-refresh-line"></i>{{ t('nazhua.refresh') }}</button>
    </header>
    <div v-if="loading || failed || !hasRows" class="nazhua-cycle-transfer__empty">
      <AppEmpty
        :tone="failed ? 'danger' : 'default'"
        :icon="failed ? 'ri-error-warning-line' : loading ? 'ri-loader-4-line' : 'ri-pie-chart-line'"
        :title="failed ? t('nazhua.loadFailed') : ''"
        :description="t(failed ? 'nazhua.requestFailed' : loading ? 'nazhua.loading' : 'nazhua.noData')"
      />
      <button v-if="failed" type="button" @click="load"><i class="ri-refresh-line"></i>{{ t('nazhua.refresh') }}</button>
    </div>
    <div v-else class="nazhua-cycle-transfer__list">
      <article v-for="row in rows" :key="row.policy_id" class="nazhua-cycle-transfer__item">
        <header>
          <strong>{{ row.name || t('nazhua.cycleTransfer') }}</strong>
          <span class="nazhua-cycle-transfer__status" :class="`is-${statusKey(row.status)}`">
            {{ t(`nazhua.cycleStatus.${statusKey(row.status)}`) }}
          </span>
        </header>
        <div class="nazhua-cycle-transfer__bar">
          <div class="nazhua-cycle-transfer__fill" :style="{ width: `${Math.min(100, usage(row))}%` }" />
          <i
            v-if="row.warning_percent && row.warning_percent > 0 && row.warning_percent < 100"
            class="nazhua-cycle-transfer__warn"
            :style="{ left: `${row.warning_percent}%` }"
          />
        </div>
        <p>
          {{ formatCompactBytes(Number(row.used_bytes || 0), 1) }}
          /
          {{ formatCompactBytes(Number(row.quota_bytes || 0), 1) }}
          ({{ usage(row).toFixed(1) }}%)
        </p>
        <p v-if="(row.remaining_bytes ?? 0) > 0 || Number(row.quota_bytes || 0) > 0">
          {{ t('nazhua.remainingBytes') }}
          {{ formatCompactBytes(Number(row.remaining_bytes ?? Math.max(Number(row.quota_bytes || 0) - Number(row.used_bytes || 0), 0)), 1) }}
        </p>
        <p v-if="row.window_start || row.window_end">
          {{ t('nazhua.windowRange') }}
          {{ [formatDateTime(row.window_start, locale), formatDateTime(row.window_end, locale)].filter(Boolean).join(' ~ ') }}
        </p>
        <p v-if="row.next_reset_at">
          {{ t('nazhua.nextReset') }}
          {{ formatDateTime(row.next_reset_at, locale) }}
        </p>
      </article>
    </div>
  </section>
</template>
