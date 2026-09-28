<template>
  <div class="player-view">
    <!-- Loading State -->
    <div v-if="loading" class="container loading-container">
      <div class="spinner"></div>
      <p>正在加载剧集资源...</p>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="container error-container">
      <p class="error-msg">{{ error }}</p>
      <button class="btn-primary" @click="loadDramaDetail">重新加载</button>
    </div>

    <!-- Main Player Layout -->
    <div v-else-if="drama" class="player-layout container">
      <!-- Left / Top: Video Player & Controls -->
      <div class="player-main">
        <VideoPlayer
          v-if="currentChapter"
          :series-id="drama.sourceId"
          :video-id="currentChapter.videoId"
          :series-title="drama.title || drama.name"
          :current-episode-title="currentChapter.title"
          :episode-index="currentChapter.index"
          :total-episodes="chapters.length"
          :variants="currentVariants"
          :danmaku-items="danmakuItems"
          :initial-time="initialWatchTime"
          @prev="playPrevEpisode"
          @next="playNextEpisode"
          @timeupdate="onPlaybackTimeUpdate"
          @ended="onEpisodeEnded"
        />

        <!-- Drama Action Bar & Meta Under Player -->
        <div class="drama-info-panel glass-panel">
          <div class="drama-info-header">
            <div class="title-meta">
              <h1 class="drama-title">{{ drama.title || drama.name }}</h1>
              <div class="meta-row">
                <span v-if="drama.score" class="badge badge-score">★ {{ drama.score }} 分</span>
                <span class="badge badge-category">{{ drama.categoryName || '短剧' }}</span>
                <span class="badge badge-heat">🔥 {{ drama.heat || '热播' }}</span>
                <span class="total-badge">共 {{ chapters.length }} 集全</span>
              </div>
            </div>

            <!-- Action Buttons -->
            <div class="action-buttons">
              <button
                class="action-btn"
                :class="{ 'btn-fav-active': isFavorite }"
                @click="toggleFavorite"
              >
                <BookmarkCheck v-if="isFavorite" :size="18" />
                <Bookmark v-else :size="18" />
                <span>{{ isFavorite ? '已追剧' : '追剧' }}</span>
              </button>

              <button class="action-btn" @click="copyShareLink">
                <Share2 :size="18" />
                <span>{{ shareCopied ? '已复制' : '分享' }}</span>
              </button>
            </div>
          </div>

          <!-- Description Accordion -->
          <div class="drama-desc-wrap" :class="{ 'desc-expanded': descExpanded }">
            <p class="drama-desc">
              {{ drama.desc || '暂无详细简介，精彩剧情直接点击分集开始观看！' }}
            </p>
            <button
              v-if="drama.desc && drama.desc.length > 90"
              class="expand-btn"
              @click="descExpanded = !descExpanded"
            >
              {{ descExpanded ? '收起简介' : '展开简介' }}
            </button>
          </div>

          <!-- Tags -->
          <div v-if="drama.tags && drama.tags.length > 0" class="tags-row">
            <span v-for="tag in drama.tags" :key="tag" class="tag-pill">
              # {{ tag }}
            </span>
          </div>
        </div>
      </div>

      <!-- Right / Side: Episode Selector Sidebar -->
      <aside class="episodes-sidebar glass-panel">
        <div class="sidebar-header">
          <div class="sidebar-title-group">
            <ListVideo :size="18" class="list-icon" />
            <span class="sidebar-title">选集播放</span>
          </div>
          <span class="episodes-count">共 {{ chapters.length }} 集</span>
        </div>

        <!-- Range Tabs for 30+ episodes -->
        <div v-if="rangeTabs.length > 1" class="range-tabs">
          <button
            v-for="(tab, idx) in rangeTabs"
            :key="idx"
            class="range-tab-btn"
            :class="{ 'range-active': currentRangeIndex === idx }"
            @click="currentRangeIndex = idx"
          >
            {{ tab.start }} - {{ tab.end }}
          </button>
        </div>

        <!-- Episode Numbers Grid -->
        <div class="episodes-grid">
          <button
            v-for="ep in currentRangeChapters"
            :key="ep.videoId"
            class="episode-btn"
            :class="{
              'ep-active': currentChapter && currentChapter.videoId === ep.videoId,
              'ep-watched': isWatched(ep.index)
            }"
            @click="switchEpisode(ep)"
          >
            <span class="ep-num">{{ ep.index }}</span>
            <div v-if="currentChapter && currentChapter.videoId === ep.videoId" class="playing-bars">
              <span></span><span></span><span></span>
            </div>
          </button>
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Bookmark, BookmarkCheck, Share2, ListVideo } from 'lucide-vue-next'
import VideoPlayer from '@/components/VideoPlayer.vue'
import { api } from '@/api'

const route = useRoute()

const loading = ref(true)
const error = ref(null)
const drama = ref(null)
const chapters = ref([])
const currentChapter = ref(null)
const currentVariants = ref([])
const danmakuItems = ref([])
const isFavorite = ref(false)
const shareCopied = ref(false)
const descExpanded = ref(false)
const currentRangeIndex = ref(0)
const initialWatchTime = ref(0)

