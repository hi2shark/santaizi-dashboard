<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = withDefaults(defineProps<{
  accent?: string
}>(), {
  accent: '',
})

const canvasRef = ref<HTMLCanvasElement | null>(null)

type Particle = {
  x: number
  y: number
  vx: number
  vy: number
  r: number
}

let ctx: CanvasRenderingContext2D | null = null
let particles: Particle[] = []
let raf = 0
let width = 0
let height = 0
let dpr = 1
let running = false
let reducedMotion = false
let accentRGB = { r: 210, g: 178, b: 6 }
let lastFrame = 0

function parseAccent(raw: string): { r: number; g: number; b: number } {
  const value = (raw || getComputedStyle(document.documentElement).getPropertyValue('--status-accent') || '#d2b206').trim()
  if (value.startsWith('#')) {
    const hex = value.length === 4
      ? value.slice(1).split('').map((c) => c + c).join('')
      : value.slice(1).padEnd(6, '0').slice(0, 6)
    return {
      r: Number.parseInt(hex.slice(0, 2), 16) || 210,
      g: Number.parseInt(hex.slice(2, 4), 16) || 178,
      b: Number.parseInt(hex.slice(4, 6), 16) || 6,
    }
  }
  const match = value.match(/(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/)
  if (match) {
    return { r: Number(match[1]), g: Number(match[2]), b: Number(match[3]) }
  }
  return { r: 210, g: 178, b: 6 }
}

function countForArea() {
  const area = width * height
  const base = Math.round(area / 22000)
  return Math.min(64, Math.max(24, base))
}

function seed() {
  const n = countForArea()
  particles = Array.from({ length: n }, () => ({
    x: Math.random() * width,
    y: Math.random() * height,
    vx: (Math.random() - 0.5) * (reducedMotion ? 0 : 0.32),
    vy: (Math.random() - 0.5) * (reducedMotion ? 0 : 0.32),
    r: 1.4 + Math.random() * 1.8,
  }))
}

function resize() {
  const canvas = canvasRef.value
  if (!canvas) return
  dpr = Math.min(window.devicePixelRatio || 1, 1.5)
  width = window.innerWidth
  height = window.innerHeight
  canvas.width = Math.floor(width * dpr)
  canvas.height = Math.floor(height * dpr)
  canvas.style.width = `${width}px`
  canvas.style.height = `${height}px`
  ctx = canvas.getContext('2d', { alpha: true })
  if (ctx) ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  accentRGB = parseAccent(props.accent)
  seed()
}

function draw(now = 0) {
  if (!ctx || !running) return
  // 约 30fps，减轻主线程压力
  if (!reducedMotion && now - lastFrame < 32) {
    raf = window.requestAnimationFrame(draw)
    return
  }
  lastFrame = now

  const { r, g, b } = accentRGB
  const isDark = document.documentElement.classList.contains('dark')
    || document.documentElement.dataset.theme === 'dark'
  const alphaDot = isDark ? 0.72 : 0.48
  const alphaLine = isDark ? 0.28 : 0.18
  const linkDist = 128
  const linkDistSq = linkDist * linkDist

  ctx.clearRect(0, 0, width, height)

  for (const p of particles) {
    if (!reducedMotion) {
      p.x += p.vx
      p.y += p.vy
      if (p.x < -16) p.x = width + 16
      if (p.x > width + 16) p.x = -16
      if (p.y < -16) p.y = height + 16
      if (p.y > height + 16) p.y = -16
    }
  }

  ctx.lineWidth = 1.15
  for (let i = 0; i < particles.length; i++) {
    const a = particles[i]!
    for (let j = i + 1; j < particles.length; j++) {
      const c = particles[j]!
      const dx = a.x - c.x
      const dy = a.y - c.y
      const distSq = dx * dx + dy * dy
      if (distSq > linkDistSq) continue
      const t = 1 - Math.sqrt(distSq) / linkDist
      ctx.strokeStyle = `rgba(${r},${g},${b},${alphaLine * t})`
      ctx.beginPath()
      ctx.moveTo(a.x, a.y)
      ctx.lineTo(c.x, c.y)
      ctx.stroke()
    }
  }

  for (const p of particles) {
    ctx.beginPath()
    ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2)
    ctx.fillStyle = `rgba(${r},${g},${b},${alphaDot})`
    ctx.fill()
    ctx.beginPath()
    ctx.arc(p.x, p.y, p.r * 2.2, 0, Math.PI * 2)
    ctx.fillStyle = `rgba(${r},${g},${b},${alphaDot * 0.22})`
    ctx.fill()
  }

  if (!reducedMotion) raf = window.requestAnimationFrame(draw)
}

function start() {
  running = true
  window.cancelAnimationFrame(raf)
  lastFrame = 0
  if (reducedMotion) {
    draw(performance.now())
    return
  }
  raf = window.requestAnimationFrame(draw)
}

function stop() {
  running = false
  window.cancelAnimationFrame(raf)
}

function onVisibility() {
  if (document.hidden) stop()
  else start()
}

onMounted(() => {
  reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  resize()
  start()
  window.addEventListener('resize', resize)
  document.addEventListener('visibilitychange', onVisibility)
})

onBeforeUnmount(() => {
  stop()
  window.removeEventListener('resize', resize)
  document.removeEventListener('visibilitychange', onVisibility)
})

watch(() => props.accent, () => {
  accentRGB = parseAccent(props.accent)
  seed()
  if (reducedMotion) draw(performance.now())
})
</script>

<template>
  <canvas ref="canvasRef" class="status-particles" aria-hidden="true" />
</template>
