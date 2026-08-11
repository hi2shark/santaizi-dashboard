<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteCollector, getCollectorToken, listCollectors, revokeCollector, rotateCollectorToken, telemetryList, type CollectorRecord, type ResourceRecord } from '@/api/adminApi'
import { AppDialog, AppEmpty } from '@santaizi/ui'
import {formatAdminValue} from '@/composables/format'
import { notifyAPIError } from '@/composables/notify'
import CollectorEditorDialog from '@/components/editors/CollectorEditorDialog.vue'

const { t, te, locale } = useI18n()
const route = useRoute()
const active = ref('collectors'), loading = ref(false), editor = ref(false)
const actionBusy = ref('')
const collectors = ref<CollectorRecord[]>([]), records = ref<ResourceRecord[]>([])
const editing = ref<CollectorRecord>()
const token = ref(''), tokenDialog = ref(false)
const datasets: Record<string, string> = { assignments: 'assignments', agents: 'agents', incidents: 'incidents', revisions: 'incident-revisions', loss: 'data-loss', alerts: 'alerts' }
const fieldLabels: Record<string,string> = { observer_id:'observer',node_uuid:'nodeUUID',session_id:'sessionID',valid_from:'validFrom',valid_to:'validTo',config_version:'configVersion',wal_pressure:'walPressure',wal_bytes:'walBytes',pending_records:'pendingRecords',pending_events:'pendingEvents',protocol_version:'protocolVersion',initial_classification:'initialClassification',current_classification:'currentClassification',revision:'revision',started_at:'startedAt',occurred_at:'occurredAt',created_at:'createdAt',updated_at:'modifiedAt',reason:'reason',lost_records:'lostRecords',severity:'severity',message:'message',active:'active',notified:'notified' }
async function load() { loading.value=true; try { if (active.value==='collectors') collectors.value=(await listCollectors()).data; else records.value=(await telemetryList(datasets[active.value] || active.value)).data } catch(e){ notifyAPIError(e,t as never,te) } finally { loading.value=false } }
function open(item?: CollectorRecord) { editing.value = item; editor.value = true }
async function editorSaved(registrationToken: string) { if (registrationToken) { token.value = registrationToken; tokenDialog.value = true }; await load() }
async function act(item:CollectorRecord,action:string){
  if(action!=='view-token')await ElMessageBox.confirm(t('confirmAction'),t('confirm'),{type:action==='delete'||action==='revoke'?'warning':'info'})
  actionBusy.value=item.id
  try{
    if(action==='delete')await deleteCollector(item.id)
    else if(action==='revoke')await revokeCollector(item.id)
    else {
      const result=action==='rotate-token'?await rotateCollectorToken(item.id):await getCollectorToken(item.id)
      token.value=result.registration_token;tokenDialog.value=true
    }
    await load()
  }catch(e){notifyAPIError(e,t as never,te)}finally{actionBusy.value=''}
}
async function copyToken(){await navigator.clipboard.writeText(token.value);ElMessage.success(t('copied'))}
function fieldLabel(key:string){return t(fieldLabels[key]||key)}
function pretty(value:unknown,key=''){return formatAdminValue(value,key,locale.value,t as never,te)}
onMounted(async()=>{await load();if(route.query.create==='1')open()})
</script>
<template>
  <div class="page-head"><div><h1>{{ t('telemetry') }}</h1></div><el-button v-if="active==='collectors'" type="primary" @click="open()"><i class="ri-add-line"></i>{{ t('createCollector') }}</el-button></div>
  <section class="surface telemetry-shell"><el-tabs v-model="active" @tab-change="load"><el-tab-pane :label="t('collectors')" name="collectors"/><el-tab-pane :label="t('observerAssignments')" name="assignments"/><el-tab-pane :label="t('agentDelivery')" name="agents"/><el-tab-pane :label="t('incidents')" name="incidents"/><el-tab-pane :label="t('incidentRevisions')" name="revisions"/><el-tab-pane :label="t('dataLoss')" name="loss"/><el-tab-pane :label="t('alerts')" name="alerts"/></el-tabs>
    <el-table v-if="active==='collectors'" v-loading="loading" :data="collectors" row-key="id"><el-table-column prop="name" :label="t('name')" min-width="160"><template #default="{row}"><div class="server-name"><span class="status-dot" :class="row.revoked?'offline':row.status||'unknown'"></span><div><strong>{{row.name}}</strong><small class="mono">{{row.id}}</small></div></div></template></el-table-column><el-table-column prop="address" :label="t('address')" min-width="200"/><el-table-column prop="generation" :label="t('generation')" width="100"/><el-table-column prop="config_version" :label="t('configVersion')" width="120"/><el-table-column prop="last_seen" :label="t('lastSeen')" width="190"><template #default="{row}">{{pretty(row.last_seen,'last_seen')}}</template></el-table-column><el-table-column :label="t('spoolSize')" width="120"><template #default="{row}">{{pretty(row.spool_size,'spool_size')}}</template></el-table-column><el-table-column :label="t('actions')" width="190" fixed="right"><template #default="{row}"><el-button size="small" circle :disabled="actionBusy===row.id" @click="open(row)"><i class="ri-edit-line"></i></el-button><el-dropdown trigger="click" :disabled="actionBusy===row.id"><el-button size="small" :loading="actionBusy===row.id"><i class="ri-more-2-fill"></i></el-button><template #dropdown><el-dropdown-menu><el-dropdown-item @click="act(row,'view-token')"><i class="ri-key-2-line"></i>{{t('viewToken')}}</el-dropdown-item><el-dropdown-item @click="act(row,'rotate-token')"><i class="ri-loop-left-line"></i>{{t('rotateToken')}}</el-dropdown-item><el-dropdown-item @click="act(row,'revoke')"><i class="ri-forbid-line"></i>{{t('revoke')}}</el-dropdown-item><el-dropdown-item divided @click="act(row,'delete')"><i class="ri-delete-bin-line"></i>{{t('delete')}}</el-dropdown-item></el-dropdown-menu></template></el-dropdown></template></el-table-column><template #empty><AppEmpty class="empty-state" icon="ri-radar-line" :description="t('noData')" /></template></el-table>
    <el-table v-else v-loading="loading" :data="records"><el-table-column v-for="key in Object.keys(records[0]||{}).slice(0,8)" :key="key" :prop="key" :label="fieldLabel(key)"><template #default="{row}"><span :class="{mono:key.includes('uuid')||key.includes('id')}">{{pretty(row[key],key)}}</span></template></el-table-column><template #empty><AppEmpty class="empty-state" icon="ri-radar-line" :description="t('noData')" /></template></el-table>
  </section>
  <CollectorEditorDialog v-model="editor" :value="editing" @saved="editorSaved"/>
  <AppDialog v-model="tokenDialog" :title="t('registrationToken')" mode="view" width="min(560px,92vw)"><el-input v-model="token" class="token-display mono" readonly><template #append><el-button @click="copyToken"><i class="ri-file-copy-line"></i></el-button></template></el-input></AppDialog>
</template>
