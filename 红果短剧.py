# -*- coding: utf-8 -*-
"""
//@name:小果短剧[短]
//@id:xiaoguo_short_drama
//@version:102
//@author:xiaoguo
//@description:小果短剧全集解锁 AList-TvBox 插件，纯后端接口驱动，输出 1080P 超清解密流。
//@config-schema:{"description":"小果短剧业务配置。配置小果后端服务地址后，电视端即可享受全集 1080P 解密播放、实时搜索、榜单与分类。来源：插件脚本声明","fields":[{"key":"server_url","label":"小果后端服务地址","type":"string","required":true,"defaultValue":"http://127.0.0.1:8080","placeholder":"http://192.168.1.100:8080 或 http://xiaoguo:8080","description":"小主机上部署的 xiaoguo 容器或后端服务访问地址（不要带末尾斜杠）。"},{"key":"quality","label":"默认播放清晰度","type":"string","required":false,"defaultValue":"1080","placeholder":"1080","description":"优先选择的清晰度（如 1080、720、540、480）。"},{"key":"proxy_cover","label":"海报走后端代理","type":"boolean","required":false,"defaultValue":true,"description":"开启后海报由小果后端代理缓存并转换为标准 JPEG，避免第三方防盗链及图片无法显示。"}]}
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


# AList-TvBox 后台扩展配置表单声明 (供 AList-TVBox 自动渲染表单编辑界面)
PLUGIN_CONFIG_SCHEMA = {
    "source": "declared",
    "description": "小果短剧业务配置。配置小果后端服务地址后，电视端即可享受全集 1080P 解密播放、实时搜索、榜单与分类。来源：插件脚本声明",
    "allowAdditional": True,
    "fields": [
        {
            "key": "server_url",
            "label": "小果后端服务地址",
            "type": "string",
            "required": True,
            "defaultValue": "http://127.0.0.1:8080",
            "placeholder": "http://192.168.1.100:8080 或 http://xiaoguo:8080",
            "description": "小主机上部署的 xiaoguo 容器或后端服务访问地址（不要带末尾斜杠）。",
        },
        {
            "key": "quality",
            "label": "默认播放清晰度",
            "type": "string",
            "required": False,
            "defaultValue": "1080",
            "placeholder": "1080",
            "description": "优先选择的清晰度（如 1080、720、540、480）。",
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
        # 默认后端地址，可通过 AList-TvBox 后台的“扩展配置”直接编辑修改
        self.server_url = "http://127.0.0.1:8080"
        self.api = self.server_url + "/api"
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

    def init(self, extend=""):
        """AList-TvBox 插件初始化，接收后台表单/JSON扩展配置"""
        if extend:
            try:
                cfg = json.loads(extend) if isinstance(extend, str) else extend
                if isinstance(cfg, dict):
                    self.server_url = (
                        cfg.get("server_url")
                        or cfg.get("site")
                        or cfg.get("host")
                        or cfg.get("bridge")
                        or cfg.get("base_url")
                        or self.server_url
                    ).rstrip("/")
                    self.quality = str(cfg.get("quality") or self.quality)
                    if "proxy_cover" in cfg:
                        self.proxy_cover = bool(cfg.get("proxy_cover"))
                elif str(extend).strip().startswith("http"):
                    self.server_url = str(extend).strip().rstrip("/")
            except Exception:
                if str(extend).strip().startswith("http"):
                    self.server_url = str(extend).strip().rstrip("/")

        self.api = self.server_url + "/api"
        self.headers["Referer"] = self.server_url + "/"
        if self.session is not None:
            self.session.headers.update(self.headers)

    def getName(self):
        return self.name

    def isAdult(self):
        return False

    def homeContent(self, filter):
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

    def categoryContent(self, tid, pg, filter, extend):
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

    def detailContent(self, ids):
        """剧集详情与全部分集列表 (纯后端驱动)"""
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
                # 遵循 AList-TvBox 标准管道符分隔格式：第1集$sid|vid
                play.append("%s$%s|%s" % (title, vid, chapter_vid))
        else:
            play = ["第%s集$%s|%s" % (i, vid, i) for i in range(1, count + 1)]

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
            "vod_play_from": self.name,
            "vod_play_url": "#".join(play),
        }
        return {"list": [vod], "parse": 0, "jx": 0}

    def searchContent(self, key, quick, pg="1"):
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

    def playerContent(self, flag, id, vipFlags):
        """播放地址解析，直接输出后端 1080P 解密流直链"""
        # id 格式为 sid|vid 或 vid
        url = "%s/play?vid=%s&quality=%s" % (self.server_url, quote(str(id)), quote(str(self.quality)))
        return {
            "parse": 0,
            "playUrl": "",
            "url": url,
            "jx": 0,
            "header": {
                "User-Agent": self.headers["User-Agent"],
                "Referer": self.server_url + "/",
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
    # 纯后端 API 通信工具
    # ==========================================
    def _api(self, path, params=None):
        """调用小果后端 API，获取标准 JSON 数据"""
        path = "/" + path.lstrip("/")
        url = self.api + path
        try:
            if self.session is not None:
                r = self.session.get(url, params=params, timeout=10)
                if r.status_code == 200:
                    res = r.json()
                    if res.get("code") == 0:
                        return res.get("data", res)
            else:
                qs = ("?" + urlencode(params)) if params else ""
                req = urllib.request.Request(url + qs, headers=self.headers)
                with urllib.request.urlopen(req, timeout=10) as resp:
                    if resp.status == 200:
                        res = json.loads(resp.read().decode("utf-8"))
                        if res.get("code") == 0:
                            return res.get("data", res)
        except Exception as e:
            print(f"[{self.name}] 后端接口请求失败 [{url}]: {e}")
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
        if self.proxy_cover and cover.startswith("http") and not cover.startswith(self.server_url):
            return f"{self.server_url}/api/proxy/image?url={quote(cover)}"
        return cover

    def _sid(self, x):
        return str(x or "").replace("hongguo:", "").strip()

    def _int(self, x, d=0):
        try:
            return int(x)
        except Exception:
            return d
