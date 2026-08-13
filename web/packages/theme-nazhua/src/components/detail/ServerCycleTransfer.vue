<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ServerRecord } from '@santaizi/api'
import { listPublicCycleTransfer } from '@santaizi/api'
import { formatBinary } from '../../utils/host'

type CycleRow = {
  policy_id?: number
  name?: string
  direction?: string
  mode?: string
  used_bytes?: number
  quota_bytes?: number
  usage_percent?: number
  status?: string
  window_start?: string
  window_end?: string
}

const props = defineProps<{ server: ServerRecord }>()
const { t } = useI18n()
const rows = ref<CycleRow[]>([])
const loading = ref(false)
const failed = ref(false)

async function load() {
  loading.value = true
  failed.value = false
  try {
    const result = await listPublicCycleTransfer(props.server.id)
    rows.value = (result.data || []) as CycleRow[]
  } catch {
    failed.value = true
    rows.value = []
  } finally {
    loading.value = false
  }
}

const hasRows = computed(() => rows.value.length > 0)

onMounted(load)
watch(() => props.server.id, load)
</script>

<template>
  <section class="nazhua-cycle-transfer">
    <header class="nazhua-cycle-transfer__head">
      <h2>{{ t('nazhua.cycleTransfer') }}</h2>
      <button type="button" @click="load"><i class="ri-refresh-line"></i>{{ t('nazhua.refresh') }}</button>
    </header>
    <p v-if="loading">{{ t('nazhua.loading') }}</p>
    <p v-else-if="failed">{{ t('nazhua.requestFailed') }}</p>
    <p v-else-if="!hasRows">{{ t('nazhua.noData') }}</p>
    <div v-else class="nazhua-cycle-transfer__list">
      <article v-for="row in rows" :key="row.policy_id" class="nazhua-cycle-transfer__item">
        <header>
          <strong>{{ row.name || t('nazhua.cycleTransfer') }}</strong>
          <span>{{ t(`nazhua.cycleStatus.${row.status === 'normal' || !row.status ? 'ok' : row.status}`, row.status || 'ok') }}</span>
        </header>
        <div class="nazhua-cycle-transfer__bar">
          <div :style="{ width: `${Math.min(100, Number(row.usage_percent || 0))}%` }" />
        </div>
        <p>
          {{ formatBinary(Number(row.used_bytes || 0)).value }}{{ formatBinary(Number(row.used_bytes || 0)).unit }}
          /
          {{ formatBinary(Number(row.quota_bytes || 0)).value }}{{ formatBinary(Number(row.quota_bytes || 0)).unit }}
          ({{ Number(row.usage_percent || 0).toFixed(1) }}%)
        </p>
      </article>
    </div>
  </section>
</template>
