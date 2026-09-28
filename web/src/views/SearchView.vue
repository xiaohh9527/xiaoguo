<template>
  <div class="search-view container">
    <!-- Big Search Input -->
    <div class="search-header glass-panel">
      <div class="search-box-large">
        <Search :size="22" class="search-icon-lg" />
        <input
          ref="inputRef"
          type="text"
          v-model="keyword"
          placeholder="搜索全网短剧、演员、题材关键词..."
          @keydown.enter="doSearch"
        />
        <button v-if="keyword" class="clear-btn-lg" @click="clearKeyword">
          <X :size="18" />
        </button>
        <button class="btn-primary search-action-btn" @click="doSearch">
          <span>搜索</span>
        </button>
      </div>

      <!-- Hot Searches -->
      <div class="hot-searches">
        <span class="hot-label">大家都在搜:</span>
        <div class="hot-tags">
          <button
            v-for="word in hotWords"
            :key="word"
            class="hot-chip"
            @click="clickHotWord(word)"
          >
            {{ word }}
          </button>
        </div>
      </div>
    </div>

    <!-- Results Section -->
    <div v-if="hasSearched" class="results-section">
      <div class="results-bar">
        <span class="results-count">
          找到 <strong>{{ dramas.length }}</strong> 部相关短剧
        </span>
      </div>

      <!-- Loading Skeleton -->
      <div v-if="loading" class="drama-grid">
        <div v-for="i in 8" :key="i" class="skeleton-card">
          <div class="skeleton-poster"></div>
          <div class="skeleton-line" style="width: 80%"></div>
        </div>
      </div>

      <!-- Results Grid -->
      <div v-else-if="dramas.length > 0" class="drama-grid animate-fade-in">
        <DramaCard v-for="drama in dramas" :key="drama.sourceId || drama.id" :drama="drama" />
      </div>

      <!-- No Results -->
      <div v-else class="empty-state">
        <Inbox :size="54" class="empty-icon" />
        <p class="empty-title">未搜索到与“{{ currentQuery }}”相关的短剧</p>
        <p class="empty-desc">建议更换或缩短关键词重新搜索</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Search, X, Inbox } from 'lucide-vue-next'
import DramaCard from '@/components/DramaCard.vue'
import { api } from '@/api'

const route = useRoute()
const router = useRouter()
const inputRef = ref(null)

const keyword = ref('')
const currentQuery = ref('')
const dramas = ref([])
const loading = ref(false)
const hasSearched = ref(false)

const hotWords = ref([
  '战神', '重生', '赘婿', '逆袭', '神豪', '豪门', '穿越', '神医', '修仙', '甜宠'
])

const doSearch = async () => {
  const q = keyword.value.trim()
  if (!q) return
  currentQuery.value = q
  hasSearched.value = true
  loading.value = true

  router.replace({ query: { keyword: q } })

  try {
    const res = await api.search(q)
    dramas.value = res.dramas || []
  } catch (err) {
    console.error(err)
    dramas.value = []
  } finally {
    loading.value = false
  }
}

const clearKeyword = () => {
  keyword.value = ''
  if (inputRef.value) inputRef.value.focus()
}

const clickHotWord = (word) => {
  keyword.value = word
  doSearch()
}

watch(() => route.query.keyword, (newVal) => {
  if (newVal && newVal !== keyword.value) {
    keyword.value = newVal
    doSearch()
  }
})

onMounted(() => {
  if (route.query.keyword) {
    keyword.value = route.query.keyword
    doSearch()
  } else if (inputRef.value) {
    inputRef.value.focus()
  }
})
</script>

<style scoped>
.search-view {
  padding-top: 32px;
  padding-bottom: 80px;
}

.search-header {
  padding: 24px;
  margin-bottom: 30px;
}

.search-box-large {
  display: flex;
  align-items: center;
  gap: 12px;
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: var(--radius-full);
  padding: 6px 6px 6px 18px;
  transition: all 0.25s ease;
}

.search-box-large:focus-within {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 4px rgba(255, 59, 92, 0.2);
}

.search-icon-lg {
  color: var(--text-muted);
  flex-shrink: 0;
}

.search-box-large input {
  flex: 1;
  background: transparent;
  color: #fff;
  font-size: 15px;
  min-width: 0;
}

.search-box-large input::placeholder {
  color: var(--text-muted);
}

.clear-btn-lg {
  color: var(--text-muted);
  padding: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.2s;
}

.clear-btn-lg:hover {
  color: #fff;
}

.search-action-btn {
  padding: 10px 24px;
  flex-shrink: 0;
}

/* Hot Searches */
.hot-searches {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 18px;
  flex-wrap: wrap;
}

.hot-label {
  font-size: 12.5px;
  color: var(--text-muted);
  font-weight: 500;
}

.hot-tags {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.hot-chip {
  font-size: 12px;
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--border-light);
  padding: 4px 12px;
  border-radius: var(--radius-full);
  transition: all 0.2s ease;
}

.hot-chip:hover {
  background: rgba(255, 59, 92, 0.15);
  color: var(--accent-primary);
  border-color: rgba(255, 59, 92, 0.3);
}

/* Results Section */
.results-bar {
  margin-bottom: 20px;
}

.results-count {
  font-size: 14px;
  color: var(--text-secondary);
}

.results-count strong {
  color: var(--accent-primary);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 70px 20px;
  color: var(--text-muted);
  text-align: center;
}

.empty-icon {
  opacity: 0.35;
  margin-bottom: 12px;
}

.empty-title {
  font-size: 16px;
  font-weight: 600;
  color: #cbd5e1;
  margin-bottom: 6px;
}

.empty-desc {
  font-size: 13px;
  color: var(--text-muted);
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

@media (max-width: 640px) {
  .search-header {
    padding: 16px;
  }
  .search-box-large {
    padding: 4px 4px 4px 12px;
  }
  .search-action-btn {
    padding: 8px 16px;
    font-size: 13px;
  }
}
</style>

