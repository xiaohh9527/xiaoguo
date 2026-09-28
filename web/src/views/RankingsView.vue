<template>
  <div class="rankings-view container">
    <!-- Header -->
    <div class="rankings-header">
      <div class="header-left">
        <h1 class="page-title">
          <Flame :size="28" class="flame-icon" />
          <span>小果短剧热门榜单</span>
        </h1>
        <p class="page-subtitle">{{ currentBoardDesc }} (每日实时更新)</p>
      </div>
    </div>

    <!-- Boards Nav -->
    <div class="boards-row">
      <button
        v-for="b in boards"
        :key="b.id"
        class="board-btn"
        :class="{ 'board-btn-active': activeBoard === b.id }"
        @click="switchBoard(b.id)"
      >
        <span>{{ b.name }}</span>
      </button>
    </div>

    <!-- Loading Skeleton -->
    <div v-if="loading" class="rankings-skeleton">
      <div v-for="i in 8" :key="i" class="skeleton-row"></div>
    </div>

    <!-- Content -->
    <div v-else class="rankings-content animate-fade-in">
      <!-- Top 3 Podium Cards -->
      <div v-if="topThree.length >= 3" class="podium-section">
        <!-- Rank 2 (Silver) -->
        <div class="podium-card rank-2" @click="goToPlayer(topThree[1].drama)">
          <div class="medal-badge silver">2</div>
          <div class="podium-cover-wrapper">
            <img :src="getCoverUrl(topThree[1].drama.cover)" class="podium-cover" referrerpolicy="no-referrer" />
          </div>
          <h3 class="podium-title">{{ topThree[1].drama.title }}</h3>
          <span class="podium-heat">🔥 {{ topThree[1].metric || topThree[1].drama.heat }}</span>
        </div>

        <!-- Rank 1 (Gold) -->
        <div class="podium-card rank-1" @click="goToPlayer(topThree[0].drama)">
          <div class="crown-icon">👑</div>
          <div class="medal-badge gold">1</div>
          <div class="podium-cover-wrapper">
            <img :src="getCoverUrl(topThree[0].drama.cover)" class="podium-cover" referrerpolicy="no-referrer" />
          </div>
          <h3 class="podium-title">{{ topThree[0].drama.title }}</h3>
          <span class="podium-heat">🔥 {{ topThree[0].metric || topThree[0].drama.heat }}</span>
        </div>

        <!-- Rank 3 (Bronze) -->
        <div class="podium-card rank-3" @click="goToPlayer(topThree[2].drama)">
          <div class="medal-badge bronze">3</div>
          <div class="podium-cover-wrapper">
            <img :src="getCoverUrl(topThree[2].drama.cover)" class="podium-cover" referrerpolicy="no-referrer" />
          </div>
          <h3 class="podium-title">{{ topThree[2].drama.title }}</h3>
          <span class="podium-heat">🔥 {{ topThree[2].metric || topThree[2].drama.heat }}</span>
        </div>
      </div>

      <!-- Rank List Items -->
      <div class="rank-list">
        <div
          v-for="item in items"
          :key="item.drama.sourceId || item.rank"
          class="rank-row glass-panel"
          @click="goToPlayer(item.drama)"
        >
          <!-- Rank Number -->
          <div class="rank-num" :class="getRankClass(item.rank)">
            {{ item.rank }}
          </div>

          <!-- Thumbnail -->
          <div class="thumb-wrapper">
            <img :src="getCoverUrl(item.drama.cover)" class="thumb-img" loading="lazy" referrerpolicy="no-referrer" />
          </div>

          <!-- Info -->
          <div class="rank-info">
            <h4 class="rank-title">{{ item.drama.title }}</h4>
            <div class="rank-tags">
              <span v-if="item.drama.categoryName" class="badge badge-category">
                {{ item.drama.categoryName }}
              </span>
              <span v-if="item.drama.score" class="badge badge-score">
                ★ {{ item.drama.score }}
              </span>
              <span v-for="t in (item.drama.tags || []).slice(0, 2)" :key="t" class="tag-item">
                {{ t }}
              </span>
            </div>
          </div>

          <!-- Metric -->
          <div class="rank-metric">
            <span class="metric-val">
              <Flame :size="14" class="flame-sm" />
              {{ item.metric || item.drama.heat || '热度上升' }}
            </span>
          </div>

          <!-- Action -->
          <div class="rank-action">
            <button class="play-btn">
              <Play :size="16" fill="currentColor" />
              <span>播放</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Flame, Play } from 'lucide-vue-next'
import { api } from '@/api'

const router = useRouter()
const boards = ref([
  { id: 'hongguo-hot', name: '热播总榜', description: '全网综合热度短剧总榜' },
  { id: 'hongguo-real', name: '真人剧榜', description: '高分精品真人微短剧排行' },
  { id: 'hongguo-comic', name: '漫剧榜', description: '高燃动态漫与漫画剧排行' },
  { id: 'hongguo-ai', name: 'AI剧榜', description: '前沿 AI 创意短剧热播排行' },
])
const activeBoard = ref('hongguo-hot')
const items = ref([])
const loading = ref(true)

const currentBoardDesc = computed(() => {
  const b = boards.value.find(item => item.id === activeBoard.value)
  return b ? b.description : '实时热度短剧榜单'
})

const topThree = computed(() => {
  return items.value.slice(0, 3)
})

const loadRankings = async () => {
  loading.value = true
  try {
    const res = await api.getRankings(activeBoard.value, 1)
    items.value = res.items || []
  } catch (err) {
    console.error('Failed to load rankings:', err)
  } finally {
    loading.value = false
  }
}

const switchBoard = (boardId) => {
  if (activeBoard.value === boardId) return
  activeBoard.value = boardId
  loadRankings()
}

