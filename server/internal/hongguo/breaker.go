package hongguo

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// breaker.go 实现 L1 熔断器：当官方网关对某条源路径连续返回
// “空包体 / 403 / 5xx / code!=0” 等明确拦截特征时，不再每次请求都
// 从头重试，而是进入指数退避冷却，冷却到期后只放一个探针请求
// （Half-Open）验证是否已经恢复。成功一次即重置。
//
// 设计要点：
//   - Key 用“逻辑源”粒度（如 "app"、"video_tab"），不要用 host 粒度，
//     避免多 host 轮询时每个 host 各自维护一份状态、永远不熔断。
//   - 只对“可判定为官方侧拦截/空包/业务码”的错误计入失败，
//     网络抖动、context 超时、本地 JSON 解析异常不计入。
//   - 冷却时间从 30s 开始翻倍，上限 5 分钟；成功后计数清零。

// source 名（熔断器 Key）。
const (
	srcApp      = "app"
	srcVideoTab = "video_tab"
	srcWeb      = "web"
	srcBackup   = "backup"
)

type breakerState struct {
	consecutiveFailures int
	cooldownUntil       time.Time
	halfOpenInFlight    bool
}

type breakerConfig struct {
	failureThreshold int           // 连续失败 N 次后熔断
	baseCooldown     time.Duration // 首次冷却时长
	maxCooldown      time.Duration // 冷却上限
}

var defaultBreakerConfig = breakerConfig{
	failureThreshold: 3,
	baseCooldown:     30 * time.Second,
	maxCooldown:      5 * time.Minute,
}

// ErrBreakerOpen 表示该源当前处于熔断冷却中，已跳过实际请求。
var ErrBreakerOpen = errors.New("源路径已熔断，冷却中")

func (c *Client) ensureBreakersLocked() {
	if c.breakers == nil {
		c.breakers = make(map[string]breakerState)
	}
}

// allow 判断当前时刻是否允许发起探针/正式请求。
// 返回 (allowed bool, isProbe bool)。isProbe 表示这次是熔断后的探针。
func (c *Client) allow(source string) (allowed bool, isProbe bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureBreakersLocked()
	st := c.breakers[source]
	if st.cooldownUntil.IsZero() || time.Now().After(st.cooldownUntil) {
		if st.consecutiveFailures > 0 && !st.cooldownUntil.IsZero() {
			// 冷却刚结束，允许一次探针
			if st.halfOpenInFlight {
				return false, false
			}
			st.halfOpenInFlight = true
			c.breakers[source] = st
			return true, true
		}
		return true, false
	}
	return false, false
}

// recordSuccess 在源成功时清零失败计数并解除熔断。
func (c *Client) recordSuccess(source string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureBreakersLocked()
	if c.breakers[source] != (breakerState{}) {
		delete(c.breakers, source)
	}
}

// recordFailure 在源明确失败（空包/403/5xx/业务 code）时记录并可能进入熔断。
func (c *Client) recordFailure(source string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureBreakersLocked()
	st := c.breakers[source]
	st.consecutiveFailures++
	st.halfOpenInFlight = false
	if st.consecutiveFailures >= defaultBreakerConfig.failureThreshold {
		cooldown := defaultBreakerConfig.baseCooldown << (st.consecutiveFailures - defaultBreakerConfig.failureThreshold)
		if cooldown > defaultBreakerConfig.maxCooldown || cooldown <= 0 {
			cooldown = defaultBreakerConfig.maxCooldown
		}
		st.cooldownUntil = time.Now().Add(cooldown)
	}
	c.breakers[source] = st
}

// isBreakableErr 判断错误是否属于“官方拦截特征”，只这类错误才计入熔断。
func isBreakableErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "len 0") ||
		strings.Contains(msg, "HTTP 403") ||
		strings.Contains(msg, "HTTP 5") ||
		strings.Contains(msg, "空内容") ||
		strings.Contains(msg, "Code:") ||
		strings.Contains(msg, "红果接口提示")
}

// breakerError 包装熔断拒绝的错误，便于上层区分“跳过”与“真实失败”。
func breakerError(source string) error {
	return fmt.Errorf("%w: source=%s", ErrBreakerOpen, source)
}
