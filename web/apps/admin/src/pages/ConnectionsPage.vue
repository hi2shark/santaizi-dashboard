<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { AppDrawer, AppEmpty } from '@santaizi/ui'
import type { ConnectionLatencyBucket, ConnectionPath } from '@santaizi/api'
import { listCollectors, listConnectionLatency, listConnectionPaths, type CollectorRecord } from '@/api/adminApi'
import CopyableId from '@/components/CopyableId.vue'
import { formatAdminValue, formatClockTime, formatDateTime, formatProductVersion } from '@/composables/format'
import { notifyAPIError } from '@/composables/notify'

const { t, te, locale } = useI18n()
const router = useRouter()
const route = useRoute()
const loading = ref(false)
const latencyLoading = ref(false)
const collectors = ref<CollectorRecord[]>([])
const paths = ref<ConnectionPath[]>([])
const collectorLatency = ref<ConnectionLatencyBucket[]>([])
const pathLatency = ref<ConnectionLatencyBucket[]>([])
const collectorLatencyMeta = reactive({ page: 1, page_size: 20, total: 0 })
const pathLatencyMeta = reactive({ page: 1, page_size: 20, total: 0 })
const observerFilter = ref('')
const linkFilter = ref('')
const collectorDrawer = ref(false)
const pathDrawer = ref(false)
const activeCollector = ref<CollectorRecord>()
const activePath = ref<ConnectionPath>()
const POLL_MS = 5000
let timer: ReturnType<typeof setInterval> | undefined
let inflight = false

type PathMatrixRow = {
  node_uuid: string
  server_name: string
  cells: Record<string, ConnectionPath>
}

const observerOptions = computed(() => {
  const names = new Map<string, string>()
  names.set('primary', t('observerKindPrimary'))
  for (const collector of collectors.value) names.set(collector.id, collector.name)
  for (const path of paths.value) {
    if (!names.has(path.observer_id)) names.set(path.observer_id, path.observer_name || path.observer_id)
  }
  return [...names.entries()].map(([id, name]) => ({ id, name }))
})

const filteredPaths = computed(() => paths.value.filter((path) => {
  if (observerFilter.value && path.observer_id !== observerFilter.value) return false
  if (linkFilter.value === 'connected' && !path.sink.connected) return false
  if (linkFilter.value === 'disconnected' && path.sink.connected) return false
  return true
}))

const matrixObservers = computed(() => {
  const ids = new Set<string>()
  for (const path of paths.value) {
    if (observerFilter.value && path.observer_id !== observerFilter.value) continue
    ids.add(path.observer_id)
  }
  return observerOptions.value.filter((item) => ids.has(item.id))
})

