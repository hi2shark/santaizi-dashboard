<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, type FormInstance } from 'element-plus'
import { AppDialog } from '@santaizi/ui'
import { createCollector, listAllServers, updateCollector, updateCollectorScope, type CollectorRecord, type ServerRecord } from '@/api/adminApi'
import { useEditorSnapshot } from '@/composables/editorSnapshot'
import { notifyAPIError } from '@/composables/notify'
import type { CollectorScope } from '@santaizi/api'

const props = defineProps<{ modelValue: boolean; value?: CollectorRecord }>()
const emit = defineEmits<{ 'update:modelValue': [boolean]; saved: [string] }>()
const { t, te } = useI18n()
const saving = ref(false)
const formRef = ref<FormInstance>()
const servers = ref<ServerRecord[]>([])
const selectedServerIds = ref<string[]>([])
const form = reactive<{ id: string; name: string; address: string; tls: boolean; insecure_tls: boolean; scopes: CollectorScope[] }>({
  id: '', name: '', address: '', tls: true, insecure_tls: false, scopes: [{ type: 'all', value: '' }],
})
const snapshotValue = computed(() => ({ form, selectedServerIds: selectedServerIds.value }))
const { dirty, capture } = useEditorSnapshot(snapshotValue, computed(() => props.modelValue))
const groups = computed(() => [...new Set(servers.value.map(server => server.tag).filter(Boolean))].sort())
const tags = groups
const transferData = computed(() => servers.value.map(server => ({ key: String(server.id), label: server.name })))
const firstServerScopeIndex = computed(() => form.scopes.findIndex(scope => scope.type === 'server'))

function collapseScopesForUi(scopes: CollectorScope[]): { ui: CollectorScope[]; serverIds: string[] } {
  const serverIds = scopes.filter(scope => scope.type === 'server').map(scope => scope.value.trim()).filter(Boolean)
  const others: CollectorScope[] = scopes
    .filter(scope => scope.type !== 'server')
    .map(scope => ({ type: scope.type, value: scope.value }))
  if (scopes.some(scope => scope.type === 'server')) {
    return { ui: [...others, { type: 'server', value: '' }], serverIds }
  }
  return { ui: others.length ? others : [{ type: 'all', value: '' }], serverIds: [] }
}

function buildScopesForSubmit(): CollectorScope[] {
  if (form.scopes.some(scope => scope.type === 'all')) return [{ type: 'all', value: '' }]
  const others: CollectorScope[] = form.scopes
    .filter(scope => scope.type !== 'server')
    .map(scope => ({ type: scope.type, value: scope.value.trim() }))
  const serverScopes: CollectorScope[] = selectedServerIds.value.map(id => ({ type: 'server', value: id }))
  return [...others, ...serverScopes]
}

function addScope() {
  if (form.scopes.some(scope => scope.type === 'all')) form.scopes = []
  if (form.scopes.some(scope => scope.type === 'server')) form.scopes.push({ type: 'group', value: '' })
  else form.scopes.push({ type: 'server', value: '' })
}

function changeScope(index: number) {
  const scope = form.scopes[index]
  if (!scope) return
  if (scope.type === 'all') {
    form.scopes = [{ type: 'all', value: '' }]
    selectedServerIds.value = []
    return
  }
  if (scope.type === 'server') {
    const others = form.scopes.filter((item, current) => current !== index && item.type !== 'server' && item.type !== 'all')
    form.scopes = [...others, { type: 'server', value: '' }]
    return
  }
  scope.value = ''
  if (!form.scopes.some(item => item.type === 'server')) selectedServerIds.value = []
}

function removeScope(index: number) {
  const removed = form.scopes[index]
  form.scopes.splice(index, 1)
  if (removed?.type === 'server' || !form.scopes.some(scope => scope.type === 'server')) selectedServerIds.value = []
  if (!form.scopes.length) form.scopes = [{ type: 'all', value: '' }]
}

async function reset(value?: CollectorRecord) {
  const rawScopes: CollectorScope[] = value?.scopes?.length
    ? value.scopes.map((scope): CollectorScope => ({
      type: scope.type as CollectorScope['type'],
      value: scope.value,
    }))
    : [{ type: 'all', value: '' }]
  const collapsed = collapseScopesForUi(rawScopes)
  Object.assign(form, {
    id: value?.id || '',
    name: value?.name || '',
    address: value?.address || '',
    tls: value?.tls ?? true,
    insecure_tls: value?.insecure_tls ?? false,
    scopes: collapsed.ui,
  })
  selectedServerIds.value = collapsed.serverIds
  try { servers.value = (await listAllServers()).data } catch (error) { notifyAPIError(error, t as never, te) }
  await nextTick()
  capture()
}

