<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { AppDrawer, AppEmpty } from '@santaizi/ui'
import type { ProbePath } from '@santaizi/api'
import { listCollectors, listProbePaths, type CollectorRecord } from '@/api/adminApi'
import CopyableId from '@/components/CopyableId.vue'
import ProbePathDrawer from '@/components/ProbePathDrawer.vue'
import { formatAdminValue, formatClockTime, formatDateTime, formatProductVersion } from '@/composables/format'
import { notifyAPIError } from '@/composables/notify'
import { isProbeCollector } from '@/domain/collectorKind'
import { probeHasNoTarget, probePathKey } from '@/domain/probePath'

const { t, te, locale } = useI18n()
const router = useRouter()
const loading = ref(false)
const collectors = ref<CollectorRecord[]>([])
const probePaths = ref<ProbePath[]>([])
const collectorFilter = ref('')
const statusFilter = ref('')
const collectorDrawer = ref(false)
const probeDrawer = ref(false)
const activeCollector = ref<CollectorRecord>()
const activeProbe = ref<ProbePath>()
const POLL_MS = 5000
const MOBILE_MATRIX_MQ = '(max-width: 860px)'
const serversAsRows = ref(typeof window !== 'undefined' && window.matchMedia(MOBILE_MATRIX_MQ).matches)
let timer: ReturnType<typeof setInterval> | undefined
let axisMq: MediaQueryList | undefined
let inflight = false

type MatrixCollector = { id: string; name: string }
type PathMatrixRow = {
  key: string
  server_id: number
  server_name: string
  cells: Record<string, ProbePath>
}
type MatrixCell = {
  probe: ProbePath
  reachable: boolean
  noTarget: boolean
  hasError: boolean
  title?: string
  text: string
  sampled: string
}

const probeCollectors = computed(() => collectors.value.filter(row => !row.revoked && isProbeCollector(row)))

const collectorOptions = computed(() => {
  const items: MatrixCollector[] = []
  const seen = new Set<string>()
  for (const row of probeCollectors.value) {
    seen.add(row.id)
    items.push({ id: row.id, name: row.name })
  }
  for (const path of probePaths.value) {
    if (seen.has(path.collector_id)) continue
    seen.add(path.collector_id)
    items.push({ id: path.collector_id, name: path.collector_name || path.collector_id })
  }
  return items
})

function probeStatus(path: ProbePath) {
  if (probeHasNoTarget(path)) return 'none'
  if (!collectorOnline(path.collector_id)) return 'offline'
  if (path.reachable && path.sampled_at) return 'reachable'
  return 'timeout'
}

const filteredProbePaths = computed(() => probePaths.value.filter((path) => {
  if (collectorFilter.value && path.collector_id !== collectorFilter.value) return false
  if (!statusFilter.value) return true
  return probeStatus(path) === statusFilter.value
}))

const matrixCollectors = computed(() => {
  if (!collectorFilter.value) return collectorOptions.value
  return collectorOptions.value.filter((item) => item.id === collectorFilter.value)
})

const matrixRows = computed(() => {
  const matching = new Set<string>()
  for (const path of filteredProbePaths.value) matching.add(String(path.server_id))
  const rows = new Map<string, PathMatrixRow>()
  for (const path of probePaths.value) {
    const key = String(path.server_id)
    if (!matching.has(key)) continue
    let row = rows.get(key)
    if (!row) {
      row = { key, server_id: path.server_id, server_name: path.server_name || '', cells: {} }
      rows.set(key, row)
    }
    if (!row.server_name && path.server_name) row.server_name = path.server_name
    row.cells[path.collector_id] = path
  }
  return [...rows.values()]
})

function pretty(value: unknown, key = '') {
  return formatAdminValue(value, key, locale.value, t as never, te)
}

function lastErrorText(value?: string | null) {
  return pretty(value, 'last_error')
}

function latencyText(ms?: number | null, sampled?: string | null) {
  if (!sampled) return '—'
  return pretty(ms, 'last_rtt_ms')
}

function sampledClock(sampled?: string | null) {
  return formatClockTime(sampled, locale.value)
}

