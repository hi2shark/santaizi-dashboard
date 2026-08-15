<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { AppDrawer, AppEmpty } from '@santaizi/ui'
import type { ConnectionLatencyBucket, ConnectionPath, ProbePath, ProbeSampleBucket, ProbeTrace } from '@santaizi/api'
import { getProbeTrace, listCollectors, listConnectionLatency, listConnectionPaths, listProbePaths, listProbeSamples, type CollectorRecord } from '@/api/adminApi'
import CopyableId from '@/components/CopyableId.vue'
import { formatAdminValue, formatClockTime, formatDateTime, formatProductVersion } from '@/composables/format'
import { notifyAPIError } from '@/composables/notify'
import { isProbeCollector } from '@/domain/collectorKind'

const { t, te, locale } = useI18n()
const router = useRouter()
const route = useRoute()
const loading = ref(false)
const latencyLoading = ref(false)
const collectors = ref<CollectorRecord[]>([])
const paths = ref<ConnectionPath[]>([])
const probePaths = ref<ProbePath[]>([])
const collectorLatency = ref<ConnectionLatencyBucket[]>([])
const pathLatency = ref<ConnectionLatencyBucket[]>([])
const probeSamples = ref<ProbeSampleBucket[]>([])
const probeTrace = ref<ProbeTrace | null>(null)
const collectorLatencyMeta = reactive({ page: 1, page_size: 20, total: 0 })
const pathLatencyMeta = reactive({ page: 1, page_size: 20, total: 0 })
const probeSampleMeta = reactive({ page: 1, page_size: 20, total: 0 })
const observerFilter = ref('')
const linkFilter = ref('')
const collectorDrawer = ref(false)
const pathDrawer = ref(false)
const probeDrawer = ref(false)
const activeCollector = ref<CollectorRecord>()
const activePath = ref<ConnectionPath>()
const activeProbe = ref<ProbePath>()
const POLL_MS = 5000
let timer: ReturnType<typeof setInterval> | undefined
let inflight = false

type MatrixObserver = { id: string; name: string; kind: 'primary' | 'collector' | 'probe' }
type PathMatrixRow = {
  key: string
  server_id?: number
  node_uuid?: string
  server_name: string
  cells: Record<string, ConnectionPath>
  probeCells: Record<string, ProbePath>
}
type MatrixCell = {
  kind: 'observer' | 'probe'
  path?: ConnectionPath
  probe?: ProbePath
  connected: boolean
  hasError: boolean
  title?: string
  text: string
  sampled: string
}

function rowIdentity(serverId?: number, nodeUuid?: string) {
  if (serverId) return `s:${serverId}`
  return `n:${nodeUuid || ''}`
}

const observerOptions = computed(() => {
  const items: MatrixObserver[] = [{ id: 'primary', name: t('observerKindPrimary'), kind: 'primary' }]
  const seen = new Set(['primary'])
  const observers = collectors.value.filter(row => !row.revoked && !isProbeCollector(row))
  const probes = collectors.value.filter(row => !row.revoked && isProbeCollector(row))
  for (const row of observers) {
    seen.add(row.id)
    items.push({ id: row.id, name: row.name, kind: 'collector' })
  }
  for (const path of paths.value) {
    if (seen.has(path.observer_id)) continue
    seen.add(path.observer_id)
    items.push({
      id: path.observer_id,
      name: path.observer_kind === 'primary' ? t('observerKindPrimary') : (path.observer_name || path.observer_id),
      kind: path.observer_kind === 'primary' ? 'primary' : 'collector',
    })
  }
  for (const row of probes) {
    seen.add(row.id)
    items.push({ id: row.id, name: row.name, kind: 'probe' })
  }
  for (const path of probePaths.value) {
    if (seen.has(path.collector_id)) continue
    seen.add(path.collector_id)
    items.push({ id: path.collector_id, name: path.collector_name || path.collector_id, kind: 'probe' })
  }
  return items
})

const filteredPaths = computed(() => paths.value.filter((path) => {
  if (observerFilter.value && path.observer_id !== observerFilter.value) return false
  if (linkFilter.value === 'connected' && !path.sink.connected) return false
  if (linkFilter.value === 'disconnected' && path.sink.connected) return false
  return true
}))

