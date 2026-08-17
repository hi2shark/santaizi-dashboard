<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ServerRecord } from '@santaizi/api'
import type { CycleTransferMap } from '../domain/serverStatusView'
import { toServerStatusViews } from '../domain/serverStatusView'
import { statusTableLayout, type StatusTableColumns } from '../domain/statusTableColumns'
import ServerRow from './ServerRow.vue'

const props = defineProps<{
  title?: string
  servers: readonly ServerRecord[]
  cycles?: CycleTransferMap
  columns: StatusTableColumns
}>()

const emit = defineEmits<{ select: [id: number] }>()
const { locale, t } = useI18n()
const views = computed(() => toServerStatusViews([...props.servers], props.cycles, Date.now(), locale.value))
const layout = computed(() => statusTableLayout(props.columns))
const tableStyle = computed(() => ({
  '--ss-table-cols': layout.value.columns,
  '--ss-table-min': `${layout.value.minWidth}px`,
}))
</script>

<template>
  <section class="status-panel">
    <header v-if="title" class="group-title">
      <span>{{ title }}</span>
      <small>{{ servers.length }}</small>
    </header>
    <div class="ss-table" :style="tableStyle" role="table">
      <div class="ss-table__head" role="row">
        <span>{{ t('status') }}</span>
        <span>{{ t('name') }}</span>
        <span>{{ t('platform') }}</span>
        <span v-if="columns.location">{{ t('location') }}</span>
        <span v-if="columns.price">{{ t('price') }}</span>
        <span>{{ t('online') }}</span>
        <span v-if="columns.availability">{{ t('availability') }}</span>
        <span>{{ t('load') }}</span>
        <span>{{ t('connCount') }}</span>
        <span>{{ t('networkSpeed') }}</span>
        <span>{{ t('traffic') }}</span>
        <span>{{ t('cores') }}</span>
        <span>{{ t('memory') }}</span>
        <span>{{ t('disk') }}</span>
        <span v-if="columns.remaining">{{ t('remaining') }}</span>
      </div>
      <ServerRow
        v-for="row in views"
        :key="row.id"
        :server="row"
        :columns="columns"
        :show-group="!title"
        @select="emit('select', row.id)"
      />
    </div>
  </section>
</template>
