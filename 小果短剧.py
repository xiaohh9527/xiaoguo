# -*- coding: utf-8 -*-
"""
//@name:小果短剧[短]
//@id:xiaoguo_short_drama
//@version:104
//@author:xiaoguo
//@description:小果短剧全集解锁 AList-TvBox 插件，纯后端接口驱动，支持多后端地址按顺序自动容灾故障切换，支持在播放器端按线路自由切换 1080P/720P/540P/480P 常用分辨率。
//@config-schema:{"description":"小果短剧业务配置。配置小果后端服务地址后，电视端即可享受全集解密播放、实时搜索、榜单与分类。支持配置多个后端地址按顺序自动故障转移，支持在播放器端多线路自由切换分辨率。来源：插件脚本声明","fields":[{"key":"server_url","label":"小果后端服务地址(支持多地址)","type":"string","required":true,"defaultValue":"http://127.0.0.1:8080","placeholder":"http://192.168.1.100:8080,http://xiaoguo.lan:8080,https://ddns.example.com:8443","description":"小主机上部署的 xiaoguo 服务访问地址（不要带末尾斜杠）。支持配置多个地址（以逗号,、分号;或换行分隔），按配置顺序优先使用第一个可连通的后端地址。"},{"key":"quality","label":"默认首选清晰度","type":"string","required":false,"defaultValue":"1080","placeholder":"1080","description":"播放器默认排在第一位的首选分辨率（常用值：1080、720、540、480）。播放器选集界面的线路列表里可随时自由切换其他常用分辨率。"},{"key":"proxy_cover","label":"海报走后端代理","type":"boolean","required":false,"defaultValue":true,"description":"开启后海报由小果后端代理缓存并转换为标准 JPEG，避免第三方防盗链及图片无法显示。"}]}
"""
import json
import os
import re
import sys
import time
import traceback
from urllib.parse import quote, unquote, urlencode

try:
    import ssl
    _ssl_context = ssl._create_unverified_context()
except Exception:
    ssl = None
    _ssl_context = None

try:
    import threading
except Exception:
    threading = None

class _DummyLock:
    def __enter__(self):
        return self
    def __exit__(self, *args):
        pass

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


# AList-TvBox 后台扩展配置表单声明 (供 AList-TVBox 自动渲染表单编辑界面)
PLUGIN_CONFIG_SCHEMA = {
    "source": "declared",
    "description": "小果短剧业务配置。配置小果后端服务地址后，电视端即可享受全集 1080P 解密播放、实时搜索、榜单与分类。支持配置多个后端地址按顺序自动故障转移。来源：插件脚本声明",
    "allowAdditional": True,
    "fields": [
        {
            "key": "server_url",
            "label": "小果后端服务地址(支持多地址)",
            "type": "string",
            "required": True,
            "defaultValue": "http://127.0.0.1:8080",
            "placeholder": "http://192.168.1.100:8080,http://xiaoguo.lan:8080,https://ddns.example.com:8443",
            "description": "小主机上部署的 xiaoguo 服务访问地址（不要带末尾斜杠）。支持配置多个地址（以逗号,、分号;或换行分隔），按配置顺序优先使用第一个可连通的后端地址。",
        },
        {
            "key": "quality",
            "label": "默认首选清晰度",
            "type": "string",
            "required": False,
            "defaultValue": "1080",
            "placeholder": "1080",
            "description": "播放器默认排在第一位的首选分辨率（常用值：1080、720、540、480）。播放器选集界面的线路列表里可随时自由切换其他常用分辨率。",
        },
        {
            "key": "proxy_cover",
            "label": "海报走后端代理",
            "type": "boolean",
            "required": False,
            "defaultValue": True,
            "description": "开启后海报由小果后端代理缓存并转换为标准 JPEG，避免第三方防盗链及图片无法显示。",
        },
    ],
}


