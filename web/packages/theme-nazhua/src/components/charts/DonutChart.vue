<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  label: string
  percent: number
  value?: string
  color?: 'blue' | 'green' | 'cyan'
}>(), {
  value: '',
  color: 'blue',
})

const safePercent = computed(() => Math.max(0, Math.min(100, Number(props.percent) || 0)))
const dashOffset = computed(() => 100 - safePercent.value)
const displayValue = computed(() => props.value || `${safePercent.value.toFixed(safePercent.value < 10 && safePercent.value % 1 ? 1 : 0)}%`)
</script>

<template>
  <div class="nazhua-donut" :class="`nazhua-donut--${color}`">
    <svg viewBox="0 0 44 44" aria-hidden="true">
      <circle class="nazhua-donut__track" cx="22" cy="22" r="18" pathLength="100" />
      <circle class="nazhua-donut__value" cx="22" cy="22" r="18" pathLength="100" :stroke-dashoffset="dashOffset" />
    </svg>
    <div class="nazhua-donut__text"><strong>{{ displayValue }}</strong><span>{{ label }}</span></div>
  </div>
</template>