const rangeSize = 30
let historySaveTimer = null
let lastSavedTime = 0

const rangeTabs = computed(() => {
  const tabs = []
  const total = chapters.value.length
  for (let i = 0; i < total; i += rangeSize) {
    const end = Math.min(i + rangeSize, total)
    tabs.push({ start: i + 1, end, startIdx: i, endIdx: end })
  }
  return tabs
})

const currentRangeChapters = computed(() => {
  const tab = rangeTabs.value[currentRangeIndex.value]
  if (!tab) return chapters.value
  return chapters.value.slice(tab.startIdx, tab.endIdx)
})

const loadDramaDetail = async () => {
  loading.value = true
  error.value = null

  try {
    const id = route.params.id
    const res = await api.getDetail(id)
    drama.value = res.drama
    chapters.value = res.chapters || []
    isFavorite.value = res.isFavorite || false

    if (chapters.value.length > 0) {
      // Check history to resume
      let saved = null
      try {
        const hist = await api.getHistory()
        if (Array.isArray(hist)) {
          saved = hist.find(h => h && h.seriesId === drama.value.sourceId)
        }
      } catch (histErr) {
        console.warn('获取观看历史失败:', histErr)
      }
      let initialEp = chapters.value[0]
      if (saved && saved.episodeIndex > 0 && saved.episodeIndex <= chapters.value.length) {
        initialEp = chapters.value[saved.episodeIndex - 1]
        initialWatchTime.value = saved.positionSec || 0
      }
      selectEpisode(initialEp)
    }
  } catch (err) {
    error.value = err.message || '加载短剧详情失败，请检查网络后重试'
  } finally {
    loading.value = false
  }
}

const selectEpisode = async (chapter) => {
  currentChapter.value = chapter

  // Sync tab range
  const idx = rangeTabs.value.findIndex(t => chapter.index >= t.start && chapter.index <= t.end)
  if (idx !== -1) {
    currentRangeIndex.value = idx
  }

  // Fetch playback info & danmaku in parallel
  try {
    const [playInfo, danmakuData] = await Promise.allSettled([
      api.getPlay(drama.value.sourceId, chapter.videoId),
      api.getDanmaku(drama.value.sourceId, chapter.videoId),
    ])

    if (playInfo.status === 'fulfilled') {
      currentVariants.value = playInfo.value.variants || []
    }
    if (danmakuData.status === 'fulfilled') {
      danmakuItems.value = danmakuData.value.items || []
    }
  } catch (e) {
    console.warn('Episode stream resolve error:', e)
  }
}

const switchEpisode = (ep) => {
  if (currentChapter.value && currentChapter.value.videoId === ep.videoId) return
  initialWatchTime.value = 0
  selectEpisode(ep)
}

const playPrevEpisode = () => {
  if (!currentChapter.value || currentChapter.value.index <= 1) return
  const prev = chapters.value[currentChapter.value.index - 2]
  if (prev) switchEpisode(prev)
}

const playNextEpisode = () => {
  if (!currentChapter.value || currentChapter.value.index >= chapters.value.length) return
  const next = chapters.value[currentChapter.value.index]
  if (next) switchEpisode(next)
}

const onPlaybackTimeUpdate = (currentSec, totalSec) => {
  if (Math.abs(currentSec - lastSavedTime) > 6 && drama.value && currentChapter.value) {
    lastSavedTime = currentSec
    saveHistoryThrottled(currentSec, totalSec)
  }
}

const saveHistoryThrottled = (pos, dur) => {
  clearTimeout(historySaveTimer)
  historySaveTimer = setTimeout(() => {
    api.saveHistory({
      seriesId: drama.value.sourceId,
      seriesTitle: drama.value.title || drama.value.name,
      cover: drama.value.cover,
      episodeIndex: currentChapter.value.index,
      episodeTitle: currentChapter.value.title,
      positionSec: pos,
      durationSec: dur,
    }).catch(() => {})
  }, 1000)
}

const onEpisodeEnded = () => {
  // Episode finished
}

const isWatched = (epIndex) => {
  // Can be enhanced with local storage tracking
  return false
}

const toggleFavorite = async () => {
  if (!drama.value) return
  try {
    if (isFavorite.value) {
      await api.removeFavorite(drama.value.sourceId)
      isFavorite.value = false
    } else {
      await api.addFavorite({
        sourceId: drama.value.sourceId,
        title: drama.value.title || drama.value.name,
        cover: drama.value.cover,
        episodes: chapters.value.length,
        category: drama.value.categoryName,
      })
      isFavorite.value = true
    }
  } catch (e) {
    console.error(e)
  }
}

const copyShareLink = async () => {
  try {
    await navigator.clipboard.writeText(window.location.href)
    shareCopied.value = true
    setTimeout(() => { shareCopied.value = false }, 2000)
  } catch (e) {
    prompt('请复制短剧链接:', window.location.href)
  }
}

watch(() => route.params.id, (newId) => {
  if (newId) loadDramaDetail()
})

