<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ServerRecord } from '@santaizi/api'
import { getPresentation } from '@santaizi/theme-server-status'
import type { ServerLocation } from '../../utils/worldMap'
import { formatBinary, stateValue } from '../../utils/host'
import ServerGlobe from './ServerGlobe.vue'

const props = defineProps<{ server: ServerRecord; location: ServerLocation | null }>()
const { t } = useI18n()
const presentation = computed(() => getPresentation(props.server.public_note))
const flagClass = computed(() => {
  const code = presentation.value.flag || props.location?.countryCode
  return code ? `fi fi-${code.toLowerCase()}` : ''
})
const spec = computed(() => {
  const cpu = Number(props.server.host?.CPU || props.server.host?.cpu || 0)
  const memory = stateValue(props.server.state, 'MemTotal', 'mem_total')
  const disk = stateValue(props.server.state, 'DiskTotal', 'disk_total')
  return [
    cpu > 0 ? `${cpu}C` : '',
    memory > 0 ? `${formatBinary(memory, 0).value}${formatBinary(memory, 0).unit}` : '',
    disk > 0 ? `${formatBinary(disk, 0).value}${formatBinary(disk, 0).unit}` : '',
  ].filter(Boolean).join('')
})
</script>

<template>
  <section class="nazhua-detail-name">
    <div class="nazhua-detail-name__main">
      <div class="nazhua-detail-name__flag">
        <span v-if="flagClass" :class="flagClass" />
        <i v-else class="ri-global-line"></i>
      </div>
      <div class="nazhua-detail-name__text">
        <h1>{{ server.name }}</h1>
        <p v-if="presentation.slogan">“{{ presentation.slogan }}”</p>
        <p v-else-if="location?.name" class="nazhua-detail-name__loc">{{ location.name }}</p>
        <div v-if="!presentation.slogan || !server.online" class="nazhua-detail-name__meta">
          <span v-if="spec" class="nazhua-detail-name__spec"><i class="ri-cpu-line"></i>{{ spec }}</span>
          <span v-if="!server.online" class="offline">
            <i class="ri-indeterminate-circle-fill"></i>
            {{ t('nazhua.offline') }}
          </span>
        </div>
      </div>
    </div>
    <ServerGlobe :location="location" />
  </section>
</template>
