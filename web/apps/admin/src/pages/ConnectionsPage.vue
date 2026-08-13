<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { AppDrawer, AppEmpty } from '@santaizi/ui'
import type { ConnectionPath } from '@santaizi/api'
import { listCollectors, listConnectionPaths, type CollectorRecord } from '@/api/adminApi'
import { formatAdminValue } from '@/composables/format'
import { notifyAPIError } from '@/composables/notify'

const { t, te, locale } = useI18n()
const router = useRouter()
const loading = ref(false)
const collectors = ref<CollectorRecord[]>([])
const paths = ref<ConnectionPath[]>([])
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

function openCollector(row: CollectorRecord) {
  activeCollector.value = row
  collectorDrawer.value = true
}

function openPath(row: ConnectionPath) {
  activePath.value = row
  pathDrawer.value = true
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

  <section class="surface table-card">
    <div class="toolbar">
      <strong>{{ t('collectorLinks') }}</strong>
    </div>
    <el-table class="desktop-only" v-loading="loading" :data="collectors" row-key="id" @row-click="openCollector">
      <el-table-column class-name="col-status" label-class-name="col-status" width="44" align="center">
        <template #default="{row}"><span class="status-dot" :class="collectorStatus(row)"></span></template>
      </el-table-column>
      <el-table-column prop="name" :label="t('name')" min-width="180">
        <template #default="{row}">
          <div class="server-name"><strong>{{ row.name }}</strong><small class="mono">{{ row.id }}</small></div>
        </template>
      </el-table-column>
      <el-table-column :label="t('lastSeen')" min-width="180">
        <template #default="{row}">{{ pretty(row.last_seen, 'last_seen') }}</template>
      </el-table-column>
      <el-table-column :label="t('lastSync')" min-width="180">
        <template #default="{row}">{{ pretty(row.last_sync, 'last_sync') }}</template>
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
            <div class="mobile-card-title"><strong>{{ row.name }}</strong><small class="mono">{{ row.id }}</small></div>
          </div>
          <dl class="mobile-card-meta">
            <div><dt>{{ t('lastSeen') }}</dt><dd>{{ pretty(row.last_seen, 'last_seen') }}</dd></div>
            <div><dt>{{ t('lastSync') }}</dt><dd>{{ pretty(row.last_sync, 'last_sync') }}</dd></div>
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
      <strong>{{ t('nodeLinks') }}</strong>
      <el-select v-model="observerFilter" clearable :placeholder="t('allObservers')">
        <el-option v-for="item in observerOptions" :key="item.id" :label="item.name" :value="item.id" />
      </el-select>
      <el-select v-model="linkFilter" clearable :placeholder="t('allLinkStatus')">
        <el-option :label="t('connected')" value="connected" />
        <el-option :label="t('disconnected')" value="disconnected" />
      </el-select>
    </div>
    <el-table class="desktop-only" v-loading="loading" :data="filteredPaths" :row-key="pathRowKey" @row-click="openPath">
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
      <el-table-column :label="t('pendingEvents')" width="120">
        <template #default="{row}">{{ pretty(row.sink.pending_events, 'pending_events') }}</template>
      </el-table-column>
      <el-table-column :label="t('lastError')" min-width="160">
        <template #default="{row}">{{ pretty(row.sink.last_error, 'last_error') }}</template>
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
            <div><dt>{{ t('pendingEvents') }}</dt><dd>{{ pretty(row.sink.pending_events, 'pending_events') }}</dd></div>
            <div><dt>{{ t('lastError') }}</dt><dd>{{ pretty(row.sink.last_error, 'last_error') }}</dd></div>
          </dl>
        </article>
      </div>
    </div>
  </section>

  <AppDrawer v-model="collectorDrawer" :title="activeCollector?.name || t('collector')" mode="view">
    <dl v-if="activeCollector" class="mobile-card-meta">
      <div><dt>{{ t('status') }}</dt><dd>{{ t(collectorStatus(activeCollector)) }}</dd></div>
      <div><dt>{{ t('lastSeen') }}</dt><dd>{{ pretty(activeCollector.last_seen, 'last_seen') }}</dd></div>
      <div><dt>{{ t('lastSync') }}</dt><dd>{{ pretty(activeCollector.last_sync, 'last_sync') }}</dd></div>
      <div><dt>{{ t('lastPrimarySeen') }}</dt><dd>{{ pretty(activeCollector.last_primary_seen, 'last_primary_seen') }}</dd></div>
      <div><dt>{{ t('connectedAgents') }}</dt><dd>{{ pretty(activeCollector.connected_agents, 'connected_agents') }}</dd></div>
      <div><dt>{{ t('pendingRecords') }}</dt><dd>{{ pretty(activeCollector.pending_records, 'pending_records') }}</dd></div>
      <div><dt>{{ t('oldestPending') }}</dt><dd>{{ pretty(activeCollector.oldest_pending, 'oldest_pending') }}</dd></div>
      <div><dt>{{ t('replicationCursor') }}</dt><dd>{{ pretty(activeCollector.replication_cursor, 'replication_cursor') }}</dd></div>
      <div><dt>{{ t('protocolVersion') }}</dt><dd>{{ pretty(activeCollector.protocol_version, 'protocol_version') }}</dd></div>
    </dl>
  </AppDrawer>

  <AppDrawer v-model="pathDrawer" :title="activePath ? (activePath.server_name || t('server')) : t('nodeLinks')" mode="view">
    <dl v-if="activePath" class="mobile-card-meta">
      <div><dt>{{ t('observer') }}</dt><dd>{{ observerLabel(activePath) }}</dd></div>
      <div><dt>{{ t('linkStatus') }}</dt><dd>{{ t(activePath.sink.connected ? 'connected' : 'disconnected') }}</dd></div>
      <div><dt>{{ t('lastObservation') }}</dt><dd>{{ pretty(activePath.last_seen, 'last_seen') }}</dd></div>
      <div><dt>{{ t('pendingEvents') }}</dt><dd>{{ pretty(activePath.sink.pending_events, 'pending_events') }}</dd></div>
      <div><dt>{{ t('ackThrough') }}</dt><dd>{{ pretty(activePath.sink.ack_through, 'ack_through') }}</dd></div>
      <div><dt>{{ t('lastError') }}</dt><dd>{{ pretty(activePath.sink.last_error, 'last_error') }}</dd></div>
      <div><dt>{{ t('nodeUUID') }}</dt><dd class="mono">{{ activePath.node_uuid }}</dd></div>
    </dl>
  </AppDrawer>
</template>
