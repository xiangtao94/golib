// Package algo -----------------------------
// @file      : cache.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/6/9 11:17
// Description:
// -------------------------------------------
package gimg

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"sort"
	"sync"

	"github.com/pierrre/imageserver"
	imageserver_cache "github.com/pierrre/imageserver/cache"
)

func NewSourceHashKeyGenerator() imageserver_cache.KeyGenerator {
	pool := &sync.Pool{
		New: func() interface{} {
			return sha256.New()
		},
	}
	return imageserver_cache.KeyGeneratorFunc(func(params imageserver.Params) string {
		h := pool.Get().(hash.Hash)
		defer pool.Put(h)

		keys := params.Keys()
		sort.Strings(keys)
		for _, key := range keys {
			value, _ := params.Get(key)
			io.WriteString(h, fmt.Sprintf("%s=%v;", key, value))
		}
		return hex.EncodeToString(h.Sum(nil))
	})
}
