package pinyin

import "strconv"

// Style 决定拼音的输出形式。
type Style uint8

// 可用的输出风格。Zero 值即 Normal。
const (
	Normal      Style = iota // 不带声调：zhong
	Tone                     // 带声调符号：zhōng
	Tone2                    // 声调数字紧跟韵母：zho1ng
	Tone3                    // 声调数字在音节末尾：zhong1
	Initials                 // 声母：zh（零声母音节返回其首字母）
	FirstLetter              // 首字母：z
)

var styleNames = [...]string{"Normal", "Tone", "Tone2", "Tone3", "Initials", "FirstLetter"}

// String 返回风格名，如 "Tone3"。未定义的风格返回 "Style(n)"。
func (s Style) String() string {
	if s.IsValid() {
		return styleNames[s]
	}
	return "Style(" + strconv.Itoa(int(s)) + ")"
}

// IsValid 报告 s 是否是已定义的风格。
//
// 传入未定义的风格不会 panic，但输出会退化为 Normal，调用方可用本方法提前校验。
func (s Style) IsValid() bool {
	return int(s) < len(styleNames)
}
