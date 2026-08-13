<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ServerRecord } from '@santaizi/api'
import { buildPublicNoteView } from '@santaizi/theme-server-status'

const props = defineProps<{ server: ServerRecord }>()
const { t } = useI18n()
const view = computed(() => buildPublicNoteView(props.server.public_note))

function hostField(...keys: string[]) {
  for (const key of keys) {
    const value = props.server.host?.[key]
    if (value !== undefined && value !== null && String(value) !== '') return String(value)
  }
  return '—'
}
</script>

<template>
  <section class="nazhua-info-box">
    <h2>{{ t('nazhua.serverInfo') }}</h2>
    <dl>
      <div><dt>{{ t('nazhua.platform') }}</dt><dd>{{ hostField('Platform', 'platform') }}</dd></div>
      <div><dt>{{ t('nazhua.arch') }}</dt><dd>{{ hostField('Arch', 'arch') }}</dd></div>
      <div><dt>{{ t('nazhua.version') }}</dt><dd>{{ hostField('Version', 'version') }}</dd></div>
      <div v-if="view.presentation.locationLabel"><dt>{{ t('nazhua.location') }}</dt><dd>{{ view.presentation.locationLabel }}</dd></div>
      <div v-if="view.bill.amountKind"><dt>{{ t('nazhua.billing') }}</dt><dd>{{ view.bill.amountValue || view.bill.amountKind }}</dd></div>
      <div v-if="view.bill.bandwidth"><dt>{{ t('nazhua.bandwidth') }}</dt><dd>{{ view.bill.bandwidth }}</dd></div>
    </dl>
    <div v-if="view.planTags.length" class="nazhua-info-box__tags">
      <span v-for="tag in view.planTags" :key="tag">{{ tag.replace(/^__|__$/g, '') }}</span>
    </div>
  </section>
</template>
