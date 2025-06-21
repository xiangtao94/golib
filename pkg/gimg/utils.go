// Package algo -----------------------------
// @file      : utils.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/6/9 11:16
// Description:
// -------------------------------------------
package gimg

import (
	"path/filepath"
	"strings"
)

func SanitizeImageKey(key string) string {
	filename := filepath.Base(key)
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}
