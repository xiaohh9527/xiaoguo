<template>
  <div class="drama-card" @click="goToPlayer">
    <div class="poster-container">
      <img
        :src="coverUrl"
        :alt="drama.title"
        class="poster-img"
        loading="lazy"
        referrerpolicy="no-referrer"
        @error="onImageError"
      />
      
      <!-- Top badges -->
      <div class="top-badges">
        <span v-if="drama.score" class="badge badge-score">
          ★ {{ drama.score }}
        </span>
        <span v-if="drama.categoryName" class="badge badge-category">
          {{ drama.categoryName }}
        </span>
      </div>

      <!-- Bottom badge -->
      <div class="bottom-badges">
        <span v-if="drama.remark || drama.totalEpisodes" class="episodes-badge">
          {{ drama.remark || `共${drama.totalEpisodes}集` }}
        </span>
      </div>

      <!-- Hover Play Overlay -->
      <div class="play-overlay">
        <div class="play-circle">
          <Play :size="24" fill="currentColor" class="play-icon" />
        </div>
      </div>
    </div>

    <!-- Metadata -->
    <div class="card-info">
      <h3 class="card-title" :title="drama.title || drama.name">
        {{ drama.title || drama.name }}
      </h3>
      <div class="card-meta">
        <span v-if="drama.heat" class="heat-text">
          <Flame :size="12" />
          {{ drama.heat }}
        </span>
        <span v-else-if="drama.views" class="views-text">
          {{ drama.views }} 次播放
        </span>
        <span v-if="drama.releaseStatus === 'finished'" class="status-finished">
          完结
        </span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { Play, Flame } from 'lucide-vue-next'
import { api } from '@/api'

const props = defineProps({
  drama: {
    type: Object,
    required: true,
  },
})

const router = useRouter()

const coverUrl = computed(() => {
  return api.getImageProxyUrl(props.drama.cover)
})

const onImageError = (e) => {
  e.target.onerror = null
  // Fallback to stylized SVG placeholder
  e.target.src = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='300' height='400' viewBox='0 0 300 400'%3E%3Crect fill='%231a2030' width='300' height='400'/%3E%3Ctext fill='%23475569' font-size='20' font-family='sans-serif' x='50%25' y='50%25' dominant-baseline='middle' text-anchor='middle'%3E小果短剧%3C/text%3E%3C/svg%3E"
}

const goToPlayer = () => {
  const id = props.drama.sourceId || props.drama.id.replace('hongguo:', '')
  router.push(`/player/${id}`)
}
</script>

<style scoped>
.drama-card {
  display: flex;
  flex-direction: column;
  cursor: pointer;
  border-radius: var(--radius-md);
  transition: transform 0.3s cubic-bezier(0.2, 0, 0, 1);
  user-select: none;
}

.drama-card:hover {
  transform: translateY(-4px);
}

.poster-container {
  position: relative;
  width: 100%;
  aspect-ratio: 3 / 4;
  border-radius: var(--radius-md);
  overflow: hidden;
  background: #141a29;
  border: 1px solid var(--border-light);
  box-shadow: var(--shadow-sm);
  transition: all 0.3s ease;
}

.drama-card:hover .poster-container {
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.5), 0 0 0 1px rgba(255, 59, 92, 0.4);
}

.poster-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.45s ease;
}

.drama-card:hover .poster-img {
  transform: scale(1.05);
}

.top-badges {
  position: absolute;
  top: 8px;
  left: 8px;
  right: 8px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 6px;
  pointer-events: none;
  z-index: 2;
}

.bottom-badges {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  padding: 24px 8px 8px;
  background: linear-gradient(to top, rgba(11, 14, 20, 0.9) 0%, transparent 100%);
  display: flex;
  justify-content: flex-end;
  pointer-events: none;
  z-index: 2;
}

.episodes-badge {
  font-size: 11px;
  color: #e2e8f0;
  font-weight: 500;
  background: rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(4px);
  padding: 2px 7px;
  border-radius: 4px;
}

/* Play Overlay */
.play-overlay {
  position: absolute;
  inset: 0;
  background: rgba(11, 14, 20, 0.45);
  backdrop-filter: blur(2px);
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.25s ease;
  z-index: 3;
}

.drama-card:hover .play-overlay {
  opacity: 1;
}

.play-circle {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: var(--accent-gradient);
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 20px rgba(255, 59, 92, 0.6);
  transform: scale(0.85);
  transition: transform 0.25s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}

.drama-card:hover .play-circle {
  transform: scale(1);
}

.play-icon {
  margin-left: 2px;
}

/* Metadata */
.card-info {
  padding: 10px 4px 4px;
}

.card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: color 0.2s ease;
}

.drama-card:hover .card-title {
  color: var(--accent-primary);
}

.card-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-muted);
}

.heat-text {
  display: flex;
  align-items: center;
  gap: 3px;
  color: #ff5470;
  font-size: 11.5px;
  font-weight: 500;
}

.views-text {
  font-size: 11px;
}

.status-finished {
  font-size: 10px;
  color: #10b981;
  background: rgba(16, 185, 129, 0.12);
  padding: 1px 5px;
  border-radius: 3px;
}

@media (max-width: 640px) {
  .card-title {
    font-size: 13px;
  }
  .card-meta {
    font-size: 11px;
  }
}
</style>


