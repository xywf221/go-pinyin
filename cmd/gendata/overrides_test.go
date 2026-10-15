package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "overrides.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseOverrides(t *testing.T) {
	path := writeFile(t, `# 注释行
U+8338: róng  # 茸  —— 鹿茸 lùróng
U+9887: pō    # 颇  —— 颇为 pōwéi
`)
	got, err := parseOverrides(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("解析出 %d 条，期望 2 条", len(got))
	}
	if got[0].cp != 0x8338 || got[0].py != "róng" || got[0].char != '茸' {
		t.Errorf("第 1 条 = %+v", got[0])
	}
	if got[1].cp != 0x9887 || got[1].py != "pō" {
		t.Errorf("第 2 条 = %+v", got[1])
	}
}

func TestParseOverridesRejectsBadInput(t *testing.T) {
	tests := []struct{ name, content string }{
		{"码点与汉字不符", "U+8339: róng  # 茸\n"},
		{"缺少冒号", "U+8338 róng  # 茸\n"},
		{"缺少读音", "U+8338:   # 茸\n"},
		{"缺少汉字注释", "U+8338: róng\n"},
		{"非法码点", "U+ZZZZ: róng  # 茸\n"},
		{"码点前缀错误", "8338: róng  # 茸\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseOverrides(writeFile(t, tt.content)); err == nil {
				t.Error("应当报错，但解析成功了")
			}
		})
	}
}

func TestApplyOverrides(t *testing.T) {
	path := writeFile(t, "U+8338: róng  # 茸\nU+9887: pō  # 颇\n")
	dict := map[rune][]string{
		0x8338: {"rōng", "róng", "rǒng"},
		0x9887: {"pō", "pǒ"},
		0x4E2D: {"zhōng", "zhòng"},
	}
	n, err := applyOverrides(dict, path)
	if err != nil {
		t.Fatal(err)
	}
	// 颇 的默认音已经是 pō，不应计入改动数。
	if n != 1 {
		t.Errorf("改动条数 = %d, 期望 1", n)
	}
	if got := dict[0x8338][0]; got != "róng" {
		t.Errorf("茸 的默认音 = %q, want %q", got, "róng")
	}
	if got := dict[0x8338]; len(got) != 3 {
		t.Errorf("校订不应改变读音数量: %v", got)
	}
	if got := dict[0x4E2D][0]; got != "zhōng" {
		t.Errorf("未校订的字不应被改动: %q", got)
	}
}

func TestApplyOverridesRejectsUnknownReading(t *testing.T) {
	path := writeFile(t, "U+8338: zzz  # 茸\n")
	dict := map[rune][]string{0x8338: {"rōng", "róng"}}
	if _, err := applyOverrides(dict, path); err == nil {
		t.Error("读音不在列表中时应当报错")
	}
}

func TestApplyOverridesRejectsOutOfScope(t *testing.T) {
	path := writeFile(t, "U+41: a  # A\n")
	dict := map[rune][]string{0x8338: {"róng"}}
	if _, err := applyOverrides(dict, path); err == nil {
		t.Error("校订不在收录范围内的字时应当报错")
	}
}

// TestShippedOverridesAreValid 用真实校订表跑一遍，确保它始终自洽。
func TestShippedOverridesAreValid(t *testing.T) {
	overrides, err := parseOverrides("../../data/overrides.txt")
	if err != nil {
		t.Fatalf("data/overrides.txt 解析失败: %v", err)
	}
	if len(overrides) == 0 {
		t.Fatal("data/overrides.txt 为空")
	}
	seen := make(map[rune]bool)
	for _, o := range overrides {
		if seen[o.cp] {
			t.Errorf("U+%04X (%c) 重复校订", o.cp, o.char)
		}
		seen[o.cp] = true
	}
}
