<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ProbePath, ProbeSampleBucket, ProbeTrace } from '@santaizi/api'
import { AppDrawer, AppEmpty } from '@santaizi/ui'
import CopyableId from '@/components/CopyableId.vue'
import { getProbeTrace, listProbeSamples } from '@/api/adminApi'
import { formatAdminValue } from '@/composables/format'
import { notifyAPIError } from '@/composables/notify'
import { formatProbeLoss, probeTargetText } from '@/domain/probePath'

const props = defineProps<{
  modelValue: boolean
  path?: ProbePath
  chipText: string
}>()
const emit = defineEmits<{ 'update:modelValue': [boolean] }>()
const { t, te, locale } = useI18n()
const loading = ref(false)
const samples = ref<ProbeSampleBucket[]>([])
const trace = ref<ProbeTrace | null>(null)
const meta = reactive({ page: 1, page_size: 20, total: 0 })

function pretty(value: unknown, key = '') {
  return formatAdminValue(value, key, locale.value, t as never, te)
}

async function load() {
  const path = props.path
  if (!path || !props.modelValue) return
  loading.value = true
  try {
    const [sampleList, nextTrace] = await Promise.all([
      listProbeSamples({
        collector_id: path.collector_id, server_id: path.server_id,
        page: meta.page, page_size: meta.page_size,
      }),
      getProbeTrace({ collector_id: path.collector_id, server_id: path.server_id }),
    ])
    samples.value = sampleList.data
    meta.total = sampleList.meta.total || sampleList.data.length
    trace.value = nextTrace
  } catch (error) {
    notifyAPIError(error, t as never, te)
  } finally {
    loading.value = false
  }
}

watch(() => [props.modelValue, props.path?.collector_id, props.path?.server_id], () => {
  if (!props.modelValue || !props.path) return
  meta.page = 1
  trace.value = null
  void load()
})
</script>

<template>
  <AppDrawer :model-value="modelValue" :title="path?.server_name || t('probeObservation')" mode="view" @update:model-value="emit('update:modelValue', $event)">
    <div class="page-stack">
      <dl v-if="path" class="mobile-card-meta">
        <div><dt>{{ t('collector') }}</dt><dd>{{ path.collector_name }}</dd></div>
        <div><dt>{{ t('target') }}</dt><dd>{{ probeTargetText(path) }}</dd></div>
        <div><dt>{{ t('latency') }}</dt><dd>{{ chipText }}</dd></div>
        <div><dt>{{ t('lastObservation') }}</dt><dd>{{ pretty(path.sampled_at, 'sampled_at') }}</dd></div>
        <div><dt>{{ t('icmp') }}</dt><dd>{{ path.icmp?.ok ? pretty(path.icmp.rtt_ms, 'rtt_ms') : t('probeTimeout') }}</dd></div>
        <div><dt>{{ t('loss') }}</dt><dd>{{ formatProbeLoss(path.icmp?.loss, locale) }}</dd></div>
        <div><dt>{{ t('lastError') }}</dt><dd><CopyableId :value="path.last_error" :compact="false" /></dd></div>
      </dl>
      <el-table v-if="path?.tcp?.length" :data="path.tcp" class="dataset-table">
        <el-table-column :label="t('tcp')" width="90"><template #default="{row}">{{ row.port }}</template></el-table-column>
        <el-table-column :label="t('status')" width="90"><template #default="{row}">{{ row.ok ? t('probeReachable') : t('probeTimeout') }}</template></el-table-column>
        <el-table-column :label="t('latency')"><template #default="{row}">{{ row.ok ? pretty(row.rtt_ms, 'rtt_ms') : '—' }}</template></el-table-column>
      </el-table>
      <el-table v-loading="loading" :data="samples" class="dataset-table">
        <el-table-column :label="t('bucketStart')" min-width="180"><template #default="{row}">{{ pretty(row.bucket_start, 'bucket_start') }}</template></el-table-column>
        <el-table-column :label="t('type')" width="80"><template #default="{row}">{{ pretty(row.kind, 'kind') }}</template></el-table-column>
        <el-table-column :label="t('tcp')" width="80"><template #default="{row}">{{ row.port || '—' }}</template></el-table-column>
        <el-table-column :label="t('minMs')" width="90"><template #default="{row}">{{ pretty(row.min_ms, 'min_ms') }}</template></el-table-column>
        <el-table-column :label="t('avgMs')" width="90"><template #default="{row}">{{ pretty(row.avg_ms, 'avg_ms') }}</template></el-table-column>
        <el-table-column :label="t('maxMs')" width="90"><template #default="{row}">{{ pretty(row.max_ms, 'max_ms') }}</template></el-table-column>
        <el-table-column :label="t('loss')" width="80"><template #default="{row}">{{ formatProbeLoss(row.loss, locale) }}</template></el-table-column>
        <template #empty><AppEmpty icon="ri-timer-line" :description="t('noData')" /></template>
      </el-table>
      <div v-if="meta.total" class="pagination">
        <el-pagination v-model:current-page="meta.page" v-model:page-size="meta.page_size" layout="total, prev, pager, next" :total="meta.total" @change="load"/>
      </div>
      <h3 v-if="trace" class="editor-section-title"><span>{{ t('probeTrace') }}</span></h3>
      <el-table v-if="trace" :data="trace.hops" class="dataset-table">
        <el-table-column :label="t('hop')" width="70"><template #default="{row}">{{ row.ttl }}</template></el-table-column>
        <el-table-column :label="t('address')" min-width="160"><template #default="{row}">{{ row.address || '—' }}</template></el-table-column>
        <el-table-column :label="t('avgMs')" width="90"><template #default="{row}">{{ pretty(row.avg_ms, 'avg_ms') }}</template></el-table-column>
        <el-table-column :label="t('loss')" width="80"><template #default="{row}">{{ formatProbeLoss(row.loss, locale) }}</template></el-table-column>
        <template #empty><AppEmpty icon="ri-route-line" :description="t('noData')" /></template>
      </el-table>
    </div>
  </AppDrawer>
</template>
