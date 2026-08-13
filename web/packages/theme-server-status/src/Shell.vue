<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useInjectedStatusStore } from '@santaizi/status-core'
import ParticleBackground from './components/ParticleBackground.vue'

defineProps<{
  publicTheme: 'server-status' | 'nazhua'
  allowThemeSwitch: boolean
  actualColorMode: string
}>()

const emit = defineEmits<{
  selectTheme: [value: 'server-status' | 'nazhua']
  selectLocale: [value: string]
  toggleColor: []
}>()

const { t } = useI18n()
const store = useInjectedStatusStore()
</script>

<template>
  <div class="server-status-shell">
    <ParticleBackground :accent="store.bootstrap?.primary_color || ''" />
    <a href="#status-main" class="skip-link">{{ t('skipContent') }}</a>
    <header class="status-nav">
      <RouterLink to="/" class="status-brand">
        <img :src="store.bootstrap?.logo_url || '/static/logo.svg'" alt="">
        <span>{{ store.bootstrap?.brand || t('appName') }}</span>
      </RouterLink>
      <nav :aria-label="t('statusNavigation')">
        <RouterLink to="/"><i class="ri-server-line"></i><span>{{ t('statusHome') }}</span></RouterLink>
        <RouterLink to="/service"><i class="ri-heart-pulse-line"></i><span>{{ t('statusServices') }}</span></RouterLink>
        <RouterLink to="/network"><i class="ri-line-chart-line"></i><span>{{ t('statusNetwork') }}</span></RouterLink>
      </nav>
      <div class="status-actions">
        <el-dropdown v-if="allowThemeSwitch" trigger="click" @command="emit('selectTheme', $event)">
          <button type="button" :aria-label="t('publicTheme')"><i class="ri-layout-masonry-line"></i></button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="server-status" :disabled="publicTheme === 'server-status'">{{ t('themeServerStatus') }}</el-dropdown-item>
              <el-dropdown-item command="nazhua" :disabled="publicTheme === 'nazhua'">{{ t('themeNazhua') }}</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-dropdown trigger="click" @command="emit('selectLocale', $event)">
          <button type="button" :aria-label="t('language')"><i class="ri-translate-2"></i></button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="zh-CN">简体中文</el-dropdown-item>
              <el-dropdown-item command="zh-TW">繁體中文</el-dropdown-item>
              <el-dropdown-item command="en-US">English</el-dropdown-item>
              <el-dropdown-item command="es-ES">Español</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <button type="button" :aria-label="t(actualColorMode === 'dark' ? 'light' : 'dark')" @click="emit('toggleColor')">
          <i :class="actualColorMode === 'dark' ? 'ri-sun-line' : 'ri-moon-line'"></i>
        </button>
        <a v-if="store.bootstrap?.authenticated" href="/admin/" :aria-label="t('adminPanel')"><i class="ri-settings-3-line"></i></a>
      </div>
    </header>
    <main id="status-main"><slot /></main>
    <footer>{{ store.bootstrap?.footer_text || t('appName') }}</footer>
  </div>
</template>
