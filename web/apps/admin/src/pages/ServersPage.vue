<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { AppDrawer, AppEmpty } from '@santaizi/ui'
import ServerEditorDialog from '@/components/editors/ServerEditorDialog.vue'
import ServerGroupManagerDialog from '@/components/editors/ServerGroupManagerDialog.vue'
import InstallAgentDialog from '@/components/InstallAgentDialog.vue'
import { batchDeleteServers, batchUpdateServerGroup, deleteOfflineHistory, deleteServer, listOfflineHistory, listServerAvailability, listServers, resetServerAvailability, resetServerSecret, updateServerDisplayIndex, type ResourceRecord, type ServerRecord } from '@/api/adminApi'
import {formatAdminValue} from '@/composables/format'
import { notifyAPIError } from '@/composables/notify'
import { parsePublicNote } from '@/domain/publicNote'

const { t, te, locale } = useI18n()
const route = useRoute()
const loading = ref(false), editor = ref(false), installDialog = ref(false), groupManager = ref(false)
const items = ref<ServerRecord[]>([]), selected = ref<ServerRecord[]>([]), editing = ref<ServerRecord>(), installServer = ref<ServerRecord>(), installSecret = ref('')
const total = ref(0)
const query = reactive({ page: 1, page_size: 20, q: '', sort: 'display_index', order: 'desc' as const })
const historyDrawer = ref(false), historyLoading = ref(false), historyServer = ref<ServerRecord>(), historyTab = ref('availability'), history = ref<ResourceRecord[]>([]), availability = ref<ResourceRecord[]>([])
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
async function removeOne(server: ServerRecord) { await ElMessageBox.confirm(t('confirmDelete'), t('dangerousAction'), { type: 'warning' }); try { await deleteServer(server.id); ElMessage.success(t('deleteSuccess')); await load() } catch (error) { notifyAPIError(error, t as never, te) } }
async function groupSelected() { try { const { value } = await ElMessageBox.prompt(t('group'), t('batchGroup'), { inputValue: selected.value[0]?.tag || '' }); await batchUpdateServerGroup(selected.value.map(server => server.id), value); await load() } catch { /* user cancelled */ } }
async function deleteSelected() { await ElMessageBox.confirm(t('confirmDelete'), t('dangerousAction'), { type: 'warning' }); try { await batchDeleteServers(selected.value.map(server => server.id)); selected.value = []; await load(); ElMessage.success(t('deleteSuccess')) } catch (error) { notifyAPIError(error, t as never, te) } }
function status(server: ServerRecord) { return server.online ? 'online' : (server.telemetry?.connectivity || 'offline') }
function display(value: unknown, key: string) { return formatAdminValue(value, key, locale.value, t as never, te) }
function showInstall(server: ServerRecord, secret = '') { installServer.value = server; installSecret.value = secret; installDialog.value = true }
async function resetSecret(server: ServerRecord) { await ElMessageBox.confirm(t('confirmResetSecret'), t('confirm'), { type: 'warning' }); try { const result = await resetServerSecret(server.id); showInstall(server, result.secret) } catch (error) { notifyAPIError(error, t as never, te) } }
async function resetAvailabilityHistory(server: ServerRecord) { await ElMessageBox.confirm(t('confirmResetAvailability'), t('confirm'), { type: 'warning' }); try { await resetServerAvailability(server.id); await load(); ElMessage.success(t('saveSuccess')) } catch (error) { notifyAPIError(error, t as never, te) } }
async function saved(server: ServerRecord, created: boolean) { await load(); if (created) showInstall(server, server.secret || '') }
async function showHistory(server: ServerRecord) { historyServer.value = server; historyDrawer.value = true; historyLoading.value = true; try { const [offline, buckets] = await Promise.all([listOfflineHistory(server.id), listServerAvailability(server.id, { limit: 200 })]); history.value = offline.data; availability.value = buckets.data } catch (error) { notifyAPIError(error, t as never, te) } finally { historyLoading.value = false } }
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
    <el-table v-loading="loading" :data="items" row-key="id" @selection-change="selected=$event">
      <el-table-column type="selection" width="46"/>
      <el-table-column prop="name" :label="t('name')" min-width="200">
        <template #default="{row}">
          <div class="server-name">
            <span class="status-dot" :class="status(row)"></span>
            <div><strong>{{ row.name }}</strong><small>{{ publicSummary(row) }}</small></div>
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
      <el-table-column :label="t('availability')" width="140">
        <template #default="{row}"><span class="state-label"><i class="ri-checkbox-circle-fill"></i>{{ t(row.telemetry?.coverage || status(row)) }}</span></template>
      </el-table-column>
      <el-table-column prop="last_active" :label="t('lastSeen')" width="190">
        <template #default="{row}">{{ display(row.last_active,'last_active') }}</template>
      </el-table-column>
      <el-table-column :label="t('actions')" width="130" fixed="right">
        <template #default="{row}">
          <el-dropdown trigger="click">
            <el-button><i class="ri-more-2-fill"></i>{{ t('actions') }}</el-button>
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
          <el-table-column prop="revision" :label="t('revision')" width="90"/>
        </el-table>
      </el-tab-pane>
      <el-tab-pane :label="t('offlineHistory')" name="offline">
        <el-table v-loading="historyLoading" :data="history">
          <el-table-column prop="started_at" :label="t('startedAt')" min-width="190"><template #default="{row}">{{display(row.started_at,'started_at')}}</template></el-table-column>
          <el-table-column prop="ended_at" :label="t('endedAt')" min-width="190"><template #default="{row}">{{display(row.ended_at,'ended_at')}}</template></el-table-column>
          <el-table-column prop="duration" :label="t('duration')" width="120"/>
          <el-table-column :label="t('actions')" width="90"><template #default="{row}"><el-button circle type="danger" plain :aria-label="t('delete')" @click="removeHistory(row)"><i class="ri-delete-bin-line"></i></el-button></template></el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>
  </AppDrawer>
</template>

<style scoped>
.sort-input { width: 112px; }
.sort-input :deep(.el-input__wrapper) { padding-left: 8px; padding-right: 8px; }
</style>
