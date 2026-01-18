package local

import "fmt"

// CheckTweetLength 检查推文长度是否符合限制
// isPremium: 是否为X Premium用户
// tweet: 输入的推文内容
func CheckTweetLength(tweet string, isPremium bool) (bool, int, string) {
	// 根据账户类型设置字符限制
	limit := 280
	if isPremium {
		limit = 25000
	}

	// 计算推文字符数（考虑表情符号等特殊字符）
	charCount := 0
	for _, r := range tweet {
		// 表情符号通常占2个字符
		if isEmoji(r) {
			charCount += 2
		} else {
			charCount += 1
		}
	}

	// 判断是否超限
	if charCount > limit {
		return false, charCount, fmt.Sprintf("推文字符数（%d）超过限制（%d）", charCount, limit)
	}
	return true, charCount, fmt.Sprintf("推文字符数（%d）符合限制（%d）", charCount, limit)
}

// isEmoji 简单判断是否为表情符号（根据Unicode范围）
func isEmoji(r rune) bool {
	// 表情符号通常在Unicode范围 U+1F300 到 U+1F9FF
	return r >= 0x1F300 && r <= 0x1F9FF
}
