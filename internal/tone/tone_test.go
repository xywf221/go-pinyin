package tone

import "testing"

func TestLookup(t *testing.T) {
	tests := []struct {
		r    rune
		base byte
		tone int
		ok   bool
	}{
		{'ā', 'a', 1, true},
		{'á', 'a', 2, true},
		{'ǎ', 'a', 3, true},
		{'à', 'a', 4, true},
		{'ǜ', 'v', 4, true},
		{'ü', 'v', 0, true},
		{'ḿ', 'm', 2, true},
		{'ǹ', 'n', 4, true},
		{'ê', 'e', 0, true},
		{'ế', 'e', 2, true},
		{'a', 0, 0, false},
		{'中', 0, 0, false},
		{'̄', 0, 0, false}, // 组合符号不在 table 中
	}
	for _, tt := range tests {
		got, ok := Lookup(tt.r)
		if ok != tt.ok || (ok && (got.Base != tt.base || got.Tone != tt.tone)) {
			t.Errorf("Lookup(%q) = {%c %d}, %v; want {%c %d}, %v",
				tt.r, got.Base, got.Tone, ok, tt.base, tt.tone, tt.ok)
		}
	}
}

func TestCombining(t *testing.T) {
	tests := []struct {
		r    rune
		tone int
		ok   bool
	}{
		{'̀', 4, true}, // 抑音符
		{'̄', 1, true}, // 长音符
		{'̌', 3, true}, // 抑扬符
		{'a', 0, false},
		{'ā', 0, false}, // 预组合字符不在 combining 中
	}
	for _, tt := range tests {
		got, ok := Combining(tt.r)
		if ok != tt.ok || (ok && got != tt.tone) {
			t.Errorf("Combining(U+%04X) = %d, %v; want %d, %v", tt.r, got, ok, tt.tone, tt.ok)
		}
	}
}

func TestBase(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"de", "de"},
		{"zhōng", "zhong"},
		{"guó", "guo"},
		{"nǚ", "nv"},
		{"lǜ", "lv"},
		{"lüè", "lve"},
		{"ḿ", "m"},
		{"ńg", "ng"},
		{"ê̄", "e"},  // 字母 + 组合长音符
		{"ê̌", "e"},  // 字母 + 组合抑扬符
		{"m̀", "m"},  // 字母 + 组合抑音符
		{"a1", "a1"}, // 非拼音字符原样保留
	}
	for _, tt := range tests {
		if got := Base(tt.in); got != tt.want {
			t.Errorf("Base(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"de", "de"}, // 轻声不写声调数字
		{"zhōng", "zhong1"},
		{"guó", "guo2"},
		{"nǚ", "nv3"},
		{"lǜ", "lv4"},
		{"ḿ", "m2"},
		{"ê̄", "e1"}, // 字母 + 组合长音符
		{"ê̌", "e3"},
		{"m̀", "m4"},
		// 单独出现的带声调字母只是韵母，不带声母。
		{"ǚ", "v3"},
		{"ü", "v"},
	}
	for _, tt := range tests {
		if got := Normalize(tt.in); got != tt.want {
			t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	// 同一个音节的两种写法归一后相同，声调不同的则必须区分开。
	if Normalize("nǚ") != Normalize("nü3") {
		t.Error("nǚ 与 nü3 应归一为同一形式")
	}
	if Normalize("zhōng") == Normalize("zhòng") {
		t.Error("zhōng 与 zhòng 不应归一为同一形式")
	}
	if Normalize("zi") == Normalize("zǐ") {
		t.Error("zi 与 zǐ 不应归一为同一形式")
	}
}
