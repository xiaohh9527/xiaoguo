// API service for Hongguo Web

const API_BASE = '/api'

async function request(path, options = {}) {
  const url = path.startsWith('http') ? path : `${API_BASE}${path}`
  try {
    const res = await fetch(url, {
      headers: {
        'Accept': 'application/json',
        ...(options.headers || {}),
      },
      ...options,
    })
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}: ${res.statusText}`)
    }
    const json = await res.json()
    if (json.code !== 0) {
      throw new Error(json.message || '请求失败')
    }
    return json.data
  } catch (err) {
    console.error(`API Error [${path}]:`, err)
    throw err
  }
}

export const api = {
  // 分类与推荐
  getGenres: () => request('/genres'),
  getCatalog: ({ genre = 'short_play', offset = 0, session_id = '', seen = [] }) => {
    const params = new URLSearchParams({
      genre,
      offset: String(offset),
      session_id,
      seen: seen.join(','),
    })
    return request(`/catalog?${params.toString()}`)
  },

  // 榜单
  getRankingBoards: () => request('/ranking-boards'),
  getRankings: (board = 'hongguo-hot', page = 1) => {
    const params = new URLSearchParams({ board, page: String(page) })
    return request(`/rankings?${params.toString()}`)
  },

  // 剧集详情与分集
  getDetail: (id) => {
    const params = new URLSearchParams({ id })
    return request(`/detail?${params.toString()}`)
  },

  // 播放解析
  getPlay: (seriesId, videoId) => {
    const params = new URLSearchParams({ series_id: seriesId, video_id: videoId })
    return request(`/play?${params.toString()}`)
  },

  // 视频流地址
  getStreamUrl: (seriesId, videoId, quality, download = false) => {
    const params = new URLSearchParams({
      series_id: seriesId,
      video_id: videoId,
      quality: quality ? String(quality) : '',
      download: download ? '1' : '0',
    })
    return `${API_BASE}/stream?${params.toString()}`
  },

  // 弹幕
  getDanmaku: (seriesId, videoId, startMs = 0, durationMs = 600000) => {
    const params = new URLSearchParams({
      series_id: seriesId,
      video_id: videoId,
      start_ms: String(startMs),
      duration_ms: String(durationMs),
    })
    return request(`/danmaku?${params.toString()}`)
  },

  // 搜索与联想词
  search: (keyword) => {
    const params = new URLSearchParams({ keyword })
    return request(`/search?${params.toString()}`)
  },
  getSuggestions: (query) => {
    const params = new URLSearchParams({ query })
    return request(`/search/suggestions?${params.toString()}`)
  },

  // 图片代理 (防止第三方图片跨域或防盗链)
  getImageProxyUrl: (url) => {
    if (!url) return ''
    if (url.startsWith('/api/proxy/image') || url.startsWith('data:')) {
      return url
    }
    const shouldProxy =
      url.includes('byteimg.com') ||
      url.includes('fqnovel') ||
      url.includes('snssdk.com') ||
      url.includes('toutiaovod.com') ||
      url.includes('pstatp.com')
    if (shouldProxy) {
      return `${API_BASE}/proxy/image?url=${encodeURIComponent(url)}`
    }
    return url
  },

  // 收藏 / 追剧
  getFavorites: async () => {
    const res = await request('/favorites')
    return Array.isArray(res) ? res : []
  },
  addFavorite: (item) => request('/favorites', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(item),
  }),
  removeFavorite: (id) => request(`/favorites?id=${encodeURIComponent(id)}`, {
    method: 'DELETE',
  }),

  // 观看历史
  getHistory: async () => {
    const res = await request('/history')
    return Array.isArray(res) ? res : []
  },
  saveHistory: (item) => request('/history', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(item),
  }),
  clearHistory: () => request('/history', {
    method: 'DELETE',
  }),
}

