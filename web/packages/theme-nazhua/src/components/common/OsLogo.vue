<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ platform?: string }>()

const known = new Set([
  'alpine', 'aosc', 'apple', 'archlinux', 'centos', 'coreos', 'debian', 'fedora',
  'freebsd', 'gentoo', 'linuxmint', 'mageia', 'mandriva', 'manjaro', 'nixos',
  'opensuse', 'redhat', 'raspberry-pi', 'sabayon', 'slackware', 'tux', 'ubuntu',
])

const aliases: Record<string, string> = {
  darwin: 'apple',
  macos: 'apple',
  osx: 'apple',
  arch: 'archlinux',
  'arch linux': 'archlinux',
  rhel: 'redhat',
  'red hat': 'redhat',
  'red hat enterprise linux': 'redhat',
  opensuse: 'opensuse',
  'open suse': 'opensuse',
  mint: 'linuxmint',
  'linux mint': 'linuxmint',
  linux: 'tux',
  gnu: 'tux',
  'gnu/linux': 'tux',
  raspberrypi: 'raspberry-pi',
  'raspberry pi': 'raspberry-pi',
}

const className = computed(() => {
  const raw = (props.platform || '').toLowerCase().trim()
  if (!raw) return 'ri-computer-line'
  if (raw.includes('windows') || raw.includes('microsoft')) return 'ri-microsoft-fill'
  const mapped = aliases[raw] || raw.replace(/\s+/g, '')
  return known.has(mapped) ? `fl-${mapped}` : 'ri-computer-line'
})
</script>

<template>
  <span :class="className" class="nazhua-os-logo" aria-hidden="true" />
</template>
