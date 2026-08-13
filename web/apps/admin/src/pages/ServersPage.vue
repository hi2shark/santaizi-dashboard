<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { AppDrawer, AppEmpty } from '@santaizi/ui'
import type { ConnectionPath } from '@santaizi/api'
import ServerEditorDialog from '@/components/editors/ServerEditorDialog.vue'
import ServerGroupManagerDialog from '@/components/editors/ServerGroupManagerDialog.vue'
import InstallAgentDialog from '@/components/InstallAgentDialog.vue'
import { batchDeleteServers, batchUpdateServerGroup, deleteOfflineHistory, deleteServer, listConnectionPaths, listOfflineHistory, listServerAvailability, listServers, resetServerAvailability, resetServerSecret, updateServerDisplayIndex, type ResourceRecord, type ServerRecord } from '@/api/adminApi'
import {formatAdminValue} from '@/composables/format'
import { notifyAPIError } from '@/composables/notify'
import { isRowSelected, toggleRowSelection } from '@/composables/selection'
import { hostAddresses } from '@/domain/hostAddress'
import { parsePublicNote } from '@/domain/publicNote'

const { t, te, locale } = useI18n()
const route = useRoute()
const loading = ref(false), editor = ref(false), installDialog = ref(false), groupManager = ref(false)
const items = ref<ServerRecord[]>([]), selected = ref<ServerRecord[]>([]), editing = ref<ServerRecord>(), installServer = ref<ServerRecord>(), installSecret = ref('')
const total = ref(0)
const query = reactive({ page: 1, page_size: 20, q: '', sort: 'display_index', order: 'desc' as const })
const historyDrawer = ref(false), historyLoading = ref(false), historyServer = ref<ServerRecord>(), historyTab = ref('availability'), history = ref<ResourceRecord[]>([]), availability = ref<ResourceRecord[]>([]), connectionPaths = ref<ConnectionPath[]>([])
const sortDraft = ref<Record<number, string>>({})
const sortSaving = ref<Record<number, boolean>>({})

