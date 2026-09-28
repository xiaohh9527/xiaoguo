<template>
  <div v-show="visible" class="danmaku-container" ref="containerRef">
    <div
      v-for="bullet in activeBullets"
      :key="bullet.uid"
      class="danmaku-item"
      :class="{ 'is-user': bullet.isUser }"
      :style="{
        top: `${bullet.top}px`,
        transform: `translateX(${bullet.x}px)`,
        color: bullet.color || '#ffffff',
      }"
    >
      {{ bullet.text }}
    </div>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'

const props = defineProps({
  visible: {
    type: Boolean,
    default: true,
  },
  items: {
    type: Array,
    default: () => [],
  },
  currentTime: {
    type: Number,
    default: 0,
  },
  isPlaying: {
    type: Boolean,
    default: true,
  },
})

const containerRef = ref(null)
const activeBullets = ref([])
const lanes = ref([0, 0, 0, 0, 0, 0, 0, 0]) // Timestamp when each lane is free
const firedMap = new Set()
let animFrameId = null
let lastTime = 0
let bulletCounter = 0

// Watch timeline to spawn bullet comments
watch(() => props.currentTime, (currSec) => {
  if (!props.visible || !props.items || props.items.length === 0) return

  const currMs = currSec * 1000
  for (const item of props.items) {
    if (!firedMap.has(item.id)) {
      if (item.timeMs >= currMs - 500 && item.timeMs <= currMs + 500) {
        firedMap.add(item.id)
        spawnBullet(item.text, false)
      }
    }
  }
})

// Clear fired history when seeking backwards significantly
watch(() => props.currentTime, (curr, prev) => {
  if (Math.abs(curr - prev) > 3) {
    firedMap.clear()
    activeBullets.value = []
  }
})

const spawnBullet = (text, isUser = false) => {
  if (!containerRef.value) return

  const containerWidth = containerRef.value.clientWidth || 800
  const containerHeight = containerRef.value.clientHeight || 450
  const laneHeight = 32
  const maxLanes = Math.max(3, Math.floor((containerHeight * 0.75) / laneHeight))

  // Pick lane with least occupation
  const now = Date.now()
  let chosenLane = 0
  let earliestLaneTime = Infinity

  for (let l = 0; l < maxLanes; l++) {
    const laneAvailable = lanes.value[l] || 0
    if (laneAvailable < now) {
      chosenLane = l
      break
    }
    if (laneAvailable < earliestLaneTime) {
      earliestLaneTime = laneAvailable
      chosenLane = l
    }
  }

  // Reserve lane for 1.8 seconds before next bullet in same lane
  lanes.value[chosenLane] = now + 1800

  const speed = isUser ? 130 : (100 + Math.random() * 40) // pixels per sec
  const top = chosenLane * laneHeight + 12

  activeBullets.value.push({
    uid: ++bulletCounter,
    text,
    top,
    x: containerWidth + 20,
    speed,
    isUser,
    color: isUser ? '#ffdd57' : '#ffffff',
  })
}

// Expose public method to send user danmaku
defineExpose({
  sendUserDanmaku: (text) => {
    spawnBullet(text, true)
  },
})

// Animation loop
const updatePhysics = (timestamp) => {
  if (!lastTime) lastTime = timestamp
  const delta = (timestamp - lastTime) / 1000
  lastTime = timestamp

  if (props.isPlaying && props.visible && activeBullets.value.length > 0) {
    const remaining = []
    for (const b of activeBullets.value) {
      b.x -= b.speed * delta
      // If still visible
      if (b.x > -400) {
        remaining.push(b)
      }
    }
    activeBullets.value = remaining
  }

  animFrameId = requestAnimationFrame(updatePhysics)
}

onMounted(() => {
  animFrameId = requestAnimationFrame(updatePhysics)
})

onUnmounted(() => {
  if (animFrameId) cancelAnimationFrame(animFrameId)
})
</script>

<style scoped>
.danmaku-container {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  z-index: 10;
}

.danmaku-item {
  position: absolute;
  left: 0;
  white-space: nowrap;
  font-size: 15px;
  font-weight: 600;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.9), 0 0 1px #000;
  letter-spacing: 0.3px;
  opacity: 0.92;
  will-change: transform;
}

.is-user {
  border: 1px solid rgba(255, 221, 87, 0.7);
  background: rgba(0, 0, 0, 0.45);
  padding: 1px 8px;
  border-radius: var(--radius-full);
}

@media (max-width: 640px) {
  .danmaku-item {
    font-size: 13px;
  }
}
</style>
