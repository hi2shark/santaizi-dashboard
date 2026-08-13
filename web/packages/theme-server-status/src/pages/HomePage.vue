<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts/core'
import { MapChart } from 'echarts/charts'
import { TooltipComponent, VisualMapComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { AppEmpty } from '@santaizi/ui'
import { useInjectedStatusStore } from '@santaizi/status-core'
import StatusTable from '../components/StatusTable.vue'

echarts.use([MapChart, TooltipComponent, VisualMapComponent, CanvasRenderer])

const { t } = useI18n()
const store = useInjectedStatusStore()
const grouped = ref(localStorage.getItem('santaizi-status-grouped') !== '0')
const mapOpen = ref(false)
const mapNode = ref<HTMLElement>()
let chart: echarts.ECharts | undefined

const all = computed(() => store.servers)
const connectionClass = computed(() => {
  if (store.loadError) return 'failed'
  if (store.connected) return 'connected'
  return ''
})
const connectionLabel = computed(() => {
  if (store.loadError) return t('loadFailed')
  if (store.connected) return t('liveConnected')
  return t('reconnecting')
})
const emptyDescription = computed(() => {
  if (store.loadError) return t('requestFailed')
  if (store.loading) return t('loading')
  return t('noData')
})

function toggle() {
  grouped.value = !grouped.value
  localStorage.setItem('santaizi-status-grouped', grouped.value ? '1' : '0')
}

async function showMap() {
  mapOpen.value = true
  setTimeout(async () => {
    const geo = await fetch('/static/theme-server-status/maps/santaizi.world.geo.json').then((v) => v.json())
    echarts.registerMap('santaizi-world', geo)
    const counts = new Map<string, number>()
    for (const row of store.servers) {
      const code = String(row.host?.CountryCode || row.host?.country_code || '').toUpperCase()
      if (code) counts.set(code, (counts.get(code) || 0) + 1)
    }
    chart = echarts.init(mapNode.value!)
    chart.setOption({
      tooltip: { trigger: 'item' },
      visualMap: {
        min: 0,
        max: Math.max(1, ...counts.values()),
        left: 20,
        bottom: 20,
        inRange: { color: ['#dbeafe', '#2563eb'] },
      },
      series: [{
        type: 'map',
        map: 'santaizi-world',
        roam: true,
        data: [...counts].map(([name, value]) => ({ name, value })),
      }],
    })
  }, 50)
}

function closeMap() {
  chart?.dispose()
  mapOpen.value = false
}

</script>

<template>
  <div class="status-container">
    <div class="status-toolbar">
      <span :class="['connection-state', connectionClass]">
        <i></i>{{ connectionLabel }}
      </span>
      <span />
      <button v-if="store.loadError" type="button" @click="store.load">
        <i class="ri-refresh-line"></i>{{ t('refresh') }}
      </button>
      <button type="button" @click="showMap">
        <i class="ri-earth-line"></i>{{ t('worldMap') }}
      </button>
      <button type="button" @click="toggle">
        <i :class="grouped ? 'ri-list-check-2' : 'ri-folder-chart-line'"></i>
        {{ t(grouped ? 'flatView' : 'groupView') }}
      </button>
    </div>

    <template v-if="store.servers.length">
      <template v-if="grouped">
        <StatusTable
          v-for="group in store.groups"
          :key="group.name"
          :title="group.name"
          :servers="group.items"
        />
      </template>
      <StatusTable v-else :servers="all" />
    </template>
    <div v-else class="empty-status status-page-empty">
      <AppEmpty
        :tone="store.loadError ? 'danger' : 'default'"
        :icon="store.loadError ? 'ri-error-warning-line' : 'ri-server-line'"
        :title="store.loadError ? t('loadFailed') : ''"
        :description="emptyDescription"
      />
      <el-button v-if="store.loadError" type="primary" @click="store.load">
        <i class="ri-refresh-line"></i>{{ t('refresh') }}
      </el-button>
    </div>

    <dialog :open="mapOpen" class="map-dialog">
      <header>
        <h2><i class="ri-earth-line"></i>{{ t('worldMap') }}</h2>
        <button type="button" @click="closeMap"><i class="ri-close-line"></i></button>
      </header>
      <div ref="mapNode" class="world-map" />
    </dialog>
  </div>
</template>
