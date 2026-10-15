package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseReadings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.txt")
	const content = `# version: 9.9.9
# 整行注释
U+4E2D: zhōng,zhòng  # 中
U+4E00: yī  # 一
U+4E48: me,yāo,mó,ma  # 么
U+91CD:    # 空读音，应被跳过
这一行不是数据
U+ZZZZ: bad  # 非法码点
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := parseReadings(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[rune][]string{
		0x4E2D: {"zhōng", "zhòng"},
		0x4E00: {"yī"},
		0x4E48: {"me", "yāo", "mó", "ma"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseReadings = %v, want %v", got, want)
	}
}

func TestParseReadingsMissingFile(t *testing.T) {
	if _, err := parseReadings(filepath.Join(t.TempDir(), "nope.txt")); err == nil {
		t.Error("读取不存在的文件应当报错")
	}
}

func TestValidateAcceptsGoodData(t *testing.T) {
	good := map[rune][]string{
		0x4E2D: {"zhōng", "zhòng"},
		0x5973: {"nǚ", "nǜ", "rǔ"},
		0x6B38: {"āi", "ê̄"},
	}
	if err := validate(good); err != nil {
		t.Errorf("validate 拒绝了合法数据: %v", err)
	}
}

func TestValidateRejectsBadData(t *testing.T) {
	tests := []struct {
		name string
		dict map[rune][]string
	}{
		{"空读音列表", map[rune][]string{0x4E2D: {}}},
		{"空字符串读音", map[rune][]string{0x4E2D: {"zhōng", ""}}},
		{"重复读音", map[rune][]string{0x4E2D: {"zhōng", "zhōng"}}},
		{"非法音节", map[rune][]string{0x4E2D: {"zzz"}}},
		{"混入英文单词", map[rune][]string{0x4E2D: {"hello"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validate(tt.dict); err == nil {
				t.Error("validate 应当拒绝该数据，但通过了")
			}
		})
	}
}

func TestPutFirst(t *testing.T) {
	tests := []struct {
		pys  []string
		want string
		to   string
	}{
		{[]string{"zhòng", "chóng"}, "chóng", "chóng,zhòng"},
		{[]string{"zhòng", "chóng"}, "zhòng", "zhòng,chóng"},
		{[]string{"a", "b"}, "c", "c,a,b"},
	}
	for _, tt := range tests {
		got := putFirst(tt.pys, tt.want)
		if want := splitCSV(tt.to); !reflect.DeepEqual(got, want) {
			t.Errorf("putFirst(%v, %q) = %v, want %v", tt.pys, tt.want, got, want)
		}
	}
}

func TestInScope(t *testing.T) {
	common := map[rune][]string{0x20000: {"x"}}
	tests := []struct {
		r    rune
		want bool
	}{
		{0x3007, true},  // 〇
		{0x3400, true},  // 扩展 A 起始
		{0x9FFF, true},  // 统一表意文字结束
		{0x33FF, false}, // 扩展 A 之前
		{0xA000, false}, // 统一表意文字之后
		{0x20000, true}, // 在《通用规范汉字表》中，范围外也收录
		{0x41, false},   // ASCII
	}
	for _, tt := range tests {
		if got := inScope(tt.r, common); got != tt.want {
			t.Errorf("inScope(U+%04X) = %v, want %v", tt.r, got, tt.want)
		}
	}
}

func TestSourceVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v.txt")
	if err := os.WriteFile(path, []byte("# version: 1.2.3\n# source: x\nU+4E00: yī\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := sourceVersion(path); got != "1.2.3" {
		t.Errorf("sourceVersion = %q, want %q", got, "1.2.3")
	}
	if got := sourceVersion(filepath.Join(dir, "nope.txt")); got != "unknown" {
		t.Errorf("缺失文件时 = %q, want %q", got, "unknown")
	}
}

func TestCountPolyphonic(t *testing.T) {
	dict := map[rune][]string{
		0x4E2D: {"zhōng", "zhòng"},
		0x4E00: {"yī"},
		0x91CD: {"zhòng", "chóng", "tóng"},
	}
	if got := countPolyphonic(dict); got != 2 {
		t.Errorf("countPolyphonic = %d, want 2", got)
	}
}

func splitCSV(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return out
}
