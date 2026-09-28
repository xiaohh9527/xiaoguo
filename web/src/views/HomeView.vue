<template>
  <div class="home-view">
    <!-- Hero Spotlight Section -->
    <section v-if="featuredDrama" class="hero-section">
      <div class="hero-backdrop" :style="{ backgroundImage: `url(${heroCoverUrl})` }"></div>
      <div class="hero-gradient"></div>
      <div class="container hero-content">
        <div class="hero-badge">
          <Flame :size="14" />
          <span>今日重磅精选</span>
        </div>
        <h1 class="hero-title">{{ featuredDrama.title || featuredDrama.name }}</h1>
        <div class="hero-meta">
          <span v-if="featuredDrama.score" class="badge badge-score">★ {{ featuredDrama.score }}</span>
          <span class="badge badge-category">{{ featuredDrama.categoryName || '短剧' }}</span>
          <span v-if="featuredDrama.totalEpisodes" class="hero-episodes">共 {{ featuredDrama.totalEpisodes }} 集</span>
          <span v-if="featuredDrama.heat" class="heat-text">
            <Flame :size="13" />
            {{ featuredDrama.heat }}
          </span>
        </div>
        <p class="hero-desc">{{ featuredDrama.desc || '精彩剧情不容错过，高燃短剧随心看！' }}</p>
        <div class="hero-actions">
          <router-link :to="`/player/${featuredDrama.sourceId}`" class="btn-primary">
            <Play :size="18" fill="currentColor" />
            <span>立即播放</span>
          </router-link>
          <button class="btn-secondary" @click="toggleHeroFav">
            <BookmarkCheck v-if="isHeroFav" :size="18" />
            <Bookmark v-else :size="18" />
            <span>{{ isHeroFav ? '已追剧' : '加入追剧' }}</span>
          </button>
        </div>
      </div>
    </section>

    <!-- Main Content Section -->
    <main class="container main-section">
      <!-- Category Tabs -->
      <div class="category-tabs-row">
        <div class="tabs-list">
          <button
            class="tab-btn"
            :class="{ 'tab-btn-active': activeGenre === '' }"
            @click="switchGenre('')"
          >
            全部精选
          </button>
          <button
            v-for="g in genres"
            :key="g.key"
            class="tab-btn"
            :class="{ 'tab-btn-active': activeGenre === g.key }"
            @click="switchGenre(g.key)"
          >
            {{ g.name }}
          </button>
        </div>
      </div>

      <!-- Drama Cards Grid -->
      <div v-if="loading && dramas.length === 0" class="drama-grid">
        <div v-for="i in 12" :key="i" class="skeleton-card">
          <div class="skeleton-poster"></div>
          <div class="skeleton-line" style="width: 80%"></div>
          <div class="skeleton-line" style="width: 50%"></div>
        </div>
      </div>

      <div v-else-if="dramas.length > 0" class="drama-grid animate-fade-in">
        <DramaCard
          v-for="drama in dramas"
          :key="drama.id || drama.sourceId"
          :drama="drama"
        />
      </div>

      <!-- Empty State -->
      <div v-else-if="!loading && dramas.length === 0" class="empty-state">
        <Inbox :size="48" class="empty-icon" />
        <p>暂无相关短剧内容</p>
        <button class="btn-primary" @click="loadData(true)">刷新重试</button>
      </div>

      <!-- Load More Button -->
      <div v-if="dramas.length > 0" class="load-more-section">
        <button
          v-if="hasMore"
          class="load-more-btn"
          :disabled="loadingMore"
          @click="loadMore"
        >
          <span v-if="loadingMore" class="loading-dots">加载中...</span>
          <span v-else>发现更多精彩短剧</span>
        </button>
        <div v-else class="no-more-text">
          已到达推荐终点，去榜单看看吧~
        </div>
      </div>
    </main>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { Play, Flame, Bookmark, BookmarkCheck, Inbox } from 'lucide-vue-next'
import DramaCard from '@/components/DramaCard.vue'
import { api } from '@/api'

const genres = ref([])
const activeGenre = ref('')
const dramas = ref([])
const featuredDrama = ref(null)
const isHeroFav = ref(false)
const loading = ref(true)
const loadingMore = ref(false)
const hasMore = ref(true)
const nextOffset = ref(0)
const sessionId = ref('')
const seenIds = ref([])

const heroCoverUrl = computed(() => {
  if (!featuredDrama.value || !featuredDrama.value.cover) return ''
  return api.getImageProxyUrl(featuredDrama.value.cover)
})

const loadGenres = async () => {
  try {
    genres.value = await api.getGenres()
  } catch (err) {
    genres.value = [
      { key: 'short_play', name: '真人剧' },
      { key: 'comic_series', name: '漫剧' },
      { key: 'ai_series', name: 'AI剧' },
      { key: 'comic', name: '动漫' },
    ]
  }
}

