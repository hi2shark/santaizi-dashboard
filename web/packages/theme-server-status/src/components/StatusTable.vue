<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ServerRecord } from '@santaizi/api'
import type { CycleTransferMap } from '../domain/serverStatusView'
import { toServerStatusViews } from '../domain/serverStatusView'
import ServerCard from './ServerCard.vue'

const props = defineProps<{
  title?: string
  servers: readonly ServerRecord[]
  cycles?: CycleTransferMap
  showAvailability: boolean
}>()

const { locale } = useI18n()
const views = computed(() => toServerStatusViews([...props.servers], props.cycles, Date.now(), locale.value))
</script>

<template>
  <section class="status-panel">
    <header v-if="title" class="group-title">
      <span>{{ title }}</span>
      <small>{{ servers.length }}</small>
    </header>
    <div class="ss-card-grid">
      <ServerCard
        v-for="row in views"
        :key="row.id"
        :server="row"
        :show-availability="showAvailability"
        :show-group="!title"
      />
    </div>
  </section>
</template>
