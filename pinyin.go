package pinyin

import (
	"strings"
	"unicode/utf8"
)

// Version 是当前版本号。
const Version = "0.1.0"

// heteroSep 分隔同一个汉字的不同读音。
const heteroSep = "/"

// Options 控制 ConvertWith 的行为。零值即默认配置：
// Normal 风格、音节直接相连、开启词组消歧、非汉字字符原样保留。
type Options struct {
	// Style 输出风格。
	Style Style

	// Separator 是相邻音节之间的分隔符，默认为空，即音节直接相连。
	// 词组内部的音节之间同样会插入分隔符。
	Separator string

	// Heteronym 为 true 时，每个汉字输出其全部读音，读音之间用 "/" 分隔。
	// 该模式与词组消歧互斥：要的是每个字的全部读音，就按单字输出。
	Heteronym bool

	// Fallback 处理字典中查不到的字符（含全部非汉字字符）。
	// 返回空串表示丢弃该字符。为 nil 时原样保留。
	Fallback func(r rune) string

	// CharByChar 为 true 时逐字转换，不做词组消歧。
	// 默认（false）会先做正向最大匹配，因此「重庆」得到 chóngqìng 而不是 zhòngqìng。
	CharByChar bool
}

// Convert 把 s 中的汉字转换为指定风格的拼音。
//
// 非汉字字符原样保留，音节之间不加分隔，多音字按词组消歧后取读音
// （例如「重庆」得到 chóngqìng），词组之外的单个汉字取最常用读音。
func Convert(s string, style Style) string {
	return ConvertWith(s, Options{Style: style})
}

// ConvertWith 按 o 的配置转换 s。o 为零值时等价于 Convert(s, Normal)。
//
// 默认先做词组消歧（正向最大匹配），因此「重庆」得到 chóngqìng 而不是 zhòngqìng。
// 详见 Options.CharByChar 与 Options.Heteronym。
func ConvertWith(s string, o Options) string {
	buf := make([]byte, 0, len(s)*2)
	wrote := false
	for i := 0; i < len(s); {
		start := len(buf)
		if wrote && o.Separator != "" {
			buf = append(buf, o.Separator...)
		}
		mark := len(buf)
		consumed := 0
		// 多音字模式要的是每个字的全部读音，词组消歧与之互斥。
		if !o.CharByChar && !o.Heteronym {
			if n, readings := matchPhrase(s, i); n > 0 {
				consumed = n
				if readings != "" {
					appendReadings(&buf, readings, o)
				} else {
					// 该词组的读音与逐字默认读音一致，按单字写出即可，
					// 但它同样占住这 n 个字节，不让更短的词组从中间插进来。
					appendRunes(&buf, s[i:i+n], o)
				}
			}
		}
		if consumed == 0 {
			r, size := utf8.DecodeRuneInString(s[i:])
			appendRune(&buf, r, o)
			consumed = size
		}
		i += consumed
		if len(buf) == mark {
			// 该位置没有产生输出，回退掉刚写入的分隔符。
			buf = buf[:start]
			continue
		}
		wrote = true
	}
	return string(buf)
}

// appendReadings 把一个词组的读音（空格分隔的带声调音节）按 o 追加到 buf。
//
// 音节之间使用 o.Separator，与逐字转换时的表现保持一致，因此
// ConvertWith("重庆", Options{Separator: "-"}) 得到 "chóng-qìng"。
func appendReadings(buf *[]byte, readings string, o Options) {
	first := true
	for len(readings) > 0 {
		p := readings
		if i := strings.IndexByte(readings, ' '); i >= 0 {
			p, readings = readings[:i], readings[i+1:]
		} else {
			readings = ""
		}
		if !first && o.Separator != "" {
			*buf = append(*buf, o.Separator...)
		}
		first = false
		sy := parseSyllable(p)
		*buf = sy.appendTo(*buf, o.Style)
	}
}

// appendRunes 把 s 中的每个字符按 o 逐字转换后追加到 buf，音节之间插入 o.Separator。
//
// 与 appendReadings 的分隔行为保持一致，因此词组内部与词组之间的分隔符表现相同。
func appendRunes(buf *[]byte, s string, o Options) {
	first := true
	for _, r := range s {
		if !first && o.Separator != "" {
			*buf = append(*buf, o.Separator...)
		}
		first = false
		appendRune(buf, r, o)
	}
}

// FirstLetters 返回 s 中每个汉字的首字母，非汉字字符原样保留。
// 等价于 Convert(s, FirstLetter)，即 pinyin.FirstLetters("中国人") == "zgr"。
func FirstLetters(s string) string {
	return Convert(s, FirstLetter)
}

// Readings 返回 s 中每个字符的全部读音（带声调符号）。
//
// 返回值与 s 的 rune 一一对应，即 len(Readings(s)) == utf8.RuneCountInString(s)。
// 非汉字字符或字典中查不到的字符，返回只含其自身的一个元素的切片。
// 每个元素的首项是该字最常用的读音。
func Readings(s string) [][]string {
	out := make([][]string, 0, utf8.RuneCountInString(s))
	for _, r := range s {
		if py, ok := lookup(r); ok {
			out = append(out, strings.Split(py, ","))
			continue
		}
		out = append(out, []string{string(r)})
	}
	return out
}

// appendRune 把字符 r 按 o 转换后追加到 buf。没有写入任何内容即表示该字符被丢弃。
func appendRune(buf *[]byte, r rune, o Options) {
	py, ok := lookup(r)
	if !ok {
		if o.Fallback != nil {
			*buf = append(*buf, o.Fallback(r)...)
			return
		}
		*buf = utf8.AppendRune(*buf, r)
		return
	}
	if !o.Heteronym {
		if i := strings.IndexByte(py, ','); i >= 0 {
			py = py[:i]
		}
		sy := parseSyllable(py)
		*buf = sy.appendTo(*buf, o.Style)
		return
	}
	// 多音字模式：输出全部读音。不同读音去掉声调后可能重合（如「中」的
	// zhōng 和 zhòng 在 Normal 风格下都是 zhong），需要去重。
	var out []string
	for _, p := range strings.Split(py, ",") {
		sy := parseSyllable(p)
		if s := string(sy.appendTo(nil, o.Style)); s != "" && !containsStr(out, s) {
			out = append(out, s)
		}
	}
	*buf = append(*buf, strings.Join(out, heteroSep)...)
}

// containsStr 报告 slice 中是否已存在 s。单个字的读音数量很少，线性扫描足够。
func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// lookup 在按码点升序排列的字典里二分查找读音串。
func lookup(r rune) (string, bool) {
	lo, hi := 0, len(dict)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if dict[mid].cp < r {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(dict) && dict[lo].cp == r {
		return dict[lo].py, true
	}
	return "", false
}