function sampledTitle(sampled?: string | null) {
  if (!sampled) return undefined
  const text = formatDateTime(sampled, locale.value)
  return text === '—' ? undefined : text
}

function collectorVersionText(row: CollectorRecord) {
  return formatProductVersion(row.software_version) || '—'
}

function collectorStatus(row: CollectorRecord) {
  return row.revoked ? 'offline' : (row.status || 'unknown')
}

function collectorOnline(id: string) {
  const row = probeCollectors.value.find(item => item.id === id)
  return Boolean(row && collectorStatus(row) === 'online')
}

function collectorRttTone(row: CollectorRecord) {
  return collectorStatus(row) === 'online' ? 'is-connected' : 'is-disconnected'
}

function collectorRttText(row: CollectorRecord) {
  if (collectorStatus(row) !== 'online') return t('offline')
  return latencyText(row.heartbeat_rtt_ms, row.heartbeat_rtt_sampled_at)
}

function collectorRttSampled(row: CollectorRecord) {
  if (collectorStatus(row) !== 'online') return ''
  return sampledClock(row.heartbeat_rtt_sampled_at)
}

function collectorRttTitle(row: CollectorRecord) {
  if (collectorStatus(row) !== 'online') return undefined
  return sampledTitle(row.heartbeat_rtt_sampled_at)
}

function probeChipText(path: ProbePath) {
  if (probeHasNoTarget(path)) return '—'
  if (!collectorOnline(path.collector_id)) return t('offline')
  if (path.reachable && path.sampled_at) return latencyText(path.display_rtt_ms, path.sampled_at)
  return t('probeTimeout')
}

function hasMatrixCell(row: PathMatrixRow, collector: MatrixCollector) {
  return Boolean(row.cells[collector.id])
}

function matrixCells(row: PathMatrixRow, collector: MatrixCollector): MatrixCell[] {
  const path = row.cells[collector.id]
  if (!path) return []
  const noTarget = probeHasNoTarget(path)
  const status = probeStatus(path)
  return [{
    probe: path,
    reachable: status === 'reachable',
    noTarget,
    hasError: Boolean(path.last_error) && collectorOnline(path.collector_id),
    title: status === 'reachable'
      ? sampledTitle(path.sampled_at)
      : (path.last_error && collectorOnline(path.collector_id) ? lastErrorText(path.last_error) : undefined),
    text: probeChipText(path),
    sampled: status === 'reachable' ? sampledClock(path.sampled_at) : '',
  }]
}

function cellTone(cell: MatrixCell) {
  if (cell.noTarget) return ''
  return cell.reachable ? 'is-connected' : 'is-disconnected'
}

function openCollector(row: CollectorRecord) {
  activeCollector.value = row
  collectorDrawer.value = true
}

function openProbe(row: ProbePath) {
  activeProbe.value = row
  probeDrawer.value = true
}

async function load(quiet = false) {
  if (inflight) return
  inflight = true
  if (!quiet) loading.value = true
  try {
    const [collectorList, probeList] = await Promise.all([
      listCollectors(),
      listProbePaths(),
    ])
    collectors.value = collectorList.data
    probePaths.value = probeList.data
  } catch (error) {
    notifyAPIError(error, t as never, te)
  } finally {
    inflight = false
    loading.value = false
  }
}

function onVisibility() {
  if (!document.hidden) void load(true)
}

function syncMatrixAxis() {
  serversAsRows.value = Boolean(axisMq?.matches)
}

onMounted(async () => {
  axisMq = window.matchMedia(MOBILE_MATRIX_MQ)
  syncMatrixAxis()
  axisMq.addEventListener('change', syncMatrixAxis)
  await load()
  timer = setInterval(() => { if (!document.hidden) void load(true) }, POLL_MS)
  document.addEventListener('visibilitychange', onVisibility)
})
onUnmounted(() => {
  axisMq?.removeEventListener('change', syncMatrixAxis)
  if (timer) clearInterval(timer)
  document.removeEventListener('visibilitychange', onVisibility)
})
</script>

