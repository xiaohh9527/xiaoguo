<template>
  <div class="categories-view container">
    <div class="page-header">
      <h1 class="page-title">
        <LayoutGrid :size="26" class="title-icon" />
        <span>短剧分类精选</span>
      </h1>
      <p class="page-subtitle">海量短剧分类浏览，发现你所钟爱的题材</p>
    </div>

    <!-- Genre Pills -->
    <div class="filter-box glass-panel">
      <div class="filter-row">
        <span class="filter-label">频道分类:</span>
        <div class="filter-options">
          <button
            v-for="g in genres"
            :key="g.key"
            class="filter-chip"
            :class="{ 'chip-active': currentGenre === g.key }"
            @click="selectGenre(g.key)"
          >
            {{ g.name }}
          </button>
        </div>
      </div>
    </div>

    <!-- Grid -->
    <div v-if="loading && items.length === 0" class="drama-grid">
      <div v-for="i in 12" :key="i" class="skeleton-card">
        <div class="skeleton-poster"></div>
        <div class="skeleton-line" style="width: 80%"></div>
      </div>
    </div>

    <div v-else-if="items.length > 0" class="drama-grid animate-fade-in">
      <DramaCard v-for="item in items" :key="item.sourceId" :drama="item" />
    </div>

    <div v-else class="empty-state">
      <Inbox :size="48" class="empty-icon" />
      <p>该分类下暂无内容</p>
    </div>

    <!-- Load More -->
    <div v-if="items.length > 0" class="load-more-wrap">
      <button
        v-if="hasMore"
        class="btn-secondary"
        :disabled="loadingMore"
        @click="loadMore"
      >
        <span v-if="loadingMore">加载中...</span>
        <span v-else>加载更多</span>
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { LayoutGrid, Inbox } from 'lucide-vue-next'
import DramaCard from '@/components/DramaCard.vue'
import { api } from '@/api'

const genres = ref([
  { key: 'short_play', name: '真人剧' },
  { key: 'comic_series', name: '漫剧' },
  { key: 'ai_series', name: 'AI剧' },
  { key: 'comic', name: '动漫' },
])
const currentGenre = ref('short_play')
const items = ref([])
const loading = ref(true)
const loadingMore = ref(false)
const hasMore = ref(true)
const offset = ref(0)
const sessionId = ref('')
const seen = ref([])

const loadData = async (reset = false) => {
  if (reset) {
    loading.value = true
    items.value = []
    offset.value = 0
    sessionId.value = ''
    seen.value = []
  }
  try {
    const res = await api.getCatalog({
      genre: currentGenre.value,
      offset: offset.value,
      session_id: sessionId.value,
      seen: seen.value,
    })
    const list = res.data || []
    if (reset) {
      items.value = list
    } else {
      items.value = [...items.value, ...list]
    }
    for (const d of list) {
      seen.value.push(d.sourceId)
    }
    offset.value = res.nextOffset || (offset.value + 18)
    sessionId.value = res.sessionId || ''
    hasMore.value = res.hasMore !== false && list.length > 0
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

const selectGenre = (key) => {
  if (currentGenre.value === key) return
  currentGenre.value = key
  loadData(true)
}

const loadMore = () => {
  if (loadingMore.value || !hasMore.value) return
  loadingMore.value = true
  loadData(false)
}

onMounted(() => {
  loadData(true)
})
</script>

<style scoped>
.categories-view {
  padding-top: 28px;
  padding-bottom: 80px;
}

.page-header {
  margin-bottom: 20px;
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

.filter-box {
  padding: 16px 20px;
  margin-bottom: 28px;
}

.filter-row {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.filter-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-muted);
}

.filter-options {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.filter-chip {
  padding: 6px 16px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-full);
  transition: all 0.2s ease;
}

.filter-chip:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
}

.chip-active {
  background: var(--accent-gradient) !important;
  color: #fff !important;
  border-color: transparent !important;
  font-weight: 600;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 20px;
  color: var(--text-muted);
}

.empty-icon {
  opacity: 0.4;
  margin-bottom: 12px;
}

.load-more-wrap {
  display: flex;
  justify-content: center;
  margin-top: 36px;
}

.skeleton-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.skeleton-poster {
  width: 100%;
  aspect-ratio: 3 / 4;
  border-radius: var(--radius-md);
  background: #1e2638;
}

.skeleton-line {
  height: 12px;
  border-radius: 4px;
  background: #1e2638;
}
</style>
