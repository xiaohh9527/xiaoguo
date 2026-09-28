<template>
  <header class="navbar" :class="{ 'navbar-scrolled': isScrolled }">
    <div class="container navbar-content">
      <!-- Logo -->
      <router-link to="/" class="logo-group">
        <div class="logo-icon">
          <svg class="fruit-svg" viewBox="0 0 32 32" fill="none">
            <path d="M16 4C14.5 4 13.5 5 14 7C14.5 9 16 11 16 11C16 11 17.5 9 18 7C18.5 5 17.5 4 16 4Z" fill="#22c55e" />
            <path d="M16 7C9 7 5 13 5 19C5 25 10 28 16 28C22 28 27 25 27 19C27 13 23 7 16 7Z" fill="url(#grad)" />
            <polygon points="14,15 20,19 14,23" fill="#ffffff" />
            <defs>
              <linearGradient id="grad" x1="5" y1="7" x2="27" y2="28" gradientUnits="userSpaceOnUse">
                <stop stop-color="#ff3b5c" />
                <stop offset="1" stop-color="#ff7b54" />
              </linearGradient>
            </defs>
          </svg>
        </div>
        <div class="logo-text">
          <span class="brand-title">小果短剧</span>
          <span class="brand-badge">WEB</span>
        </div>
      </router-link>

      <!-- Desktop Nav Links -->
      <nav class="nav-links">
        <router-link to="/" class="nav-item" active-class="nav-active" exact>
          <Compass :size="17" />
          <span>精选推荐</span>
        </router-link>
        <router-link to="/rankings" class="nav-item" active-class="nav-active">
          <Flame :size="17" />
          <span>热度榜单</span>
        </router-link>
        <router-link to="/categories" class="nav-item" active-class="nav-active">
          <LayoutGrid :size="17" />
          <span>分类剧库</span>
        </router-link>
        <router-link to="/favorites" class="nav-item" active-class="nav-active">
          <Bookmark :size="17" />
          <span>我的追剧</span>
        </router-link>
        <router-link to="/history" class="nav-item" active-class="nav-active">
          <Clock :size="17" />
          <span>播放历史</span>
        </router-link>
      </nav>

      <!-- Search Box with instant suggestions -->
      <div class="search-container" ref="searchContainerRef">
        <div class="search-input-wrapper" :class="{ 'search-focused': isSearchFocused }">
          <Search :size="16" class="search-icon" />
          <input
            ref="searchInputRef"
            type="text"
            v-model="searchQuery"
            placeholder="搜索短剧、主演、题材..."
            @focus="onSearchFocus"
            @input="onSearchInput"
            @keydown.enter="submitSearch"
            @keydown.esc="closeSuggestions"
          />
          <button v-if="searchQuery" class="clear-btn" @click="clearSearch">
            <X :size="14" />
          </button>
        </div>

        <!-- Autocomplete Suggestions Dropdown -->
        <div v-if="showSuggestions && suggestions.length > 0" class="suggestions-dropdown glass-panel">
          <div class="suggestions-header">
            <span>搜索联想</span>
          </div>
          <div
            v-for="(item, idx) in suggestions"
            :key="idx"
            class="suggestion-item"
            @mousedown.prevent="selectSuggestion(item)"
          >
            <Search :size="14" class="item-icon" />
            <span class="item-name">{{ item.name }}</span>
            <span v-if="item.type === 'short_play_name'" class="item-tag">短剧</span>
          </div>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { Compass, Flame, LayoutGrid, Bookmark, Clock, Search, X } from 'lucide-vue-next'
import { api } from '@/api'

const router = useRouter()
const isScrolled = ref(false)
const searchQuery = ref('')
const isSearchFocused = ref(false)
const showSuggestions = ref(false)
const suggestions = ref([])
const searchContainerRef = ref(null)
const searchInputRef = ref(null)
let suggestDebounceTimer = null

const handleScroll = () => {
  isScrolled.value = window.scrollY > 20
}

const onSearchFocus = () => {
  isSearchFocused.value = true
  if (searchQuery.value.trim()) {
    showSuggestions.value = true
    loadSuggestions()
  }
}

const onSearchInput = () => {
  clearTimeout(suggestDebounceTimer)
  if (!searchQuery.value.trim()) {
    suggestions.value = []
    showSuggestions.value = false
    return
  }
  suggestDebounceTimer = setTimeout(() => {
    loadSuggestions()
  }, 200)
}

const loadSuggestions = async () => {
  if (!searchQuery.value.trim()) return
  try {
    const list = await api.getSuggestions(searchQuery.value.trim())
    suggestions.value = list || []
    showSuggestions.value = suggestions.value.length > 0
  } catch (err) {
    suggestions.value = []
  }
}

const selectSuggestion = (item) => {
  searchQuery.value = item.name
  showSuggestions.value = false
  submitSearch()
}

