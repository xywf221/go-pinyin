package pinyin

import (
	"unicode/utf8"

	"github.com/xywf221/go-pinyin/internal/tone"
)

// maxBaseLen 是去掉声调后音节的最大字节数，最长的 "zhuang" 为 6。
const maxBaseLen = 8

// syllable 是解析后的单个音节。内部用定长数组存放基本形式，不产生堆分配。
type syllable struct {
	raw    string // 原始带声调写法，如 "zhōng"
	base   [maxBaseLen]byte
	n      int // base 中的有效字节数
	tone   int // 声调 0-4，0 为轻声
	toneAt int // base 中承载声调的字母下标，-1 表示无声调
}

// parseSyllable 把带声调音节拆成「基本形式 + 声调 + 声调位置」。
func parseSyllable(s string) syllable {
	sy := syllable{raw: s, toneAt: -1}
	for _, r := range s {
		if info, ok := tone.Lookup(r); ok {
			if sy.toneAt < 0 {
				sy.toneAt = sy.n
			}
			if sy.n < maxBaseLen {
				sy.base[sy.n] = info.Base
				sy.n++
			}
			if info.Tone != 0 {
				sy.tone = info.Tone
			}
			continue
		}
		if t, ok := tone.Combining(r); ok {
			// 组合符号作用于它前面的那个字母。
			if sy.toneAt < 0 && sy.n > 0 {
				sy.toneAt = sy.n - 1
			}
			sy.tone = t
			continue
		}
		// 数据里不应出现非拼音字符；真出现了就收下 ASCII，其余忽略。
		if r < utf8.RuneSelf && sy.n < maxBaseLen {
			sy.base[sy.n] = byte(r)
			sy.n++
		}
	}
	return sy
}

// appendTo 按风格把音节追加到 b，返回追加后的切片。
func (sy *syllable) appendTo(b []byte, style Style) []byte {
	switch style {
	case Tone:
		return append(b, sy.raw...)
	case Tone2:
		// 声调数字紧跟承载声调的那个韵母字母。
		if sy.tone == 0 || sy.toneAt < 0 {
			return append(b, sy.base[:sy.n]...)
		}
		b = append(b, sy.base[:sy.toneAt+1]...)
		b = append(b, byte('0'+sy.tone))
		return append(b, sy.base[sy.toneAt+1:sy.n]...)
	case Tone3:
		b = append(b, sy.base[:sy.n]...)
		if sy.tone != 0 {
			b = append(b, byte('0'+sy.tone))
		}
		return b
	case Initials:
		return append(b, initialOf(sy.base[:sy.n])...)
	case FirstLetter:
		if sy.n > 0 {
			b = append(b, sy.base[0])
		}
		return b
	default: // Normal
		return append(b, sy.base[:sy.n]...)
	}
}

// letterStr 单字母字符串表，让声母/首字母的返回不产生分配。
var letterStr = [26]string{
	"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m",
	"n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
}

// initialOf 取音节的声母。
//
// 拼音里只有 zh/ch/sh 是双字母声母，其余声母都是单字母，因此规则可简化为：
// 以 zh/ch/sh 开头返回这两个字母，否则返回首字母。
// 零声母音节（a/o/e 开头）也返回首字母。
func initialOf(base []byte) string {
	if len(base) >= 2 {
		switch {
		case base[0] == 'z' && base[1] == 'h':
			return "zh"
		case base[0] == 'c' && base[1] == 'h':
			return "ch"
		case base[0] == 's' && base[1] == 'h':
			return "sh"
		}
	}
	if len(base) == 0 {
		return ""
	}
	if c := base[0]; c >= 'a' && c <= 'z' {
		return letterStr[c-'a']
	}
	return string(base[:1])
}
