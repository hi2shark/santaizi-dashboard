<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { getPublicNetwork } from '@santaizi/api'
import { AppEmpty } from '@santaizi/ui'

echarts.use([LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

const props = defineProps<{ serverId: number }>()
const { t } = useI18n()
const node = ref<HTMLElement>()
const loading = ref(false)
const failed = ref(false)
const empty = ref(false)
let chart: echarts.ECharts | undefined
let requestSeq = 0

async function render() {
  if (!props.serverId) return
  const seq = ++requestSeq
  loading.value = true
  failed.value = false
  empty.value = false
  try {
    const rows = (await getPublicNetwork(props.serverId)).data
    if (seq !== requestSeq) return
    if (!rows.length) {
      empty.value = true
      chart?.dispose()
      chart = undefined
      return
    }
    loading.value = false
    await nextTick()
    if (!node.value || seq !== requestSeq) return
    chart?.dispose()
    chart = echarts.init(node.value)
    const isDark = document.documentElement.classList.contains('dark')
    chart.setOption({
      backgroundColor: 'transparent',
      tooltip: { trigger: 'axis' },
      legend: { top: 8 },
      grid: { left: 48, right: 24, top: 56, bottom: 36 },
      xAxis: { type: 'time' },
      yAxis: { type: 'value', name: 'ms' },
      series: rows.map((row, index) => ({
        name: String(row.monitor_name || `${t('nazhua.networkMonitor')} ${index + 1}`),
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: ((row.created_at as unknown[]) || []).map((time, i) => [time, ((row.avg_delay as unknown[]) || [])[i]]),
      })),
    })
  } catch {
    if (seq !== requestSeq) return
    failed.value = true
    chart?.dispose()
    chart = undefined
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

function onResize() {
  chart?.resize()
}

watch(() => props.serverId, render, { immediate: true })
onMounted(() => window.addEventListener('resize', onResize))
onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  chart?.dispose()
})
</script>

<template>
  <section class="nazhua-monitor">
    <h2>{{ t('nazhua.networkMonitor') }}</h2>
    <div v-show="!failed && !empty && !loading" ref="node" class="nazhua-monitor__chart" />
    <div v-if="failed || empty || loading" class="nazhua-monitor__empty">
      <AppEmpty
        :tone="failed ? 'danger' : 'default'"
        :icon="failed ? 'ri-error-warning-line' : loading ? 'ri-loader-4-line' : 'ri-line-chart-line'"
        :title="failed ? t('nazhua.loadFailed') : ''"
        :description="t(failed ? 'nazhua.requestFailed' : loading ? 'nazhua.loading' : 'nazhua.noData')"
      />
      <button v-if="failed" type="button" @click="render"><i class="ri-refresh-line"></i>{{ t('nazhua.refresh') }}</button>
    </div>
  </section>
</template>
