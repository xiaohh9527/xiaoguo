<template>
  <div
    class="player-wrapper"
    ref="playerWrapperRef"
    :class="{ 'is-fullscreen': isFullscreen, 'hide-controls': hideControls && isPlaying }"
    @mousemove="onMouseMove"
    @mouseleave="onMouseLeave"
  >
    <!-- Video Element -->
    <video
      ref="videoRef"
      class="video-element"
      :src="streamUrl"
      playsinline
      webkit-playsinline
      x5-playsinline
      crossorigin="anonymous"
      @click="togglePlay"
      @play="onPlay"
      @pause="onPause"
      @timeupdate="onTimeUpdate"
      @loadedmetadata="onLoadedMetadata"
      @progress="onProgress"
      @ended="onEnded"
      @waiting="isBuffering = true"
      @playing="isBuffering = false"
      @error="onError"
    ></video>

    <!-- Danmaku Overlay -->
    <DanmakuOverlay
      ref="danmakuRef"
      :visible="danmakuEnabled"
      :items="danmakuItems"
      :current-time="currentTime"
      :is-playing="isPlaying"
    />

    <!-- Buffering Spinner -->
    <div v-if="isBuffering" class="loading-overlay">
      <div class="spinner"></div>
      <span>视频缓冲中...</span>
    </div>

    <!-- Center Play/Pause Indicator on Click -->
    <transition name="pop">
      <div v-if="showCenterIcon" class="center-play-badge" @click="togglePlay">
        <Play v-if="!isPlaying" :size="38" fill="currentColor" />
        <Pause v-else :size="38" fill="currentColor" />
      </div>
    </transition>

    <!-- Top Overlay: Title & Quality Badge -->
    <div class="top-bar">
      <div class="top-title">
        <span class="series-name">{{ seriesTitle }}</span>
        <span class="episode-name">{{ currentEpisodeTitle }}</span>
      </div>
    </div>

    <!-- Bottom Controls Bar -->
    <div class="controls-bar" @click.stop>
      <!-- Progress Bar -->
      <div
        class="progress-container"
        ref="progressContainerRef"
        @mousedown="startSeek"
        @mousemove="onProgressHover"
        @mouseleave="hoverProgress = null"
      >
        <div class="progress-bg">
          <div class="progress-buffered" :style="{ width: `${bufferedPercent}%` }"></div>
          <div class="progress-played" :style="{ width: `${playedPercent}%` }">
            <div class="scrubber-handle"></div>
          </div>
        </div>
        <!-- Time tooltip on hover -->
        <div
          v-if="hoverProgress !== null"
          class="time-tooltip"
          :style="{ left: `${hoverProgress.percent}%` }"
        >
          {{ formatTime(hoverProgress.seconds) }}
        </div>
      </div>

      <!-- Controls Row -->
      <div class="controls-row">
        <!-- Left: Play/Prev/Next/Time/Volume -->
        <div class="controls-left">
          <button class="ctrl-btn" @click="togglePlay" :title="isPlaying ? '暂停 (空格)' : '播放 (空格)'">
            <Pause v-if="isPlaying" :size="20" />
            <Play v-else :size="20" />
          </button>

          <button
            class="ctrl-btn"
            :disabled="!hasPrev"
            @click="$emit('prev')"
            title="上一集"
          >
            <SkipBack :size="18" />
          </button>

          <button
            class="ctrl-btn"
            :disabled="!hasNext"
            @click="$emit('next')"
            title="下一集"
          >
            <SkipForward :size="18" />
          </button>

          <!-- Time Display -->
          <div class="time-display">
            <span>{{ formatTime(currentTime) }}</span>
            <span class="time-divider">/</span>
            <span>{{ formatTime(duration) }}</span>
          </div>

          <!-- Volume -->
          <div class="volume-group" @mouseenter="showVolumeSlider = true" @mouseleave="showVolumeSlider = false">
            <button class="ctrl-btn" @click="toggleMute" title="静音 (M)">
              <VolumeX v-if="isMuted || volume === 0" :size="19" />
              <Volume1 v-else-if="volume < 0.5" :size="19" />
              <Volume2 v-else :size="19" />
            </button>
            <div class="volume-slider-wrapper" :class="{ 'volume-open': showVolumeSlider }">
              <input
                type="range"
                min="0"
                max="1"
                step="0.05"
                v-model.number="volume"
                @input="onVolumeChange"
                class="volume-slider"
              />
            </div>
          </div>
        </div>

        <!-- Center: Danmaku Input -->
        <div class="controls-center">
          <div class="danmaku-input-box">
            <input
              type="text"
              v-model="danmakuText"
              placeholder="发条弹幕互动一下吧~"
              @keydown.enter="sendDanmaku"
              @focus="onDanmakuFocus"
              @blur="onDanmakuBlur"
            />
            <button class="send-btn" @click="sendDanmaku">
              发送
            </button>
          </div>
        </div>

        <!-- Right: Danmaku Toggle, Speed, Quality, Download, PiP, Fullscreen -->
        <div class="controls-right">
          <!-- Danmaku Toggle -->
          <button
            class="ctrl-btn"
            :class="{ 'ctrl-active': danmakuEnabled }"
            @click="danmakuEnabled = !danmakuEnabled"
            :title="danmakuEnabled ? '关闭弹幕' : '开启弹幕'"
          >
            <MessageSquare :size="18" />
            <span class="btn-label-sm">弹</span>
          </button>

          <!-- Speed Selector Popover -->
          <div class="dropdown-wrapper">
            <button class="menu-btn" @click.stop="toggleMenu('speed')">
              {{ playbackRate === 1 ? '倍速' : `${playbackRate}x` }}
            </button>
            <div v-if="openMenu === 'speed'" class="popover-menu glass-panel">
              <div
                v-for="rate in [0.75, 1.0, 1.25, 1.5, 2.0]"
                :key="rate"
                class="menu-item"
                :class="{ 'menu-active': playbackRate === rate }"
                @click="setRate(rate)"
              >
                {{ rate === 1 ? '1.0x 正常' : `${rate}x` }}
              </div>
            </div>
          </div>

          <!-- Quality Selector Popover -->
          <div class="dropdown-wrapper" v-if="variants && variants.length > 0">
            <button class="menu-btn" @click.stop="toggleMenu('quality')">
              {{ currentQualityLabel }}
            </button>
            <div v-if="openMenu === 'quality'" class="popover-menu glass-panel">
              <div
                v-for="v in uniqueQualityVariants"
                :key="v.quality"
                class="menu-item"
                :class="{ 'menu-active': currentQuality === v.quality }"
                @click="setQuality(v.quality)"
              >
                {{ getQualityText(v.quality) }}
              </div>
            </div>
          </div>

          <!-- Download Button -->
          <a
            :href="downloadUrl"
            download
            class="ctrl-btn"
            title="下载本集 (无加密标准 MP4)"
          >
            <Download :size="18" />
          </a>

          <!-- Fullscreen Button -->
          <button class="ctrl-btn" @click="toggleFullscreen" title="全屏 (F)">
            <Minimize v-if="isFullscreen" :size="19" />
            <Maximize v-else :size="19" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import {
  Play, Pause, SkipBack, SkipForward, VolumeX, Volume1, Volume2,
  Maximize, Minimize, MessageSquare, Download
} from 'lucide-vue-next'
import DanmakuOverlay from './DanmakuOverlay.vue'
import { api } from '@/api'