const getCoverUrl = (url) => api.getImageProxyUrl(url)

const getRankClass = (rank) => {
  if (rank === 1) return 'rank-gold'
  if (rank === 2) return 'rank-silver'
  if (rank === 3) return 'rank-bronze'
  return ''
}

const goToPlayer = (drama) => {
  const id = drama.sourceId || drama.id.replace('hongguo:', '')
  router.push(`/player/${id}`)
}

onMounted(() => {
  loadRankings()
})
</script>

<style scoped>
.rankings-view {
  padding-top: 28px;
  padding-bottom: 80px;
}

.rankings-header {
  margin-bottom: 20px;
}

.flame-icon {
  color: var(--accent-primary);
}

.page-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 26px;
  font-weight: 800;
  color: #fff;
}

.page-subtitle {
  font-size: 13px;
  color: var(--text-muted);
  margin-top: 4px;
}

/* Boards Row */
.boards-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 30px;
  overflow-x: auto;
  padding-bottom: 4px;
}

.board-btn {
  padding: 8px 20px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-full);
  white-space: nowrap;
  transition: all 0.2s ease;
}

.board-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
}

.board-btn-active {
  background: var(--accent-gradient) !important;
  color: #fff !important;
  box-shadow: 0 4px 16px rgba(255, 59, 92, 0.4);
  border-color: transparent !important;
}

/* Podium Section */
.podium-section {
  display: grid;
  grid-template-columns: 1fr 1.15fr 1fr;
  gap: 16px;
  align-items: flex-end;
  margin-bottom: 36px;
}

.podium-card {
  position: relative;
  background: var(--bg-card);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-lg);
  padding: 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.2, 0, 0, 1);
}

.podium-card:hover {
  transform: translateY(-6px);
  border-color: var(--border-hover);
  box-shadow: var(--shadow-lg);
}

.podium-card.rank-1 {
  background: linear-gradient(180deg, rgba(255, 184, 0, 0.12) 0%, rgba(22, 28, 42, 0.9) 100%);
  border-color: rgba(255, 184, 0, 0.35);
  box-shadow: 0 8px 32px rgba(255, 184, 0, 0.15);
  padding-top: 24px;
}

.crown-icon {
  font-size: 26px;
  position: absolute;
  top: -18px;
  left: 50%;
  transform: translateX(-50%);
  filter: drop-shadow(0 2px 8px rgba(255, 184, 0, 0.6));
}

.medal-badge {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  font-weight: 800;
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 12px;
}

.medal-badge.gold {
  background: var(--accent-gold-gradient);
  color: #000;
}

.medal-badge.silver {
  background: linear-gradient(135deg, #e2e8f0 0%, #94a3b8 100%);
  color: #0f172a;
}

.medal-badge.bronze {
  background: linear-gradient(135deg, #f59e0b 0%, #b45309 100%);
  color: #fff;
}

.podium-cover-wrapper {
  width: 100%;
  max-width: 160px;
  aspect-ratio: 3 / 4;
  border-radius: var(--radius-md);
  overflow: hidden;
  margin-bottom: 10px;
  box-shadow: var(--shadow-md);
}

.podium-cover {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.podium-title {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
  line-height: 1.35;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}

.podium-heat {
  font-size: 12px;
  color: #ff5470;
  font-weight: 600;
  margin-top: 4px;
}

/* Rank List */
.rank-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.rank-row {
  display: flex;
  align-items: center;
  padding: 12px 18px;
  gap: 16px;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all 0.2s ease;
}

.rank-row:hover {
  background: var(--bg-card-hover);
  border-color: rgba(255, 59, 92, 0.3);
  transform: translateX(4px);
}

.rank-num {
  font-size: 18px;
  font-weight: 800;
  width: 32px;
  text-align: center;
  color: var(--text-muted);
}

.rank-gold {
  color: var(--accent-gold) !important;
  font-size: 22px;
}

.rank-silver {
  color: #cbd5e1 !important;
  font-size: 20px;
}

.rank-bronze {
  color: #f59e0b !important;
  font-size: 20px;
}

.thumb-wrapper {
  width: 48px;
  height: 64px;
  border-radius: 6px;
  overflow: hidden;
  flex-shrink: 0;
  background: #1e2638;
}

.thumb-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.rank-info {
  flex: 1;
  min-width: 0;
}

.rank-title {
  font-size: 15px;
  font-weight: 600;
  color: #fff;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rank-tags {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 6px;
}

.tag-item {
  font-size: 11px;
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.05);
  padding: 1px 6px;
  border-radius: 3px;
}

.rank-metric {
  text-align: right;
  flex-shrink: 0;
}

.metric-val {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  color: #ff5470;
  font-size: 13px;
  font-weight: 600;
}

.flame-sm {
  color: #ff3b5c;
}

.rank-action {
  flex-shrink: 0;
}

.play-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: rgba(255, 59, 92, 0.15);
  color: var(--accent-primary);
  border: 1px solid rgba(255, 59, 92, 0.3);
  font-size: 13px;
  font-weight: 600;
  padding: 6px 14px;
  border-radius: var(--radius-full);
  transition: all 0.2s ease;
}

.rank-row:hover .play-btn {
  background: var(--accent-gradient);
  color: #fff;
  border-color: transparent;
}

/* Skeleton */
.skeleton-row {
  height: 80px;
  border-radius: var(--radius-md);
  background: #161c2a;
  margin-bottom: 10px;
  animation: shimmer 1.5s infinite;
}

@media (max-width: 640px) {
  .podium-section {
    display: none;
  }
  .rank-row {
    padding: 10px 12px;
    gap: 12px;
  }
  .rank-title {
    font-size: 13.5px;
  }
  .rank-action {
    display: none;
  }
}
</style>

