package hongguo

import (
	"sync"
	"time"
)

// device_health.go 实现 L1 设备健康度：每个 DeviceProfile 在内存中维护
// 成功/失败计数与最近失败时间。选取当前设备时优先返回“健康”的，
// 连续失败超过阈值的会被移至候选队尾并发起自动轮换，从而避免单设备
// 被风控后全站连续撞墙。

type deviceHealth struct {
	successes       int
	failures        int
	consecutive     int
	lastSuccessAt   time.Time
	lastFailureAt   time.Time
	lastFailureErr  string
	degradedUntil   time.Time // 降权到期时间（到期后自动恢复试探）
	cooldownFactor  int       // 第几次进入降权（用于指数退避）
}

var (
	deviceHealthMu sync.Mutex
	deviceHealths  = make(map[string]*deviceHealth)
)

// deviceHealthPenaltyThreshold 连续失败达到该阈值即进入降权。
const deviceHealthPenaltyThreshold = 3

// recordDeviceSuccess 标记设备请求成功，清零连续失败并解除降权。
func recordDeviceSuccess(deviceID string) {
	if deviceID == "" {
		return
	}
	deviceHealthMu.Lock()
	defer deviceHealthMu.Unlock()
	h, ok := deviceHealths[deviceID]
	if !ok {
		h = &deviceHealth{}
		deviceHealths[deviceID] = h
	}
	h.successes++
	h.consecutive = 0
	h.lastSuccessAt = time.Now()
	h.degradedUntil = time.Time{}
	h.cooldownFactor = 0
}

// recordDeviceFailure 标记设备请求失败；超过阈值后进入指数退避降权。
// 返回是否为“新触发的降权”（用于触发轮换）。
func recordDeviceFailure(deviceID, errStr string) bool {
	if deviceID == "" {
		return false
	}
	deviceHealthMu.Lock()
	defer deviceHealthMu.Unlock()
	h, ok := deviceHealths[deviceID]
	if !ok {
		h = &deviceHealth{}
		deviceHealths[deviceID] = h
	}
	h.failures++
	h.consecutive++
	h.lastFailureAt = time.Now()
	h.lastFailureErr = errStr
	if h.consecutive >= deviceHealthPenaltyThreshold {
		h.cooldownFactor++
		secs := 15 << (h.cooldownFactor - 1) // 15s, 30s, 60s, 120s...
		if secs > 300 {
			secs = 300
		}
		h.degradedUntil = time.Now().Add(time.Duration(secs) * time.Second)
		return true
	}
	return false
}

// isDeviceDegraded 判断设备是否处于降权期。
func isDeviceDegraded(deviceID string) bool {
	if deviceID == "" {
		return false
	}
	deviceHealthMu.Lock()
	defer deviceHealthMu.Unlock()
	h, ok := deviceHealths[deviceID]
	if !ok {
		return false
	}
	return time.Now().Before(h.degradedUntil)
}

// deviceScore 返回设备健康分（越高越健康），用于在候选池中排序。
func deviceScore(deviceID string) int {
	if deviceID == "" {
		return 0
	}
	deviceHealthMu.Lock()
	defer deviceHealthMu.Unlock()
	h, ok := deviceHealths[deviceID]
	if !ok {
		return 100
	}
	score := 100 + h.successes*2 - h.failures*3 - h.consecutive*15
	if time.Now().Before(h.degradedUntil) {
		score -= 200
	}
	if score < -500 {
		score = -500
	}
	return score
}

// resetDeviceHealth 供测试或管理员手动重置。
func resetDeviceHealth() {
	deviceHealthMu.Lock()
	defer deviceHealthMu.Unlock()
	deviceHealths = make(map[string]*deviceHealth)
}

// deviceHealthSnapshot 供健康接口/日志导出（L3 阶段会复用）。
func deviceHealthSnapshot() map[string]deviceHealth {
	deviceHealthMu.Lock()
	defer deviceHealthMu.Unlock()
	out := make(map[string]deviceHealth, len(deviceHealths))
	for k, v := range deviceHealths {
		out[k] = *v
	}
	return out
}