const props = defineProps({
  seriesId: { type: String, required: true },
  videoId: { type: String, required: true },
  seriesTitle: { type: String, default: '' },
  currentEpisodeTitle: { type: String, default: '' },
  episodeIndex: { type: Number, default: 1 },
  totalEpisodes: { type: Number, default: 1 },
  variants: { type: Array, default: () => [] },
  danmakuItems: { type: Array, default: () => [] },
  initialTime: { type: Number, default: 0 },
})

const emit = defineEmits(['prev', 'next', 'timeupdate', 'ended'])

const videoRef = ref(null)
const playerWrapperRef = ref(null)
const danmakuRef = ref(null)
const progressContainerRef = ref(null)

const isPlaying = ref(false)
const isBuffering = ref(false)
const isFullscreen = ref(false)
const showCenterIcon = ref(false)
const hideControls = ref(false)
const currentTime = ref(0)
const duration = ref(0)
const buffered = ref(0)
const volume = ref(1)
const isMuted = ref(false)
const playbackRate = ref(1.0)
const currentQuality = ref(0)
const danmakuEnabled = ref(true)
const danmakuText = ref('')
const openMenu = ref(null)
const showVolumeSlider = ref(false)
const hoverProgress = ref(null)
const isSeeking = ref(false)
let hideTimer = null
let centerIconTimer = null

