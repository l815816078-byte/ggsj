package core

import (
	"sync"
	"time"
)

// cacheEntry 一条缓存记录：值 + 过期时间。
type cacheEntry struct {
	value    interface{}
	expireAt time.Time
}

var (
	cacheMu    sync.RWMutex
	cacheStore = map[string]cacheEntry{}
)

// CacheGet 读取缓存；key 不存在或已过期返回 (nil, false)。
func CacheGet(key string) (interface{}, bool) {
	cacheMu.RLock()
	e, ok := cacheStore[key]
	cacheMu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expireAt) {
		// 惰性删除过期条目
		cacheMu.Lock()
		delete(cacheStore, key)
		cacheMu.Unlock()
		return nil, false
	}
	return e.value, true
}

// CacheSet 写入缓存，ttl<=0 表示不缓存（直接忽略）。
func CacheSet(key string, value interface{}, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	cacheMu.Lock()
	cacheStore[key] = cacheEntry{value: value, expireAt: time.Now().Add(ttl)}
	cacheMu.Unlock()
}

// CacheDelete 删除缓存（key 不存在时静默忽略）。
func CacheDelete(key string) {
	cacheMu.Lock()
	delete(cacheStore, key)
	cacheMu.Unlock()
}

// CacheGetOrSet 读缓存；未命中或已过期则执行 fn 生成新值并写缓存。
// fn 返回 ok=false 时不写缓存（调用方用于表达"这次没算到有效值"）。
func CacheGetOrSet(key string, ttl time.Duration, fn func() (interface{}, bool)) (interface{}, bool) {
	if v, ok := CacheGet(key); ok {
		return v, true
	}
	v, ok := fn()
	if !ok {
		return nil, false
	}
	CacheSet(key, v, ttl)
	return v, true
}
