<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ListMode, SortOrder, SortProp } from '../../composables/useServerListFilters'

const emit = defineEmits<{
  'update:tagFilter': [value: string]
  'update:onlineFilter': [value: 'all' | 'online' | 'offline']
  'update:listMode': [value: ListMode]
  'update:sortProp': [value: SortProp]
  'update:sortOrder': [value: SortOrder]
}>()

const { t } = useI18n()
const props = defineProps<{
  groups: Array<{ name: string; count: number }>
  tagFilter: string
  onlineFilter: 'all' | 'online' | 'offline'
  listMode: ListMode
  sortProp: SortProp
  sortOrder: SortOrder
  showOnlineFilter: boolean
}>()

const sortLabel = computed(() => t({
  display_index: 'nazhua.sortWeight',
  name: 'nazhua.sortName',
  online: 'nazhua.sortOnline',
}[props.sortProp]))

function toggleGroup(name: string) {
  emit('update:tagFilter', props.tagFilter === name ? '' : name)
}

function toggleOnline(value: 'online' | 'offline') {
  emit('update:onlineFilter', props.onlineFilter === value ? 'all' : value)
}
</script>

<template>
  <div class="nazhua-filter">
    <div class="nazhua-filter__groups" :aria-label="t('nazhua.group')">
      <el-button v-for="group in groups" :key="group.name" :type="tagFilter === group.name ? 'primary' : 'default'" :title="`${group.count}`" @click="toggleGroup(group.name)">
        {{ group.name }}
      </el-button>
    </div>
    <div class="nazhua-filter__tools">
      <el-dropdown trigger="click" @command="emit('update:sortProp', $event as SortProp)">
        <el-button class="nazhua-filter__sort" :aria-label="t('nazhua.sort')">
          <span>{{ sortLabel }}</span>
          <span
            class="nazhua-filter__sort-order"
            role="button"
            tabindex="0"
            :aria-label="t('nazhua.sort')"
            @click.stop="emit('update:sortOrder', sortOrder === 'asc' ? 'desc' : 'asc')"
            @keydown.enter.stop="emit('update:sortOrder', sortOrder === 'asc' ? 'desc' : 'asc')"
            @keydown.space.prevent.stop="emit('update:sortOrder', sortOrder === 'asc' ? 'desc' : 'asc')"
          ><i :class="sortOrder === 'asc' ? 'ri-arrow-up-line' : 'ri-arrow-down-line'"></i></span>
        </el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="display_index">{{ t('nazhua.sortWeight') }}</el-dropdown-item>
            <el-dropdown-item command="name">{{ t('nazhua.sortName') }}</el-dropdown-item>
            <el-dropdown-item command="online">{{ t('nazhua.sortOnline') }}</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
      <div v-if="showOnlineFilter" class="nazhua-filter__online">
        <el-button :type="onlineFilter === 'online' ? 'primary' : 'default'" @click="toggleOnline('online')">{{ t('nazhua.online') }}</el-button>
        <el-button :type="onlineFilter === 'offline' ? 'primary' : 'default'" @click="toggleOnline('offline')">{{ t('nazhua.offline') }}</el-button>
      </div>
      <el-button-group class="nazhua-filter__modes">
        <el-button :type="listMode === 'card' ? 'primary' : 'default'" :aria-label="t('nazhua.modeCard')" @click="emit('update:listMode', 'card')"><i class="ri-gallery-view-2"></i></el-button>
        <el-button :type="listMode === 'row' ? 'primary' : 'default'" :aria-label="t('nazhua.modeRow')" @click="emit('update:listMode', 'row')"><i class="ri-list-view"></i></el-button>
        <el-button :type="listMode === 'server-status' ? 'primary' : 'default'" :aria-label="t('nazhua.modeServerStatus')" @click="emit('update:listMode', 'server-status')"><i class="ri-server-line"></i></el-button>
      </el-button-group>
    </div>
  </div>
</template>
