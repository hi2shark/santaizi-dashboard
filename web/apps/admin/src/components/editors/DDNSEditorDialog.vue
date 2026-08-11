<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, type FormInstance } from 'element-plus'
import { AppDialog } from '@santaizi/ui'
import { createDDNSProfile, listDDNSProviders, updateDDNSProfile } from '@/api/adminApi'
import { useEditorSnapshot } from '@/composables/editorSnapshot'
import { notifyAPIError } from '@/composables/notify'
import type { DDNSProvider } from '@santaizi/api'
import type { DDNSProfileRecord, KeyValueRow } from '@/types/admin'

const props = defineProps<{ modelValue: boolean; value?: DDNSProfileRecord }>()
const emit = defineEmits<{ 'update:modelValue': [boolean]; saved: [] }>()
const { t, te } = useI18n()
const formRef = ref<FormInstance>()
const saving = ref(false)
const providers = ref<DDNSProvider[]>([])
const webhookHeaders = ref<KeyValueRow[]>([])
const form = reactive<DDNSProfileRecord>({ id: 0, name: '', provider: '', domains: [], enable_ipv4: true, enable_ipv6: false, access_id: '', access_secret: '', max_retries: 3, webhook_url: '', webhook_method: 'post', webhook_request_type: 'json', webhook_headers: [], webhook_body: '' })
const provider = computed(() => providers.value.find(item => item.name === form.provider))
const snapshotValue = computed(() => ({ form, webhookHeaders: webhookHeaders.value }))
const { dirty, capture } = useEditorSnapshot(snapshotValue, computed(() => props.modelValue))

function clampNumber(value: unknown, min: number, max: number, fallback: number) {
  const next = Number(value)
  if (!Number.isFinite(next)) return fallback
  return Math.min(max, Math.max(min, next))
}

function reset(value?: DDNSProfileRecord) {
  Object.assign(form, {
    id: value?.id || 0, name: value?.name || '', provider: value?.provider || '', domains: [...(value?.domains || [])],
    enable_ipv4: value?.enable_ipv4 ?? true, enable_ipv6: value?.enable_ipv6 ?? false,
    access_id: value?.access_id || '', access_secret: value?.access_secret || '', max_retries: value?.max_retries || 3,
    webhook_url: value?.webhook_url || '', webhook_method: value?.webhook_method || 'post', webhook_request_type: value?.webhook_request_type || 'json',
    webhook_headers: [...(value?.webhook_headers || [])], webhook_body: value?.webhook_body || '',
  })
  webhookHeaders.value = (value?.webhook_headers || []).map(item => ({ ...item }))
  nextTick(capture)
}
async function loadProviders() {
  try { providers.value = (await listDDNSProviders()).data; const first = providers.value[0]; if (!form.provider && first) form.provider = first.name }
  catch (error) { notifyAPIError(error, t as never, te) }
}
async function submit() {
  await formRef.value?.validate()
  saving.value = true
  try {
    const { id: _id, ...write } = form
    const payload = { ...write, webhook_headers: webhookHeaders.value.filter(row => row.key.trim()).map(row => ({ key: row.key.trim(), value: row.value })) }
    if (form.id) await updateDDNSProfile(form.id, payload); else await createDDNSProfile(payload)
    capture(); emit('update:modelValue', false); emit('saved'); ElMessage.success(t('saveSuccess'))
  } catch (error) { notifyAPIError(error, t as never, te) }
  finally { saving.value = false }
}
watch(() => props.modelValue, value => { if (value) { reset(props.value); void loadProviders() } })
</script>

<template>
  <AppDialog :model-value="modelValue" :title="form.id ? t('editDDNSProfile') : t('createDDNSProfile')" mode="edit" :dirty="dirty" :submitting="saving" width="min(900px, 96vw)" @update:model-value="emit('update:modelValue', $event)">
    <el-form ref="formRef" :model="form" label-position="top" @submit.prevent="submit">
      <div class="editor-grid">
        <el-form-item :label="t('name')" prop="name" :rules="[{ required: true, message: t('required') }]"><el-input v-model="form.name" /></el-form-item>
        <el-form-item :label="t('provider')" prop="provider" :rules="[{ required: true, message: t('required') }]"><el-select v-model="form.provider" class="field-full"><el-option v-for="item in providers" :key="String(item.id)" :label="item.name" :value="item.name" /></el-select></el-form-item>
        <el-form-item class="span-2" :label="t('domains')" prop="domains" :rules="[{ type: 'array', required: true, min: 1, message: t('required') }]"><el-select v-model="form.domains" multiple filterable allow-create default-first-option class="field-full" :placeholder="t('enterDomain')" /></el-form-item>
        <el-form-item :label="t('recordProtocols')"><el-checkbox v-model="form.enable_ipv4">IPv4</el-checkbox><el-checkbox v-model="form.enable_ipv6">IPv6</el-checkbox></el-form-item>
        <el-form-item :label="t('maxRetries')"><el-input v-model.number="form.max_retries" inputmode="numeric" class="field-full" @blur="form.max_retries = clampNumber(form.max_retries, 0, 20, 3)" /></el-form-item>
        <el-form-item v-if="provider?.access_id" :label="t('DDNSAccessID')"><el-input v-model="form.access_id" autocomplete="off" /></el-form-item>
        <el-form-item v-if="provider?.access_secret" :label="t('DDNSAccessSecret')"><el-input v-model="form.access_secret" type="password" show-password autocomplete="new-password" /></el-form-item>
      </div>
      <div v-if="provider?.webhook_url" class="editor-section">
        <h3>{{ t('webhookSettings') }}</h3>
        <div class="editor-grid">
          <el-form-item class="span-2" :label="t('WebhookURL')"><el-input v-model="form.webhook_url" /></el-form-item>
          <el-form-item v-if="provider.webhook_method" :label="t('requestMethod')"><el-segmented v-model="form.webhook_method" :options="[{ label: 'GET', value: 'get' }, { label: 'POST', value: 'post' }]" /></el-form-item>
          <el-form-item v-if="provider.webhook_request_type" :label="t('requestType')"><el-segmented v-model="form.webhook_request_type" :options="[{ label: t('bodyJSON'), value: 'json' }, { label: t('bodyForm'), value: 'form' }]" /></el-form-item>
        </div>
        <template v-if="provider.webhook_headers">
          <div class="editor-section-title"><h3>{{ t('requestHeaders') }}</h3><el-button @click="webhookHeaders.push({ key: '', value: '' })"><i class="ri-add-line"></i>{{ t('addHeader') }}</el-button></div>
          <div class="key-value-list"><div v-for="(header, index) in webhookHeaders" :key="index" class="key-value-row"><el-input v-model="header.key" :placeholder="t('headerName')"/><el-input v-model="header.value" :placeholder="t('headerValue')"/><el-button circle :aria-label="t('delete')" @click="webhookHeaders.splice(index, 1)"><i class="ri-delete-bin-6-line"></i></el-button></div></div>
        </template>
        <el-form-item v-if="provider.webhook_request_body" :label="t('requestBody')"><el-input v-model="form.webhook_body" type="textarea" :rows="6" class="mono" /></el-form-item>
      </div>
    </el-form>
    <template #footer="{ close }"><el-button :disabled="saving" @click="close()">{{ t('cancel') }}</el-button><el-button type="primary" :loading="saving" @click="submit">{{ t('save') }}</el-button></template>
  </AppDialog>
</template>
