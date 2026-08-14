<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { AppDrawer, AppEmpty } from '@santaizi/ui'
import type { ConnectionLatencyBucket, ConnectionPath } from '@santaizi/api'
import { listCollectors, listConnectionLatency, listConnectionPaths, type CollectorRecord } from '@/api/adminApi'
import CopyableId from '@/components/CopyableId.vue'
import { formatAdminValue } from '@/composables/format'
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
let timer: ReturnType<typeof setInterval> | undefined

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

function collectorStatus(row: CollectorRecord) {
  return row.revoked ? 'offline' : (row.status || 'unknown')
}

function pathRowKey(row: ConnectionPath) {
  return `${row.node_uuid}:${row.observer_id}`
}

function observerLabel(path: ConnectionPath) {
  if (path.observer_kind === 'primary') return t('observerKindPrimary')
  return path.observer_name || path.observer_id
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
  if (!quiet) loading.value = true
  try {
    const [collectorList, pathList] = await Promise.all([listCollectors(), listConnectionPaths()])
    collectors.value = collectorList.data
    paths.value = pathList.data
  } catch (error) {
    notifyAPIError(error, t as never, te)
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  const observerId = String(route.query.observer_id || '')
  if (observerId) observerFilter.value = observerId
  await load()
  timer = setInterval(() => { void load(true) }, 15000)
})
onUnmounted(() => { if (timer) clearInterval(timer) })
</script>

