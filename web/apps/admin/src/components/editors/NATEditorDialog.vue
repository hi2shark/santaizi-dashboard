<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, type FormInstance } from 'element-plus'
import { AppDialog } from '@santaizi/ui'
import { createNATTunnel, listAllServers, updateNATTunnel, type ServerRecord } from '@/api/adminApi'
import { useEditorSnapshot } from '@/composables/editorSnapshot'
import { notifyAPIError } from '@/composables/notify'
import type { NATTunnelRecord } from '@/types/admin'

const props = defineProps<{ modelValue: boolean; value?: NATTunnelRecord }>()
const emit = defineEmits<{ 'update:modelValue': [boolean]; saved: [] }>()
const { t, te } = useI18n()
const saving = ref(false)
const formRef = ref<FormInstance>()
const servers = ref<ServerRecord[]>([])
const form = reactive<NATTunnelRecord>({ id: 0, name: '', server_id: 0, target: '', domain: '' })
const { dirty, capture } = useEditorSnapshot(form, computed(() => props.modelValue))
function reset(value?: NATTunnelRecord) { Object.assign(form, { id: value?.id || 0, name: value?.name || '', server_id: value?.server_id || 0, target: value?.target || '', domain: value?.domain || '' }); nextTick(capture) }
async function loadServers() { try { servers.value = (await listAllServers()).data } catch (error) { notifyAPIError(error, t as never, te) } }
async function submit() {
  await formRef.value?.validate(); saving.value = true
  try { const { id: _id, ...payload } = form; if (form.id) await updateNATTunnel(form.id, payload); else await createNATTunnel(payload); capture(); emit('update:modelValue', false); emit('saved'); ElMessage.success(t('saveSuccess')) }
  catch (error) { notifyAPIError(error, t as never, te) } finally { saving.value = false }
}
watch(() => props.modelValue, value => { if (value) { reset(props.value); void loadServers() } })
</script>

<template>
  <AppDialog :model-value="modelValue" :title="form.id ? t('editNATTunnel') : t('createNATTunnel')" mode="edit" :dirty="dirty" :submitting="saving" width="min(680px, 94vw)" @update:model-value="emit('update:modelValue', $event)">
    <el-form ref="formRef" :model="form" label-position="top" @submit.prevent="submit">
      <el-form-item :label="t('name')" prop="name" :rules="[{ required: true, message: t('required') }]"><el-input v-model="form.name" /></el-form-item>
      <el-form-item :label="t('server')" prop="server_id" :rules="[{ required: true, message: t('required') }]"><el-select v-model="form.server_id" filterable class="field-full"><el-option v-for="server in servers" :key="server.id" :label="server.name" :value="server.id" /></el-select></el-form-item>
      <el-form-item :label="t('localService')" prop="target" :rules="[{ required: true, message: t('required') }]"><el-input v-model="form.target" placeholder="127.0.0.1:8080" /></el-form-item>
      <el-form-item :label="t('bindDomain')" prop="domain" :rules="[{ required: true, message: t('required') }]"><el-input v-model="form.domain" placeholder="app.example.com" /></el-form-item>
    </el-form>
    <template #footer="{ close }"><el-button :disabled="saving" @click="close()">{{ t('cancel') }}</el-button><el-button type="primary" :loading="saving" @click="submit">{{ t('save') }}</el-button></template>
  </AppDialog>
</template>
