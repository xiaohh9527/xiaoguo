<template>
  <nav class="mobile-tabs">
    <router-link to="/" class="tab-item" active-class="tab-active" exact>
      <Compass :size="20" />
      <span>精选</span>
    </router-link>
    <router-link to="/rankings" class="tab-item" active-class="tab-active">
      <Flame :size="20" />
      <span>榜单</span>
    </router-link>
    <router-link to="/categories" class="tab-item" active-class="tab-active">
      <LayoutGrid :size="20" />
      <span>分类</span>
    </router-link>
    <router-link to="/search" class="tab-item" active-class="tab-active">
      <Search :size="20" />
      <span>搜索</span>
    </router-link>
    <router-link to="/favorites" class="tab-item" active-class="tab-active">
      <Bookmark :size="20" />
      <span>追剧</span>
    </router-link>
  </nav>
</template>

<script setup>
import { Compass, Flame, LayoutGrid, Search, Bookmark } from 'lucide-vue-next'
</script>

<style scoped>
.mobile-tabs {
  display: none;
}

@media (max-width: 900px) {
  .mobile-tabs {
    display: flex;
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    height: 56px;
    background: rgba(11, 14, 20, 0.92);
    backdrop-filter: blur(20px);
    -webkit-backdrop-filter: blur(20px);
    border-top: 1px solid rgba(255, 255, 255, 0.08);
    z-index: 99;
    align-items: center;
    justify-content: space-around;
    padding-bottom: env(safe-area-inset-bottom);
  }

  .tab-item {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 3px;
    color: var(--text-muted);
    font-size: 11px;
    font-weight: 500;
    text-decoration: none;
    padding: 6px 12px;
    transition: all 0.2s ease;
  }

  .tab-item:active {
    transform: scale(0.92);
  }

  .tab-active {
    color: var(--accent-primary) !important;
    font-weight: 600;
  }
}
</style>
