<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { createApiToken, deleteApiToken, getApiToken, listApiTokens, type APIToken } from '@santaizi/api'
import { AppDialog, AppEmpty } from '@santaizi/ui'
import { notifyAPIError } from '@/composables/notify'
const { t, te } = useI18n()
const items = ref<APIToken[]>([]), loading = ref(false), saving = ref(false), note = ref(''), dialog = ref(false), token = ref('')
async function load() { loading.value = true; try { items.value = (await listApiTokens()).data } catch (e) { notifyAPIError(e, t as never, te) } finally { loading.value = false } }
async function create() { saving.value = true; try { const result = await createApiToken({ note: note.value }); token.value = result.token || ''; dialog.value = true; note.value = ''; await load() } catch (e) { notifyAPIError(e, t as never, te) } finally { saving.value = false } }
async function reveal(row: APIToken) { try { token.value = (await getApiToken(row.id)).token || ''; dialog.value = true } catch (e) { notifyAPIError(e, t as never, te) } }
async function remove(row: APIToken) { await ElMessageBox.confirm(t('confirmDelete'), t('dangerousAction'), { type: 'warning' }); try { await deleteApiToken(row.id); await load() } catch (e) { notifyAPIError(e, t as never, te) } }
async function copy() { await navigator.clipboard.writeText(token.value); ElMessage.success(t('copied')) }
onMounted(load)
</script>
<template>
  <div class="page-head"><div><h1>{{ t('apiTokens') }}</h1></div><a class="el-button" href="/docs/api/" target="_blank"><i class="ri-book-open-line"></i>{{ t('apiDocs') }}</a></div>
  <section class="surface token-create"><el-input v-model="note" :placeholder="t('tokenNote')" @keyup.enter="!saving&&note.trim()&&create()"/><el-button type="primary" :loading="saving" :disabled="!note.trim()" @click="create"><i class="ri-key-2-line"></i>{{ t('issueToken') }}</el-button></section>
  <section class="surface table-card"><el-table v-loading="loading" :data="items"><el-table-column prop="note" :label="t('tokenNote')"/><el-table-column prop="token_prefix" :label="t('token')" width="170"><template #default="{row}"><span class="mono">{{ row.token_prefix }}…</span></template></el-table-column><el-table-column prop="created_at" :label="t('createdAt')" width="200"/><el-table-column :label="t('actions')" width="130"><template #default="{row}"><el-button circle plain :aria-label="t('viewToken')" @click="reveal(row)"><i class="ri-eye-line"></i></el-button><el-button circle type="danger" plain :aria-label="t('delete')" @click="remove(row)"><i class="ri-delete-bin-line"></i></el-button></template></el-table-column><template #empty><AppEmpty class="empty-state" icon="ri-key-2-line" :description="t('noData')" /></template></el-table></section>
  <AppDialog v-model="dialog" :title="t('token')" mode="view"><el-input v-model="token" readonly class="mono token-display"><template #append><el-button :aria-label="t('copy')" @click="copy"><i class="ri-file-copy-line"></i></el-button></template></el-input></AppDialog>
</template>