const loadData = async (reset = false) => {
  if (reset) {
    loading.value = true
    dramas.value = []
    nextOffset.value = 0
    sessionId.value = ''
    seenIds.value = []
  }

  try {
    const res = await api.getCatalog({
      genre: activeGenre.value || 'short_play',
      offset: nextOffset.value,
      session_id: sessionId.value,
      seen: seenIds.value,
    })

    const newItems = res.data || []
    if (reset && newItems.length > 0) {
      featuredDrama.value = newItems[0]
      checkHeroFav()
      dramas.value = newItems.slice(1)
    } else {
      dramas.value = [...dramas.value, ...newItems]
    }

    for (const item of newItems) {
      seenIds.value.push(item.sourceId)
    }
    nextOffset.value = res.nextOffset || (nextOffset.value + 18)
    sessionId.value = res.sessionId || ''
    hasMore.value = res.hasMore !== false && newItems.length > 0
  } catch (err) {
    console.error('Failed to load catalog:', err)
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

const loadMore = () => {
  if (loadingMore.value || !hasMore.value) return
  loadingMore.value = true
  loadData(false)
}

const switchGenre = (genreKey) => {
  if (activeGenre.value === genreKey) return
  activeGenre.value = genreKey
  loadData(true)
}

const checkHeroFav = async () => {
  if (!featuredDrama.value) return
  try {
    const list = await api.getFavorites()
    if (Array.isArray(list)) {
      isHeroFav.value = list.some(item => item && item.sourceId === featuredDrama.value.sourceId)
    } else {
      isHeroFav.value = false
    }
  } catch (e) {}
}

const toggleHeroFav = async () => {
  if (!featuredDrama.value) return
  try {
    if (isHeroFav.value) {
      await api.removeFavorite(featuredDrama.value.sourceId)
      isHeroFav.value = false
    } else {
      await api.addFavorite({
        sourceId: featuredDrama.value.sourceId,
        title: featuredDrama.value.title || featuredDrama.value.name,
        cover: featuredDrama.value.cover,
        episodes: featuredDrama.value.totalEpisodes,
        category: featuredDrama.value.categoryName,
      })
      isHeroFav.value = true
    }
  } catch (err) {
    console.error(err)
  }
}

onMounted(async () => {
  await loadGenres()
  await loadData(true)
})
</script>

<style scoped>
.home-view {
  min-height: 100vh;
  padding-bottom: 80px;
}

/* Hero Section */
.hero-section {
  position: relative;
  min-height: 480px;
  display: flex;
  align-items: center;
  overflow: hidden;
  margin-top: -64px;
  padding-top: 64px;
}

.hero-backdrop {
  position: absolute;
  inset: -20px;
  background-size: cover;
  background-position: center 25%;
  filter: blur(28px) brightness(0.35);
  transform: scale(1.1);
  z-index: 1;
}

.hero-gradient {
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at 75% 40%, rgba(255, 59, 92, 0.15) 0%, transparent 60%),
              linear-gradient(to top, var(--bg-primary) 0%, rgba(11, 14, 20, 0.6) 50%, var(--bg-primary) 100%);
  z-index: 2;
}

.hero-content {
  position: relative;
  z-index: 3;
  padding-top: 40px;
  padding-bottom: 40px;
  max-width: 720px;
}

.hero-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: rgba(255, 59, 92, 0.18);
  border: 1px solid rgba(255, 59, 92, 0.35);
  color: #ff5470;
  font-size: 12px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: var(--radius-full);
  margin-bottom: 16px;
}

.hero-title {
  font-size: 38px;
  font-weight: 800;
  color: #ffffff;
  line-height: 1.25;
  letter-spacing: -0.5px;
  margin-bottom: 14px;
  text-shadow: 0 4px 16px rgba(0, 0, 0, 0.6);
}

.hero-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 14px;
}

.hero-episodes {
  font-size: 13px;
  color: var(--text-secondary);
}

.hero-desc {
  font-size: 14.5px;
  color: #cbd5e1;
  line-height: 1.6;
  margin-bottom: 24px;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.hero-actions {
  display: flex;
  align-items: center;
  gap: 14px;
}

/* Main Section */
.main-section {
  padding-top: 24px;
}

.category-tabs-row {
  margin-bottom: 24px;
  overflow-x: auto;
  scrollbar-width: none;
}

.category-tabs-row::-webkit-scrollbar {
  display: none;
}

.tabs-list {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tab-btn {
  padding: 7px 18px;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-full);
  white-space: nowrap;
  transition: all 0.2s ease;
}

.tab-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  color: white;
}

.tab-btn-active {
  background: var(--accent-gradient) !important;
  color: white !important;
  font-weight: 600;
  box-shadow: 0 4px 14px rgba(255, 59, 92, 0.35);
  border-color: transparent !important;
}

/* Skeleton Loaders */
.skeleton-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.skeleton-poster {
  width: 100%;
  aspect-ratio: 3 / 4;
  border-radius: var(--radius-md);
  background: linear-gradient(90deg, #141a29 25%, #1e2638 50%, #141a29 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
}

.skeleton-line {
  height: 12px;
  border-radius: 4px;
  background: #1e2638;
}

@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

/* Empty State */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding: 80px 20px;
  color: var(--text-muted);
}

.empty-icon {
  opacity: 0.4;
}

/* Load More */
.load-more-section {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 0;
}

.load-more-btn {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.12);
  color: var(--text-primary);
  font-size: 14px;
  font-weight: 600;
  padding: 11px 28px;
  border-radius: var(--radius-full);
  transition: all 0.25s ease;
}

.load-more-btn:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.12);
  border-color: var(--accent-primary);
  transform: translateY(-2px);
}

.no-more-text {
  font-size: 13px;
  color: var(--text-muted);
}

@media (max-width: 768px) {
  .hero-section {
    min-height: 380px;
  }
  .hero-title {
    font-size: 26px;
  }
  .hero-desc {
    font-size: 13px;
  }
}
</style>
