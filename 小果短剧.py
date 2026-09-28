# -*- coding: utf-8 -*-
"""
小果短剧 TVBox Python 源
支持：
  1. 精选分类：真人短剧、动态漫剧、AI短剧、精品动漫
  2. 热门榜单：热播总榜、真人榜、漫剧榜、AI剧榜
  3. 多维筛选：逆袭、都市、战神、豪门、现代、年代、家庭、悬疑等
  4. 关键词实时搜索与剧集选集
  5. 双模播放解析：
     - 桥接模式 (推荐)：配合本项目 Go 后端 (http://127.0.0.1:8080)，解锁全部集数并提供 1080P 超清解密与流式直链
     - 独立模式：无需依赖外部程序，直接抓取官方无加密直链与备用解析

TVBox 配置示例：
  {
    "key": "xiaoguo",
    "name": "小果短剧",
    "type": 3,
    "api": "小果短剧.py",
    "searchable": 1,
    "quickSearch": 1,
    "filterable": 1,
    "ext": "http://127.0.0.1:8080"
  }
"""
import base64
import json
import re
import sys
import time
import traceback
from urllib.parse import quote, unquote, urlencode, urlparse

try:
    import requests
except ImportError:
    requests = None

import urllib.error
import urllib.request

sys.path.append("../../")
try:
    from base.spider import Spider
except ImportError:
    class Spider:
        def getProxyUrl(self):
            return "http://127.0.0.1:9978/proxy?do=py&type=media"


