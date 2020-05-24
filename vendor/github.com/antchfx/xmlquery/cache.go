package xmlquery

import (
	"sync"

	"github.com/golang/groupcache/lru"

	"github.com/antchfx/xpath"
)

// DisableSelectorCache will disable caching for the query selector if value is true.
var DisableSelectorCache = false

// SelectorCacheMaxEntries allows how many selector object can be caching. Default is 50.
// Will disable caching if SelectorCacheMaxEntries <= 0.
var SelectorCacheMaxEntries = 50

var (
	cacheOnce  sync.Once
	cache      *lru.Cache
	cacheMutex sync.Mutex
)

func getQuery(expr string) (*xpath.Expr, error) {
	if DisableSelectorCache || SelectorCacheMaxEntries <= 0 {
		return xpath.Compile(expr)
	}
	cacheOnce.Do(func() {
		cache = lru.New(SelectorCacheMaxEntries)
	})
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	if v, ok := cache.Get(expr); ok {
		return v.(*xpath.Expr), nil
	}
	v, err := xpath.Compile(expr)
	if err != nil {
		return nil, err
	}
	cache.Add(expr, v)
	return v, nil

}
