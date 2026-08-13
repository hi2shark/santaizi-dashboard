<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useInjectedStatusStore } from '@santaizi/status-core'
import { resolveServerLocation } from '../utils/worldMap'
import WorldMap from '../components/home/WorldMap.vue'
import ServerNameBlock from '../components/detail/ServerNameBlock.vue'
import ServerStatusBox from '../components/detail/ServerStatusBox.vue'
import ServerInfoBox from '../components/detail/ServerInfoBox.vue'
import ServerCycleTransfer from '../components/detail/ServerCycleTransfer.vue'
import ServerMonitor from '../components/detail/ServerMonitor.vue'

const props = defineProps<{ serverId: string }>()
const route = useRoute()
const router = useRouter()
const store = useInjectedStatusStore()
const viewportWidth = ref(window.innerWidth)

const id = computed(() => Number(props.serverId || route.params.serverId))
const server = computed(() => store.servers.find((row) => row.id === id.value))
const location = computed(() => (server.value ? resolveServerLocation(server.value) : null))
const mapLocations = computed(() => {
  const value = location.value
  if (!value || typeof value.x !== 'number' || typeof value.y !== 'number') return []
  return [{
    key: value.code,
    x: value.x,
    y: value.y,
    size: 4,
    label: value.name || value.code,
    status: server.value?.online ? 'online' as const : 'offline' as const,
  }]
})
const mapWidth = computed(() => Math.max(300, Math.min(860, viewportWidth.value - 40)))

watch(server, (value) => {
  if (store.servers.length && !value) router.replace({ name: 'home' })
}, { immediate: true })

function updateViewport() {
  viewportWidth.value = window.innerWidth
}

onMounted(() => window.addEventListener('resize', updateViewport))
onBeforeUnmount(() => window.removeEventListener('resize', updateViewport))
</script>

<template>
  <div v-if="server" class="nazhua-detail" :class="{ offline: !server.online }">
    <WorldMap v-if="mapLocations.length" :locations="mapLocations" :width="mapWidth" />
    <ServerNameBlock :server="server" :location="location" />
    <ServerStatusBox :server="server" />
    <ServerCycleTransfer :server="server" />
    <ServerInfoBox :server="server" />
    <ServerMonitor :server-id="server.id" />
  </div>
</template>
