<script setup lang="ts">
withDefaults(
  defineProps<{
    description: string
    icon?: string
  }>(),
  {
    icon: 'ri-inbox-line',
  },
)
</script>

<template>
  <div class="app-empty">
    <i :class="icon" class="empty-icon" aria-hidden="true" />
    <p class="desc">{{ description }}</p>
  </div>
</template>

<style scoped>
.app-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--layout-gap);
  padding: 32px var(--space-5);
  color: var(--text-muted);
  min-height: 200px;
}
.empty-icon {
  font-size: 40px;
  line-height: 1;
  color: var(--text-muted);
  opacity: 0.7;
}
.desc {
  margin: 0;
  font-size: var(--font-size-secondary);
  line-height: var(--line-height-tight);
  text-align: center;
}
</style>