async function load() {
  loading.value = true
  try {
    const result = await listServers(query)
    items.value = result.data
    total.value = result.meta.total || result.data.length
    sortDraft.value = Object.fromEntries(result.data.map(server => [server.id, String(server.display_index)]))
  } catch (error) {
    notifyAPIError(error, t as never, te)
  } finally {
    loading.value = false
  }
}
function open(item?: ServerRecord) { editing.value = item; editor.value = true }
function publicSummary(server: ServerRecord) { const parsed = parsePublicNote(server.public_note ? JSON.stringify(server.public_note) : ''); return parsed.form.presentation.slogan || parsed.form.presentation.locationLabel || server.note || '—' }
function hasPublicSummary(server: ServerRecord) { return publicSummary(server) !== '—' }
function reportedAddresses(server: ServerRecord) { return hostAddresses(server.host) }
async function removeOne(server: ServerRecord) { await ElMessageBox.confirm(t('confirmDelete'), t('dangerousAction'), { type: 'warning' }); try { await deleteServer(server.id); ElMessage.success(t('deleteSuccess')); await load() } catch (error) { notifyAPIError(error, t as never, te) } }
async function groupSelected() { try { const { value } = await ElMessageBox.prompt(t('group'), t('batchGroup'), { inputValue: selected.value[0]?.tag || '' }); await batchUpdateServerGroup(selected.value.map(server => server.id), value); await load() } catch { /* user cancelled */ } }
async function deleteSelected() { await ElMessageBox.confirm(t('confirmDelete'), t('dangerousAction'), { type: 'warning' }); try { await batchDeleteServers(selected.value.map(server => server.id)); selected.value = []; await load(); ElMessage.success(t('deleteSuccess')) } catch (error) { notifyAPIError(error, t as never, te) } }
function status(server: ServerRecord) { return server.online ? 'online' : (server.telemetry?.connectivity || 'offline') }
function display(value: unknown, key: string) { return formatAdminValue(value, key, locale.value, t as never, te) }
function observerName(id: string) { return id === 'primary' ? t('observerKindPrimary') : id }
function seenObserverText(row: ResourceRecord) {
  const evidence = Array.isArray(row.observer_evidence) ? row.observer_evidence as Array<{ observer_id?: string; seen?: boolean }> : []
  const names = evidence.filter(item => item.seen && item.observer_id).map(item => observerName(String(item.observer_id)))
  return names.length ? names.join(', ') : '—'
}
function pathRowKey(row: ConnectionPath) { return `${row.node_uuid}:${row.observer_id}` }
function pathObserverLabel(path: ConnectionPath) {
  if (path.observer_kind === 'primary') return t('observerKindPrimary')
  return path.observer_name || path.observer_id
}
function onSelect(row: ServerRecord, checked: boolean | string | number) { selected.value = toggleRowSelection(selected.value, row, !!checked) }
function showInstall(server: ServerRecord, secret = '') { installServer.value = server; installSecret.value = secret; installDialog.value = true }
async function resetSecret(server: ServerRecord) { await ElMessageBox.confirm(t('confirmResetSecret'), t('confirm'), { type: 'warning' }); try { const result = await resetServerSecret(server.id); showInstall(server, result.secret) } catch (error) { notifyAPIError(error, t as never, te) } }
async function resetAvailabilityHistory(server: ServerRecord) { await ElMessageBox.confirm(t('confirmResetAvailability'), t('confirm'), { type: 'warning' }); try { await resetServerAvailability(server.id); await load(); ElMessage.success(t('saveSuccess')) } catch (error) { notifyAPIError(error, t as never, te) } }
async function saved(server: ServerRecord, created: boolean) { await load(); if (created) showInstall(server, server.secret || '') }
async function showHistory(server: ServerRecord) {
  historyServer.value = server
  historyDrawer.value = true
  historyTab.value = 'availability'
  historyLoading.value = true
  try {
    const [offline, buckets, paths] = await Promise.all([
      listOfflineHistory(server.id),
      listServerAvailability(server.id, { limit: 200 }),
      listConnectionPaths({ server_id: server.id }),
    ])
    history.value = offline.data
    availability.value = buckets.data
    connectionPaths.value = paths.data
  } catch (error) {
    notifyAPIError(error, t as never, te)
  } finally {
    historyLoading.value = false
  }
}
async function removeHistory(row: ResourceRecord) { await ElMessageBox.confirm(t('confirmDelete'), t('dangerousAction'), { type: 'warning' }); await deleteOfflineHistory(Number(row.id)); await showHistory(historyServer.value!) }

async function commitDisplayIndex(server: ServerRecord) {
  if (sortSaving.value[server.id]) return
  const next = Number(String(sortDraft.value[server.id] ?? '').trim())
  if (!Number.isFinite(next)) {
    sortDraft.value = { ...sortDraft.value, [server.id]: String(server.display_index) }
    return
  }
  if (next === server.display_index) {
    sortDraft.value = { ...sortDraft.value, [server.id]: String(server.display_index) }
    return
  }
  sortSaving.value = { ...sortSaving.value, [server.id]: true }
  try {
    await updateServerDisplayIndex(server.id, next)
    ElMessage.success(t('saveSuccess'))
    await load()
  } catch (error) {
    sortDraft.value = { ...sortDraft.value, [server.id]: String(server.display_index) }
    notifyAPIError(error, t as never, te)
  } finally {
    sortSaving.value = { ...sortSaving.value, [server.id]: false }
  }
}

onMounted(async () => { await load(); if (route.query.create === '1') open() })
</script>