const hasPrev = computed(() => props.episodeIndex > 1)
const hasNext = computed(() => props.episodeIndex < props.totalEpisodes)

const uniqueQualityVariants = computed(() => {
  const seen = new Set()
  const list = []
  for (const v of props.variants) {
    if (!seen.has(v.quality) && v.quality > 0) {
      seen.add(v.quality)
      list.push(v)
    }
  }
  return list.sort((a, b) => b.quality - a.quality)
})

const currentQualityLabel = computed(() => {
  if (!currentQuality.value) return '高清'
  return getQualityText(currentQuality.value)
})

const getQualityText = (q) => {
  if (q >= 1080) return '1080P 超清'
  if (q >= 720) return '720P 高清'
  if (q >= 540) return '540P 标清'
  if (q >= 480) return '480P 流畅'
  return `${q}P`
}

const streamUrl = computed(() => {
  if (!props.seriesId || !props.videoId) return ''
  return api.getStreamUrl(props.seriesId, props.videoId, currentQuality.value, false)
})

const downloadUrl = computed(() => {
  if (!props.seriesId || !props.videoId) return ''
  return api.getStreamUrl(props.seriesId, props.videoId, currentQuality.value, true)
})

const playedPercent = computed(() => {
  if (!duration.value) return 0
  return (currentTime.value / duration.value) * 100
})

const bufferedPercent = computed(() => {
  if (!duration.value) return 0
  return (buffered.value / duration.value) * 100
})

// Initialize quality
watch(() => props.variants, (newVars) => {
  if (newVars && newVars.length > 0 && !currentQuality.value) {
    currentQuality.value = newVars[0].quality || 1080
  }
}, { immediate: true })

// Reset on episode change
watch(() => props.videoId, () => {
  currentTime.value = 0
  buffered.value = 0
  if (videoRef.value) {
    videoRef.value.currentTime = 0
    videoRef.value.play().catch(() => {})
  }
})

const togglePlay = () => {
  if (!videoRef.value) return
  if (videoRef.value.paused) {
    videoRef.value.play().catch(() => {})
  } else {
    videoRef.value.pause()
  }
  triggerCenterIcon()
}

const triggerCenterIcon = () => {
  showCenterIcon.value = true
  clearTimeout(centerIconTimer)
  centerIconTimer = setTimeout(() => {
    showCenterIcon.value = false
  }, 500)
}

const onPlay = () => {
  isPlaying.value = true
  isBuffering.value = false
}

const onPause = () => {
  isPlaying.value = false
}

const onTimeUpdate = () => {
  if (!videoRef.value || isSeeking.value) return
  currentTime.value = videoRef.value.currentTime
  emit('timeupdate', currentTime.value, duration.value)
}

const onLoadedMetadata = () => {
  if (!videoRef.value) return
  duration.value = videoRef.value.duration || 0
  if (props.initialTime > 0 && props.initialTime < duration.value) {
    videoRef.value.currentTime = props.initialTime
  }
  videoRef.value.play().catch(() => {
    isPlaying.value = false
  })
}

