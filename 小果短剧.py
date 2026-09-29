# -*- coding: utf-8 -*-
"""
小果短剧 (Xiaoguo) AList-TvBox 插件 Python 源
适配规范：AList-TvBox / TVBox Python Spider 插件规范
支持特性：
  - 完整桥接模式：配合小主机 xiaoguo 后端 (默认 http://127.0.0.1:8080) 输出 1080P 超清解密流
  - 扩展配置 (extend)：支持在 AList-TvBox 插件设置中自定义 backend 地址
  - 免后端自动回退：桥接后端未启动时，自动降级至官方网页通道
  - 全集解锁、实时搜索、排行榜单、精细选集

AList-TvBox 插件配置参数 (ext)：
  "http://192.168.1.100:8080"
  或 JSON 格式：
  {"site": "http://192.168.1.100:8080"}
"""
import json
import os
import re
import sys
import time
import traceback
from urllib.parse import quote, unquote, urlencode

try:
    import requests
except ImportError:
    requests = None

import urllib.error
import urllib.request

try:
    from base.spider import Spider as BaseSpider
except Exception:
    class BaseSpider:
        pass


class Spider(BaseSpider):
    def __init__(self):
        # 默认桥接后端地址 (可在 AList-TvBox 中通过 extend 参数配置)
        self.host = "http://127.0.0.1:8080"
        self.api = self.host + "/api"
        self.name = "小果短剧"
        self.web_site = "https://hongguoduanju.com"
        self.headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
            "Referer": self.host + "/",
        }
        if requests is not None:
            self.session = requests.Session()
            self.session.headers.update(self.headers)
        else:
            self.session = None
        self.class_cache = None
        self.filter_cache = {}

    def init(self, extend=""):
        """AList-TvBox 插件初始化，解析 extend 配置"""
        if extend:
            try:
                cfg = json.loads(extend) if isinstance(extend, str) else extend
                if isinstance(cfg, dict):
                    self.host = (cfg.get("site") or cfg.get("host") or cfg.get("bridge") or cfg.get("base_url") or self.host).rstrip("/")
                elif str(extend).strip().startswith("http"):
                    self.host = str(extend).strip().rstrip("/")
            except Exception:
                if str(extend).strip().startswith("http"):
                    self.host = str(extend).strip().rstrip("/")
            self.api = self.host + "/api"
            self.headers["Referer"] = self.host + "/"
            if self.session is not None:
                self.session.headers.update(self.headers)

    def getName(self):
        return self.name

    def isAdult(self):
        return False

    def homeContent(self, filter):
        classes = self._classes()
        data = self._api("/catalog", {"genre": "short_play", "offset": "0"})
        items = self._list(data)
        return {
            "class": classes,
            "filters": self._filters(classes),
            "list": [self._vod(x) for x in items],
            "parse": 0,
            "jx": 0,
        }

    def categoryContent(self, tid, pg, filter, extend):
        extend = extend or {}
        page = max(1, self._int(pg, 1))
        tid = str(tid or "short_play")

        # 1. 榜单分类
        if tid.startswith("rank_"):
            board_map = {
                "rank_hot": "hongguo-hot",
                "rank_real": "hongguo-real",
                "rank_comic": "hongguo-comic",
                "rank_ai": "hongguo-ai",
            }
            board = board_map.get(tid, "hongguo-hot")
            data = self._api("/rankings", {"board": board, "page": str(page)})
            items = data.get("items", []) if isinstance(data, dict) else []
            vod_list = []
            for it in items:
                dr = it.get("drama", {}) if isinstance(it, dict) else {}
                sid = self._sid(dr.get("sourceId") or dr.get("id"))
                vod_list.append({
                    "vod_id": sid,
                    "vod_name": str(dr.get("title") or dr.get("name") or sid),
                    "vod_pic": self._pic(dr),
                    "vod_remarks": str(it.get("metric") or dr.get("heat") or f"Top {it.get('rank', '')}"),
                    "vod_year": f"★{dr.get('score')}" if dr.get("score") else "",
                })
            has_more = isinstance(data, dict) and data.get("hasMore", False)
            return {
                "page": page,
                "pagecount": page + (1 if has_more else 0),
                "limit": 20,
                "total": 99999,
                "list": vod_list,
                "parse": 0,
                "jx": 0,
            }

        # 2. 剧库分类 (真人剧 / 漫剧 / AI剧 / 动漫)
        offset = (page - 1) * 18
        data = self._api("/catalog", {"genre": tid, "offset": str(offset)})
        items = self._list(data)
        vod_list = [self._vod(x) for x in items]
        has_more = isinstance(data, dict) and data.get("hasMore", False)
        return {
            "page": page,
            "pagecount": page + (1 if has_more else 0),
            "limit": 18,
            "total": 99999,
            "list": vod_list,
            "parse": 0,
            "jx": 0,
        }

    def detailContent(self, ids):
        vid = self._sid(ids[0])
        data = self._api("/detail", {"id": vid})
        if not isinstance(data, dict):
            return {"list": [], "parse": 0, "jx": 0}

        drama = data.get("drama", {}) if isinstance(data, dict) else {}
        chapters = data.get("chapters", []) if isinstance(data, dict) else []
        name = drama.get("title") or drama.get("name") or vid
        count = drama.get("totalEpisodes") or len(chapters) or 1

        play = []
        if chapters:
            for i, c in enumerate(chapters, 1):
                idx = c.get("index") or i
                title = c.get("title") or ("第%s集" % idx)
                chapter_vid = str(c.get("videoId") or "")
                play.append("%s$%s|%s" % (title, vid, chapter_vid))
        else:
            play = ["第%s集$%s|%s" % (i, vid, i) for i in range(1, count + 1)]

        vod = {
            "vod_id": vid,
            "vod_name": name,
            "vod_pic": self._pic(drama),
            "type_name": drama.get("categoryName") or ",".join(drama.get("tags") or []),
            "vod_year": f"★{drama.get('score')}" if drama.get("score") else "",
            "vod_area": "中国大陆",
            "vod_remarks": drama.get("remark") or ("全%s集" % count),
            "vod_actor": "",
            "vod_director": "",
            "vod_content": drama.get("desc") or name,
            "vod_play_from": self.name,
            "vod_play_url": "#".join(play),
        }
        return {"list": [vod], "parse": 0, "jx": 0}

    def searchContent(self, key, quick, pg="1"):
        data = self._api("/search", {"keyword": str(key)})
        items = data.get("dramas", []) if isinstance(data, dict) else self._list(data)
        vod_list = [self._vod(x) for x in items]
        return {
            "page": int(pg),
            "pagecount": 1,
            "limit": 18,
            "total": len(vod_list),
            "list": vod_list,
            "parse": 0,
            "jx": 0,
        }

    def playerContent(self, flag, id, vipFlags):
        """播放解析，输出桥接流直链"""
        # id 格式为 sid|vid 或 vid
        url = "%s/play?vid=%s" % (self.host, quote(str(id)))
        return {
            "parse": 0,
            "playUrl": "",
            "url": url,
            "jx": 0,
            "header": {
                "User-Agent": self.headers["User-Agent"],
                "Referer": self.host + "/",
            },
        }

    def localProxy(self, params):
        return None

    def isVideoFormat(self, url):
        return ".mp4" in url or ".m3u8" in url or "video_mp4" in url

    def manualVideoCheck(self):
        return False

    def destroy(self):
        self.class_cache = None
        self.filter_cache.clear()

    # ==========================================
    # 内部请求与数据处理封装
    # ==========================================
    def _api(self, path, params=None):
        """优先调用桥接后端接口，失败时自动走网页回退"""
        path = "/" + path.lstrip("/")
        # 1. 尝试桥接后端 (self.api)
        try:
            url = self.api + path
            if self.session is not None:
                r = self.session.get(url, params=params, timeout=6)
                if r.status_code == 200:
                    res = r.json()
                    if res.get("code") == 0:
                        return res.get("data", res)
            else:
                qs = ("?" + urlencode(params)) if params else ""
                req = urllib.request.Request(url + qs, headers=self.headers)
                with urllib.request.urlopen(req, timeout=6) as resp:
                    if resp.status == 200:
                        res = json.loads(resp.read().decode("utf-8"))
                        if res.get("code") == 0:
                            return res.get("data", res)
        except Exception:
            pass

        # 2. 桥接后端不可用时，网页通道自动回退
        return self._web_fallback(path, params)

    def _web_fallback(self, path, params=None):
        """网页官方通道回退抓取"""
        params = params or {}
        # 分类回退
        if path == "/catalog":
            genre = params.get("genre", "short_play")
            offset = self._int(params.get("offset", 0))
            page = (offset // 18) + 1
            route_map = {
                "short_play": "real-drama",
                "comic_series": "comic-drama",
                "ai_series": "ai-drama",
                "comic": "comic",
            }
            route = route_map.get(genre, "real-drama")
            html = self._raw_get(f"{self.web_site}/category/{route}?page={page}")
            data = self._extract_router(html)
            page_data = data.get("loaderData", {}).get("category_$", {}) or data.get("loaderData", {}).get("category_page", {})
            rows = page_data.get("recommendList", [])
            dramas = []
            for r in rows:
                dramas.append({
                    "sourceId": str(r.get("series_id") or ""),
                    "title": r.get("series_name") or r.get("title") or "",
                    "cover": r.get("series_cover") or r.get("cover") or "",
                    "totalEpisodes": r.get("episode_cnt"),
                    "remark": r.get("episode_right_text"),
                })
            return {"data": dramas, "hasMore": len(rows) >= 18}

        # 榜单回退
        if path == "/rankings":
            board = params.get("board", "hongguo-hot")
            page = params.get("page", "1")
            path_map = {
                "hongguo-hot": "hot-drama",
                "hongguo-real": "hot-real-drama",
                "hongguo-comic": "hot-comic-drama",
                "hongguo-ai": "hot-ai-drama",
            }
            rank_path = path_map.get(board, "hot-drama")
            html = self._raw_get(f"{self.web_site}/rank/{rank_path}?page={page}")
            arts = re.findall(r'<article[^>]*aria-labelledby="rank-title-(\d+)"[^>]*>(.*?)</article>', html, re.DOTALL)
            items = []
            for rank_idx, (sid, art) in enumerate(arts, 1):
                tm = re.search(r'id="rank-title-\d+"[^>]*>(.*?)<', art)
                title = tm.group(1).strip() if tm else sid
                cm = re.search(r'<img[^>]+src="([^"]+)"', art)
                cover = cm.group(1) if cm else ""
                hm = re.search(r'(\d+(?:\.\d+)?[万亿]?热度)', art)
                heat = hm.group(1) if hm else ""
                sm = re.search(r'评分\s*(\d+\.\d+)', art)
                score = sm.group(1) if sm else ""
                items.append({
                    "rank": rank_idx,
                    "metric": heat,
                    "drama": {
                        "sourceId": sid,
                        "title": title,
                        "cover": cover,
                        "score": score,
                        "heat": heat,
                    },
                })
            return {"items": items, "hasMore": len(items) >= 20}

        # 详情回退
        if path == "/detail":
            sid = params.get("id", "")
            html = self._raw_get(f"{self.web_site}/detail?series_id={quote(sid)}")
            data = self._extract_router(html)
            s = data.get("loaderData", {}).get("detail_page", {}).get("seriesDetail", {})
            vids = [str(v) for v in (s.get("vid_list") or []) if str(v)]
            chapters = []
            for i, vid in enumerate(vids, 1):
                chapters.append({"index": i, "title": f"第{i}集", "videoId": vid})
            drama = {
                "sourceId": sid,
                "title": s.get("series_name") or "",
                "cover": s.get("series_cover") or "",
                "desc": s.get("series_intro") or "",
                "totalEpisodes": len(vids),
                "remark": s.get("episode_right_text") or (f"全{len(vids)}集" if vids else ""),
                "tags": s.get("tags") or [],
            }
            return {"drama": drama, "chapters": chapters}

        # 搜索回退
        if path == "/search":
            keyword = params.get("keyword", "")
            html = self._raw_get(f"{self.web_site}/search/{quote(keyword)}")
            data = self._extract_router(html)
            page = data.get("loaderData", {}).get("search_(keyword)/page", {}) or data.get("loaderData", {}).get("search_page", {})
            dramas = []
            for it in page.get("searchList", []):
                vd = it.get("video_data") or it
                sid = str(vd.get("series_id") or it.get("keyword") or "")
                if sid:
                    dramas.append({
                        "sourceId": sid,
                        "title": vd.get("series_title") or it.get("name") or "",
                        "cover": vd.get("series_cover") or "",
                        "remark": vd.get("episode_right_text") or (f"全{vd.get('episode_cnt')}集" if vd.get("episode_cnt") else ""),
                    })
            return {"dramas": dramas}

        return {}

    def _raw_get(self, url):
        try:
            h = dict(self.headers)
            h["Referer"] = self.web_site + "/"
            if self.session is not None:
                r = self.session.get(url, headers=h, timeout=12)
                r.encoding = "utf-8"
                return r.text
            req = urllib.request.Request(url, headers=h)
            with urllib.request.urlopen(req, timeout=12) as resp:
                return resp.read().decode("utf-8", errors="replace")
        except Exception:
            return ""

    def _extract_router(self, html):
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
                                try:
                                    return json.loads(html[pos:i+1])
                                except Exception:
                                    return {}
        return {}

    def _classes(self):
        if self.class_cache:
            return self.class_cache
        arr = [
            {"type_id": "short_play", "type_name": "真人短剧"},
            {"type_id": "comic_series", "type_name": "动态漫剧"},
            {"type_id": "ai_series", "type_name": "AI 短剧"},
            {"type_id": "comic", "type_name": "精品动漫"},
            {"type_id": "rank_hot", "type_name": "🔥 热播总榜"},
            {"type_id": "rank_real", "type_name": "🎭 真人热榜"},
            {"type_id": "rank_comic", "type_name": "🎨 漫剧热榜"},
            {"type_id": "rank_ai", "type_name": "🤖 AI 剧热榜"},
        ]
        self.class_cache = arr
        return arr

    def _filters(self, classes):
        common = [
            {"key": "sort", "name": "排序", "value": [{"n": "默认", "v": ""}, {"n": "最新", "v": "online_time"}]},
        ]
        fs = {}
        for c in classes:
            fs[c["type_id"]] = common
        return fs

    def _list(self, data):
        if isinstance(data, list):
            return data
        if not isinstance(data, dict):
            return []
        if isinstance(data.get("data"), list):
            return data["data"]
        if isinstance(data.get("list"), list):
            return data["list"]
        if isinstance(data.get("items"), list):
            return data["items"]
        if isinstance(data.get("dramas"), list):
            return data["dramas"]
        return []

    def _vod(self, item):
        item = item or {}
        sid = self._sid(item.get("sourceId") or item.get("id") or item.get("series_id") or "")
        name = item.get("title") or item.get("name") or item.get("series_name") or sid
        cnt = item.get("totalEpisodes") or item.get("episode_cnt")
        remarks = item.get("remark") or item.get("episode_right_text") or (f"全{cnt}集" if cnt else "")
        score = f"★{item.get('score')}" if item.get("score") else ""
        return {
            "vod_id": sid,
            "vod_name": name,
            "vod_pic": self._pic(item),
            "vod_remarks": remarks,
            "vod_year": score,
        }

    def _pic(self, item):
        if not isinstance(item, dict):
            return ""
        pic = item.get("cover") or item.get("series_cover") or item.get("vod_pic") or item.get("pic") or ""
        if not pic:
            return ""
        if ".heic" in pic:
            pic = pic.split("~")[0]
        # 如果是第三方图片，走小果后端代理避免防盗链
        if pic.startswith("http") and ("fqnovel" in pic or "byteimg" in pic):
            return "%s/api/proxy/image?url=%s" % (self.host, quote(pic))
        return pic

    def _sid(self, x):
        return str(x or "").replace("hongguo:", "").strip()

    def _int(self, x, d=0):
        try:
            return int(x)
        except Exception:
            return d