<template>
  <div class="page-head"><h1>{{ t('servers') }}</h1><el-button type="primary" @click="open()"><i class="ri-add-line"></i>{{ t('createServer') }}</el-button></div>
  <section class="surface table-card">
    <div class="toolbar">
      <el-input v-model="query.q" class="search-input" clearable :placeholder="t('search')" @keyup.enter="query.page=1;load()"><template #prefix><i class="ri-search-line"></i></template></el-input>
      <el-button @click="query.page=1;load()"><i class="ri-filter-3-line"></i>{{ t('filter') }}</el-button>
      <el-button @click="groupManager=true"><i class="ri-folder-settings-line"></i>{{ t('manageGroups') }}</el-button>
      <template v-if="selected.length">
        <el-button @click="groupSelected"><i class="ri-folder-transfer-line"></i>{{ t('batchGroup') }}</el-button>
        <el-button type="danger" plain @click="deleteSelected"><i class="ri-delete-bin-6-line"></i>{{ t('batchDelete') }}</el-button>
      </template>
      <span class="toolbar-spacer"></span>
      <el-button @click="load"><i class="ri-refresh-line"></i>{{ t('refresh') }}</el-button>
    </div>
    <el-table class="desktop-only" v-loading="loading" :data="items" row-key="id" @selection-change="selected=$event">
      <el-table-column type="selection" width="46"/>
      <el-table-column class-name="col-status" label-class-name="col-status" width="44" align="center">
        <template #default="{row}">
          <span class="status-dot" :class="status(row)"></span>
        </template>
      </el-table-column>
      <el-table-column prop="name" :label="t('name')" min-width="200">
        <template #default="{row}">
          <div class="server-name">
            <strong>{{ row.name }}</strong><small>{{ publicSummary(row) }}</small>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="tag" :label="t('group')" width="140">
        <template #default="{row}"><el-tag effect="plain">{{ row.tag || 'default' }}</el-tag></template>
      </el-table-column>
      <el-table-column :label="t('displayIndex')" width="140">
        <template #default="{row}">
          <el-input
            v-model="sortDraft[row.id]"
            class="sort-input"
            inputmode="numeric"
            :disabled="!!sortSaving[row.id]"
            @keyup.enter="commitDisplayIndex(row)"
            @blur="commitDisplayIndex(row)"
          />
        </template>
      </el-table-column>
      <el-table-column :label="t('host')" min-width="170">
        <template #default="{row}">{{ row.host?.Platform || row.host?.platform || '—' }}</template>
      </el-table-column>
      <el-table-column :label="`${t('ipv4')} / ${t('ipv6')}`" min-width="200">
        <template #default="{row}">
          <div class="server-ip">
            <span>{{ reportedAddresses(row).ipv4 || '—' }}</span>
            <span>{{ reportedAddresses(row).ipv6 || '—' }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column :label="t('availability')" width="140">
        <template #default="{row}"><span class="state-label"><i class="ri-checkbox-circle-fill"></i>{{ t(row.telemetry?.coverage || status(row)) }}</span></template>
      </el-table-column>
      <el-table-column prop="last_active" :label="t('lastSeen')" width="190">
        <template #default="{row}">{{ display(row.last_active,'last_active') }}</template>
      </el-table-column>
      <el-table-column :label="t('actions')" width="72" fixed="right">
        <template #default="{row}">
          <el-dropdown trigger="click">
            <el-button text class="actions-more" :aria-label="t('actions')"><i class="ri-more-fill"></i></el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="open(row)"><i class="ri-edit-line"></i>{{ t('edit') }}</el-dropdown-item>
                <el-dropdown-item @click="showInstall(row)"><i class="ri-download-cloud-2-line"></i>{{ t('installAgent') }}</el-dropdown-item>
                <el-dropdown-item @click="showHistory(row)"><i class="ri-history-line"></i>{{ t('offlineHistory') }}</el-dropdown-item>
                <el-dropdown-item @click="resetSecret(row)"><i class="ri-key-2-line"></i>{{ t('resetSecret') }}</el-dropdown-item>
                <el-dropdown-item @click="resetAvailabilityHistory(row)"><i class="ri-restart-line"></i>{{ t('resetAvailability') }}</el-dropdown-item>
                <el-dropdown-item divided @click="removeOne(row)"><i class="ri-delete-bin-6-line"></i>{{ t('delete') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-table-column>
      <template #empty><AppEmpty icon="ri-server-line" :description="t('noData')"/></template>
    </el-table>
    <div class="mobile-only" v-loading="loading">
      <AppEmpty v-if="!items.length && !loading" icon="ri-server-line" :description="t('noData')"/>
      <div v-else class="mobile-card-list">
        <article v-for="row in items" :key="row.id" class="mobile-card mobile-card--server">
          <div class="mobile-card-head">
            <el-checkbox :model-value="isRowSelected(selected, row)" @change="onSelect(row, $event)" />
            <span class="mobile-card-status"><span class="status-dot" :class="status(row)"></span></span>
            <div class="mobile-card-title">
              <strong>{{ row.name }}</strong>
              <small v-if="hasPublicSummary(row)">{{ publicSummary(row) }}</small>
            </div>
            <div class="mobile-card-actions">
              <el-dropdown trigger="click">
                <el-button text class="actions-more" :aria-label="t('actions')"><i class="ri-more-fill"></i></el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item @click="open(row)"><i class="ri-edit-line"></i>{{ t('edit') }}</el-dropdown-item>
                    <el-dropdown-item @click="showInstall(row)"><i class="ri-download-cloud-2-line"></i>{{ t('installAgent') }}</el-dropdown-item>
                    <el-dropdown-item @click="showHistory(row)"><i class="ri-history-line"></i>{{ t('offlineHistory') }}</el-dropdown-item>
                    <el-dropdown-item @click="resetSecret(row)"><i class="ri-key-2-line"></i>{{ t('resetSecret') }}</el-dropdown-item>
                    <el-dropdown-item @click="resetAvailabilityHistory(row)"><i class="ri-restart-line"></i>{{ t('resetAvailability') }}</el-dropdown-item>
                    <el-dropdown-item divided @click="removeOne(row)"><i class="ri-delete-bin-6-line"></i>{{ t('delete') }}</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </div>
          <div class="mobile-card-chips">
            <el-tag effect="plain">{{ row.tag || 'default' }}</el-tag>
            <span class="state-label"><i class="ri-checkbox-circle-fill"></i>{{ t(row.telemetry?.coverage || status(row)) }}</span>
          </div>
          <dl class="mobile-card-meta mobile-card-meta--stats">
            <div><dt>{{ t('host') }}</dt><dd>{{ row.host?.Platform || row.host?.platform || '—' }}</dd></div>
            <div><dt>{{ t('lastSeen') }}</dt><dd>{{ display(row.last_active,'last_active') }}</dd></div>
            <div><dt>{{ t('ipv4') }}</dt><dd>{{ reportedAddresses(row).ipv4 || '—' }}</dd></div>
            <div><dt>{{ t('ipv6') }}</dt><dd>{{ reportedAddresses(row).ipv6 || '—' }}</dd></div>
          </dl>
          <dl class="mobile-card-meta mobile-card-meta--sort">
            <div>
              <dt>{{ t('displayIndex') }}</dt>
              <dd>
                <el-input
                  v-model="sortDraft[row.id]"
                  class="sort-input"
                  inputmode="numeric"
                  :disabled="!!sortSaving[row.id]"
                  @keyup.enter="commitDisplayIndex(row)"
                  @blur="commitDisplayIndex(row)"
                />
              </dd>
            </div>
          </dl>
        </article>
      </div>
    </div>
    <div class="pagination"><el-pagination v-model:current-page="query.page" v-model:page-size="query.page_size" layout="total, sizes, prev, pager, next" :total="total" @change="load"/></div>
  </section>
  <ServerEditorDialog v-model="editor" :value="editing" @saved="saved"/>
  <ServerGroupManagerDialog v-model="groupManager" @changed="load"/>
  <InstallAgentDialog v-model="installDialog" :server="installServer" :secret="installSecret"/>
  <AppDrawer v-model="historyDrawer" :title="`${t('availabilityHistory')} · ${historyServer?.name || ''}`" mode="view" size="min(980px,96vw)">
    <el-tabs v-model="historyTab">
      <el-tab-pane :label="t('availabilityHistory')" name="availability">
        <el-table v-loading="historyLoading" :data="availability">
          <el-table-column prop="bucket_start" :label="t('bucketStart')" min-width="190"><template #default="{row}">{{display(row.bucket_start,'bucket_start')}}</template></el-table-column>
          <el-table-column prop="host" :label="t('host')" width="120"><template #default="{row}">{{t(String(row.host||'unknown'))}}</template></el-table-column>
          <el-table-column prop="connectivity" :label="t('connectivity')" width="140"><template #default="{row}">{{t(String(row.connectivity||'unknown'))}}</template></el-table-column>
          <el-table-column prop="expected_observers" :label="t('expectedObservers')" width="130"/>
          <el-table-column prop="healthy_observers" :label="t('healthyObservers')" width="130"/>
          <el-table-column prop="seen_observers" :label="t('seenObservers')" width="120"/>
          <el-table-column :label="t('observerEvidence')" min-width="180"><template #default="{row}">{{ seenObserverText(row) }}</template></el-table-column>
          <el-table-column prop="revision" :label="t('revision')" width="90"/>
        </el-table>
      </el-tab-pane>
      <el-tab-pane :label="t('nodeLinks')" name="connections">
        <el-table v-loading="historyLoading" :data="connectionPaths" :row-key="pathRowKey">
          <el-table-column class-name="col-status" label-class-name="col-status" width="44" align="center">
            <template #default="{row}"><span class="status-dot" :class="row.sink.connected ? 'online' : 'offline'"></span></template>
          </el-table-column>
          <el-table-column :label="t('observer')" min-width="160"><template #default="{row}">{{ pathObserverLabel(row) }}</template></el-table-column>
          <el-table-column :label="t('linkStatus')" width="110"><template #default="{row}">{{ t(row.sink.connected ? 'connected' : 'disconnected') }}</template></el-table-column>
          <el-table-column :label="t('lastObservation')" min-width="180"><template #default="{row}">{{ display(row.last_seen, 'last_seen') }}</template></el-table-column>
          <el-table-column :label="t('pendingEvents')" width="120"><template #default="{row}">{{ display(row.sink.pending_events, 'pending_events') }}</template></el-table-column>
          <template #empty><AppEmpty icon="ri-links-line" :title="t('noPathsTitle')" :description="t('noPathsHint')"/></template>
        </el-table>
      </el-tab-pane>
      <el-tab-pane :label="t('offlineHistory')" name="offline">
        <el-table v-loading="historyLoading" :data="history">
          <el-table-column prop="started_at" :label="t('startedAt')" min-width="190"><template #default="{row}">{{display(row.started_at,'started_at')}}</template></el-table-column>
          <el-table-column prop="ended_at" :label="t('endedAt')" min-width="190"><template #default="{row}">{{display(row.ended_at,'ended_at')}}</template></el-table-column>
          <el-table-column prop="duration" :label="t('duration')" width="120"/>
          <el-table-column :label="t('actions')" width="72"><template #default="{row}"><el-button text class="actions-icon" type="danger" :aria-label="t('delete')" @click="removeHistory(row)"><i class="ri-delete-bin-line"></i></el-button></template></el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>
  </AppDrawer>
</template>

<style scoped>
.sort-input { width: 112px; }
.sort-input :deep(.el-input__wrapper) { padding-left: 8px; padding-right: 8px; }
.server-ip { min-width: 0; font-size: 12px; line-height: 1.35; }
.server-ip span { display: block; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
</style>