<template>
  <div class="page-head">
    <h1>{{ t('connections') }}</h1>
    <el-button @click="load()"><i class="ri-refresh-line"></i>{{ t('refresh') }}</el-button>
  </div>

  <div class="page-stack">
    <section class="surface table-card">
      <div class="toolbar">
        <h2 class="table-card-title">{{ t('collectorLinks') }} <small>{{ collectors.length }}</small></h2>
      </div>
      <el-table class="desktop-only dataset-table" v-loading="loading" table-layout="fixed" :data="collectors" row-key="id" @row-click="openCollector">
        <el-table-column class-name="col-status" label-class-name="col-status" width="44" align="center">
          <template #default="{row}"><span class="status-dot" :class="collectorStatus(row)"></span></template>
        </el-table-column>
        <el-table-column prop="name" :label="t('name')" min-width="180">
          <template #default="{row}">
            <div class="server-name">
              <strong>{{ row.name }}</strong>
              <CopyableId :value="row.id" />
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('lastSeen')" min-width="180">
          <template #default="{row}">{{ pretty(row.last_seen, 'last_seen') }}</template>
        </el-table-column>
        <el-table-column :label="t('lastSync')" min-width="180">
          <template #default="{row}">{{ pretty(row.last_sync, 'last_sync') }}</template>
        </el-table-column>
        <el-table-column :label="t('heartbeatLatency')" width="120">
          <template #default="{row}">{{ latencyText(row.heartbeat_rtt_ms, row.heartbeat_rtt_sampled_at) }}</template>
        </el-table-column>
        <el-table-column :label="t('connectedAgents')" width="120">
          <template #default="{row}">{{ pretty(row.connected_agents, 'connected_agents') }}</template>
        </el-table-column>
        <el-table-column :label="t('pendingRecords')" width="120">
          <template #default="{row}">{{ pretty(row.pending_records, 'pending_records') }}</template>
        </el-table-column>
        <template #empty>
          <AppEmpty class="empty-state" icon="ri-radar-line" :title="t('noCollectorsTitle')" :description="t('noCollectorsHint')" />
        </template>
      </el-table>
      <div class="mobile-only" v-loading="loading">
        <AppEmpty v-if="!collectors.length && !loading" class="empty-state" icon="ri-radar-line" :title="t('noCollectorsTitle')" :description="t('noCollectorsHint')" />
        <div v-else class="mobile-card-list">
          <article v-for="row in collectors" :key="row.id" class="mobile-card" @click="openCollector(row)">
            <div class="mobile-card-head">
              <span class="mobile-card-status"><span class="status-dot" :class="collectorStatus(row)"></span></span>
              <div class="mobile-card-title">
                <strong>{{ row.name }}</strong>
                <CopyableId :value="row.id" />
              </div>
            </div>
            <dl class="mobile-card-meta">
              <div><dt>{{ t('lastSeen') }}</dt><dd>{{ pretty(row.last_seen, 'last_seen') }}</dd></div>
              <div><dt>{{ t('lastSync') }}</dt><dd>{{ pretty(row.last_sync, 'last_sync') }}</dd></div>
              <div><dt>{{ t('heartbeatLatency') }}</dt><dd>{{ latencyText(row.heartbeat_rtt_ms, row.heartbeat_rtt_sampled_at) }}</dd></div>
              <div><dt>{{ t('connectedAgents') }}</dt><dd>{{ pretty(row.connected_agents, 'connected_agents') }}</dd></div>
              <div><dt>{{ t('pendingRecords') }}</dt><dd>{{ pretty(row.pending_records, 'pending_records') }}</dd></div>
            </dl>
          </article>
        </div>
      </div>
      <div v-if="!collectors.length && !loading" class="pagination">
        <el-button type="primary" @click="router.push('/telemetry?create=1')"><i class="ri-add-line"></i>{{ t('createCollector') }}</el-button>
      </div>
    </section>

    <section class="surface table-card">
      <div class="toolbar">
        <h2 class="table-card-title">{{ t('nodeLinks') }} <small>{{ filteredPaths.length }}</small></h2>
        <span class="toolbar-spacer"></span>
        <el-select v-model="observerFilter" class="toolbar-filter" clearable :placeholder="t('allObservers')">
          <el-option v-for="item in observerOptions" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <el-select v-model="linkFilter" class="toolbar-filter" clearable :placeholder="t('allLinkStatus')">
          <el-option :label="t('connected')" value="connected" />
          <el-option :label="t('disconnected')" value="disconnected" />
        </el-select>
      </div>
      <el-table class="desktop-only dataset-table" v-loading="loading" table-layout="fixed" :data="filteredPaths" :row-key="pathRowKey" @row-click="openPath">
        <el-table-column class-name="col-status" label-class-name="col-status" width="44" align="center">
          <template #default="{row}"><span class="status-dot" :class="row.sink.connected ? 'online' : 'offline'"></span></template>
        </el-table-column>
        <el-table-column :label="t('server')" min-width="160">
          <template #default="{row}">{{ row.server_name || '—' }}</template>
        </el-table-column>
        <el-table-column :label="t('observer')" min-width="160">
          <template #default="{row}">{{ observerLabel(row) }}</template>
        </el-table-column>
        <el-table-column :label="t('linkStatus')" width="110">
          <template #default="{row}">{{ t(row.sink.connected ? 'connected' : 'disconnected') }}</template>
        </el-table-column>
        <el-table-column :label="t('lastObservation')" min-width="180">
          <template #default="{row}">{{ pretty(row.last_seen, 'last_seen') }}</template>
        </el-table-column>
        <el-table-column :label="t('latency')" width="110">
          <template #default="{row}">{{ latencyText(row.sink.last_rtt_ms, row.sink.rtt_sampled_at) }}</template>
        </el-table-column>
        <el-table-column :label="t('pendingEvents')" width="120">
          <template #default="{row}">{{ pretty(row.sink.pending_events, 'pending_events') }}</template>
        </el-table-column>
        <el-table-column :label="t('lastError')" min-width="200">
          <template #default="{row}">
            <span class="cell-ellipsis" :title="lastErrorText(row.sink.last_error)">{{ lastErrorText(row.sink.last_error) }}</span>
          </template>
        </el-table-column>
        <template #empty>
          <AppEmpty class="empty-state" icon="ri-links-line" :title="t('noPathsTitle')" :description="t('noPathsHint')" />
        </template>
      </el-table>
      <div class="mobile-only" v-loading="loading">
        <AppEmpty v-if="!filteredPaths.length && !loading" class="empty-state" icon="ri-links-line" :title="t('noPathsTitle')" :description="t('noPathsHint')" />
        <div v-else class="mobile-card-list">
          <article v-for="row in filteredPaths" :key="`${row.node_uuid}-${row.observer_id}`" class="mobile-card" @click="openPath(row)">
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
</template>