const onProgress = () => {
  if (!videoRef.value) return
  const buf = videoRef.value.buffered
  if (buf.length > 0) {
    buffered.value = buf.end(buf.length - 1)
  }
}

const onEnded = () => {
  emit('ended')
  if (hasNext.value) {
    emit('next')
  }
}

const onError = (e) => {
  console.warn('Video load error, attempting recovery...', e)
  isBuffering.value = false
}

// Seeking
const startSeek = (e) => {
  isSeeking.value = true
  seekToPosition(e)

  const onMouseMove = (ev) => seekToPosition(ev)
  const onMouseUp = () => {
    isSeeking.value = false
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
  }
  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}

const seekToPosition = (e) => {
  if (!progressContainerRef.value || !duration.value || !videoRef.value) return
  const rect = progressContainerRef.value.getBoundingClientRect()
  const pos = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
  const target = pos * duration.value
  currentTime.value = target
  videoRef.value.currentTime = target
}

const onProgressHover = (e) => {
  if (!progressContainerRef.value || !duration.value) return
  const rect = progressContainerRef.value.getBoundingClientRect()
  const pos = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
  hoverProgress.value = {
    percent: pos * 100,
    seconds: pos * duration.value,
  }
}

// Volume
const toggleMute = () => {
  if (!videoRef.value) return
  isMuted.value = !isMuted.value
  videoRef.value.muted = isMuted.value
}

const onVolumeChange = () => {
  if (!videoRef.value) return
  videoRef.value.volume = volume.value
  isMuted.value = volume.value === 0
}

// Speed & Quality
const toggleMenu = (name) => {
  openMenu.value = openMenu.value === name ? null : name
}

const setRate = (rate) => {
  playbackRate.value = rate
  if (videoRef.value) videoRef.value.playbackRate = rate
  openMenu.value = null
}

const setQuality = (q) => {
  const currentPos = currentTime.value
  currentQuality.value = q
  openMenu.value = null
  // Re-load with maintained position
  setTimeout(() => {
    if (videoRef.value) {
      videoRef.value.currentTime = currentPos
      videoRef.value.play().catch(() => {})
    }
  }, 100)
}

// Fullscreen
const toggleFullscreen = () => {
  if (!playerWrapperRef.value) return
  if (!document.fullscreenElement) {
    playerWrapperRef.value.requestFullscreen().catch(() => {})
  } else {
    document.exitFullscreen().catch(() => {})
  }
}

const onFullscreenChange = () => {
  isFullscreen.value = !!document.fullscreenElement
}

// Danmaku
const sendDanmaku = () => {
  const t = danmakuText.value.trim()
  if (!t) return
  if (danmakuRef.value) {
    danmakuRef.value.sendUserDanmaku(t)
  }
  danmakuText.value = ''
}

// Auto-hide controls
const onMouseMove = () => {
  hideControls.value = false
  clearTimeout(hideTimer)
  hideTimer = setTimeout(() => {
    if (isPlaying.value && !openMenu.value) {
      hideControls.value = true
    }
  }, 3500)
}

const onMouseLeave = () => {
  if (isPlaying.value && !openMenu.value) {
    hideControls.value = true
  }
}

const formatTime = (seconds) => {
  if (isNaN(seconds) || seconds < 0) return '00:00'
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
}

// Keyboard shortcuts
const onKeyDown = (e) => {
  if (['INPUT', 'TEXTAREA'].includes(document.activeElement?.tagName)) return
  switch (e.code) {
    case 'Space':
      e.preventDefault()
      togglePlay()
      break
    case 'ArrowLeft':
      e.preventDefault()
      if (videoRef.value) videoRef.value.currentTime = Math.max(0, videoRef.value.currentTime - 5)
      break
    case 'ArrowRight':
      e.preventDefault()
      if (videoRef.value) videoRef.value.currentTime = Math.min(duration.value, videoRef.value.currentTime + 5)
      break
    case 'ArrowUp':
      e.preventDefault()
      volume.value = Math.min(1, volume.value + 0.1)
      onVolumeChange()
      break
    case 'ArrowDown':
      e.preventDefault()
      volume.value = Math.max(0, volume.value - 0.1)
      onVolumeChange()
      break
    case 'KeyF':
      e.preventDefault()
      toggleFullscreen()
      break
    case 'KeyM':
      e.preventDefault()
      toggleMute()
      break
  }
}

