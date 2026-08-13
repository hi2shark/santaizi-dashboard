<script setup lang="ts">
defineProps<{
  label: string
  percent: number
  detail?: string
}>()
</script>

<template>
  <div class="ss-metric">
    <div class="ss-metric__meta">
      <span>{{ label }}</span>
      <em v-if="detail">{{ detail }}</em>
    </div>
    <div
      class="ss-metric-bar"
      role="meter"
      :aria-valuenow="Number(percent.toFixed(1))"
      aria-valuemin="0"
      aria-valuemax="100"
      :aria-label="label"
    >
      <span class="ss-metric-bar__fill" :style="{ width: `${Math.max(0, Math.min(100, percent))}%` }"></span>
      <span class="ss-metric-bar__label">{{ percent.toFixed(1) }}%</span>
    </div>
  </div>
</template>
