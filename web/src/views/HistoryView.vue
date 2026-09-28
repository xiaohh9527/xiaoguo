<template>
  <div class="history-view container">
    <div class="page-header">
      <div class="header-info">
        <h1 class="page-title">
          <Clock :size="26" class="title-icon" />
          <span>播放历史记录</span>
        </h1>
        <p class="page-subtitle">记录你在各剧集中的观看进度，随时无缝续播</p>
      </div>

      <button
        v-if="historyList.length > 0"
        class="clear-history-btn"
        @click="clearAllHistory"
      >
        <Trash2 :size="15" />
        <span>清空全部历史</span>
      </button>
    </div>

    <!-- History List -->
    <div v-if="historyList.length > 0" class="history-list animate-fade-in">
      <div
        v-for="item in historyList"
        :key="item.seriesId"
        class="history-row glass-panel"
        @click="continueWatch(item)"
      >
        <div class="thumb-container">
          <img :src="getCoverUrl(item.cover)" class="thumb-img" referrerpolicy="no-referrer" />
          <div class="progress-bar-bottom">
            <div
              class="progress-fill"
              :style="{ width: `${getProgressPercent(item)}%` }"
            ></div>
          </div>
        </div>

        <div class="history-info">
          <h3 class="series-name">{{ item.seriesTitle }}</h3>
          <div class="episode-status">
            <span class="ep-text">看到第 {{ item.episodeIndex }} 集</span>
            <span v-if="item.durationSec > 0" class="pos-text">
              ({{ formatTime(item.positionSec) }} / {{ formatTime(item.durationSec) }})
            </span>
          </div>
          <span class="date-text">{{ formatDate(item.updatedAt) }}</span>
        </div>

        <div class="history-action">
          <button class="continue-btn">
            <Play :size="15" fill="currentColor" />
            <span>继续观看</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else class="empty-state">
      <Clock :size="54" class="empty-icon" />
      <p class="empty-title">还没有任何播放记录</p>
      <p class="empty-desc">观看短剧时会自动为你保存播放进度</p>
      <router-link to="/" class="btn-primary">
        <span>去看看热门推荐</span>
      </router-link>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Clock, Play, Trash2 } from 'lucide-vue-next'
import { api } from '@/api'

const router = useRouter()
const historyList = ref([])

const loadHistory = async () => {
  try {
    const res = await api.getHistory()
    historyList.value = Array.isArray(res) ? res : []
  } catch (err) {
    console.error(err)
    historyList.value = []
  }
}

const clearAllHistory = async () => {
  if (!confirm('确定要清空全部播放历史吗？')) return
  try {
    await api.clearHistory()
    historyList.value = []
  } catch (err) {
    console.error(err)
  }
}

const continueWatch = (item) => {
  router.push(`/player/${item.seriesId}`)
}

const getCoverUrl = (url) => api.getImageProxyUrl(url)

const getProgressPercent = (item) => {
  if (!item.durationSec) return 0
  return Math.min(100, (item.positionSec / item.durationSec) * 100)
}

const formatTime = (seconds) => {
  if (!seconds || isNaN(seconds)) return '00:00'
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
}

const formatDate = (dateStr) => {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return d.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

onMounted(() => {
  loadHistory()
})
</script>

<style scoped>
.history-view {
  padding-top: 28px;
  padding-bottom: 80px;
}

.page-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 24px;
}

.page-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 26px;
  font-weight: 800;
  color: #fff;
}

.title-icon {
  color: var(--accent-primary);
}

.page-subtitle {
  font-size: 13px;
  color: var(--text-muted);
  margin-top: 4px;
}

.clear-history-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--text-muted);
  padding: 6px 12px;
  border-radius: var(--radius-sm);
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-light);
  transition: all 0.2s;
}

.clear-history-btn:hover {
  color: #ff5470;
  border-color: rgba(255, 59, 92, 0.3);
  background: rgba(255, 59, 92, 0.1);
}

/* History List */
.history-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.history-row {
  display: flex;
  align-items: center;
  padding: 14px 18px;
  gap: 18px;
  cursor: pointer;
  transition: all 0.25s ease;
}

.history-row:hover {
  background: var(--bg-card-hover);
  border-color: rgba(255, 59, 92, 0.3);
  transform: translateX(4px);
}

.thumb-container {
  position: relative;
  width: 60px;
  height: 80px;
  border-radius: 8px;
  overflow: hidden;
  flex-shrink: 0;
  background: #1e2638;
}

.thumb-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.progress-bar-bottom {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 4px;
  background: rgba(0, 0, 0, 0.6);
}

.progress-fill {
  height: 100%;
  background: var(--accent-primary);
}

.history-info {
  flex: 1;
  min-width: 0;
}

.series-name {
  font-size: 15.5px;
  font-weight: 700;
  color: #fff;
  margin-bottom: 6px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.episode-status {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text-secondary);
}

.ep-text {
  color: var(--accent-primary);
  font-weight: 600;
}

.pos-text {
  color: var(--text-muted);
  font-size: 12px;
}

.date-text {
  font-size: 11.5px;
  color: var(--text-muted);
  display: block;
  margin-top: 6px;
}

.history-action {
  flex-shrink: 0;
}

.continue-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: var(--accent-gradient);
  color: white;
  font-size: 13px;
  font-weight: 600;
  padding: 8px 16px;
  border-radius: var(--radius-full);
  box-shadow: var(--shadow-accent);
  transition: transform 0.2s ease;
}

.history-row:hover .continue-btn {
  transform: scale(1.05);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 20px;
  text-align: center;
}

.empty-icon {
  opacity: 0.3;
  margin-bottom: 12px;
}

.empty-title {
  font-size: 17px;
  font-weight: 700;
  color: #fff;
  margin-bottom: 6px;
}

.empty-desc {
  font-size: 13px;
  color: var(--text-muted);
  margin-bottom: 24px;
}

@media (max-width: 640px) {
  .history-row {
    padding: 10px 12px;
    gap: 12px;
  }
  .thumb-container {
    width: 48px;
    height: 64px;
  }
  .series-name {
    font-size: 14px;
  }
  .continue-btn {
    padding: 6px 12px;
    font-size: 12px;
  }
}
</style>
