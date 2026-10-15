// Package tone 提供汉语拼音声调字符的对照表。
//
// 核心库用它把带声调音节拆成「基本形式 + 声调」，数据生成器用它做音节合法性校验。
// 两个包共用同一份对照表，避免规则漂移。
package tone

import "strings"

// Info 描述一个带声调字母。
type Info struct {
	Base byte // 对应的 ASCII 基本字母，ü 系一律记作 'v'
	Tone int  // 声调 1-4；0 表示该字母本身不带声调
}

// table 是数据源中出现过的全部带声调字符。
var table = map[rune]Info{
	'ā': {'a', 1}, 'á': {'a', 2}, 'ǎ': {'a', 3}, 'à': {'a', 4},
	'ē': {'e', 1}, 'é': {'e', 2}, 'ě': {'e', 3}, 'è': {'e', 4},
	'ī': {'i', 1}, 'í': {'i', 2}, 'ǐ': {'i', 3}, 'ì': {'i', 4},
	'ō': {'o', 1}, 'ó': {'o', 2}, 'ǒ': {'o', 3}, 'ò': {'o', 4},
	'ū': {'u', 1}, 'ú': {'u', 2}, 'ǔ': {'u', 3}, 'ù': {'u', 4},
	'ǖ': {'v', 1}, 'ǘ': {'v', 2}, 'ǚ': {'v', 3}, 'ǜ': {'v', 4},
	'ü': {'v', 0},
	'ń': {'n', 2}, 'ň': {'n', 3}, 'ǹ': {'n', 4},
	'ḿ': {'m', 2},
	'ê': {'e', 0}, 'ế': {'e', 2}, 'ề': {'e', 4},
}

// combining 是组合声调符号：U+0300 抑音符、U+0304 长音符、U+030C 抑扬符。
// ê̄、ê̌、m̄、m̀ 这类音节没有预组合字符，只能以「字母 + 组合符号」的形式出现。
var combining = map[rune]int{
	'̀': 4, // 抑音符
	'̄': 1, // 长音符
	'̌': 3, // 抑扬符
}

// Lookup 查询带声调字母对应的基本字母与声调。
func Lookup(r rune) (Info, bool) {
	i, ok := table[r]
	return i, ok
}

// Combining 查询组合声调符号对应的声调。
func Combining(r rune) (int, bool) {
	t, ok := combining[r]
	return t, ok
}

// Base 去掉声调，返回 ASCII 基本形式（ü 写作 v）。
// 组合声调符号会被丢弃，其余无法识别的字符原样保留。
func Base(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case isAccented(r):
			info := table[r]
			b.WriteByte(info.Base)
		case isCombining(r):
			// 组合符号不产生字符。
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Normalize 把带声调音节归一为「基本形式 + 声调数字」的规范形式，供等价比较使用。
//
// 声调为 0（轻声）时省略数字，因此 zhōng → "zhong1"、de → "de"、nǚ → "nv3"、
// m̀ → "m4"。同一音节的不同书写方式（预组合字符与组合符号、ü 与 v）归一后相同，
// 生成器据此判断词组读音是否与逐字默认读音一致。
func Normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 1)
	t := 0
	for _, r := range s {
		if info, ok := table[r]; ok {
			b.WriteByte(info.Base)
			if info.Tone != 0 {
				t = info.Tone
			}
			continue
		}
		if v, ok := combining[r]; ok {
			t = v
			continue
		}
		b.WriteRune(r)
	}
	if t != 0 {
		b.WriteByte(byte('0' + t))
	}
	return b.String()
}

func isAccented(r rune) bool {
	_, ok := table[r]
	return ok
}

func isCombining(r rune) bool {
	_, ok := combining[r]
	return ok
}
