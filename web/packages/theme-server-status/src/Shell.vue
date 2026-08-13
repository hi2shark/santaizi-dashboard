<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useInjectedStatusStore } from '@santaizi/status-core'

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
const brand = computed(() => store.bootstrap?.brand?.trim() || t('appName'))
const version = computed(() => store.bootstrap?.version?.trim() || '')
const footerText = computed(() => store.bootstrap?.footer_text?.trim() || '')
const poweredLine = computed(() => {
  const line = t('poweredBy', { name: brand.value })
  return version.value ? `${line} ${version.value}` : line
})
</script>

<template>
  <div class="server-status-shell">
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
          <button type="button" class="status-icon-btn" :aria-label="t('publicTheme')"><i class="ri-layout-masonry-line"></i></button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="server-status" :disabled="publicTheme === 'server-status'">{{ t('themeServerStatus') }}</el-dropdown-item>
              <el-dropdown-item command="nazhua" :disabled="publicTheme === 'nazhua'">{{ t('themeNazhua') }}</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-dropdown trigger="click" @command="emit('selectLocale', $event)">
          <button type="button" class="status-icon-btn" :aria-label="t('language')"><i class="ri-translate-2"></i></button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="zh-CN">简体中文</el-dropdown-item>
              <el-dropdown-item command="zh-TW">繁體中文</el-dropdown-item>
              <el-dropdown-item command="en-US">English</el-dropdown-item>
              <el-dropdown-item command="es-ES">Español</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <button type="button" class="status-icon-btn" :aria-label="t(actualColorMode === 'dark' ? 'light' : 'dark')" @click="emit('toggleColor')">
          <i :class="actualColorMode === 'dark' ? 'ri-sun-line' : 'ri-moon-line'"></i>
        </button>
        <a v-if="store.bootstrap?.authenticated" class="status-icon-btn" href="/admin/" :aria-label="t('adminPanel')"><i class="ri-settings-3-line"></i></a>
      </div>
    </header>
    <main id="status-main"><slot /></main>
    <footer>
      <span v-if="footerText">{{ footerText }}</span>
      <span>{{ poweredLine }}</span>
      <a href="https://github.com/naiba/nezha" target="_blank" rel="noopener noreferrer">{{ t('upstreamCredit') }}</a>
    </footer>
  </div>
</template>
