import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '@/views/HomeView.vue'
import RankingsView from '@/views/RankingsView.vue'
import CategoriesView from '@/views/CategoriesView.vue'
import SearchView from '@/views/SearchView.vue'
import PlayerView from '@/views/PlayerView.vue'
import FavoritesView from '@/views/FavoritesView.vue'
import HistoryView from '@/views/HistoryView.vue'

const routes = [
  {
    path: '/',
    name: 'Home',
    component: HomeView,
    meta: { title: '小果短剧 - 精选推荐' },
  },
  {
    path: '/rankings',
    name: 'Rankings',
    component: RankingsView,
    meta: { title: '热度榜单 - 小果短剧' },
  },
  {
    path: '/categories',
    name: 'Categories',
    component: CategoriesView,
    meta: { title: '分类剧库 - 小果短剧' },
  },
  {
    path: '/search',
    name: 'Search',
    component: SearchView,
    meta: { title: '短剧搜索 - 小果短剧' },
  },
  {
    path: '/player/:id',
    name: 'Player',
    component: PlayerView,
    meta: { title: '播放详情 - 小果短剧' },
  },
  {
    path: '/favorites',
    name: 'Favorites',
    component: FavoritesView,
    meta: { title: '我的追剧 - 小果短剧' },
  },
  {
    path: '/history',
    name: 'History',
    component: HistoryView,
    meta: { title: '播放历史 - 小果短剧' },
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/',
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) {
      return savedPosition
    }
    return { top: 0 }
  },
})

router.beforeEach((to, from, next) => {
  if (to.meta.title) {
    document.title = to.meta.title
  }
  next()
})

export default router