const filteredProbePaths = computed(() => probePaths.value.filter((path) => {
  if (observerFilter.value && path.collector_id !== observerFilter.value) return false
  if (path.target?.source === 'none') return !linkFilter.value
  if (linkFilter.value === 'connected' && !path.reachable) return false
  if (linkFilter.value === 'disconnected' && path.reachable) return false
  return true
}))

const matrixObservers = computed(() => {
  if (!observerFilter.value) return observerOptions.value
  return observerOptions.value.filter((item) => item.id === observerFilter.value)
})

const matrixRows = computed(() => {
  const matching = new Set<string>()
  for (const path of filteredPaths.value) matching.add(rowIdentity(path.server_id, path.node_uuid))
  for (const path of filteredProbePaths.value) matching.add(rowIdentity(path.server_id))
  const rows = new Map<string, PathMatrixRow>()
  const take = (key: string, serverId: number | undefined, nodeUuid: string | undefined, name: string) => {
    let row = rows.get(key)
    if (!row) {
      row = { key, server_id: serverId, node_uuid: nodeUuid, server_name: name, cells: {}, probeCells: {} }
      rows.set(key, row)
    }
    if (!row.server_name && name) row.server_name = name
    if (!row.server_id && serverId) row.server_id = serverId
    if (!row.node_uuid && nodeUuid) row.node_uuid = nodeUuid
    return row
  }
  for (const path of paths.value) {
    const key = rowIdentity(path.server_id, path.node_uuid)
    if (!matching.has(key)) continue
    take(key, path.server_id, path.node_uuid, path.server_name || '').cells[path.observer_id] = path
  }
  for (const path of probePaths.value) {
    const key = rowIdentity(path.server_id)
    if (!matching.has(key)) continue
    take(key, path.server_id, undefined, path.server_name || '').probeCells[path.collector_id] = path
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

function collectorReplicationText(row: CollectorRecord) {
  if (!row.pending_records) return t('caughtUp')
  const oldest = row.oldest_pending ? new Date(row.oldest_pending) : null
  if (!oldest || Number.isNaN(oldest.valueOf())) return t('replicating')
  const lagMs = Date.now() - oldest.getTime()
  if (lagMs < 0) return t('replicating')
  const hours = Math.floor(lagMs / 3_600_000)
  if (hours >= 1) return t('replicationLagHours', { n: hours })
  const minutes = Math.floor(lagMs / 60_000)
  if (minutes >= 1) return t('replicationLagMinutes', { n: minutes })
  return t('replicationLagSeconds', { n: Math.max(1, Math.floor(lagMs / 1000)) })
}

function pathRowKey(row: ConnectionPath) {
  return `${row.node_uuid}:${row.observer_id}`
}

function probeRowKey(row: ProbePath) {
  return `${row.server_id}:${row.collector_id}`
}

function observerLabel(path: ConnectionPath) {
  if (path.observer_kind === 'primary') return t('observerKindPrimary')
  return path.observer_name || path.observer_id
}

function hasMatrixCell(row: PathMatrixRow, observer: MatrixObserver) {
  return observer.kind === 'probe' ? Boolean(row.probeCells[observer.id]) : Boolean(row.cells[observer.id])
}

function probeChipText(path: ProbePath) {
  if (path.target?.source === 'none') return '—'
  if (path.reachable && path.sampled_at) return latencyText(path.display_rtt_ms, path.sampled_at)
  return t('probeTimeout')
}

function matrixCells(row: PathMatrixRow, observer: MatrixObserver): MatrixCell[] {
  if (observer.kind === 'probe') {
    const path = row.probeCells[observer.id]
    if (!path) return []
    const noTarget = path.target?.source === 'none'
    return [{
      kind: 'probe',
      probe: path,
      connected: path.reachable,
      hasError: Boolean(path.last_error),
      title: path.last_error ? lastErrorText(path.last_error) : sampledTitle(path.sampled_at),
      text: probeChipText(path),
      sampled: noTarget ? '' : sampledClock(path.sampled_at),
    }]
  }
  const path = row.cells[observer.id]
  if (!path) return []
  return [{
    kind: 'observer',
    path,
    connected: path.sink.connected,
    hasError: Boolean(path.sink.last_error),
    title: path.sink.last_error ? lastErrorText(path.sink.last_error) : (path.sink.connected ? sampledTitle(path.sink.rtt_sampled_at) : undefined),
    text: path.sink.connected
      ? latencyText(path.sink.last_rtt_ms, path.sink.rtt_sampled_at)
      : t('disconnected'),
    sampled: path.sink.connected ? sampledClock(path.sink.rtt_sampled_at) : '',
  }]
}

function cellTone(cell: MatrixCell) {
  if (cell.kind === 'probe' && cell.probe?.target?.source === 'none') return ''
  return cell.connected ? 'is-connected' : 'is-disconnected'
}

function openMatrixCell(cell: MatrixCell) {
  if (cell.kind === 'probe' && cell.probe) openProbe(cell.probe)
  else if (cell.path) openPath(cell.path)
}

function formatLoss(value?: number | null) {
  if (value == null || !Number.isFinite(value)) return '—'
  const percent = value <= 1 ? value * 100 : value
  return `${new Intl.NumberFormat(locale.value, { maximumFractionDigits: 1 }).format(percent)}%`
}

async function loadCollectorLatency() {
  const collector = activeCollector.value
  if (!collector || isProbeCollector(collector)) return
  latencyLoading.value = true
  try {
    const result = await listConnectionLatency({
      collector_id: collector.id, page: collectorLatencyMeta.page, page_size: collectorLatencyMeta.page_size,
    })
    collectorLatency.value = result.data
    collectorLatencyMeta.total = result.meta.total || result.data.length
  } catch (error) {
    notifyAPIError(error, t as never, te)
  } finally {
    latencyLoading.value = false
  }
}

async function loadPathLatency() {
  const path = activePath.value
  if (!path) return
  latencyLoading.value = true
  try {
    const result = await listConnectionLatency({
      kind: 'path', server_id: path.server_id || undefined, observer_id: path.observer_id,
      page: pathLatencyMeta.page, page_size: pathLatencyMeta.page_size,
    })
    pathLatency.value = result.data
    pathLatencyMeta.total = result.meta.total || result.data.length
  } catch (error) {
    notifyAPIError(error, t as never, te)
  } finally {
    latencyLoading.value = false
  }
}

async function loadProbeDetail() {
  const path = activeProbe.value
  if (!path) return
  latencyLoading.value = true
  try {
    const [samples, trace] = await Promise.all([
      listProbeSamples({
        collector_id: path.collector_id, server_id: path.server_id,
        page: probeSampleMeta.page, page_size: probeSampleMeta.page_size,
      }),
      getProbeTrace({ collector_id: path.collector_id, server_id: path.server_id }),
    ])
    probeSamples.value = samples.data
    probeSampleMeta.total = samples.meta.total || samples.data.length
    probeTrace.value = trace
  } catch (error) {
    notifyAPIError(error, t as never, te)
  } finally {
    latencyLoading.value = false
  }
}

function openCollector(row: CollectorRecord) {
  activeCollector.value = row
  collectorLatencyMeta.page = 1
  collectorDrawer.value = true
  void loadCollectorLatency()
}

function openPath(row: ConnectionPath) {
  activePath.value = row
  pathLatencyMeta.page = 1
  pathDrawer.value = true
  void loadPathLatency()
}

function openProbe(row: ProbePath) {
  activeProbe.value = row
  probeSampleMeta.page = 1
  probeTrace.value = null
  probeDrawer.value = true
  void loadProbeDetail()
}

async function load(quiet = false) {
  if (inflight) return
  inflight = true
  if (!quiet) loading.value = true
  try {
    const [collectorList, pathList, probeList] = await Promise.all([
      listCollectors(),
      listConnectionPaths(),
      listProbePaths().catch((error) => {
        notifyAPIError(error, t as never, te)
        return { data: [] as ProbePath[] }
      }),
    ])
    collectors.value = collectorList.data
    paths.value = pathList.data
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

onMounted(async () => {
  const observerId = String(route.query.observer_id || '')
  if (observerId) observerFilter.value = observerId
  await load()
  timer = setInterval(() => { if (!document.hidden) void load(true) }, POLL_MS)
  document.addEventListener('visibilitychange', onVisibility)
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
  document.removeEventListener('visibilitychange', onVisibility)
})
</script>

<template>
  <div class="connections-page">
  <div class="page-head">
    <h1>{{ t('connections') }}</h1>
    <el-button @click="load()"><i class="ri-refresh-line"></i>{{ t('refresh') }}</el-button>
  </div>

  <div class="page-stack">
    <section class="surface table-card">
      <div class="toolbar">
        <h2 class="table-card-title">{{ t('collectorLinks') }} <small>{{ collectors.length }}</small></h2>
      </div>
      <div class="collector-grid" v-loading="loading">
        <AppEmpty v-if="!collectors.length && !loading" class="empty-state" icon="ri-radar-line" :title="t('noCollectorsTitle')" :description="t('noCollectorsHint')" />
        <article
          v-for="row in collectors"
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
            <template v-if="isProbeCollector(row)">
              <span>{{ collectorVersionText(row) }}</span>
            </template>
            <template v-else>
              <span>{{ pretty(row.connected_agents, 'connected_agents') }} {{ t('connectedAgents') }}</span>
              <span>{{ collectorReplicationText(row) }}</span>
            </template>
          </div>
        </article>
      </div>
      <div v-if="!collectors.length && !loading" class="pagination">
        <el-button type="primary" @click="router.push('/telemetry?create=1')"><i class="ri-add-line"></i>{{ t('createCollector') }}</el-button>
      </div>
    </section>

    <section class="surface table-card">
      <div class="toolbar">
        <h2 class="table-card-title">{{ t('nodeLinks') }} <small>{{ filteredPaths.length + filteredProbePaths.length }}</small></h2>
        <div class="toolbar-filters mobile-only">
          <el-select v-model="observerFilter" class="toolbar-filter" clearable :placeholder="t('allObservers')">
            <el-option v-for="item in observerOptions" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
          <el-select v-model="linkFilter" class="toolbar-filter" clearable :placeholder="t('allLinkStatus')">
            <el-option :label="t('connected')" value="connected" />
            <el-option :label="t('disconnected')" value="disconnected" />
          </el-select>
        </div>
      </div>
      <div class="path-matrix-wrap desktop-only" v-loading="loading">
        <AppEmpty v-if="!matrixRows.length && !loading" class="empty-state" icon="ri-links-line" :title="t('noPathsTitle')" :description="t('noPathsHint')" />
        <div
          v-else
          class="path-matrix"
          role="table"
          :style="{ '--obs-count': String(matrixObservers.length || 1) }"
        >
          <div class="path-matrix__head" role="row">
            <div class="col-server" role="columnheader">{{ t('server') }}</div>
            <div
              v-for="obs in matrixObservers"
              :key="obs.id"
              class="col-observer"
              :class="{ 'is-probe': obs.kind === 'probe' }"
              role="columnheader"
            >
              <i v-if="obs.kind === 'probe'" class="ri-radar-line" aria-hidden="true"></i>
              <span>{{ obs.name }}</span>
            </div>
          </div>
          <div
            v-for="row in matrixRows"
            :key="row.key"
            class="path-matrix__row"
            role="row"
          >
            <div class="col-server" role="rowheader">{{ row.server_name || '—' }}</div>
            <div
              v-for="obs in matrixObservers"
              :key="obs.id"
              class="col-observer"
              role="cell"
            >
              <button
                v-for="cell in matrixCells(row, obs)"
                :key="cell.kind === 'probe' && cell.probe ? probeRowKey(cell.probe) : pathRowKey(cell.path!)"
                type="button"
                class="path-matrix__cell"
                :class="cellTone(cell)"
                :title="cell.title"
                @click="openMatrixCell(cell)"
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
        </div>
      </div>
      <div class="mobile-only" v-loading="loading">
        <AppEmpty v-if="!filteredPaths.length && !filteredProbePaths.length && !loading" class="empty-state" icon="ri-links-line" :title="t('noPathsTitle')" :description="t('noPathsHint')" />
        <div v-else class="mobile-card-list">
          <article v-for="row in filteredPaths" :key="pathRowKey(row)" class="mobile-card" @click="openPath(row)">
            <div class="mobile-card-head">
              <span class="mobile-card-status"><span class="status-dot" :class="row.sink.connected ? 'online' : 'offline'"></span></span>
              <div class="mobile-card-title"><strong>{{ row.server_name || '—' }}</strong><small>{{ observerLabel(row) }}</small></div>
            </div>
            <dl class="mobile-card-meta">
              <div><dt>{{ t('linkStatus') }}</dt><dd>{{ t(row.sink.connected ? 'connected' : 'disconnected') }}</dd></div>
              <div><dt>{{ t('lastObservation') }}</dt><dd>{{ pretty(row.last_seen, 'last_seen') }}</dd></div>
              <div><dt>{{ t('latency') }}</dt><dd>{{ latencyText(row.sink.last_rtt_ms, row.sink.rtt_sampled_at) }}</dd></div>
              <div><dt>{{ t('pendingEvents') }}</dt><dd>{{ pretty(row.sink.pending_events, 'pending_events') }}</dd></div>
              <div>
                <dt>{{ t('lastError') }}</dt>
                <dd><span class="cell-ellipsis" :title="lastErrorText(row.sink.last_error)">{{ lastErrorText(row.sink.last_error) }}</span></dd>
              </div>
            </dl>
          </article>
          <article v-for="row in filteredProbePaths" :key="probeRowKey(row)" class="mobile-card" @click="openProbe(row)">
            <div class="mobile-card-head">
              <span class="mobile-card-status"><span class="status-dot" :class="row.reachable ? 'online' : 'offline'"></span></span>
              <div class="mobile-card-title"><strong>{{ row.server_name || '—' }}</strong><small>{{ row.collector_name }}</small></div>
            </div>
            <dl class="mobile-card-meta">
              <div><dt>{{ t('latency') }}</dt><dd>{{ probeChipText(row) }}</dd></div>
              <div><dt>{{ t('lastObservation') }}</dt><dd>{{ pretty(row.sampled_at, 'sampled_at') }}</dd></div>
              <div>
                <dt>{{ t('lastError') }}</dt>
                <dd><span class="cell-ellipsis" :title="lastErrorText(row.last_error)">{{ lastErrorText(row.last_error) }}</span></dd>
              </div>
            </dl>
          </article>
        </div>
      </div>
    </section>
  </div>

  <AppDrawer v-model="collectorDrawer" :title="activeCollector?.name || t('collector')" mode="view">
    <div class="page-stack">
    <dl v-if="activeCollector" class="mobile-card-meta">
      <div><dt>{{ t('id') }}</dt><dd><CopyableId :value="activeCollector.id" :compact="false" /></dd></div>
      <div><dt>{{ t('collectorKind') }}</dt><dd>{{ t(isProbeCollector(activeCollector) ? 'collectorKindProbe' : 'collectorKindObserver') }}</dd></div>
      <div><dt>{{ t('status') }}</dt><dd>{{ t(collectorStatus(activeCollector)) }}</dd></div>
      <div><dt>{{ t('lastSeen') }}</dt><dd>{{ pretty(activeCollector.last_seen, 'last_seen') }}</dd></div>
      <div v-if="!isProbeCollector(activeCollector)"><dt>{{ t('lastSync') }}</dt><dd>{{ pretty(activeCollector.last_sync, 'last_sync') }}</dd></div>
      <div><dt>{{ t('lastPrimarySeen') }}</dt><dd>{{ pretty(activeCollector.last_primary_seen, 'last_primary_seen') }}</dd></div>
      <div><dt>{{ t('heartbeatLatency') }}</dt><dd>{{ latencyText(activeCollector.heartbeat_rtt_ms, activeCollector.heartbeat_rtt_sampled_at) }}</dd></div>
      <div v-if="!isProbeCollector(activeCollector)"><dt>{{ t('replicationLatency') }}</dt><dd>{{ latencyText(activeCollector.replication_rtt_ms, activeCollector.replication_rtt_sampled_at) }}</dd></div>
      <div v-if="!isProbeCollector(activeCollector)"><dt>{{ t('connectedAgents') }}</dt><dd>{{ pretty(activeCollector.connected_agents, 'connected_agents') }}</dd></div>
      <div v-if="!isProbeCollector(activeCollector)"><dt>{{ t('pendingRecords') }}</dt><dd>{{ pretty(activeCollector.pending_records, 'pending_records') }}</dd></div>
      <div v-if="!isProbeCollector(activeCollector)"><dt>{{ t('oldestPending') }}</dt><dd>{{ pretty(activeCollector.oldest_pending, 'oldest_pending') }}</dd></div>
      <div v-if="!isProbeCollector(activeCollector)"><dt>{{ t('replicationCursor') }}</dt><dd>{{ pretty(activeCollector.replication_cursor, 'replication_cursor') }}</dd></div>
      <div><dt>{{ t('protocolVersion') }}</dt><dd>{{ pretty(activeCollector.protocol_version, 'protocol_version') }}</dd></div>
      <div><dt>{{ t('collectorVersion') }}</dt><dd>{{ collectorVersionText(activeCollector) }}</dd></div>
    </dl>
    <el-table v-if="activeCollector && !isProbeCollector(activeCollector)" v-loading="latencyLoading" :data="collectorLatency" class="dataset-table">
      <el-table-column :label="t('bucketStart')" min-width="180"><template #default="{row}">{{ pretty(row.bucket_start, 'bucket_start') }}</template></el-table-column>
      <el-table-column :label="t('type')" width="90"><template #default="{row}">{{ pretty(row.kind, 'kind') }}</template></el-table-column>
      <el-table-column :label="t('minMs')" width="90"><template #default="{row}">{{ pretty(row.min_ms, 'min_ms') }}</template></el-table-column>
      <el-table-column :label="t('avgMs')" width="90"><template #default="{row}">{{ pretty(row.avg_ms, 'avg_ms') }}</template></el-table-column>
      <el-table-column :label="t('maxMs')" width="90"><template #default="{row}">{{ pretty(row.max_ms, 'max_ms') }}</template></el-table-column>
      <template #empty><AppEmpty icon="ri-timer-line" :description="t('noData')" /></template>
    </el-table>
    <div v-if="activeCollector && !isProbeCollector(activeCollector) && collectorLatencyMeta.total" class="pagination">
      <el-pagination v-model:current-page="collectorLatencyMeta.page" v-model:page-size="collectorLatencyMeta.page_size" layout="total, prev, pager, next" :total="collectorLatencyMeta.total" @change="loadCollectorLatency"/>
    </div>
    </div>
  </AppDrawer>

  <AppDrawer v-model="pathDrawer" :title="activePath ? (activePath.server_name || t('server')) : t('nodeLinks')" mode="view">
    <div class="page-stack">
    <dl v-if="activePath" class="mobile-card-meta">
      <div><dt>{{ t('observer') }}</dt><dd>{{ observerLabel(activePath) }}</dd></div>
      <div><dt>{{ t('linkStatus') }}</dt><dd>{{ t(activePath.sink.connected ? 'connected' : 'disconnected') }}</dd></div>
      <div><dt>{{ t('lastObservation') }}</dt><dd>{{ pretty(activePath.last_seen, 'last_seen') }}</dd></div>
      <div><dt>{{ t('latency') }}</dt><dd>{{ latencyText(activePath.sink.last_rtt_ms, activePath.sink.rtt_sampled_at) }}</dd></div>
      <div><dt>{{ t('pendingEvents') }}</dt><dd>{{ pretty(activePath.sink.pending_events, 'pending_events') }}</dd></div>
      <div><dt>{{ t('ackThrough') }}</dt><dd>{{ pretty(activePath.sink.ack_through, 'ack_through') }}</dd></div>
      <div><dt>{{ t('lastError') }}</dt><dd><CopyableId :value="activePath.sink.last_error" :compact="false" /></dd></div>
      <div><dt>{{ t('nodeUUID') }}</dt><dd><CopyableId :value="activePath.node_uuid" :compact="false" /></dd></div>
    </dl>
    <el-table v-loading="latencyLoading" :data="pathLatency" class="dataset-table">
      <el-table-column :label="t('bucketStart')" min-width="180"><template #default="{row}">{{ pretty(row.bucket_start, 'bucket_start') }}</template></el-table-column>
      <el-table-column :label="t('minMs')" width="90"><template #default="{row}">{{ pretty(row.min_ms, 'min_ms') }}</template></el-table-column>
      <el-table-column :label="t('avgMs')" width="90"><template #default="{row}">{{ pretty(row.avg_ms, 'avg_ms') }}</template></el-table-column>
      <el-table-column :label="t('maxMs')" width="90"><template #default="{row}">{{ pretty(row.max_ms, 'max_ms') }}</template></el-table-column>
      <template #empty><AppEmpty icon="ri-timer-line" :description="t('noData')" /></template>
    </el-table>
    <div v-if="pathLatencyMeta.total" class="pagination">
      <el-pagination v-model:current-page="pathLatencyMeta.page" v-model:page-size="pathLatencyMeta.page_size" layout="total, prev, pager, next" :total="pathLatencyMeta.total" @change="loadPathLatency"/>
    </div>
    </div>
  </AppDrawer>

  <AppDrawer v-model="probeDrawer" :title="activeProbe ? (activeProbe.server_name || t('server')) : t('nodeLinks')" mode="view">
    <div class="page-stack">
    <dl v-if="activeProbe" class="mobile-card-meta">
      <div><dt>{{ t('collector') }}</dt><dd>{{ activeProbe.collector_name }}</dd></div>
      <div><dt>{{ t('target') }}</dt><dd>{{ activeProbe.target?.hostname || activeProbe.target?.ipv4 || activeProbe.target?.ipv6 || '—' }}</dd></div>
      <div><dt>{{ t('latency') }}</dt><dd>{{ probeChipText(activeProbe) }}</dd></div>
      <div><dt>{{ t('lastObservation') }}</dt><dd>{{ pretty(activeProbe.sampled_at, 'sampled_at') }}</dd></div>
      <div><dt>{{ t('icmp') }}</dt><dd>{{ activeProbe.icmp?.ok ? pretty(activeProbe.icmp.rtt_ms, 'rtt_ms') : t('probeTimeout') }}</dd></div>
      <div><dt>{{ t('loss') }}</dt><dd>{{ formatLoss(activeProbe.icmp?.loss) }}</dd></div>
      <div><dt>{{ t('lastError') }}</dt><dd><CopyableId :value="activeProbe.last_error" :compact="false" /></dd></div>
    </dl>
    <el-table v-if="activeProbe?.tcp?.length" :data="activeProbe.tcp" class="dataset-table">
      <el-table-column :label="t('tcp')" width="90"><template #default="{row}">{{ row.port }}</template></el-table-column>
      <el-table-column :label="t('status')" width="90"><template #default="{row}">{{ row.ok ? t('connected') : t('probeTimeout') }}</template></el-table-column>
      <el-table-column :label="t('latency')"><template #default="{row}">{{ row.ok ? pretty(row.rtt_ms, 'rtt_ms') : '—' }}</template></el-table-column>
    </el-table>
    <el-table v-loading="latencyLoading" :data="probeSamples" class="dataset-table">
      <el-table-column :label="t('bucketStart')" min-width="180"><template #default="{row}">{{ pretty(row.bucket_start, 'bucket_start') }}</template></el-table-column>
      <el-table-column :label="t('type')" width="80"><template #default="{row}">{{ pretty(row.kind, 'kind') }}</template></el-table-column>
      <el-table-column :label="t('tcp')" width="80"><template #default="{row}">{{ row.port || '—' }}</template></el-table-column>
      <el-table-column :label="t('minMs')" width="90"><template #default="{row}">{{ pretty(row.min_ms, 'min_ms') }}</template></el-table-column>
      <el-table-column :label="t('avgMs')" width="90"><template #default="{row}">{{ pretty(row.avg_ms, 'avg_ms') }}</template></el-table-column>
      <el-table-column :label="t('maxMs')" width="90"><template #default="{row}">{{ pretty(row.max_ms, 'max_ms') }}</template></el-table-column>
      <el-table-column :label="t('loss')" width="80"><template #default="{row}">{{ formatLoss(row.loss) }}</template></el-table-column>
      <template #empty><AppEmpty icon="ri-timer-line" :description="t('noData')" /></template>
    </el-table>
    <div v-if="probeSampleMeta.total" class="pagination">
      <el-pagination v-model:current-page="probeSampleMeta.page" v-model:page-size="probeSampleMeta.page_size" layout="total, prev, pager, next" :total="probeSampleMeta.total" @change="loadProbeDetail"/>
    </div>
    <h3 v-if="probeTrace" class="editor-section-title"><span>{{ t('probeTrace') }}</span></h3>
    <el-table v-if="probeTrace" :data="probeTrace.hops" class="dataset-table">
      <el-table-column :label="t('hop')" width="70"><template #default="{row}">{{ row.ttl }}</template></el-table-column>
      <el-table-column :label="t('address')" min-width="160"><template #default="{row}">{{ row.address || '—' }}</template></el-table-column>
      <el-table-column :label="t('avgMs')" width="90"><template #default="{row}">{{ pretty(row.avg_ms, 'avg_ms') }}</template></el-table-column>
      <el-table-column :label="t('loss')" width="80"><template #default="{row}">{{ formatLoss(row.loss) }}</template></el-table-column>
      <template #empty><AppEmpty icon="ri-route-line" :description="t('noData')" /></template>
    </el-table>
    </div>
  </AppDrawer>
  </div>
</template>
