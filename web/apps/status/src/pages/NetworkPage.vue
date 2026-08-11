<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { getPublicNetwork } from '@santaizi/api'
import { AppEmpty } from '@santaizi/ui'
import { useStatusStore } from '../stores/status'

echarts.use([LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])
const { t } = useI18n()
const store = useStatusStore()
const selected = ref(0)
const node = ref<HTMLElement>()
const loading = ref(false)
const failed = ref(false)
let chart: echarts.EChartsType | undefined

async function render() {
  if (!selected.value) return
  loading.value = true
  try {
    const rows = (await getPublicNetwork(selected.value)).data
    await nextTick()
    chart?.dispose()
    chart = echarts.init(node.value!)
    chart.setOption({
      tooltip: { trigger: 'axis' },
      legend: { top: 8 },
      grid: { left: 42, right: 22, top: 55, bottom: 35 },
      xAxis: { type: 'time' },
      yAxis: { type: 'value', name: 'ms' },
      series: rows.map((row, index) => ({
        name: String(row.monitor_name || `${t('network')} ${index + 1}`),
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: (row.created_at as unknown[] || []).map((time, i) => [time, (row.avg_delay as unknown[] || [])[i]]),
      })),
    })
    failed.value = false
  } catch {
    failed.value = true
  } finally {
    loading.value = false
  }
}

watch(selected, render)
onMounted(async () => {
  await store.load()
  selected.value = store.servers[0]?.id || 0
})
onBeforeUnmount(() => chart?.dispose())
</script>

<template>
  <div class="status-container">
    <section class="status-panel network-panel">
      <header class="network-header">
        <div><h1>{{ t('statusNetwork') }}</h1></div>
        <el-select v-model="selected" style="min-width:220px">
          <el-option v-for="server in store.servers" :key="server.id" :label="server.name" :value="server.id" />
        </el-select>
      </header>
      <div v-show="selected && !failed" ref="node" class="network-chart" />
      <div v-if="failed || !store.servers.length" class="empty-status">
        <AppEmpty :icon="failed ? 'ri-error-warning-line' : 'ri-line-chart-line'" :description="t(failed ? 'loadFailed' : 'noData')" />
        <el-button v-if="failed" @click="render"><i class="ri-refresh-line"></i>{{ t('refresh') }}</el-button>
      </div>
    </section>
  </div>
</template>