const matrixRows = computed(() => {
  const matching = new Set(filteredPaths.value.map((path) => path.node_uuid))
  const rows: PathMatrixRow[] = []
  const seen = new Set<string>()
  for (const path of paths.value) {
    if (seen.has(path.node_uuid) || !matching.has(path.node_uuid)) continue
    seen.add(path.node_uuid)
    const cells: Record<string, ConnectionPath> = {}
    for (const item of paths.value) {
      if (item.node_uuid === path.node_uuid) cells[item.observer_id] = item
    }
    rows.push({ node_uuid: path.node_uuid, server_name: path.server_name || '', cells })
  }
  return rows
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

function observerLabel(path: ConnectionPath) {
  if (path.observer_kind === 'primary') return t('observerKindPrimary')
  return path.observer_name || path.observer_id
}

function matrixCells(row: PathMatrixRow, observerId: string) {
  const path = row.cells[observerId]
  if (!path) return []
  return [{
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

async function loadCollectorLatency() {
  const collector = activeCollector.value
  if (!collector) return
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

async function load(quiet = false) {
  if (inflight) return
  inflight = true
  if (!quiet) loading.value = true
  try {
    const [collectorList, pathList] = await Promise.all([listCollectors(), listConnectionPaths()])
    collectors.value = collectorList.data
    paths.value = pathList.data
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
            <span>{{ pretty(row.connected_agents, 'connected_agents') }} {{ t('connectedAgents') }}</span>
            <span>{{ collectorReplicationText(row) }}</span>
          </div>
        </article>
      </div>
      <div v-if="!collectors.length && !loading" class="pagination">
        <el-button type="primary" @click="router.push('/telemetry?create=1')"><i class="ri-add-line"></i>{{ t('createCollector') }}</el-button>
      </div>
    </section>

    <section class="surface table-card">
      <div class="toolbar">
        <h2 class="table-card-title">{{ t('nodeLinks') }} <small>{{ filteredPaths.length }}</small></h2>
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
              role="columnheader"
            >{{ obs.name }}</div>
          </div>
          <div
            v-for="row in matrixRows"
            :key="row.node_uuid"
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
                v-for="cell in matrixCells(row, obs.id)"
                :key="pathRowKey(cell.path)"
                type="button"
                class="path-matrix__cell"
                :class="cell.connected ? 'is-connected' : 'is-disconnected'"
                :title="cell.title"
                @click="openPath(cell.path)"
              >
                <span class="rtt-main">
                  <i v-if="cell.hasError" class="ri-error-warning-line" aria-hidden="true"></i>
                  <span class="rtt-value">{{ cell.text }}</span>
                </span>
                <span v-if="cell.sampled" class="rtt-sampled">{{ cell.sampled }}</span>
              </button>
              <span v-if="!row.cells[obs.id]" class="path-matrix__empty" aria-hidden="true"></span>
            </div>
          </div>
        </div>
      </div>
      <div class="mobile-only" v-loading="loading">
        <AppEmpty v-if="!filteredPaths.length && !loading" class="empty-state" icon="ri-links-line" :title="t('noPathsTitle')" :description="t('noPathsHint')" />
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
        </div>
      </div>
    </section>
  </div>

  <AppDrawer v-model="collectorDrawer" :title="activeCollector?.name || t('collector')" mode="view">
    <div class="page-stack">
    <dl v-if="activeCollector" class="mobile-card-meta">
      <div><dt>{{ t('id') }}</dt><dd><CopyableId :value="activeCollector.id" :compact="false" /></dd></div>
      <div><dt>{{ t('status') }}</dt><dd>{{ t(collectorStatus(activeCollector)) }}</dd></div>
      <div><dt>{{ t('lastSeen') }}</dt><dd>{{ pretty(activeCollector.last_seen, 'last_seen') }}</dd></div>
      <div><dt>{{ t('lastSync') }}</dt><dd>{{ pretty(activeCollector.last_sync, 'last_sync') }}</dd></div>
      <div><dt>{{ t('lastPrimarySeen') }}</dt><dd>{{ pretty(activeCollector.last_primary_seen, 'last_primary_seen') }}</dd></div>
      <div><dt>{{ t('heartbeatLatency') }}</dt><dd>{{ latencyText(activeCollector.heartbeat_rtt_ms, activeCollector.heartbeat_rtt_sampled_at) }}</dd></div>
      <div><dt>{{ t('replicationLatency') }}</dt><dd>{{ latencyText(activeCollector.replication_rtt_ms, activeCollector.replication_rtt_sampled_at) }}</dd></div>
      <div><dt>{{ t('connectedAgents') }}</dt><dd>{{ pretty(activeCollector.connected_agents, 'connected_agents') }}</dd></div>
      <div><dt>{{ t('pendingRecords') }}</dt><dd>{{ pretty(activeCollector.pending_records, 'pending_records') }}</dd></div>
      <div><dt>{{ t('oldestPending') }}</dt><dd>{{ pretty(activeCollector.oldest_pending, 'oldest_pending') }}</dd></div>
      <div><dt>{{ t('replicationCursor') }}</dt><dd>{{ pretty(activeCollector.replication_cursor, 'replication_cursor') }}</dd></div>
      <div><dt>{{ t('protocolVersion') }}</dt><dd>{{ pretty(activeCollector.protocol_version, 'protocol_version') }}</dd></div>
      <div><dt>{{ t('collectorVersion') }}</dt><dd>{{ collectorVersionText(activeCollector) }}</dd></div>
    </dl>
    <el-table v-loading="latencyLoading" :data="collectorLatency" class="dataset-table">
      <el-table-column :label="t('bucketStart')" min-width="180"><template #default="{row}">{{ pretty(row.bucket_start, 'bucket_start') }}</template></el-table-column>
      <el-table-column :label="t('type')" width="90"><template #default="{row}">{{ pretty(row.kind, 'kind') }}</template></el-table-column>
      <el-table-column :label="t('minMs')" width="90"><template #default="{row}">{{ pretty(row.min_ms, 'min_ms') }}</template></el-table-column>
      <el-table-column :label="t('avgMs')" width="90"><template #default="{row}">{{ pretty(row.avg_ms, 'avg_ms') }}</template></el-table-column>
      <el-table-column :label="t('maxMs')" width="90"><template #default="{row}">{{ pretty(row.max_ms, 'max_ms') }}</template></el-table-column>
      <template #empty><AppEmpty icon="ri-timer-line" :description="t('noData')" /></template>
    </el-table>
    <div v-if="collectorLatencyMeta.total" class="pagination">
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
  </div>
</template>
