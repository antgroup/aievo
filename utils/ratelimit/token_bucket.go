package ratelimit

import (
	"context"
	"sync"
	"time"
)

// TokenBucket 实现令牌桶算法的限流器
type TokenBucket struct {
	rate       float64    // 令牌生成速率(个/秒)
	capacity   int64      // 桶的容量
	tokens     float64    // 当前令牌数量
	lastUpdate time.Time  // 上次更新时间
	mutex      sync.Mutex // 互斥锁保护并发访问
}

// NewTokenBucket 创建一个新的令牌桶限流器
func NewTokenBucket(rate float64, capacity int64) *TokenBucket {
	return &TokenBucket{
		rate:       rate,
		capacity:   capacity,
		tokens:     float64(capacity),
		lastUpdate: time.Now(),
	}
}

// Allow 判断是否允许通过一个请求，需要消耗指定数量的令牌
func (tb *TokenBucket) Allow() bool {
	return tb.AllowN(1)
}

// AllowN 判断是否允许消耗N个令牌
func (tb *TokenBucket) AllowN(n int64) bool {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	now := time.Now()
	// 计算从上次更新到现在生成的令牌数
	elapsed := now.Sub(tb.lastUpdate).Seconds()
	tb.tokens = min(float64(tb.capacity), tb.tokens+elapsed*tb.rate)
	tb.lastUpdate = now

	// 如果令牌不足，返回false
	if tb.tokens < float64(n) {
		return false
	}

	// 消耗令牌
	tb.tokens -= float64(n)
	return true
}

// Wait 等待直到获取到指定数量的令牌
func (tb *TokenBucket) Wait(ctx context.Context) error {
	return tb.WaitN(ctx, 1)
}

// WaitN 等待直到获取到N个令牌
func (tb *TokenBucket) WaitN(ctx context.Context, n int64) error {
	for {
		tb.mutex.Lock()
		now := time.Now()
		elapsed := now.Sub(tb.lastUpdate).Seconds()
		tb.tokens = min(float64(tb.capacity), tb.tokens+elapsed*tb.rate)
		tb.lastUpdate = now

		if tb.tokens >= float64(n) {
			tb.tokens -= float64(n)
			tb.mutex.Unlock()
			return nil
		}

		// 计算需要等待的时间
		waitTime := time.Duration(float64(n-int64(tb.tokens)) / tb.rate * float64(time.Second))
		tb.mutex.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			continue
		}
	}
}
