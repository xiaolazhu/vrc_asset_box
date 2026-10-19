//go:build !windows

package desktop

import "os"

// SystemLocale 在非 Windows 环境读取常用区域设置变量。
func SystemLocale() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}
