package cache

import "github.com/dgraph-io/ristretto/v2"

func NewRistretto() (*ristretto.Cache[string, any], error) {
	return ristretto.NewCache[string, any](&ristretto.Config[string, any]{
		NumCounters: 1e7,
		MaxCost:     1 << 29, // 512MB
		BufferItems: 64,
	})
}
