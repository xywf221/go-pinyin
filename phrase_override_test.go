package pinyin

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// phraseOverride 是 data/phrase_overrides.txt 里的一条人工校订。
type phraseOverride struct {
	phrase   string
	readings []string
}

// loadPhraseOverrides 读取人工校订表。
func loadPhraseOverrides(t *testing.T) []phraseOverride {
	t.Helper()
	f, err := os.Open("data/phrase_overrides.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	var out []phraseOverride
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		body, _, _ := strings.Cut(line, "#")
		body = strings.TrimSpace(body)
		if body == "" {
			continue
		}
		phrase, rest, ok := strings.Cut(body, ":")
		phrase = strings.TrimSpace(phrase)
		fields := strings.Fields(rest)
		if !ok || phrase == "" || len(fields) == 0 {
			t.Fatalf("data/phrase_overrides.txt:%d 格式有误: %q", n, line)
		}
		out = append(out, phraseOverride{phrase: phrase, readings: fields})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("data/phrase_overrides.txt 里没有任何校订")
	}
	return out
}

// phraseOverrideReadings 返回 词组 → 读音（空格分隔）的映射，
// 供那些拿源表当基准的测试先把人工校订套上去。
func phraseOverrideReadings(t *testing.T) map[string]string {
	t.Helper()
	out := make(map[string]string)
	for _, o := range loadPhraseOverrides(t) {
		out[o.phrase] = strings.Join(o.readings, " ")
	}
	return out
}

// TestPhraseOverridesApplied 逐条核对人工校订在最终输出里确实生效。
//
// 这道测试与 cmd/gendata 里的校验互补：生成器只看结构（词组在不在源表里、音节数对不对），
// 这里看端到端结果。校订回单字默认读音的条目（如「古朴 gǔ pǔ」）在 phrase_data.go 里
// 不写读音字段，光看生成数据分辨不出校订是生效了还是被静默丢弃，只有比对 Convert 的
// 输出才能锁住。
func TestPhraseOverridesApplied(t *testing.T) {
	for _, o := range loadPhraseOverrides(t) {
		want := strings.Join(o.readings, "")
		if got := Convert(o.phrase, Tone); got != want {
			t.Errorf("Convert(%q, Tone) = %q, want %q", o.phrase, got, want)
		}
		// 词组前面接上非汉字后，正向最大匹配同样要从词组的起点整词命中。
		// 前缀用 ASCII 而不是汉字：任何汉字都可能与词组前两个字构成更长的词
		// （「我在外差」会被切成「我 + 在外 + 差」，「差」就落到单字读音上去了）。
		sentence := "#" + o.phrase
		if got := Convert(sentence, Tone); got != "#"+want {
			t.Errorf("Convert(%q, Tone) = %q, want %q", sentence, got, "#"+want)
		}
	}
}

// TestPhraseOverridesAreLoadBearing 确认每条校订都确实改变了源表的读数，
// 避免校订表里混进「源表本来就对」的死条目。
func TestPhraseOverridesAreLoadBearing(t *testing.T) {
	overrides := loadPhraseOverrides(t)
	for _, o := range overrides {
		src := sourcePhraseReading(t, o.phrase)
		if src == "" {
			t.Errorf("%q 不在 data/phrase_pinyin.txt 中", o.phrase)
			continue
		}
		if src == strings.Join(o.readings, " ") {
			t.Errorf("%q 的校订读音与源表相同，该条已失效", o.phrase)
		}
	}
}

// sourcePhraseReading 在源表里查词组的读音，查不到返回空串。
func sourcePhraseReading(t *testing.T, phrase string) string {
	t.Helper()
	f, err := os.Open("data/phrase_pinyin.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	prefix := phrase + ":"
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		return strings.Join(strings.Fields(strings.TrimPrefix(line, prefix)), " ")
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return ""
}
