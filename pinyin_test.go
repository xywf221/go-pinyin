package pinyin

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestConvertStyles(t *testing.T) {
	tests := []struct {
		in                         string
		normal, tone, tone2, tone3 string
		initials, firstLetter      string
	}{
		{"中国人", "zhongguoren", "zhōngguórén", "zho1ngguo2re2n", "zhong1guo2ren2", "zhgr", "zgr"},
		{"女", "nv", "nǚ", "nv3", "nv3", "n", "n"},
		{"绿", "lv", "lǜ", "lv4", "lv4", "l", "l"},
		// j/q/x/y 后的 u 不是 ü，保持原样写出。
		{"具", "ju", "jù", "ju4", "ju4", "j", "j"},
		{"去", "qu", "qù", "qu4", "qu4", "q", "q"},
		{"鱼", "yu", "yú", "yu2", "yu2", "y", "y"},
		// 零声母音节：声母风格返回首字母。
		{"啊", "a", "a", "a", "a", "a", "a"},
		{"儿", "er", "ér", "e2r", "er2", "e", "e"},
		{"〇", "ling", "líng", "li2ng", "ling2", "l", "l"},
		// 多音字默认取最常用读音。
		{"长", "zhang", "zhǎng", "zha3ng", "zhang3", "zh", "z"},
		{"行", "xing", "xíng", "xi2ng", "xing2", "x", "x"},
		{"乐", "le", "lè", "le4", "le4", "l", "l"},
		{"的", "de", "de", "de", "de", "d", "d"},
		{"一", "yi", "yī", "yi1", "yi1", "y", "y"},
		// 没有预组合字符的罕见音节，靠组合声调符号表达。
		{"嗯", "n", "ń", "n2", "n2", "n", "n"},
		{"呣", "m", "ḿ", "m2", "m2", "m", "m"},
	}
	for _, tt := range tests {
		got := []string{
			Convert(tt.in, Normal),
			Convert(tt.in, Tone),
			Convert(tt.in, Tone2),
			Convert(tt.in, Tone3),
			Convert(tt.in, Initials),
			Convert(tt.in, FirstLetter),
		}
		want := []string{tt.normal, tt.tone, tt.tone2, tt.tone3, tt.initials, tt.firstLetter}
		names := []string{"Normal", "Tone", "Tone2", "Tone3", "Initials", "FirstLetter"}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("Convert(%q, %s) = %q, want %q", tt.in, names[i], got[i], want[i])
			}
		}
	}
}

func TestPhraseAmbiguityIsResolved(t *testing.T) {
	// 重 的首音是 zhòng，但「重庆」是词组例外，默认启用消歧后得到 chóng qìng。
	// 词组匹配的完整行为见 phrase_test.go。
	if got := Convert("重庆", Tone); got != "chóngqìng" {
		t.Errorf("Convert(重庆, Tone) = %q, want %q", got, "chóngqìng")
	}
}