class Spider(BaseSpider):
    def __init__(self):
        # 候选后端地址列表与当前活跃后端地址，可通过 AList-TvBox 后台的“扩展配置”直接编辑修改
        self.server_urls = ["http://127.0.0.1:8080"]
        self.active_server_url = self.server_urls[0]
        self.server_url = self.active_server_url
        self.api = self._clean_base_api(self.server_url)
        self.name = "小果短剧"
        self.quality = "1080"
        self.proxy_cover = True
        self.headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
            "Referer": self.server_url + "/",
        }
        if requests is not None:
            self.session = requests.Session()
            self.session.headers.update(self.headers)
        else:
            self.session = None
        self.class_cache = None
        self._lock = threading.Lock() if threading is not None else _DummyLock()
        self._last_primary_probe = 0

    def _clean_base_api(self, url):
        u = str(url or "").rstrip("/")
        if u.endswith("/api"):
            return u
        return u + "/api"

    def _clean_origin_url(self, url):
        u = str(url or "").rstrip("/")
        if u.endswith("/api"):
            u = u[:-4].rstrip("/")
        return u

    def _build_api_url(self, base_url, path):
        clean_base = self._clean_origin_url(base_url)
        clean_path = "/" + str(path or "").lstrip("/")
        if not clean_path.startswith("/api/"):
            clean_path = "/api" + clean_path
        return clean_base + clean_path

    def _parse_server_urls(self, raw_val):
        """解析并格式化多后端地址列表，保持优先级顺序并去重"""
        urls = []

        def _add_url(u):
            if not u:
                return
            u_str = str(u).strip().strip("\"'").strip()
            if not (u_str.startswith("http://") or u_str.startswith("https://")):
                return
            clean_u = u_str.rstrip("/")
            if clean_u and clean_u not in urls:
                urls.append(clean_u)

        if isinstance(raw_val, (list, tuple)):
            for item in raw_val:
                if isinstance(item, str):
                    for sub in re.split(r"[\r\n,;\s\|]+", item):
                        _add_url(sub)
                else:
                    _add_url(item)
        elif isinstance(raw_val, str):
            raw_str = raw_val.strip()
            if raw_str.startswith("[") and raw_str.endswith("]"):
                try:
                    parsed_list = json.loads(raw_str)
                    if isinstance(parsed_list, list):
                        for item in parsed_list:
                            _add_url(item)
                except Exception:
                    pass
            if not urls:
                for sub in re.split(r"[\r\n,;\s\|]+", raw_str):
                    _add_url(sub)
        elif raw_val:
            _add_url(raw_val)

        return urls if urls else ["http://127.0.0.1:8080"]

    def _set_active_server(self, url):
        """原子切换当前活跃后端地址，并同步更新请求头与上下文"""
        clean_url = str(url or "").rstrip("/")
        if not clean_url:
            return
        origin_url = self._clean_origin_url(clean_url)
        with self._lock:
            self.active_server_url = origin_url
            self.server_url = origin_url
            self.api = origin_url + "/api"
            self.headers["Referer"] = origin_url + "/"
            self._last_primary_probe = time.time()
            if self.session is not None:
                self.session.headers.update(self.headers)

    def init(self, extend="", *args, **kwargs):
        """AList-TvBox 插件初始化，接收后台表单/JSON扩展配置"""
        parsed_urls = None
        if extend:
            try:
                cfg = json.loads(extend) if isinstance(extend, str) else extend
                if isinstance(cfg, dict):
                    raw_server = (
                        cfg.get("server_urls")
                        or cfg.get("server_url")
                        or cfg.get("site")
                        or cfg.get("host")
                        or cfg.get("bridge")
                        or cfg.get("base_url")
                    )
                    if raw_server:
                        parsed_urls = self._parse_server_urls(raw_server)
                    self.quality = str(cfg.get("quality") or self.quality)
                    if "proxy_cover" in cfg:
                        self.proxy_cover = bool(cfg.get("proxy_cover"))
                else:
                    parsed_urls = self._parse_server_urls(extend)
            except Exception:
                parsed_urls = self._parse_server_urls(extend)

        if parsed_urls:
            self.server_urls = parsed_urls
            self._set_active_server(self.server_urls[0])
            self.class_cache = None
        else:
            self._set_active_server(self.server_urls[0])

    def getName(self):
        return self.name

    def isAdult(self):
        return False

    def homeContent(self, filter=True, *args, **kwargs):
        """首页推荐内容与分类列表 (纯后端驱动)"""
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

    def homeVideoContent(self):
        """TVBox 首页推荐视频兼容接口"""
        return self.categoryContent("short_play", "1", None, {})

    def categoryContent(self, tid="short_play", pg="1", filter=False, extend=None, *args, **kwargs):
        """分类与排行榜单翻页 (纯后端驱动)"""
        page = max(1, self._int(pg, 1))
        tid = str(tid or "short_play")

        # 1. 榜单分类 (直接调用后端 /api/rankings)
        if tid.startswith("rank:") or tid.startswith("hongguo-") or tid.startswith("rank_"):
            board = tid.replace("rank:", "").replace("rank_", "hongguo-")
            if board == "rank_hot":
                board = "hongguo-hot"
            data = self._api("/rankings", {"board": board, "page": str(page)})
            items = data.get("items", []) if isinstance(data, dict) else []
            vod_list = []
            for it in items:
                dr = it.get("drama", {}) if isinstance(it, dict) else {}
                sid = self._sid(dr.get("sourceId") or dr.get("id"))
                vod_list.append({
                    "vod_id": sid,
                    "vod_name": str(dr.get("title") or dr.get("name") or sid),
                    "vod_pic": self._pic(dr.get("cover")),
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

        # 2. 剧库分类 (直接调用后端 /api/catalog)
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

    def detailContent(self, ids, *args, **kwargs):
        """剧集详情与全部分集列表 (纯后端驱动，支持播放器端常用分辨率多线路自由切换)"""
        vid = self._sid(ids[0])
        data = self._api("/detail", {"id": vid})
        if not isinstance(data, dict):
            return {"list": [], "parse": 0, "jx": 0}

        drama = data.get("drama", {}) if isinstance(data, dict) else {}
        chapters = data.get("chapters", []) if isinstance(data, dict) else []
        name = drama.get("title") or drama.get("name") or vid
        count = drama.get("totalEpisodes") or len(chapters) or 1

        # 常用分辨率档位列表，按配置的默认首选清晰度置顶排序
        all_qualities = ["1080", "720", "540", "480"]
        pref_q = str(self.quality or "1080").strip()
        sorted_qualities = [pref_q] if pref_q in all_qualities else [all_qualities[0]]
        for q in all_qualities:
            if q not in sorted_qualities:
                sorted_qualities.append(q)

        play_from = []
        play_urls = []

        for q in sorted_qualities:
            line_name = f"{self.name}·{q}P"
            play_items = []
            if chapters:
                for i, c in enumerate(chapters, 1):
                    idx = c.get("index") or i
                    title = c.get("title") or ("第%s集" % idx)
                    chapter_vid = str(c.get("videoId") or "")
                    # 遵循 AList-TvBox 标准管道符分隔格式：第1集$sid|vid|quality
                    play_items.append("%s$%s|%s|%s" % (title, vid, chapter_vid, q))
            else:
                for i in range(1, count + 1):
                    play_items.append("第%s集$%s|%s|%s" % (i, vid, i, q))

            play_from.append(line_name)
            play_urls.append("#".join(play_items))

        vod = {
            "vod_id": vid,
            "vod_name": name,
            "vod_pic": self._pic(drama.get("cover")),
            "type_name": drama.get("categoryName") or ",".join(drama.get("tags") or []),
            "vod_year": f"★{drama.get('score')}" if drama.get("score") else "",
            "vod_area": "中国大陆",
            "vod_remarks": drama.get("remark") or ("全%s集" % count),
            "vod_actor": "",
            "vod_director": "",
            "vod_content": drama.get("desc") or name,
            "vod_play_from": "$$$".join(play_from),
            "vod_play_url": "$$$".join(play_urls),
        }
        return {"list": [vod], "parse": 0, "jx": 0}

    def searchContent(self, key, quick=False, pg="1", *args, **kwargs):
        """关键词搜索 (纯后端驱动)"""
        keyword = str(key).strip()
        data = self._api("/search", {"keyword": keyword})
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

    def searchContentPage(self, key, quick=False, pg="1", *args, **kwargs):
        """TVBox 分页搜索兼容方法"""
        return self.searchContent(key, quick, pg)

    def playerContent(self, flag="", id="", vipFlags=False, *args, **kwargs):
        """播放地址解析，直接输出所选分辨率的后端解密流直链"""
        # id 格式支持: sid|vid|quality, sid|vid, vid 等
        target_q = str(self.quality or "1080").strip()
        clean_vid = str(id or "").strip()

        # 1. 尝试从 id 中提取目标 quality (格式: sid|vid|quality 或 sid*vid*quality)
        parts = re.split(r"[\|\*]", clean_vid)
        if len(parts) >= 3 and parts[2].isdigit():
            clean_vid = f"{parts[0]}|{parts[1]}"
            target_q = parts[2]
        elif len(parts) == 2:
            clean_vid = f"{parts[0]}|{parts[1]}"

        # 2. 尝试从当前播放线路名 flag 中提取分辨率 (例如 '小果短剧·720P' -> '720')
        flag_match = re.search(r"(\d{3,4})", str(flag or ""))
        if flag_match:
            target_q = flag_match.group(1)

        origin_url = self._clean_origin_url(self.server_url)
        url = "%s/play?vid=%s&quality=%s" % (origin_url, quote(str(clean_vid)), quote(str(target_q)))
        return {
            "parse": 0,
            "playUrl": "",
            "url": url,
            "jx": 0,
            "header": {
                "User-Agent": self.headers["User-Agent"],
                "Referer": origin_url + "/",
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

    # ==========================================
    # 纯后端 API 通信工具 (支持多后端顺序容灾选优)
    # ==========================================
    def _get_candidate_urls(self):
        """获取按优先级排序的候选请求服务器列表"""
        with self._lock:
            urls = list(self.server_urls)
            active = self.active_server_url
            last_probe = self._last_primary_probe

        if not urls:
            return ["http://127.0.0.1:8080"]
        if len(urls) == 1:
            return urls

        primary = urls[0]
        now = time.time()

        # 如果当前活跃的已经是最高优先级的首选地址
        if active == primary:
            return urls

        # 当前处于备用降级地址：检查是否到达恢复探测冷却时间（180秒）
        if now - last_probe >= 180:
            with self._lock:
                self._last_primary_probe = now
            # 优先从最高优先级地址重新开始尝试
            return urls
        else:
            # 冷却期内优先复用当前已通的备用地址以保障秒开体验，其余地址作为后备
            candidates = [active]
            for u in urls:
                if u != active:
                    candidates.append(u)
            return candidates

    def _do_request(self, base_url, path, params=None):
        """向指定 base_url 发起单次请求，成功返回 (True, data)，失败返回 (False, None)"""
        url = self._build_api_url(base_url, path)
        origin_url = self._clean_origin_url(base_url)
        try:
            if self.session is not None:
                headers = dict(self.headers)
                headers["Referer"] = origin_url + "/"
                r = self.session.get(url, params=params, headers=headers, timeout=8)
                if r.status_code == 200:
                    res = r.json()
                    if isinstance(res, dict) and res.get("code") == 0:
                        return True, res.get("data", res)
            else:
                qs = ("?" + urlencode(params)) if params else ""
                headers = dict(self.headers)
                headers["Referer"] = origin_url + "/"
                req = urllib.request.Request(url + qs, headers=headers)
                open_kwargs = {"timeout": 8}
                if _ssl_context is not None and url.startswith("https://"):
                    open_kwargs["context"] = _ssl_context
                with urllib.request.urlopen(req, **open_kwargs) as resp:
                    if resp.status == 200:
                        res = json.loads(resp.read().decode("utf-8"))
                        if isinstance(res, dict) and res.get("code") == 0:
                            return True, res.get("data", res)
        except Exception:
            pass
        return False, None

    def _api(self, path, params=None):
        """调用小果后端 API，支持多后端地址按顺序探测与自动故障转移"""
        path = "/" + path.lstrip("/")
        candidates = self._get_candidate_urls()
        for base_url in candidates:
            success, data = self._do_request(base_url, path, params)
            if success:
                if self._clean_origin_url(base_url) != self.active_server_url:
                    print(f"[{self.name}] 切换至可用后端服务: {base_url}")
                    self._set_active_server(base_url)
                return data

        print(f"[{self.name}] 所有后端接口均无法连通: {candidates}, 接口路径: {path}")
        return {}

    def _classes(self):
        """获取分类与榜单定义"""
        if self.class_cache:
            return self.class_cache
        classes = []
        # 1. 剧库分类
        genres_data = self._api("/genres", {})
        if isinstance(genres_data, list) and genres_data:
            for g in genres_data:
                classes.append({"type_id": g.get("key"), "type_name": g.get("name")})
        else:
            classes += [
                {"type_id": "short_play", "type_name": "真人短剧"},
                {"type_id": "comic_series", "type_name": "动态漫剧"},
                {"type_id": "ai_series", "type_name": "AI 短剧"},
                {"type_id": "comic", "type_name": "精品动漫"},
            ]
        # 2. 榜单分类
        boards_data = self._api("/ranking-boards", {})
        if isinstance(boards_data, list) and boards_data:
            for b in boards_data:
                classes.append({"type_id": "rank:" + b.get("id"), "type_name": "🔥 " + b.get("name")})
        else:
            classes += [
                {"type_id": "rank:hongguo-hot", "type_name": "🔥 热播总榜"},
                {"type_id": "rank:hongguo-real", "type_name": "🔥 真人热榜"},
                {"type_id": "rank:hongguo-comic", "type_name": "🔥 漫剧热榜"},
                {"type_id": "rank:hongguo-ai", "type_name": "🔥 AI 剧热榜"},
            ]
        self.class_cache = classes
        return classes

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
            "vod_pic": self._pic(item.get("cover") or item.get("series_cover") or item.get("pic")),
            "vod_remarks": remarks,
            "vod_year": score,
        }

    def _pic(self, cover):
        if not cover:
            return ""
        # 去除 HEIC 格式后缀
        if ".heic" in cover:
            cover = cover.split("~")[0]
        # 走小果后端图片代理，避免跨域或防盗链
        origin_url = self._clean_origin_url(self.server_url)
        if (
            self.proxy_cover
            and cover.startswith("http")
            and not any(cover.startswith(self._clean_origin_url(u)) for u in self.server_urls)
            and "/api/proxy/image" not in cover
        ):
            return f"{origin_url}/api/proxy/image?url={quote(cover)}"
        return cover

    def _sid(self, x):
        return str(x or "").replace("hongguo:", "").strip()

    def _int(self, x, d=0):
        try:
            return int(x)
        except Exception:
            return d