class Spider(Spider):
    site = "https://hongguoduanju.com"
    headers = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36",
        "Accept-Language": "zh-CN,zh;q=0.9",
        "Referer": "https://hongguoduanju.com/",
    }

    def __init__(self):
        # 默认使用本项目 Go 后端地址
        self.bridge = "http://127.0.0.1:8080"
        self._bridge_alive = None
        self._bridge_check_time = 0
        self._cache = {}

    def init(self, extend=""):
        """初始化扩展参数，支持传入桥接后端地址"""
        if isinstance(extend, dict):
            self.bridge = str(extend.get("bridge") or extend.get("host") or self.bridge).rstrip("/")
        elif extend:
            text = str(extend).strip()
            try:
                data = json.loads(text)
                self.bridge = str(data.get("bridge") or data.get("host") or self.bridge).rstrip("/")
            except Exception:
                if text.startswith("http"):
                    self.bridge = text.rstrip("/")
        self._bridge_alive = None
        self._bridge_check_time = 0

    def _check_bridge(self):
        """检测 Go 后端是否可用 (带 30 秒缓存)"""
        now = time.time()
        if self._bridge_alive is not None and now - self._bridge_check_time < 30:
            return self._bridge_alive
        if not self.bridge:
            self._bridge_alive = False
            return False
        try:
            req = urllib.request.Request(self.bridge + "/api/genres", headers={"User-Agent": "TVBox"})
            with urllib.request.urlopen(req, timeout=1.5) as resp:
                self._bridge_alive = (resp.status == 200)
        except Exception:
            self._bridge_alive = False
        self._bridge_check_time = now
        return self._bridge_alive

    def _http_get(self, url, headers=None, timeout=15):
        """兼容 requests 与 urllib 的 GET 请求"""
        h = dict(self.headers)
        if headers:
            h.update(headers)
        if requests is not None:
            r = requests.get(url, headers=h, timeout=timeout)
            r.encoding = "utf-8"
            return r.text
        req = urllib.request.Request(url, headers=h)
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.read().decode("utf-8", errors="replace")

    def _extract_router_data(self, html):
        """从网页 HTML 中提取 _ROUTER_DATA JSON"""
        for marker in ("_ROUTER_DATA = ", "window._ROUTER_DATA = "):
            pos = html.find(marker)
            if pos != -1:
                pos += len(marker)
                while pos < len(html) and html[pos] in " \t\r\n":
                    pos += 1
                depth = 0
                in_s = False
                esc = False
                for i in range(pos, len(html)):
                    c = html[i]
                    if esc:
                        esc = False
                    elif c == "\\":
                        esc = True
                    elif c == '"':
                        in_s = not in_s
                    elif not in_s:
                        if c == "{":
                            depth += 1
                        elif c == "}":
                            depth -= 1
                            if depth == 0:
                                return json.loads(html[pos:i+1])
        return {}

    def _clean_cover(self, cover):
        """清理图片地址，防止 HEIC 格式导致 TVBox 无法渲染"""
        if not cover:
            return ""
        if ".heic" in cover:
            cover = cover.split("~")[0]
        if self._check_bridge():
            return f"{self.bridge}/api/proxy/image?url={quote(cover)}"
        return cover

    def homeContent(self, filter_):
        classes = [
            {"type_id": "short_play", "type_name": "真人短剧"},
            {"type_id": "comic_series", "type_name": "动态漫剧"},
            {"type_id": "ai_series", "type_name": "AI 短剧"},
            {"type_id": "comic", "type_name": "精品动漫"},
            {"type_id": "rank_hot", "type_name": "🔥 热播总榜"},
            {"type_id": "rank_real", "type_name": "🎭 真人热榜"},
            {"type_id": "rank_comic", "type_name": "🎨 漫剧热榜"},
            {"type_id": "rank_ai", "type_name": "🤖 AI 剧热榜"},
        ]
        themes = [
            {"n": "全部题材", "v": ""},
            {"n": "逆袭", "v": "comeback"},
            {"n": "都市", "v": "urban"},
            {"n": "战神", "v": "legend"},
            {"n": "豪门", "v": "clan"},
            {"n": "现代", "v": "romance"},
            {"n": "年代", "v": "period"},
            {"n": "成长", "v": "growth"},
            {"n": "家庭", "v": "family"},
            {"n": "悬疑", "v": "suspense"},
            {"n": "萌宝", "v": "cute-kids"},
            {"n": "惊悚", "v": "thriller"},
            {"n": "古装", "v": "costume"},
            {"n": "奇幻", "v": "fantasy"},
            {"n": "喜剧", "v": "comedy"},
            {"n": "动作", "v": "action-adventure"},
            {"n": "科幻", "v": "sci-fi"},
            {"n": "惊奇", "v": "wonder"},
            {"n": "青春", "v": "youth"},
        ]
        filters = {
            "short_play": [
                {"key": "theme", "name": "题材", "value": themes},
            ],
        }
        return {
            "class": classes,
            "filters": filters,
            "list": self.homeVideoContent().get("list", []),
        }

    def homeVideoContent(self):
        try:
            return self.categoryContent("short_play", "1", None, {})
        except Exception:
            traceback.print_exc()
            return {"list": []}

    def categoryContent(self, tid, pg, *a):
        page = max(1, int(pg or 1))
        tid = str(tid or "short_play")
        filters = a[1] if len(a) > 1 and isinstance(a[1], dict) else {}
        theme = filters.get("theme", "")

        # 1. 榜单分类
        if tid.startswith("rank_"):
            board_map = {
                "rank_hot": ("hongguo-hot", "hot-drama"),
                "rank_real": ("hongguo-real", "hot-real-drama"),
                "rank_comic": ("hongguo-comic", "hot-comic-drama"),
                "rank_ai": ("hongguo-ai", "hot-ai-drama"),
            }
            board_id, path = board_map.get(tid, ("hongguo-hot", "hot-drama"))

            # 优先从本地 Bridge 获取榜单
            if self._check_bridge():
                try:
                    raw = self._http_get(f"{self.bridge}/api/rankings?board={board_id}&page={page}")
                    d = json.loads(raw).get("data", {})
                    items = d.get("items", [])
                    vod_list = []
                    for it in items:
                        dr = it.get("drama", {})
                        vod_list.append({
                            "vod_id": str(dr.get("sourceId") or dr.get("id", "")).replace("hongguo:", ""),
                            "vod_name": str(dr.get("title") or dr.get("name", "")),
                            "vod_pic": self._clean_cover(dr.get("cover")),
                            "vod_remarks": str(it.get("metric") or dr.get("heat") or f"Top {it.get('rank', '')}"),
                            "vod_year": f"★{dr.get('score')}" if dr.get("score") else "",
                        })
                    return {
                        "list": vod_list,
                        "page": page,
                        "pagecount": page + (1 if d.get("hasMore") else 0),
                        "limit": 20,
                        "total": len(vod_list),
                    }
                except Exception:
                    pass

            # 独立抓取网页榜单
            try:
                addr = f"{self.site}/rank/{path}" + (f"?page={page}" if page > 1 else "")
                html = self._http_get(addr)
                arts = re.findall(r'<article[^>]*aria-labelledby="rank-title-(\d+)"[^>]*>(.*?)</article>', html, re.DOTALL)
                vod_list = []
                for sid, art in arts:
                    tm = re.search(r'id="rank-title-\d+"[^>]*>(.*?)<', art)
                    title = tm.group(1).strip() if tm else sid
                    cm = re.search(r'<img[^>]+src="([^"]+)"', art)
                    cover = cm.group(1) if cm else ""
                    hm = re.search(r'(\d+(?:\.\d+)?[万亿]?热度)', art)
                    heat = hm.group(1) if hm else ""
                    sm = re.search(r'评分\s*(\d+\.\d+)', art)
                    score = f"★{sm.group(1)}" if sm else ""
                    vod_list.append({
                        "vod_id": sid,
                        "vod_name": title,
                        "vod_pic": self._clean_cover(cover),
                        "vod_remarks": heat,
                        "vod_year": score,
                    })
                return {
                    "list": vod_list,
                    "page": page,
                    "pagecount": page + 1 if len(vod_list) >= 20 else page,
                    "limit": 20,
                    "total": len(vod_list),
                }
            except Exception:
                traceback.print_exc()
                return {"list": [], "page": page, "pagecount": page}

        # 2. 剧库分类 (真人剧 / 漫剧 / AI剧 / 动漫)
        if self._check_bridge() and not theme:
            try:
                offset = (page - 1) * 18
                raw = self._http_get(f"{self.bridge}/api/catalog?genre={tid}&offset={offset}")
                d = json.loads(raw).get("data", {})
                dramas = d.get("data", [])
                vod_list = []
                for dr in dramas:
                    cnt = dr.get("totalEpisodes") or dr.get("episodeCount")
                    remark = dr.get("remark") or (f"全{cnt}集" if cnt else "")
                    vod_list.append({
                        "vod_id": str(dr.get("sourceId") or dr.get("id", "")).replace("hongguo:", ""),
                        "vod_name": str(dr.get("title") or dr.get("name", "")),
                        "vod_pic": self._clean_cover(dr.get("cover")),
                        "vod_remarks": remark,
                        "vod_year": f"★{dr.get('score')}" if dr.get("score") else "",
                    })
                has_more = d.get("hasMore", False)
                return {
                    "list": vod_list,
                    "page": page,
                    "pagecount": page + (1 if has_more else 0),
                    "limit": 18,
                    "total": page * 18 + (18 if has_more else 0),
                }
            except Exception:
                pass

        # 独立抓取网页分类
        route_map = {
            "short_play": "real-drama",
            "comic_series": "comic-drama",
            "ai_series": "ai-drama",
            "comic": "comic",
        }
        route = route_map.get(tid, "real-drama")
        qs = [f"page={page}"]
        if theme:
            qs.append(f"theme={theme}")
        url = f"{self.site}/category/{route}?" + "&".join(qs)
        try:
            html = self._http_get(url)
            data = self._extract_router_data(html)
            page_data = data.get("loaderData", {}).get("category_$", {}) or data.get("loaderData", {}).get("category_page", {})
            items = page_data.get("recommendList", [])
            vod_list = []
            for it in items:
                sid = str(it.get("series_id") or "")
                cnt = it.get("episode_cnt") or len(it.get("vid_list") or [])
                rem = it.get("episode_right_text") or (f"全{cnt}集" if cnt else "")
                vod_list.append({
                    "vod_id": sid,
                    "vod_name": str(it.get("series_name") or it.get("title") or ""),
                    "vod_pic": self._clean_cover(it.get("series_cover") or it.get("cover")),
                    "vod_remarks": rem,
                })
            pagination = page_data.get("pagination", {})
            total_pages = int(pagination.get("totalPages") or page)
            return {
                "list": vod_list,
                "page": page,
                "pagecount": max(page, total_pages),
                "limit": len(vod_list),
                "total": total_pages * len(vod_list),
            }
        except Exception:
            traceback.print_exc()
            return {"list": [], "page": page, "pagecount": page}

    def detailContent(self, ids):
        sid = str(ids[0]).replace("hongguo:", "").strip()

        # 1. 尝试使用 Bridge 后端详情 (包含全部剧集与高清分集)
        if self._check_bridge():
            try:
                raw = self._http_get(f"{self.bridge}/api/detail?id={sid}")
                d = json.loads(raw).get("data", {})
                drama = d.get("drama", {})
                chapters = d.get("chapters", [])
                eps = []
                for i, c in enumerate(chapters):
                    idx = c.get("index") or (i + 1)
                    title = c.get("title") or f"第{idx}集"
                    vid = c.get("videoId") or ""
                    eps.append(f"{title}${sid}*{vid}")
                cnt = drama.get("totalEpisodes") or len(chapters)
                return {"list": [{
                    "vod_id": sid,
                    "vod_name": str(drama.get("title") or drama.get("name") or ""),
                    "vod_pic": self._clean_cover(drama.get("cover")),
                    "type_name": drama.get("categoryName") or ",".join(drama.get("tags") or []),
                    "vod_remarks": f"共{cnt}集",
                    "vod_year": drama.get("score", ""),
                    "vod_content": str(drama.get("desc") or ""),
                    "vod_play_from": "小果短剧",
                    "vod_play_url": "#".join(eps),
                }]}
            except Exception:
                pass

        # 2. 独立网页版详情解析
        try:
            url = f"{self.site}/detail?series_id={quote(sid)}"
            html = self._http_get(url)
            data = self._extract_router_data(html)
            s = data.get("loaderData", {}).get("detail_page", {}).get("seriesDetail", {})
            vids = [str(v) for v in (s.get("vid_list") or []) if str(v)]
            eps = [f"第{i + 1}集${sid}*{v}" for i, v in enumerate(vids)]
            tags = s.get("tags") or []
            return {"list": [{
                "vod_id": sid,
                "vod_name": str(s.get("series_name") or ""),
                "vod_pic": self._clean_cover(s.get("series_cover")),
                "type_name": ",".join(str(x) for x in tags),
                "vod_remarks": s.get("episode_right_text") or (f"全{len(vids)}集" if vids else ""),
                "vod_content": str(s.get("series_intro") or ""),
                "vod_play_from": "小果短剧",
                "vod_play_url": "#".join(eps),
            }]}
        except Exception:
            traceback.print_exc()
            return {"list": []}

    def searchContent(self, key, *a):
        keyword = str(key).strip()
        if not keyword:
            return {"list": [], "page": 1}

        # 1. 尝试使用 Bridge 后端搜索
        if self._check_bridge():
            try:
                raw = self._http_get(f"{self.bridge}/api/search?keyword={quote(keyword)}")
                entry = json.loads(raw).get("data", {})
                dramas = entry.get("dramas", [])
                vod_list = []
                for dr in dramas:
                    cnt = dr.get("totalEpisodes") or dr.get("episodeCount")
                    rem = dr.get("remark") or (f"全{cnt}集" if cnt else "")
                    vod_list.append({
                        "vod_id": str(dr.get("sourceId") or dr.get("id", "")).replace("hongguo:", ""),
                        "vod_name": str(dr.get("title") or dr.get("name", "")),
                        "vod_pic": self._clean_cover(dr.get("cover")),
                        "vod_remarks": rem,
                        "vod_year": f"★{dr.get('score')}" if dr.get("score") else "",
                    })
                if vod_list:
                    return {"list": vod_list, "page": 1}
            except Exception:
                pass

        # 2. 独立网页版搜索
        vod_list = []
        seen = set()
        try:
            url = f"{self.site}/search/{quote(keyword)}"
            html = self._http_get(url)
            data = self._extract_router_data(html)
            page = data.get("loaderData", {}).get("search_(keyword)/page", {}) or data.get("loaderData", {}).get("search_page", {})
            for it in page.get("searchList", []):
                vd = it.get("video_data") or it
                sid = str(vd.get("series_id") or it.get("keyword") or "")
                if sid and sid not in seen:
                    seen.add(sid)
                    title = vd.get("series_title") or it.get("name") or ""
                    cover = vd.get("series_cover") or ""
                    rem = vd.get("episode_right_text") or (f"全{vd.get('episode_cnt')}集" if vd.get("episode_cnt") else "")
                    vod_list.append({
                        "vod_id": sid,
                        "vod_name": title,
                        "vod_pic": self._clean_cover(cover),
                        "vod_remarks": rem,
                    })
        except Exception:
            pass

        # 3. 联想词补全候选
        if len(vod_list) < 5:
            try:
                sug_url = f"{self.site}/incent_resource/suggestion?app_id=8662&count=10&query={quote(keyword)}"
                sug_raw = self._http_get(sug_url)
                sug_json = json.loads(sug_raw)
                for item in sug_json.get("suggest_list", []):
                    vd = item.get("video_data") or {}
                    sid = str(vd.get("series_id") or "")
                    if sid and sid not in seen:
                        seen.add(sid)
                        vod_list.append({
                            "vod_id": sid,
                            "vod_name": item.get("name") or vd.get("series_title") or "",
                            "vod_pic": self._clean_cover(vd.get("series_cover")),
                            "vod_remarks": vd.get("episode_right_text") or "",
                        })
            except Exception:
                pass

        return {"list": vod_list, "page": 1}

    def searchContentPage(self, key, quick, pg=1):
        return self.searchContent(key, quick, pg)

    def playerContent(self, flag, pid, *a):
        pid = str(pid).strip()
        sid, vid = "", pid
        if "*" in pid:
            sid, vid = pid.split("*", 1)

        # 1. 如果 Bridge 后端在线，优先使用 Bridge 1080P 超清解密流
        if self._check_bridge():
            play_url = f"{self.bridge}/play?vid={quote(pid)}"
            return {
                "parse": 0,
                "playUrl": "",
                "url": play_url,
                "header": {
                    "User-Agent": self.headers["User-Agent"],
                    "Referer": self.site + "/",
                },
            }

        # 2. 独立模式：尝试获取网页版官方直链 (前几集免费剧集直接播放)
        if sid and vid:
            try:
                url = f"{self.site}/player/{sid}/{vid}"
                html = self._http_get(url)
                data = self._extract_router_data(html)
                page = data.get("loaderData", {}).get("player_(series_id)/(vid)/page", {})
                info = page.get("video_player_info", {})
                main_url = info.get("main_url")
                if main_url:
                    return {
                        "parse": 0,
                        "playUrl": "",
                        "url": main_url,
                        "header": {
                            "User-Agent": self.headers["User-Agent"],
                            "Referer": self.site + "/",
                        },
                    }
            except Exception:
                pass

        # 3. 备用接口解析
        try:
            ref = json.dumps({
                "content_type": 1004,
                "series_id": sid or vid,
                "vid": vid,
                "video_platform": 3,
            })
            b64_id = base64.b64encode(ref.encode("utf-8")).decode("ascii")
            backup_api = f"https://djapi.999888456.xyz/api/hongguo/play?id={quote(b64_id)}"
            resp_text = self._http_get(backup_api)
            if not resp_text.startswith("v2."):
                res_obj = json.loads(resp_text)
                for opt in res_obj.get("key_urls", []):
                    u = opt.get("src", "")
                    if u.startswith("http"):
                        return {
                            "parse": 0,
                            "playUrl": "",
                            "url": u,
                            "header": {"User-Agent": self.headers["User-Agent"]},
                        }
        except Exception:
            pass

        # 兜底：返回 Bridge 播放地址
        return {
            "parse": 0,
            "playUrl": "",
            "url": f"{self.bridge}/play?vid={quote(pid)}",
            "header": {
                "User-Agent": self.headers["User-Agent"],
                "Referer": self.site + "/",
            },
        }

    def localProxy(self, params):
        return None

    def isVideoFormat(self, url):
        return ".mp4" in url or ".m3u8" in url or "video_mp4" in url

    def manualVideoCheck(self):
        return False

    def destroy(self):
        self._cache.clear()

    def getName(self):
        return "小果短剧"

