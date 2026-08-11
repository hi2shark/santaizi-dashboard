<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useStatusStore } from './stores/status'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const store = useStatusStore()
const theme = ref(localStorage.getItem('santaizi-status-theme') || 'system')
const transparent = ref(localStorage.getItem('santaizi-status-transparent') === '1')
const actualTheme = computed(() => theme.value === 'system' ? (matchMedia('(prefers-color-scheme:dark)').matches ? 'dark' : 'light') : theme.value)

function toggleTheme() {
  theme.value = actualTheme.value === 'dark' ? 'light' : 'dark'
  localStorage.setItem('santaizi-status-theme', theme.value)
}
function toggleTransparent() {
  transparent.value = !transparent.value
  localStorage.setItem('santaizi-status-transparent', transparent.value ? '1' : '0')
}
function setLocale(value: string) {
  locale.value = value
  localStorage.setItem('santaizi-locale', value)
}

watch(actualTheme, (value) => {
  document.documentElement.dataset.theme = value
  document.documentElement.classList.toggle('dark', value === 'dark')
  document.documentElement.style.colorScheme = value === 'dark' ? 'dark' : 'light'
}, { immediate: true })

watch(() => store.bootstrap, (value) => {
  if (!value) return
  if (value.primary_color) document.documentElement.style.setProperty('--status-accent', value.primary_color)
  let style = document.querySelector<HTMLStyleElement>('#santaizi-site-style')
  if (!style) {
    style = document.createElement('style')
    style.id = 'santaizi-site-style'
    document.head.append(style)
  }
  style.textContent = value.custom_css || ''
}, { deep: true })

onMounted(async () => {
  await store.load()
  if (store.bootstrap?.requires_view_password && !store.bootstrap.view_password_verified && route.path !== '/view-password') {
    await router.replace('/view-password')
  }
})
</script>

<template>
  <div class="status-app" :class="{ transparent }" :style="store.bootstrap?.background_url ? { backgroundImage: `linear-gradient(var(--status-overlay),var(--status-overlay)),url(${store.bootstrap.background_url})` } : undefined">
    <a href="#status-main" class="skip-link">{{ t('skipContent') }}</a>
    <header class="status-nav">
      <a href="/" class="status-brand"><img :src="store.bootstrap?.logo_url || '/static/logo.svg'" alt=""><span>{{ store.bootstrap?.brand || t('appName') }}</span></a>
      <nav :aria-label="t('statusNavigation')">
        <RouterLink to="/"><i class="ri-server-line"></i><span>{{ t('statusHome') }}</span></RouterLink>
        <RouterLink to="/service"><i class="ri-heart-pulse-line"></i><span>{{ t('statusServices') }}</span></RouterLink>
        <RouterLink to="/network"><i class="ri-line-chart-line"></i><span>{{ t('statusNetwork') }}</span></RouterLink>
      </nav>
      <div class="status-actions">
        <select :value="locale" :aria-label="t('language')" @change="setLocale(($event.target as HTMLSelectElement).value)">
          <option value="zh-CN">简中</option>
          <option value="zh-TW">繁中</option>
          <option value="en-US">EN</option>
          <option value="es-ES">ES</option>
        </select>
        <button type="button" :aria-label="t('transparent')" @click="toggleTransparent"><i class="ri-contrast-drop-2-line"></i></button>
        <button type="button" :aria-label="t('light')" @click="toggleTheme"><i :class="actualTheme === 'dark' ? 'ri-sun-line' : 'ri-moon-line'"></i></button>
        <a v-if="store.bootstrap?.authenticated" href="/admin/"><i class="ri-settings-3-line"></i></a>
      </div>
    </header>
    <main id="status-main"><RouterView /></main>
    <footer>{{ store.bootstrap?.footer_text || t('appName') }}</footer>
  </div>
</template>
