<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { AppEmpty } from '@santaizi/ui'
import DDNSEditorDialog from '@/components/editors/DDNSEditorDialog.vue'
import { deleteDDNSProfile, listDDNSProfiles } from '@/api/adminApi'
import { notifyAPIError } from '@/composables/notify'
import type { DDNSProfileRecord } from '@/types/admin'

const { t, te } = useI18n()
const loading = ref(false), editor = ref(false), total = ref(0)
const items = ref<DDNSProfileRecord[]>([]), selected = ref<DDNSProfileRecord[]>([]), editing = ref<DDNSProfileRecord>()
const query = reactive({ page: 1, page_size: 20, q: '', sort: 'id', order: 'desc' as const })
async function load() { loading.value = true; try { const result = await listDDNSProfiles(query); items.value = result.data; total.value = result.meta.total || result.data.length } catch (error) { notifyAPIError(error, t as never, te) } finally { loading.value = false } }
function open(item?: DDNSProfileRecord) { editing.value = item; editor.value = true }
async function remove(rows: DDNSProfileRecord[]) { await ElMessageBox.confirm(t('confirmDelete'), t('dangerousAction'), { type: 'warning' }); try { await Promise.all(rows.map(row => deleteDDNSProfile(row.id))); selected.value = []; await load(); ElMessage.success(t('deleteSuccess')) } catch (error) { notifyAPIError(error, t as never, te) } }
onMounted(load)
</script>
<template>
  <div class="page-head"><h1>{{ t('ddns') }}</h1><el-button type="primary" @click="open()"><i class="ri-add-line"></i>{{ t('createDDNSProfile') }}</el-button></div>
  <section class="surface table-card"><div class="toolbar"><el-input v-model="query.q" class="search-input" clearable :placeholder="t('search')" @keyup.enter="query.page=1;load()"><template #prefix><i class="ri-search-line"></i></template></el-input><el-button @click="query.page=1;load()"><i class="ri-search-line"></i>{{ t('submitSearch') }}</el-button><el-button v-if="selected.length" type="danger" plain @click="remove(selected)"><i class="ri-delete-bin-6-line"></i>{{ t('batchDelete') }}</el-button><span class="toolbar-spacer"></span><el-button @click="load"><i class="ri-refresh-line"></i>{{ t('refresh') }}</el-button></div>
    <el-table v-loading="loading" :data="items" row-key="id" @selection-change="selected=$event"><el-table-column type="selection" width="46"/><el-table-column prop="name" :label="t('name')" min-width="160"/><el-table-column prop="provider_name" :label="t('provider')" width="160"><template #default="{row}">{{ row.provider_name || row.provider }}</template></el-table-column><el-table-column :label="t('domains')" min-width="260"><template #default="{row}"><div class="metric-tags"><el-tag v-for="domain in row.domains" :key="domain" effect="plain">{{ domain }}</el-tag></div></template></el-table-column><el-table-column :label="t('recordProtocols')" width="140"><template #default="{row}"><el-tag v-if="row.enable_ipv4" effect="plain">IPv4</el-tag><el-tag v-if="row.enable_ipv6" effect="plain">IPv6</el-tag></template></el-table-column><el-table-column :label="t('actions')" width="110" fixed="right"><template #default="{row}"><div class="inline-actions"><el-button circle :aria-label="t('edit')" @click="open(row)"><i class="ri-edit-2-line"></i></el-button><el-button circle type="danger" plain :aria-label="t('delete')" @click="remove([row])"><i class="ri-delete-bin-6-line"></i></el-button></div></template></el-table-column><template #empty><AppEmpty icon="ri-global-line" :description="t('noData')"/></template></el-table><div class="pagination"><el-pagination v-model:current-page="query.page" v-model:page-size="query.page_size" layout="total, sizes, prev, pager, next" :total="total" @change="load"/></div></section>
  <DDNSEditorDialog v-model="editor" :value="editing" @saved="load"/>
</template>

