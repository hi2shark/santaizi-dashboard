<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ProbeSampleBucket } from '@santaizi/api'
import { formatAdminValue } from '@/composables/format'
import { formatProbeLoss } from '@/domain/probePath'

const props = defineProps<{
  points: ProbeSampleBucket[]
}>()

const { t, te, locale } = useI18n()
const maxMs = computed(() => Math.max(0, ...props.points.map(point => Number(point.avg_ms) || 0)))

function fillHeight(point: ProbeSampleBucket) {
  const ms = Number(point.avg_ms) || 0
  if (!maxMs.value || ms <= 0) return point.success_count ? 0 : 2
  return Math.max(2, (ms / maxMs.value) * 100)
}

function tip(point: ProbeSampleBucket) {
  const when = formatAdminValue(point.bucket_start, 'bucket_start', locale.value, t as never, te)
  const rtt = formatAdminValue(point.avg_ms, 'avg_ms', locale.value, t as never, te)
  return `${when} · ${rtt} · ${formatProbeLoss(point.loss, locale.value)}`
}
</script>

<template>
  <div class="probe-rtt-bars">
    <el-tooltip v-for="(point, index) in points" :key="`${point.bucket_start}-${point.kind}-${point.port}-${index}`" :content="tip(point)" placement="top" :show-after="200">
      <button type="button" class="probe-rtt-slot" :class="{ 'is-fail': !point.success_count && point.fail_count }" :aria-label="tip(point)">
        <span class="probe-rtt-fill" :style="{ height: `${fillHeight(point)}%` }"></span>
      </button>
    </el-tooltip>
  </div>
</template>

<style scoped>
.probe-rtt-bars {
  display: flex;
  align-items: stretch;
  gap: 3px;
  height: 88px;
}
.probe-rtt-slot {
  flex: 1 1 0;
  min-width: 0;
  display: flex;
  align-items: flex-end;
  margin: 0;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: color-mix(in srgb, var(--sz-primary) 12%, var(--sz-surface));
  cursor: default;
}
.probe-rtt-slot.is-fail {
  background: color-mix(in srgb, var(--sz-danger) 12%, var(--sz-surface));
}
.probe-rtt-fill {
  display: block;
  width: 100%;
  border-radius: 4px;
  background: var(--sz-primary);
}
.probe-rtt-slot.is-fail .probe-rtt-fill { background: var(--sz-danger); }
</style>