async function submit() {
  await formRef.value?.validate()
  const scopes = buildScopesForSubmit()
  const hasAll = scopes.some(scope => scope.type === 'all')
  const incomplete = scopes.some(scope => scope.type !== 'all' && !scope.value.trim())
  if (!scopes.length || incomplete || (hasAll && scopes.length !== 1)) {
    ElMessage.warning(t('invalidCollectorScope'))
    return
  }
  saving.value = true
  try {
    let token = ''
    const payload = { name: form.name, address: form.address, tls: form.tls, insecure_tls: form.insecure_tls, scopes }
    if (form.id) {
      await updateCollector(form.id, payload)
      await updateCollectorScope(form.id, { scopes })
    } else {
      const result = await createCollector(payload)
      token = result.registration_token
    }
    capture()
    emit('update:modelValue', false)
    emit('saved', token)
    ElMessage.success(t('saveSuccess'))
  } catch (error) { notifyAPIError(error, t as never, te) }
  finally { saving.value = false }
}

watch(() => props.modelValue, value => { if (value) void reset(props.value) })
</script>

<template>
  <AppDialog :model-value="modelValue" :title="form.id ? t('editCollector') : t('createCollector')" mode="edit" :dirty="dirty" :submitting="saving" width="min(920px, 96vw)" @update:model-value="emit('update:modelValue', $event)">
    <el-form ref="formRef" :model="form" label-position="top" @submit.prevent="submit">
      <div class="editor-grid">
        <el-form-item :label="t('name')" prop="name" :rules="[{ required: true, message: t('required') }]"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="t('address')" prop="address" :rules="[{ required: true, message: t('required') }]"><el-input v-model="form.address" placeholder="collector.example.com:5555" /></el-form-item>
      </div>
      <div class="switch-grid"><label><span>{{ t('tls') }}</span><el-switch v-model="form.tls" /></label><label v-if="form.tls"><span>{{ t('insecureTLS') }}</span><el-switch v-model="form.insecure_tls" /></label></div>
      <div class="editor-section">
        <div class="editor-section-title"><h3>{{ t('scope') }}</h3><el-button @click="addScope"><i class="ri-add-line"></i>{{ t('addScope') }}</el-button></div>
        <div class="scope-list">
          <div v-for="(scope, index) in form.scopes" :key="index" class="scope-block">
            <div class="typed-scope-row">
              <el-select v-model="scope.type" @change="changeScope(index)">
                <el-option :label="t('scopeAll')" value="all"/>
                <el-option :label="t('scopeServer')" value="server"/>
                <el-option :label="t('scopeGroup')" value="group"/>
                <el-option :label="t('scopeTag')" value="tag"/>
              </el-select>
              <span v-if="scope.type === 'server'" class="scope-all-value">{{ selectedServerIds.length ? `${t('selectedServers')} · ${selectedServerIds.length}` : t('PleaseSelect') }}</span>
              <el-select v-else-if="scope.type === 'group'" v-model="scope.value" filterable allow-create><el-option v-for="group in groups" :key="group" :label="group" :value="group"/></el-select>
              <el-select v-else-if="scope.type === 'tag'" v-model="scope.value" filterable allow-create><el-option v-for="tag in tags" :key="tag" :label="tag" :value="tag"/></el-select>
              <span v-else class="scope-all-value">{{ t('allServers') }}</span>
              <el-button circle :disabled="form.scopes.length === 1" :aria-label="t('delete')" @click="removeScope(index)"><i class="ri-delete-bin-6-line"></i></el-button>
            </div>
            <el-transfer
              v-if="scope.type === 'server' && index === firstServerScopeIndex"
              v-model="selectedServerIds"
              filterable
              :filter-placeholder="t('searchServers')"
              :data="transferData"
              :titles="[t('availableServers'), t('selectedServers')]"
              class="server-transfer"
            />
          </div>
        </div>
      </div>
    </el-form>
    <template #footer="{ close }"><el-button :disabled="saving" @click="close()">{{ t('cancel') }}</el-button><el-button type="primary" :loading="saving" @click="submit">{{ t('save') }}</el-button></template>
  </AppDialog>
</template>
