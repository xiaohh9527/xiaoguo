<template>
  <div class="app-layout">
    <Navbar />
    <router-view v-slot="{ Component }">
      <transition name="fade" mode="out-in">
        <component :is="Component" />
      </transition>
    </router-view>
    <MobileTabs />

    <!-- Desktop Footer -->
    <footer class="app-footer">
      <div class="container footer-content">
        <p class="footer-copy">
          © 2026 小果短剧 Web 应用 · 提取自官方接口 · 仅供学习与交流使用
        </p>
        <div class="footer-links">
          <span>Go 1.23 + Vue 3 极速架构</span>
          <span>·</span>
          <span>CENC 17ms 秒级解密</span>
          <span>·</span>
          <span>1080P 高清原画</span>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup>
import Navbar from '@/components/Navbar.vue'
import MobileTabs from '@/components/MobileTabs.vue'
</script>

<style>
.app-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-enter-from {
  opacity: 0;
  transform: translateY(6px);
}

.fade-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

.app-footer {
  margin-top: auto;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(11, 14, 20, 0.95);
  padding: 24px 0;
}

.footer-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  text-align: center;
  font-size: 12px;
  color: var(--text-muted);
}

.footer-links {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-muted);
  opacity: 0.75;
}

@media (max-width: 900px) {
  .app-footer {
    padding-bottom: 74px; /* Space for MobileTabs */
  }
}
</style>

