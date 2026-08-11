<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { AppDialog } from '@santaizi/ui'
import { getProbeCapabilities, getServerCredential, getServerInstallPreview, type ServerRecord } from '@/api/adminApi'
import { useEditorSnapshot } from '@/composables/editorSnapshot'
import { notifyAPIError } from '@/composables/notify'
import type { MonitoringOptions, ProbeCapabilitiesMetadata } from '@/types/admin'

const props = defineProps<{ modelValue: boolean; server?: ServerRecord; secret?: string }>()
const emit = defineEmits<{ 'update:modelValue': [boolean] }>()
const { t, te } = useI18n()
const loading = ref(false)
const platform = ref<'linux' | 'macos' | 'windows'>('linux')
const profile = ref<'standard' | 'light' | 'alive'>('standard')
const cleanInstall = ref(true)
const cleanConfirmed = ref(false)
const secret = ref('')
const command = ref('')
const metadata = ref<ProbeCapabilitiesMetadata>({ required: [], optional: [], presets: {} })
const capabilities = reactive<MonitoringOptions>({ cpu: true, memory: true, disk: true, network: true, connections: true, processes: true, temperature: true, gpu: true, host_info: true, ip_report: true, http_probe: true, icmp_probe: true, tcp_probe: true, nat: false })
const snapshotValue = computed(() => ({ profile: profile.value, cleanInstall: cleanInstall.value, capabilities }))
const { dirty, capture } = useEditorSnapshot(snapshotValue, computed(() => props.modelValue))
const capabilityRows = computed(() => [
  ['cpu', 'ri-cpu-line'], ['memory', 'ri-database-2-line'], ['disk', 'ri-hard-drive-3-line'], ['network', 'ri-exchange-line'],
  ['connections', 'ri-links-line'], ['processes', 'ri-stack-line'], ['temperature', 'ri-temp-hot-line'], ['gpu', 'ri-cpu-line'],
  ['host_info', 'ri-computer-line'], ['ip_report', 'ri-map-pin-line'], ['http_probe', 'ri-global-line'], ['icmp_probe', 'ri-pulse-line'],
  ['tcp_probe', 'ri-router-line'], ['nat', 'ri-route-line'],
] as Array<[keyof MonitoringOptions, string]>)
function applyProfile(value: typeof profile.value) {
  profile.value = value
  Object.assign(capabilities, metadata.value.presets[value] || metadata.value.presets.standard)
}
async function refreshPreview() {
  if (!props.server) return
  const preview = await getServerInstallPreview(props.server.id, { platform: platform.value, clean_install: cleanInstall.value, options: { ...capabilities } })
  command.value = preview.command
}
async function open() {
  if (!props.server) return
  loading.value = true; platform.value = 'linux'; profile.value = 'standard'; cleanInstall.value = true; cleanConfirmed.value = false; command.value = ''
  try {
    const [credential, available] = await Promise.all([
      props.secret ? Promise.resolve({ secret: props.secret }) : getServerCredential(props.server), getProbeCapabilities(),
    ])
    secret.value = credential.secret; metadata.value = available
    applyProfile('standard'); await refreshPreview()
  } catch (error) { notifyAPIError(error, t as never, te) }
  finally { loading.value = false; await nextTick(); capture() }
}
async function copy() {
  if (cleanInstall.value && !cleanConfirmed.value) { ElMessage.warning(t('confirmCleanInstallRequired')); return }
  await refreshPreview()
  await navigator.clipboard.writeText(command.value)
  capture(); ElMessage.success(t('copied'))
}
async function copySecret() { await navigator.clipboard.writeText(secret.value); ElMessage.success(t('copied')) }
function selectProfile(value: string | number | boolean) { applyProfile(value as typeof profile.value) }
watch(() => props.modelValue, value => { if (value) void open() })
watch([platform, snapshotValue], () => { command.value = ''; if (props.modelValue && !loading.value) void refreshPreview() }, { deep: true })
</script>

<template>
  <AppDialog :model-value="modelValue" :title="`${t('installAgent')} · ${server?.name || ''}`" mode="edit" :dirty="dirty" :submitting="loading" width="min(960px, 97vw)" @update:model-value="emit('update:modelValue', $event)">
    <div v-loading="loading">
      <el-form label-position="top">
        <el-form-item :label="t('secret')"><el-input :model-value="secret" readonly class="mono"><template #append><el-button :aria-label="t('copy')" @click="copySecret"><i class="ri-file-copy-line"></i></el-button></template></el-input></el-form-item>
        <el-form-item :label="t('monitoringPreset')"><el-segmented :model-value="profile" :options="[{ label: t('presetStandard'), value: 'standard' }, { label: t('presetLight'), value: 'light' }, { label: t('presetHeartbeat'), value: 'alive' }]" @change="selectProfile" /></el-form-item>
        <div class="capability-grid">
          <label v-for="([key, icon]) in capabilityRows" :key="key" class="capability-item"><span><i :class="icon"></i>{{ t(`capability_${key}`) }}</span><el-switch v-model="capabilities[key]" /></label>
        </div>
        <div class="clean-install-box"><el-checkbox v-model="cleanInstall">{{ t('cleanInstall') }}</el-checkbox><el-checkbox v-if="cleanInstall" v-model="cleanConfirmed">{{ t('confirmCleanInstall') }}</el-checkbox></div>
      </el-form>
      <el-tabs v-model="platform" class="install-tabs"><el-tab-pane :label="t('linux')" name="linux"/><el-tab-pane :label="t('macos')" name="macos"/><el-tab-pane :label="t('windows')" name="windows"/></el-tabs>
      <el-input :model-value="command" readonly type="textarea" :rows="6" class="mono" />
    </div>
    <template #footer="{ close }"><el-button :disabled="loading" @click="close()">{{ t('close') }}</el-button><el-button type="primary" :disabled="loading || (cleanInstall && !cleanConfirmed)" @click="copy"><i class="ri-file-copy-line"></i>{{ t('copyCommand') }}</el-button></template>
  </AppDialog>
</template>
