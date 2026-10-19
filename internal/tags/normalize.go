package tags

import "strings"

// NormalizeName 供名称匹配与唯一性检查共同使用，
// 有意保留内部空白及拼写差异。
func NormalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