<template>
  <div class="probes-page">
  <div class="page-head">
    <h1>{{ t('probeObservation') }}</h1>
    <el-button @click="load()"><i class="ri-refresh-line"></i>{{ t('refresh') }}</el-button>
  </div>

  <div class="page-stack">
    <section class="surface table-card connections-collectors">
      <div class="toolbar">
        <h2 class="table-card-title">{{ t('collectors') }} <small>{{ probeCollectors.length }}</small></h2>
      </div>
      <div class="collector-grid" v-loading="loading">
        <AppEmpty v-if="!probeCollectors.length && !loading" class="empty-state" icon="ri-radar-line" :title="t('noProbesTitle')" :description="t('noProbesHint')" />
        <article
          v-for="row in probeCollectors"
          :key="row.id"
          class="collector-tile"
          role="button"
          tabindex="0"
          @click="openCollector(row)"
          @keydown.enter.prevent="openCollector(row)"
          @keydown.space.prevent="openCollector(row)"
        >
          <span class="status-dot" :class="collectorStatus(row)"></span>
          <div class="collector-tile__id">
            <strong>{{ row.name }}</strong>
            <CopyableId :value="row.id" />
          </div>
          <span class="rtt-chip" :class="collectorRttTone(row)" :title="collectorRttTitle(row)">
            <span class="rtt-value">{{ collectorRttText(row) }}</span>
            <span v-if="collectorRttSampled(row)" class="rtt-sampled">{{ collectorRttSampled(row) }}</span>
          </span>
          <div class="collector-tile__metrics">
            <span>{{ collectorVersionText(row) }}</span>
          </div>
        </article>
      </div>
      <div v-if="!probeCollectors.length && !loading" class="pagination">
        <el-button type="primary" @click="router.push('/telemetry?create=1')"><i class="ri-add-line"></i>{{ t('createCollector') }}</el-button>
      </div>
    </section>

    <section class="surface table-card connections-nodes">
      <div class="toolbar">
        <h2 class="table-card-title">{{ t('probePaths') }} <small>{{ filteredProbePaths.length }}</small></h2>
        <div class="toolbar-filters mobile-only">
          <el-select v-model="collectorFilter" class="toolbar-filter" clearable :placeholder="t('allProbes')">
            <el-option v-for="item in collectorOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
          <el-select v-model="statusFilter" class="toolbar-filter" clearable :placeholder="t('allProbeStatus')">
            <el-option :label="t('probeReachable')" value="reachable" />
            <el-option :label="t('probeTimeout')" value="timeout" />
            <el-option :label="t('probeNoTarget')" value="none" />
          </el-select>
        </div>
      </div>
      <div class="path-matrix-wrap" v-loading="loading">
        <AppEmpty v-if="!matrixRows.length && !loading" class="empty-state" icon="ri-radar-line" :title="t('noProbePathsTitle')" :description="t('noProbePathsHint')" />
        <div
          v-else
          class="path-matrix"
          :class="serversAsRows ? 'is-servers-y' : 'is-servers-x'"
          role="table"
          :style="serversAsRows
            ? { '--obs-count': String(matrixCollectors.length || 1) }
            : { '--server-count': String(matrixRows.length || 1) }"
        >
          <div class="path-matrix__head" role="row">
            <div class="col-corner" role="columnheader">{{ serversAsRows ? t('server') : '' }}</div>
            <template v-if="serversAsRows">
              <div
                v-for="obs in matrixCollectors"
                :key="obs.id"
                class="col-observer is-probe"
                role="columnheader"
              >
                <i class="ri-radar-line" aria-hidden="true"></i>
                <span>{{ obs.name }}</span>
              </div>
            </template>
            <template v-else>
              <div
                v-for="row in matrixRows"
                :key="row.key"
                class="col-server"
                role="columnheader"
                :title="row.server_name || undefined"
              >{{ row.server_name || '—' }}</div>
            </template>
          </div>
          <template v-if="serversAsRows">
            <div
              v-for="row in matrixRows"
              :key="row.key"
              class="path-matrix__row"
              role="row"
            >
              <div class="path-matrix__stub col-server" role="rowheader" :title="row.server_name || undefined">{{ row.server_name || '—' }}</div>
              <div
                v-for="obs in matrixCollectors"
                :key="obs.id"
                class="col-observer"
                role="cell"
              >
                <button
                  v-for="cell in matrixCells(row, obs)"
                  :key="probePathKey(cell.probe)"
                  type="button"
                  class="path-matrix__cell"
                  :class="cellTone(cell)"
                  :title="cell.title"
                  @click="openProbe(cell.probe)"
                >
                  <span class="rtt-main">
                    <i v-if="cell.hasError" class="ri-error-warning-line" aria-hidden="true"></i>
                    <span class="rtt-value">{{ cell.text }}</span>
                  </span>
                  <span v-if="cell.sampled" class="rtt-sampled">{{ cell.sampled }}</span>
                </button>
                <span v-if="!hasMatrixCell(row, obs)" class="path-matrix__empty" aria-hidden="true"></span>
              </div>
            </div>
          </template>
          <template v-else>
            <div
              v-for="obs in matrixCollectors"
              :key="obs.id"
              class="path-matrix__row"
              role="row"
            >
              <div
                class="path-matrix__stub col-observer is-probe"
                role="rowheader"
                :title="obs.name"
              >
                <i class="ri-radar-line" aria-hidden="true"></i>
                <span>{{ obs.name }}</span>
              </div>
              <div
                v-for="row in matrixRows"
                :key="row.key"
                class="col-observer"
                role="cell"
              >
                <button
                  v-for="cell in matrixCells(row, obs)"
                  :key="probePathKey(cell.probe)"
                  type="button"
                  class="path-matrix__cell"
                  :class="cellTone(cell)"
                  :title="cell.title"
                  @click="openProbe(cell.probe)"
                >
                  <span class="rtt-main">
                    <i v-if="cell.hasError" class="ri-error-warning-line" aria-hidden="true"></i>
                    <span class="rtt-value">{{ cell.text }}</span>
                  </span>
                  <span v-if="cell.sampled" class="rtt-sampled">{{ cell.sampled }}</span>
                </button>
                <span v-if="!hasMatrixCell(row, obs)" class="path-matrix__empty" aria-hidden="true"></span>
              </div>
            </div>
          </template>
        </div>
      </div>
    </section>
  </div>

  <AppDrawer v-model="collectorDrawer" :title="activeCollector?.name || t('collector')" mode="view">
    <div class="page-stack">
    <dl v-if="activeCollector" class="mobile-card-meta">
      <div><dt>{{ t('id') }}</dt><dd><CopyableId :value="activeCollector.id" :compact="false" /></dd></div>
      <div><dt>{{ t('collectorKind') }}</dt><dd>{{ t('collectorKindProbe') }}</dd></div>
      <div><dt>{{ t('status') }}</dt><dd>{{ t(collectorStatus(activeCollector)) }}</dd></div>
      <div><dt>{{ t('lastSeen') }}</dt><dd>{{ pretty(activeCollector.last_seen, 'last_seen') }}</dd></div>
      <div><dt>{{ t('lastPrimarySeen') }}</dt><dd>{{ pretty(activeCollector.last_primary_seen, 'last_primary_seen') }}</dd></div>
      <div><dt>{{ t('heartbeatLatency') }}</dt><dd>{{ collectorStatus(activeCollector) === 'online' ? latencyText(activeCollector.heartbeat_rtt_ms, activeCollector.heartbeat_rtt_sampled_at) : t('offline') }}</dd></div>
      <div><dt>{{ t('protocolVersion') }}</dt><dd>{{ pretty(activeCollector.protocol_version, 'protocol_version') }}</dd></div>
      <div><dt>{{ t('collectorVersion') }}</dt><dd>{{ collectorVersionText(activeCollector) }}</dd></div>
    </dl>
    </div>
  </AppDrawer>

  <ProbePathDrawer v-model="probeDrawer" :path="activeProbe" :chip-text="activeProbe ? probeChipText(activeProbe) : '—'" />
  </div>
</template>