func TestNonHanIsPreserved(t *testing.T) {
	tests := []struct{ in, want string }{
		{"hello 中国! 123", "hello zhongguo! 123"},
		{"中国人abc", "zhongguorenabc"},
		{"  ", "  "},
		{"", ""},
		{"🎉中文", "🎉zhongwen"},
	}
	for _, tt := range tests {
		if got := Convert(tt.in, Normal); got != tt.want {
			t.Errorf("Convert(%q, Normal) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFallback(t *testing.T) {
	drop := Options{Fallback: func(r rune) string { return "" }}
	if got := ConvertWith("a中b", drop); got != "zhong" {
		t.Errorf("丢弃非汉字: got %q, want %q", got, "zhong")
	}

	// Fallback 只作用于字典中查不到的字符，汉字照常转换。
	upper := Options{Fallback: func(r rune) string { return strings.ToUpper(string(r)) }}
	if got := ConvertWith("a中", upper); got != "Azhong" {
		t.Errorf("改写非汉字: got %q, want %q", got, "Azhong")
	}
}

func TestSeparator(t *testing.T) {
	o := Options{Style: Tone, Separator: "-"}
	if got := ConvertWith("中国人", o); got != "zhōng-guó-rén" {
		t.Errorf("got %q, want %q", got, "zhōng-guó-rén")
	}
	// 分隔符只出现在相邻汉字之间，非汉字不参与。
	o = Options{Style: Normal, Separator: " "}
	if got := ConvertWith("中a国", o); got != "zhong a guo" {
		t.Errorf("got %q, want %q", got, "zhong a guo")
	}
}

func TestHeteronym(t *testing.T) {
	o := Options{Style: Normal, Heteronym: true}
	if got := ConvertWith("重", o); got != "zhong/chong/tong" {
		t.Errorf("got %q, want %q", got, "zhong/chong/tong")
	}
	// 中的 zhōng / zhòng 去掉声调后重合，应去重。
	if got := ConvertWith("中", o); got != "zhong" {
		t.Errorf("去声调后重合的读音应去重: got %q", got)
	}
	// 带声调时两个读音并不重合。
	o.Style = Tone
	if got := ConvertWith("中", o); got != "zhōng/zhòng" {
		t.Errorf("got %q, want %q", got, "zhōng/zhòng")
	}
}

func TestFirstLetters(t *testing.T) {
	tests := []struct{ in, want string }{
		{"中国人", "zgr"},
		{"中华人民共和国", "zhrmghg"},
		{"中国人abc", "zgrabc"},
		{"hello", "hello"},
	}
	for _, tt := range tests {
		if got := FirstLetters(tt.in); got != tt.want {
			t.Errorf("FirstLetters(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestReadings(t *testing.T) {
	got := Readings("重")
	want := []string{"zhòng", "chóng", "tóng"}
	if len(got) != 1 || strings.Join(got[0], ",") != strings.Join(want, ",") {
		t.Errorf("Readings(重) = %v, want %v", got, want)
	}

	// 返回值与输入 rune 一一对应，非汉字返回其自身。
	in := "a中🎉"
	got = Readings(in)
	if len(got) != utf8.RuneCountInString(in) {
		t.Fatalf("Readings(%q) 长度 = %d, want %d", in, len(got), utf8.RuneCountInString(in))
	}
	if len(got[0]) != 1 || got[0][0] != "a" {
		t.Errorf("Readings 非汉字项 = %v, want [a]", got[0])
	}
	if len(got[1]) < 2 || got[1][0] != "zhōng" {
		t.Errorf("Readings 汉字项 = %v", got[1])
	}
}

func TestParseSyllableCombining(t *testing.T) {
	tests := []struct {
		in           string
		base         string
		tone, toneAt int
	}{
		{"zhōng", "zhong", 1, 2},
		{"guó", "guo", 2, 2},
		{"nǚ", "nv", 3, 1},
		{"lǜ", "lv", 4, 1},
		{"de", "de", 0, -1},
		// ê̄ / ê̌ / m̄ / m̀ 以「字母 + 组合符号」形式出现。
		{"ê̄", "e", 1, 0},
		{"ê̌", "e", 3, 0},
		{"m̀", "m", 4, 0},
		{"ḿ", "m", 2, 0},
	}
	for _, tt := range tests {
		sy := parseSyllable(tt.in)
		if got := string(sy.base[:sy.n]); got != tt.base || sy.tone != tt.tone || sy.toneAt != tt.toneAt {
			t.Errorf("parseSyllable(%q) = {base:%q tone:%d toneAt:%d}, want {%q %d %d}",
				tt.in, got, sy.tone, sy.toneAt, tt.base, tt.tone, tt.toneAt)
		}
	}
}

func TestInitialOf(t *testing.T) {
	tests := []struct{ base, want string }{
		{"zhong", "zh"}, {"chong", "ch"}, {"shang", "sh"},
		{"zhao", "zh"}, {"zao", "z"}, {"cao", "c"}, {"sao", "s"},
		{"nv", "n"}, {"er", "e"}, {"ai", "a"}, {"ou", "o"},
		{"", ""},
		// 防御分支：数据源若混入非字母开头的内容，原样返回首字节。
		{"1x", "1"},
	}
	for _, tt := range tests {
		if got := initialOf([]byte(tt.base)); got != tt.want {
			t.Errorf("initialOf(%q) = %q, want %q", tt.base, got, tt.want)
		}
	}
}

// TestDictIntegrity 校验生成的数据表结构正确。
func TestDictIntegrity(t *testing.T) {
	if len(dict) < 26000 {
		t.Fatalf("字典条目过少: %d", len(dict))
	}
	for i, e := range dict {
		if e.cp == 0 || e.py == "" {
			t.Fatalf("dict[%d] 为空条目: %+v", i, e)
		}
		if i > 0 && dict[i-1].cp >= e.cp {
			t.Fatalf("dict 未按码点严格升序: dict[%d]=%U, dict[%d]=%U", i-1, dict[i-1].cp, i, e.cp)
		}
		for _, p := range strings.Split(e.py, ",") {
			if p == "" {
				t.Fatalf("dict[%d] (%c) 存在空读音: %q", i, e.cp, e.py)
			}
			if sy := parseSyllable(p); sy.n == 0 {
				t.Fatalf("dict[%d] (%c) 读音 %q 解析为空", i, e.cp, p)
			}
		}
	}
}

func TestStyleString(t *testing.T) {
	for s, want := range map[Style]string{
		Normal: "Normal", Tone: "Tone", Tone2: "Tone2",
		Tone3: "Tone3", Initials: "Initials", FirstLetter: "FirstLetter",
	} {
		if got := s.String(); got != want {
			t.Errorf("Style(%d).String() = %q, want %q", s, got, want)
		}
	}
	if got := Style(99).String(); got != "Style(99)" {
		t.Errorf("got %q", got)
	}
	for _, s := range allStyles {
		if !s.IsValid() {
			t.Errorf("Style(%d).IsValid() = false", s)
		}
	}
	if Style(99).IsValid() {
		t.Error("Style(99).IsValid() = true, 期望 false")
	}
	// 非法风格不 panic，退化为 Normal。
	if got, want := Convert("中", Style(99)), Convert("中", Normal); got != want {
		t.Errorf("非法风格的输出 = %q, 期望退化为 %q", got, want)
	}
}

func TestLookupMiss(t *testing.T) {
	if _, ok := lookup('A'); ok {
		t.Error("lookup('A') 不应命中")
	}
	if _, ok := lookup(0x30000); ok {
		t.Error("lookup(0x30000) 不应命中")
	}
}

var benchText = "中华人民共和国成立于一九四九年，首都是北京。"

// benchPhraseText 含大量多音字词组，用来观察词组消歧的开销。
var benchPhraseText = "重庆的银行行长在长沙研究音乐，西藏的藏族同胞还钱买便宜的长安汽车。"

func BenchmarkConvert(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Convert(benchText, Tone)
	}
}

// BenchmarkConvertCharByChar 与 BenchmarkConvert 的差值即词组消歧的开销。
func BenchmarkConvertCharByChar(b *testing.B) {
	o := Options{Style: Tone, CharByChar: true}
	for i := 0; i < b.N; i++ {
		_ = ConvertWith(benchText, o)
	}
}

func BenchmarkConvertPhraseHeavy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Convert(benchPhraseText, Tone)
	}
}

func BenchmarkFirstLetters(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = FirstLetters(benchText)
	}
}

// benchMixedText 是汉字与拉丁字母混排的文本，用来观察词组匹配在非汉字上的开销。
var benchMixedText = "hello 中华人民共和国 world 1949 Beijing 北京, ok!"

func BenchmarkConvertMixed(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Convert(benchMixedText, Tone)
	}
}
