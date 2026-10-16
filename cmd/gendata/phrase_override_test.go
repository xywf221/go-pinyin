package main

import (
	"strings"
	"testing"
)

func TestParsePhraseOverrides(t *testing.T) {
	src := strings.Join([]string{
		"# 只有注释的行",
		"",
		"   ",
		"重庆: chóng qìng  # 说明里的 # 不影响解析",
		"半间不界: bàn gān bù gà",
	}, "\n")

	got, err := parsePhraseOverrides(writeTemp(t, "o.txt", src))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("解析出 %d 条，want 2: %v", len(got), got)
	}
	if g := got["重庆"]; g.readings != "chóng qìng" {
		t.Errorf("重庆 = %q, want %q", g.readings, "chóng qìng")
	}
	if g := got["半间不界"]; g.readings != "bàn gān bù gà" {
		t.Errorf("半间不界 = %q, want %q", g.readings, "bàn gān bù gà")
	}
}

func TestParsePhraseOverridesErrors(t *testing.T) {
	tests := []struct{ name, src, wantErr string }{
		{"缺少冒号", "重庆 chóng qìng", "格式应为"},
		{"缺少读音", "重庆:", "格式应为"},
		{"缺少词组", ": chóng qìng", "格式应为"},
		{"重复校订", "重庆: chóng qìng\n重庆: zhòng qìng", "重复校订"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePhraseOverrides(writeTemp(t, "o.txt", tt.src))
			if err == nil {
				t.Fatal("应当报错")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want 含 %q", err, tt.wantErr)
			}
		})
	}

	if _, err := parsePhraseOverrides("不存在的文件.txt"); err == nil {
		t.Error("文件不存在应当报错")
	}
}

// applyPhrase 跑一遍「建表 → 校订」，供 applyPhraseOverrides 的测试使用。
func applyPhrase(t *testing.T, source, overrides string) ([]phraseEntry, int, error) {
	t.Helper()
	got, _, err := buildPhrases(miniDict(), writeTemp(t, "p.txt", source))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parsePhraseOverrides(writeTemp(t, "o.txt", overrides))
	if err != nil {
		t.Fatal(err)
	}
	n, err := applyPhraseOverrides(got, miniDict(), parsed)
	return got, n, err
}

func TestApplyPhraseOverridesOK(t *testing.T) {
	got, n, err := applyPhrase(t,
		"重庆: zhòng qìng\n一下子: yī xià zi",
		"重庆: chóng qìng\n一下子: yī xià zǐ")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("改动 %d 条，want 2", n)
	}
	for _, e := range got {
		if e.phrase == "重庆" && e.readings != "chóng qìng" {
			t.Errorf("重庆 = %q, want %q", e.readings, "chóng qìng")
		}
		if e.phrase == "一下子" && e.readings != "yī xià zǐ" {
			t.Errorf("一下子 = %q, want %q", e.readings, "yī xià zǐ")
		}
		// 两条校订后的读音都还原不出单字默认读音（重的 zhòng、子的 zi），
		// 因此要保留读音字段。
		if !e.differs {
			t.Errorf("%q 校订后的读音与单字默认读音不同，应当保留读音字段", e.phrase)
		}
	}
}

// TestApplyPhraseOverridesToDefaults 覆盖「校订回单字默认读音」的情形：
// 源表在词组里给了错读音，而单字默认读音本来就是对的，校订后读音字段被清空。
func TestApplyPhraseOverridesToDefaults(t *testing.T) {
	got, n, err := applyPhrase(t,
		"重庆: chóng qìng",
		"重庆: zhòng qìng")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("改动 %d 条，want 1", n)
	}
	e := got[0]
	if e.differs {
		t.Error("校订回单字默认读音后不该保留读音字段")
	}
	if e.readings != "zhòng qìng" {
		t.Errorf("readings = %q, want %q", e.readings, "zhòng qìng")
	}
}

func TestApplyPhraseOverridesErrors(t *testing.T) {
	tests := []struct{ name, source, overrides, wantErr string }{
		{"词组不在源表", "重庆: zhòng qìng", "中国: zhōng guó", "不在源表中"},
		{"音节数不符", "重庆: zhòng qìng", "重庆: chóng", "却给了"},
		{"非规范音节", "重庆: zhòng qìng", "重庆: chóng qxìng", "不在规范音节清单中"},
		{"与源表相同", "重庆: zhòng qìng", "重庆: zhòng qìng", "与源表相同"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := applyPhrase(t, tt.source, tt.overrides)
			if err == nil {
				t.Fatal("应当报错")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want 含 %q", err, tt.wantErr)
			}
		})
	}
}