onMounted(() => {
  document.addEventListener('fullscreenchange', onFullscreenChange)
  window.addEventListener('keydown', onKeyDown)
  document.addEventListener('click', () => { openMenu.value = null })
})

onUnmounted(() => {
  document.removeEventListener('fullscreenchange', onFullscreenChange)
  window.removeEventListener('keydown', onKeyDown)
  clearTimeout(hideTimer)
  clearTimeout(centerIconTimer)
})
</script>

<style scoped>
.player-wrapper {
  position: relative;
  width: 100%;
  aspect-ratio: 16 / 9;
  background: #000000;
  border-radius: var(--radius-md);
  overflow: hidden;
  box-shadow: var(--shadow-lg);
  user-select: none;
}

@media (max-width: 900px) {
  .player-wrapper {
    aspect-ratio: 9 / 16;
    max-height: 80vh;
    border-radius: 0;
  }
}

.player-wrapper.is-fullscreen {
  width: 100vw !important;
  height: 100vh !important;
  aspect-ratio: auto !important;
  border-radius: 0 !important;
}

.video-element {
  width: 100%;
  height: 100%;
  object-fit: contain;
  background: #000;
  display: block;
}

/* Loading Spinner */
.loading-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: white;
  font-size: 13px;
  font-weight: 500;
  z-index: 20;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 3px solid rgba(255, 255, 255, 0.2);
  border-top-color: var(--accent-primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* Center Play Indicator */
.center-play-badge {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 72px;
  height: 72px;
  border-radius: 50%;
  background: rgba(22, 28, 42, 0.85);
  backdrop-filter: blur(10px);
  border: 1px solid rgba(255, 255, 255, 0.2);
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
  z-index: 15;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.6);
}

.pop-enter-active, .pop-leave-active {
  transition: all 0.25s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}
.pop-enter-from, .pop-leave-to {
  opacity: 0;
  transform: translate(-50%, -50%) scale(0.6);
}

/* Top Bar */
.top-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  padding: 16px 20px 32px;
  background: linear-gradient(to bottom, rgba(0, 0, 0, 0.8) 0%, transparent 100%);
  z-index: 25;
  transition: opacity 0.3s ease;
  pointer-events: none;
}

.top-title {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #fff;
  font-size: 15px;
  font-weight: 600;
}

.episode-name {
  color: var(--accent-primary);
  font-weight: 500;
}

/* Controls Bar */
.controls-bar {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  padding: 30px 18px 12px;
  background: linear-gradient(to top, rgba(0, 0, 0, 0.92) 0%, rgba(0, 0, 0, 0.4) 60%, transparent 100%);
  z-index: 25;
  transition: opacity 0.35s ease, transform 0.35s ease;
}

.hide-controls .controls-bar,
.hide-controls .top-bar {
  opacity: 0;
  pointer-events: none;
}

/* Progress Bar */
.progress-container {
  position: relative;
  height: 18px;
  display: flex;
  align-items: center;
  cursor: pointer;
  margin-bottom: 6px;
}

.progress-bg {
  position: relative;
  width: 100%;
  height: 4px;
  background: rgba(255, 255, 255, 0.2);
  border-radius: 2px;
  transition: height 0.15s ease;
}

.progress-container:hover .progress-bg {
  height: 6px;
}

.progress-buffered {
  position: absolute;
  top: 0;
  left: 0;
  bottom: 0;
  background: rgba(255, 255, 255, 0.4);
  border-radius: 2px;
  transition: width 0.2s linear;
}

