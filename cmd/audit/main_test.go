package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTemp 把内容写进临时文件，返回路径。
func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadSource(t *testing.T) {
	src := strings.Join([]string{
		"# 注释",
		"",
		"重庆: chóng qìng",
		"朝阳: zhāo yáng",
		"朝阳_2: cháo yáng", // 多音标记，去掉后缀后与上一条重复，保留先出现的
		"肚子: dǔ zi",
		"标点: zhè shì # 行内注释",
	}, "\n")

	s, err := loadSource("测试", writeTemp(t, "r.txt", src))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"重庆": "chóng qìng",
		"朝阳": "zhāo yáng",
		"肚子": "dǔ zi",
		"标点": "zhè shì",
	}
	if len(s.table) != len(want) {
		t.Fatalf("解析出 %d 条，want %d: %v", len(s.table), len(want), s.table)
	}
	for phrase, readings := range want {
		if got := strings.Join(s.table[phrase], " "); got != readings {
			t.Errorf("%s = %q, want %q", phrase, got, readings)
		}
	}
}

func TestLoadSourceStripsBOM(t *testing.T) {
	s, err := loadSource("测试", writeTemp(t, "bom.txt", "\ufeff重庆: chóng qìng\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.table["重庆"]; !ok {
		t.Errorf("带 BOM 的首行未解析: %v", s.table)
	}
}

func TestLoadSourceErrors(t *testing.T) {
	if _, err := loadSource("测试", writeTemp(t, "bad.txt", "没有冒号\n")); err == nil {
		t.Error("缺少冒号应当报错")
	}
	if _, err := loadSource("测试", writeTemp(t, "empty.txt", "# 只有注释\n")); err == nil {
		t.Error("空文件应当报错")
	}
	if _, err := loadSource("测试", "不存在的文件.txt"); err == nil {
		t.Error("文件不存在应当报错")
	}
}

func TestApplyOverrides(t *testing.T) {
	s, err := loadSource("测试", writeTemp(t, "r.txt", "秦桧: qín guì\n漂河: piāo hé\n"))
	if err != nil {
		t.Fatal(err)
	}
	n, err := applyOverrides(s, writeTemp(t, "o.txt", "秦桧: qín huì\n"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("改动 %d 条，want 1", n)
	}
	if got := strings.Join(s.table["秦桧"], " "); got != "qín huì" {
		t.Errorf("秦桧 = %q, want %q", got, "qín huì")
	}
}

func TestApplyOverridesErrors(t *testing.T) {
	s, err := loadSource("测试", writeTemp(t, "r.txt", "秦桧: qín guì\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applyOverrides(s, writeTemp(t, "o.txt", "不存在: bù cún zài\n")); err == nil {
		t.Error("词组不在表中应当报错")
	}
	if _, err := applyOverrides(s, writeTemp(t, "o.txt", "秦桧 没有冒号\n")); err == nil {
		t.Error("格式错误应当报错")
	}
	if _, err := applyOverrides(s, "不存在的文件.txt"); err == nil {
		t.Error("文件不存在应当报错")
	}
}

func TestSplitTone(t *testing.T) {
	tests := []struct {
		in   string
		base string
		tone int
	}{
		{"zhong1", "zhong", 1},
		{"zhong", "zhong", 0},
		{"nv3", "nv", 3},
		{"de", "de", 0},
		{"", "", 0},
	}
	for _, tt := range tests {
		base, tone := splitTone(tt.in)
		if base != tt.base || tone != tt.tone {
			t.Errorf("splitTone(%q) = (%q, %d), want (%q, %d)", tt.in, base, tone, tt.base, tt.tone)
		}
	}
}

func TestDiffPosition(t *testing.T) {
	tests := []struct {
		char, ours, rival string
		want              byte
	}{
		{"中", "zhōng", "zhōng", 0},
		{"中", "zhòng", "zhōng", 'B'},
		{"妈", "mā", "má", 'B'},
		// 重 chóng 与 zhòng 声母就不同（ch 对 zh），属于 A 而不是 B。
		{"重", "chóng", "zhòng", 'A'},
		{"朴", "piáo", "pǔ", 'A'},
		{"说", "shuì", "shuō", 'A'},
		{"子", "zǐ", "zi", 'C'},
		{"子", "zi", "zǐ", 'C'},
		{"一", "yì", "yī", 'D'},
		{"不", "bú", "bù", 'D'},
		{"不", "bú", "bu", 'D'},
		{"庆", "qìng", "qìng", 0},
	}
	for _, tt := range tests {
		if got := diffPosition([]rune(tt.char)[0], tt.ours, tt.rival); got != tt.want {
			t.Errorf("diffPosition(%q, %q, %q) = %q, want %q", tt.char, tt.ours, tt.rival, got, tt.want)
		}
	}
}

func TestDiffCategory(t *testing.T) {
	tests := []struct {
		phrase      string
		ours, rival []string
		want        byte
	}{
		{"秦桧", []string{"qín", "guì"}, []string{"qín", "huì"}, 'A'},
		{"半间不界", []string{"bàn", "jiān", "bù", "jiè"}, []string{"bàn", "gān", "bù", "gà"}, 'A'},
		{"肚子", []string{"dǔ", "zi"}, []string{"dù", "zi"}, 'B'},
		{"肚子", []string{"dù", "zi"}, []string{"dù", "zǐ"}, 'C'},
		{"一气呵成", []string{"yī", "qì", "hē", "chéng"}, []string{"yí", "qì", "hē", "chéng"}, 'D'},
		{"重庆", []string{"chóng", "qìng"}, []string{"chóng", "qìng"}, 0},
		// 音节数与字数不符的参考数据不可信，不参与判定。
		{"重庆", []string{"chóng", "qìng"}, []string{"chóng"}, 0},
	}
	for _, tt := range tests {
		if got := diffCategory(tt.phrase, tt.ours, tt.rival); got != tt.want {
			t.Errorf("diffCategory(%q, %v, %v) = %q, want %q", tt.phrase, tt.ours, tt.rival, got, tt.want)
		}
	}
}

func TestSeverityOrder(t *testing.T) {
	if severity('A') <= severity('B') || severity('B') <= severity('C') ||
		severity('C') <= severity('D') || severity('D') <= severity(0) {
		t.Error("类别严重程度应为 A > B > C > D > 一致")
	}
}

func TestRowRivalAndPeak(t *testing.T) {
	r := row{
		phrase: "秦桧",
		ours:   []string{"qín", "guì"},
		votes: map[string][]string{
			"qín huì": {"汉典", "CC-CEDICT"},
		},
	}
	if got, ok := r.rival(); !ok || got != "qín huì" {
		t.Errorf("rival = %q, %v", got, ok)
	}
	if got := r.peak(); got != 2 {
		t.Errorf("peak = %d, want 2", got)
	}
	if got := r.category(); got != 'A' {
		t.Errorf("category = %q, want A", got)
	}

	// 所有来源都与我们一致时，没有可竞争的读音。
	same := row{phrase: "重庆", ours: []string{"chóng", "qìng"},
		votes: map[string][]string{"chóng qìng": {"汉典"}}}
	if _, ok := same.rival(); ok {
		t.Error("无人反对时不应有 rival")
	}
	if got := same.category(); got != 0 {
		t.Errorf("category = %q, want 0", got)
	}
}

func TestReport(t *testing.T) {
	ours := &source{name: "本仓库", table: map[string][]string{
		"秦桧":   {"qín", "guì"},
		"古朴":   {"gǔ", "piáo"},
		"重庆":   {"chóng", "qìng"},
		"一口气":  {"yī", "kǒu", "qì"},
		"半间不界": {"bàn", "jiān", "bù", "jiè"},
	}}
	han := &source{name: "汉典", table: map[string][]string{
		"秦桧":   {"qín", "huì"},
		"古朴":   {"gǔ", "pǔ"},
		"重庆":   {"chóng", "qìng"},
		"一口气":  {"yí", "kǒu", "qì"},
		"半间不界": {"bàn", "gān", "bù", "gà"},
	}}
	cc := &source{name: "CC-CEDICT", table: map[string][]string{
		"秦桧": {"qín", "huì"},
		"古朴": {"gǔ", "pǔ"},
	}}
	var buf bytes.Buffer
	report(&buf, ours, []*source{han, cc}, 2, 6, 2, "")
	out := buf.String()

	// 秦桧、古朴 各有汉典与 CC-CEDICT 两票反对我们；半间不界、一口气 只有汉典一票。
	if !strings.Contains(out, "至少 2 个来源一致、且与我们不同：2 条") {
		t.Errorf("-min-agree 2 应留下 2 条：\n%s", out)
	}
	for _, want := range []string{"A 秦桧", "A 古朴", "qín huì", "gǔ pǔ"} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺少 %q：\n%s", want, out)
		}
	}
	// 一票的条目、以及读音一致的重庆都不该出现在清单里。
	for _, unwanted := range []string{"半间不界", "一口气", "重庆"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("输出不该包含 %q：\n%s", unwanted, out)
		}
	}
}