const submitSearch = () => {
  const q = searchQuery.value.trim()
  if (!q) return
  showSuggestions.value = false
  if (searchInputRef.value) {
    searchInputRef.value.blur()
  }
  router.push({ path: '/search', query: { keyword: q } })
}

const clearSearch = () => {
  searchQuery.value = ''
  suggestions.value = []
  showSuggestions.value = false
}

const closeSuggestions = () => {
  showSuggestions.value = false
}

const handleClickOutside = (e) => {
  if (searchContainerRef.value && !searchContainerRef.value.contains(e.target)) {
    showSuggestions.value = false
    isSearchFocused.value = false
  }
}

onMounted(() => {
  window.addEventListener('scroll', handleScroll, { passive: true })
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  window.removeEventListener('scroll', handleScroll)
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.navbar {
  position: sticky;
  top: 0;
  z-index: 100;
  height: 64px;
  background: rgba(11, 14, 20, 0.7);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  transition: all 0.3s ease;
}

.navbar-scrolled {
  background: rgba(11, 14, 20, 0.94);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4);
}

.navbar-content {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
}

/* Logo */
.logo-group {
  display: flex;
  align-items: center;
  gap: 10px;
  text-decoration: none;
  flex-shrink: 0;
}

.logo-icon {
  width: 38px;
  height: 38px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 59, 92, 0.1);
  border: 1px solid rgba(255, 59, 92, 0.25);
  border-radius: 12px;
  transition: transform 0.3s ease;
}

.logo-group:hover .logo-icon {
  transform: scale(1.06);
}

.fruit-svg {
  width: 26px;
  height: 26px;
  filter: drop-shadow(0 2px 6px rgba(255, 59, 92, 0.5));
}

.logo-text {
  display: flex;
  align-items: center;
  gap: 6px;
}

.brand-title {
  font-size: 19px;
  font-weight: 800;
  letter-spacing: -0.3px;
  background: linear-gradient(120deg, #ffffff 0%, #cbd5e1 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.brand-badge {
  font-size: 10px;
  font-weight: 700;
  padding: 1px 6px;
  background: var(--accent-gradient);
  color: white;
  border-radius: 6px;
  letter-spacing: 0.5px;
}

/* Nav Links */
.nav-links {
  display: flex;
  align-items: center;
  gap: 6px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 7px 14px;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-secondary);
  border-radius: var(--radius-full);
  transition: all 0.2s ease;
}

.nav-item:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.06);
}

.nav-active {
  color: #fff !important;
  background: rgba(255, 59, 92, 0.15) !important;
  border: 1px solid rgba(255, 59, 92, 0.3);
  font-weight: 600;
}

/* Search Container */
.search-container {
  position: relative;
  width: 280px;
  flex-shrink: 0;
}

.search-input-wrapper {
  display: flex;
  align-items: center;
  gap: 9px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: var(--radius-full);
  padding: 8px 14px;
  transition: all 0.25s ease;
}

.search-input-wrapper:hover {
  border-color: rgba(255, 255, 255, 0.2);
  background: rgba(255, 255, 255, 0.08);
}

.search-focused {
  border-color: var(--accent-primary) !important;
  background: rgba(22, 28, 42, 0.95) !important;
  box-shadow: 0 0 0 3px rgba(255, 59, 92, 0.2);
}

.search-icon {
  color: var(--text-muted);
  flex-shrink: 0;
}

.search-input-wrapper input {
  width: 100%;
  color: var(--text-primary);
  font-size: 13.5px;
  background: transparent;
}

.search-input-wrapper input::placeholder {
  color: var(--text-muted);
}

.clear-btn {
  color: var(--text-muted);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 2px;
  border-radius: 50%;
  transition: color 0.2s;
}

.clear-btn:hover {
  color: var(--text-primary);
}

/* Suggestions Dropdown */
.suggestions-dropdown {
  position: absolute;
  top: calc(100% + 8px);
  left: 0;
  right: 0;
  padding: 8px;
  box-shadow: var(--shadow-lg);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-md);
  z-index: 150;
  animation: fadeIn 0.2s ease forwards;
}

.suggestions-header {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  padding: 6px 10px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  margin-bottom: 4px;
}

.suggestion-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: var(--radius-sm);
  font-size: 13px;
  cursor: pointer;
  transition: background 0.15s ease;
}

.suggestion-item:hover {
  background: rgba(255, 255, 255, 0.08);
}

.item-icon {
  color: var(--text-muted);
  flex-shrink: 0;
}

.item-name {
  flex: 1;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-tag {
  font-size: 10px;
  padding: 1px 5px;
  background: rgba(255, 59, 92, 0.15);
  color: var(--accent-primary);
  border-radius: 4px;
}

@media (max-width: 900px) {
  .nav-links {
    display: none;
  }
  .search-container {
    width: 200px;
  }
}

@media (max-width: 500px) {
  .search-container {
    width: 150px;
  }
  .search-input-wrapper {
    padding: 6px 10px;
  }
}
</style>

