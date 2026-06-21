package model

import (
	"strconv"
	"strings"
)

// avatar_media_id 在 DB 是 bigint(#550,雪花 media id;0 = 无头像哨兵),但 wire/proto 仍是
// 十进制字符串(ADR media-msg-id-bigint-migration #529 §2)。这两个 helper 在 user-rpc 边界做
// int64 ↔ string 的归一转换,空串与 0 互为「无头像」。

// ParseAvatarMediaID 把 wire 上的十进制头像 media id 串转成 DB int64;空/空白串 → 0(无头像)。
// 非空但非十进制串返回 strconv 错误,由调用方折成 InvalidArgument。
func ParseAvatarMediaID(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

// FormatAvatarMediaID 把 DB int64 头像 media id 转回 wire 串;0(无头像)→ ""。
func FormatAvatarMediaID(id int64) string {
	if id == 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}
