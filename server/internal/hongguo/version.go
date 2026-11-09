package hongguo

// version.go 是全项目的官方客户端参数唯一来源。
// 官方每次升级客户端版本时，只需要修改这里的常量并重新跑测试，
// 而不需要在 playback.go / client.go / comment_sign.go 中逐处散落数字。
const (
	// appAid 应用 ID。官方在注册/签名/播放三处引用必须一致。
	appAid = "8662"

	// appID 在 x-argus / x-ladon 中的 app 标识字符串，与 aid 相关但不等同。
	// 参考 HGTV 1.3.5.1 的 LS0/o;->e 中构造：
	//   bytesField(5, deviceID) / bytesField(6, "1611921764") / bytesField(7, versionName)
	// 以及 "1611921764" 作为 x-ladon 第二参数、"1588093228" 作为 x-helios 第二参数。
	appLadonID  = "1611921764"
	appHeliosID = "1588093228"

	appName              = "novelread"
	appPackage           = "com.phoenix.read"
	appVersionCode       = "73932"
	appVersionName       = "7.3.9.32"
	appManifestVerCode   = "73932"
	appUpdateVerCode     = "73932"
	appPassportSDKVer    = "5051452"
	appSDKVersion        = "3.7.0-rc.34-fanqie-xiaoshuo"
	appSDKTgtVersion     = 29
	appReleaseBuild      = "b5e187f"
	appGitHash           = "3a88f4e"
	appSigHash           = "56a962410c494bbaf0b58dba20cae56f"
	appChannelDefault    = "xiaomi_8662_64"
	appChannelRegistered = "69310258a"
	appDevicePlatform    = "android"
	appOS                = "android"
	appSSMix             = "a"
	appLanguage          = "zh"
	appOSAPI             = "25" // AGENTS.md 强制锁定：请求官方接口必须 os_api=25
	appCronetVersion     = "8d40f833_2026-03-03"
	appTTNetVersion      = "4.2.243.31-douyin"
	appBdturingSDKVer    = "4.0.3.cn"
	appLcID              = "101"

	// 标准默认设备形态（注册新设备时使用）。
	appDeviceType  = "25053RT47C"
	appDeviceBrand = "Redmi"
	appDeviceManu  = "Xiaomi"
	appOSVersion   = "12"
	appResolution  = "1280*2772"
	appDPI         = "520"
	appRomVersion  = "W528JS release-keys"
	appUserAgent   = "com.phoenix.read/73932 (Linux; U; Android 12; zh_CN; 25053RT47C; Build/W528JS; Cronet/TTNetVersion:8d40f833 2026-03-03 QuicVersion:21ac1950 2025-11-18)"

	// 播放模型端点（签名链路按路径分流）。
	pathMultiVideoModel  = "/novel/player/multi_video_model/player/v1"
	pathMultiVideoModel2 = "/novel/player/multi_video_model/v1/"
	pathVideoTabModel    = "/reading/bookapi/video_tab/multi_video_model/v1/"

	// 设备注册端点。
	pathDeviceRegister = "/service/2/device_register/?tt_data=a&cronet_version=8d40f833_2026-03-03&ttnet_version=4.2.243.31-douyin&use_store_region_cookie=1"
)
