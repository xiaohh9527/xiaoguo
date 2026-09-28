<template>
  <div class="favorites-view container">
    <div class="page-header">
      <div class="header-info">
        <h1 class="page-title">
          <Bookmark :size="26" class="title-icon" />
          <span>我的追剧清单</span>
        </h1>
        <p class="page-subtitle">共收藏 {{ favorites.length }} 部短剧</p>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="drama-grid">
      <div v-for="i in 6" :key="i" class="skeleton-card">
        <div class="skeleton-poster"></div>
        <div class="skeleton-line" style="width: 80%"></div>
      </div>
    </div>

    <!-- Favorites Grid -->
    <div v-else-if="favorites.length > 0" class="drama-grid animate-fade-in">
      <div
        v-for="item in favorites"
        :key="item.sourceId"
        class="fav-card-wrap"
      >
        <DramaCard
          :drama="{
            id: 'hongguo:' + item.sourceId,
            sourceId: item.sourceId,
            title: item.title,
            cover: item.cover,
            totalEpisodes: item.episodes,
            categoryName: item.category,
          }"
        />
        <button class="remove-fav-btn" @click.stop="removeFav(item.sourceId)" title="取消追剧">
          <Trash2 :size="14" />
        </button>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else class="empty-state">
      <Bookmark :size="54" class="empty-icon" />
      <p class="empty-title">追剧清单还是空的</p>
      <p class="empty-desc">在短剧播放页点击“追剧”即可收藏到这里</p>
      <router-link to="/" class="btn-primary">
        <span>去浏览热门短剧</span>
      </router-link>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { Bookmark, Trash2 } from 'lucide-vue-next'
import DramaCard from '@/components/DramaCard.vue'
import { api } from '@/api'

const favorites = ref([])
const loading = ref(true)

const loadFavorites = async () => {
  loading.value = true
  try {
    const res = await api.getFavorites()
    favorites.value = Array.isArray(res) ? res : []
  } catch (err) {
    console.error(err)
    favorites.value = []
  } finally {
    loading.value = false
  }
}

const removeFav = async (sourceId) => {
  try {
    await api.removeFavorite(sourceId)
    favorites.value = favorites.value.filter(item => item.sourceId !== sourceId)
  } catch (err) {
    console.error(err)
  }
}

onMounted(() => {
  loadFavorites()
})
</script>

<style scoped>
.favorites-view {
  padding-top: 28px;
  padding-bottom: 80px;
}

.page-header {
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

.fav-card-wrap {
  position: relative;
}

.remove-fav-btn {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.65);
  backdrop-filter: blur(4px);
  color: #cbd5e1;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: all 0.2s ease;
  z-index: 5;
}

.fav-card-wrap:hover .remove-fav-btn {
  opacity: 1;
}

.remove-fav-btn:hover {
  background: var(--accent-primary);
  color: #fff;
  transform: scale(1.1);
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