.progress-played {
  position: absolute;
  top: 0;
  left: 0;
  bottom: 0;
  background: var(--accent-gradient);
  border-radius: 2px;
}

.scrubber-handle {
  position: absolute;
  right: -6px;
  top: 50%;
  transform: translateY(-50%) scale(0);
  width: 12px;
  height: 12px;
  background: #ffffff;
  border-radius: 50%;
  box-shadow: 0 0 8px rgba(255, 59, 92, 0.8);
  transition: transform 0.15s ease;
}

.progress-container:hover .scrubber-handle {
  transform: translateY(-50%) scale(1);
}

.time-tooltip {
  position: absolute;
  bottom: 22px;
  transform: translateX(-50%);
  background: rgba(18, 24, 38, 0.95);
  border: 1px solid rgba(255, 255, 255, 0.15);
  padding: 3px 7px;
  border-radius: 4px;
  font-size: 11px;
  color: #fff;
  white-space: nowrap;
  pointer-events: none;
}

/* Controls Row */
.controls-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}

.controls-left, .controls-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ctrl-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #e2e8f0;
  padding: 6px;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.ctrl-btn:hover:not(:disabled) {
  color: #fff;
  background: rgba(255, 255, 255, 0.12);
}

.ctrl-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.ctrl-active {
  color: var(--accent-primary) !important;
}

.btn-label-sm {
  font-size: 11px;
  font-weight: 700;
  margin-left: 2px;
}

.time-display {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: #cbd5e1;
  font-variant-numeric: tabular-nums;
  margin-left: 6px;
}

.time-divider {
  color: var(--text-muted);
}

/* Volume Slider */
.volume-group {
  display: flex;
  align-items: center;
  gap: 4px;
  position: relative;
}

.volume-slider-wrapper {
  width: 0;
  overflow: hidden;
  transition: width 0.25s ease;
  display: flex;
  align-items: center;
}

.volume-slider-wrapper.volume-open,
.volume-group:hover .volume-slider-wrapper {
  width: 70px;
}

.volume-slider {
  width: 65px;
  height: 4px;
  accent-color: var(--accent-primary);
  cursor: pointer;
}

/* Danmaku Input */
.controls-center {
  flex: 1;
  max-width: 320px;
}

.danmaku-input-box {
  display: flex;
  align-items: center;
  background: rgba(255, 255, 255, 0.12);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: var(--radius-full);
  padding: 3px 4px 3px 12px;
}

.danmaku-input-box input {
  flex: 1;
  background: transparent;
  color: #fff;
  font-size: 12px;
}

.danmaku-input-box input::placeholder {
  color: rgba(255, 255, 255, 0.5);
}

.send-btn {
  background: var(--accent-gradient);
  color: white;
  font-size: 11px;
  font-weight: 600;
  padding: 4px 10px;
  border-radius: var(--radius-full);
  transition: transform 0.15s ease;
}

.send-btn:hover {
  transform: scale(1.05);
}

/* Dropdown Menus (Speed, Quality) */
.dropdown-wrapper {
  position: relative;
}

.menu-btn {
  font-size: 13px;
  font-weight: 600;
  color: #e2e8f0;
  padding: 4px 8px;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.menu-btn:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.12);
}

.popover-menu {
  position: absolute;
  bottom: calc(100% + 12px);
  right: 0;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  z-index: 30;
  min-width: 90px;
  box-shadow: var(--shadow-lg);
}

.menu-item {
  font-size: 12px;
  font-weight: 500;
  color: #cbd5e1;
  padding: 6px 12px;
  border-radius: 4px;
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.15s ease;
}

.menu-item:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
}

.menu-active {
  color: var(--accent-primary) !important;
  font-weight: 700;
  background: rgba(255, 59, 92, 0.12);
}

@media (max-width: 768px) {
  .controls-center {
    display: none;
  }
}
</style>
