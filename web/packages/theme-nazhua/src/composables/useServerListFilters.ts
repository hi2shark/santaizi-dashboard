import { computed, ref, toValue, type MaybeRefOrGetter } from 'vue'
import type { ServerRecord } from '@santaizi/api'
import { resolveServerLocation, count2size } from '../utils/worldMap'

export type ListMode = 'card' | 'row' | 'server-status'
export type SortProp = 'display_index' | 'name' | 'online'
export type SortOrder = 'asc' | 'desc'

export interface ServerListQuery {
  tag: string
  online: 'all' | 'online' | 'offline'
  search: string
  sort: SortProp
  order: SortOrder
}

export function filterAndSortServers(source: ServerRecord[], query: ServerListQuery) {
  const q = query.search.trim().toLowerCase()
  const list = source.filter((server) => {
    if (query.tag && (server.tag || 'default') !== query.tag) return false
    if (query.online === 'online' && !server.online) return false
    if (query.online === 'offline' && server.online) return false
    if (q) {
      const hay = [
        server.name,
        server.tag,
        String(server.host?.Platform || server.host?.platform || ''),
        String(server.host?.CountryCode || server.host?.country_code || ''),
      ].join(' ').toLowerCase()
      if (!hay.includes(q)) return false
    }
    return true
  })
  const direction = query.order === 'asc' ? 1 : -1
  return [...list].sort((a, b) => {
    if (query.sort === 'name') return a.name.localeCompare(b.name) * direction
    if (query.sort === 'online') return (Number(a.online) - Number(b.online)) * direction
    return (a.display_index - b.display_index) * direction
  })
}

export function useServerListFilters(servers: MaybeRefOrGetter<ServerRecord[]>) {
  const listMode = ref<ListMode>((localStorage.getItem('santaizi-nazhua-list-mode') as ListMode) || 'card')
  const tagFilter = ref('')
  const onlineFilter = ref<'all' | 'online' | 'offline'>('all')
  const searchWord = ref('')
  const sortProp = ref<SortProp>('display_index')
  const sortOrder = ref<SortOrder>('desc')

  const groups = computed(() => {
    const map = new Map<string, ServerRecord[]>()
    for (const server of toValue(servers)) {
      const name = server.tag || 'default'
      map.set(name, [...(map.get(name) || []), server])
    }
    return [...map.entries()].map(([name, items]) => ({ name, count: items.length }))
  })

  const serverCount = computed(() => {
    const list = toValue(servers)
    return {
      total: list.length,
      online: list.filter((s) => s.online).length,
      offline: list.filter((s) => !s.online).length,
    }
  })

  const filteredServers = computed(() => {
    return filterAndSortServers(toValue(servers), {
      tag: tagFilter.value,
      online: onlineFilter.value,
      search: searchWord.value,
      sort: sortProp.value,
      order: sortOrder.value,
    })
  })

  const mapLocations = computed(() => {
    const locations: Array<{ key: string; x: number; y: number; size: number; label: string; status: 'online' | 'offline' | 'mixed' }> = []
    const buckets = new Map<string, { x: number; y: number; count: number; labels: string[]; online: number; offline: number }>()
    for (const server of filteredServers.value) {
      const loc = resolveServerLocation(server)
      if (!loc || typeof loc.x !== 'number' || typeof loc.y !== 'number') continue
      const bucket = buckets.get(loc.code) || { x: loc.x, y: loc.y, count: 0, labels: [], online: 0, offline: 0 }
      bucket.count += 1
      bucket.labels.push(server.name)
      if (server.online) bucket.online += 1
      else bucket.offline += 1
      buckets.set(loc.code, bucket)
    }
    buckets.forEach((bucket, key) => {
      locations.push({
        key,
        x: bucket.x,
        y: bucket.y,
        size: count2size(bucket.count),
        label: bucket.labels.join('\n'),
        status: bucket.offline === 0 ? 'online' : bucket.online === 0 ? 'offline' : 'mixed',
      })
    })
    return locations
  })

  function setListMode(mode: ListMode) {
    listMode.value = mode
    localStorage.setItem('santaizi-nazhua-list-mode', mode)
  }

  return {
    listMode,
    tagFilter,
    onlineFilter,
    searchWord,
    sortProp,
    sortOrder,
    groups,
    serverCount,
    filteredServers,
    mapLocations,
    setListMode,
  }
}

export function useNavbarStats(servers: MaybeRefOrGetter<ServerRecord[]>) {
  return computed(() => {
    let transferIn = 0
    let transferOut = 0
    let speedIn = 0
    let speedOut = 0
    for (const server of toValue(servers)) {
      if (!server.online || !server.state) continue
      transferIn += Number(server.state.NetInTransfer || server.state.net_in_transfer || 0)
      transferOut += Number(server.state.NetOutTransfer || server.state.net_out_transfer || 0)
      speedIn += Number(server.state.NetInSpeed || server.state.net_in_speed || 0)
      speedOut += Number(server.state.NetOutSpeed || server.state.net_out_speed || 0)
    }
    return { transferIn, transferOut, speedIn, speedOut }
  })
}