onMounted(() => {
  loadDramaDetail()
})

onUnmounted(() => {
  clearTimeout(historySaveTimer)
})
</script>

<style scoped>
.player-view {
  min-height: 100vh;
  padding-top: 20px;
  padding-bottom: 80px;
}

.loading-container, .error-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  min-height: 60vh;
  color: var(--text-muted);
}

.spinner {
  width: 44px;
  height: 44px;
  border: 3px solid rgba(255, 255, 255, 0.1);
  border-top-color: var(--accent-primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.error-msg {
  color: #ff5470;
  font-size: 15px;
}

/* Player Layout Grid */
.player-layout {
  display: grid;
  grid-template-columns: 1fr 340px;
  gap: 24px;
  align-items: start;
}

@media (max-width: 1024px) {
  .player-layout {
    grid-template-columns: 1fr;
  }
}

/* Main Panel */
.player-main {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.drama-info-panel {
  padding: 22px;
}

.drama-info-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 16px;
}

.drama-title {
  font-size: 22px;
  font-weight: 800;
  color: #fff;
  line-height: 1.3;
}

.meta-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 8px;
}

.total-badge {
  font-size: 12px;
  color: var(--text-muted);
}

.action-buttons {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.action-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: var(--radius-full);
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid var(--border-light);
  color: #e2e8f0;
  font-size: 13px;
  font-weight: 600;
  transition: all 0.2s ease;
}

.action-btn:hover {
  background: rgba(255, 255, 255, 0.15);
  color: #fff;
}

.btn-fav-active {
  background: rgba(255, 59, 92, 0.2) !important;
  border-color: rgba(255, 59, 92, 0.4) !important;
  color: var(--accent-primary) !important;
}

.drama-desc-wrap {
  position: relative;
  font-size: 13.5px;
  color: var(--text-secondary);
  line-height: 1.6;
}

.drama-desc {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.desc-expanded .drama-desc {
  display: block;
}

.expand-btn {
  color: var(--accent-primary);
  font-size: 12.5px;
  font-weight: 600;
  margin-top: 4px;
}

.tags-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

.tag-pill {
  font-size: 12px;
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.04);
  padding: 3px 10px;
  border-radius: 4px;
}

/* Sidebar */
.episodes-sidebar {
  padding: 20px;
  display: flex;
  flex-direction: column;
  max-height: calc(100vh - 100px);
  position: sticky;
  top: 84px;
}

@media (max-width: 1024px) {
  .episodes-sidebar {
    max-height: 480px;
    position: static;
  }
}

.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.sidebar-title-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.list-icon {
  color: var(--accent-primary);
}

.sidebar-title {
  font-size: 16px;
  font-weight: 700;
  color: #fff;
}

.episodes-count {
  font-size: 12px;
  color: var(--text-muted);
}

/* Range Tabs */
.range-tabs {
  display: flex;
  align-items: center;
  gap: 6px;
  overflow-x: auto;
  padding-bottom: 8px;
  margin-bottom: 12px;
  scrollbar-width: thin;
}

.range-tab-btn {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  padding: 4px 10px;
  border-radius: var(--radius-full);
  background: rgba(255, 255, 255, 0.05);
  white-space: nowrap;
  transition: all 0.2s;
}

.range-tab-btn:hover {
  color: #fff;
}

.range-active {
  background: rgba(255, 59, 92, 0.2) !important;
  color: var(--accent-primary) !important;
}

/* Episodes Grid */
.episodes-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 8px;
  overflow-y: auto;
  padding-right: 4px;
}

.episode-btn {
  position: relative;
  height: 42px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-sm);
  color: var(--text-secondary);
  font-size: 13.5px;
  font-weight: 600;
  transition: all 0.2s ease;
}

.episode-btn:hover {
  background: rgba(255, 255, 255, 0.12);
  color: #fff;
  border-color: rgba(255, 255, 255, 0.2);
}

.ep-active {
  background: var(--accent-gradient) !important;
  color: #fff !important;
  border-color: transparent !important;
  box-shadow: 0 4px 12px rgba(255, 59, 92, 0.4);
}

/* Playing Equalizer Animation */
.playing-bars {
  position: absolute;
  bottom: 3px;
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 8px;
}

.playing-bars span {
  width: 2px;
  background: #ffffff;
  border-radius: 1px;
  animation: eq 0.6s infinite ease-in-out alternate;
}

.playing-bars span:nth-child(1) { height: 4px; animation-delay: 0.1s; }
.playing-bars span:nth-child(2) { height: 8px; animation-delay: 0.3s; }
.playing-bars span:nth-child(3) { height: 5px; animation-delay: 0.2s; }

@keyframes eq {
  from { height: 2px; }
  to { height: 8px; }
}

@media (max-width: 640px) {
  .player-layout {
    gap: 14px;
  }
  .drama-info-panel {
    padding: 16px;
  }
  .drama-title {
    font-size: 18px;
  }
  .episodes-grid {
    grid-template-columns: repeat(5, 1fr);
  }
}
</style>
