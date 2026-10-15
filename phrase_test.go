package pinyin

import (
	"bufio"
	"math/rand"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	syll "github.com/xywf221/go-pinyin/internal/syllable"
	"github.com/xywf221/go-pinyin/internal/tone"
)

// TestPhraseDisambiguation 校验词组消歧：多音字在词组中取的是词组的读音。
func TestPhraseDisambiguation(t *testing.T) {
	tests := []struct {
		in       string
		tone     string // 词组消歧后的读音
		byRune   string // 关闭消歧、逐字取默认读音的结果
		initials string
	}{
		{"重庆", "chóngqìng", "zhòngqìng", "chq"},
		{"长沙", "chángshā", "zhǎngshā", "chsh"},
		{"厦门", "xiàmén", "shàmén", "xm"},
		{"银行", "yínháng", "yínxíng", "yh"},
		{"头发", "tóufà", "tóufā", "tf"},
		{"西藏", "xīzàng", "xīcáng", "xz"},
		{"重复", "chóngfù", "zhòngfù", "chf"},
		{"爱好", "àihào", "àihǎo", "ah"},
		{"主角", "zhǔjué", "zhǔjiǎo", "zhj"},
		{"藏族", "zàngzú", "cángzú", "zz"},
		{"还钱", "huánqián", "háiqián", "hq"},
		{"便宜", "piányi", "biànyí", "py"},
		{"音乐", "yīnyuè", "yīnlè", "yy"},
		{"长跑", "chángpǎo", "zhǎngpǎo", "chp"},
		{"长安", "chángān", "zhǎngān", "cha"},
		{"为什么", "wèishénme", "wèishénme", "wshm"},
		// 词组内的一字变调按词组的读音走。
		{"一模一样", "yīmúyīyàng", "yīmóyīyàng", "ymyy"},
		// 逐字读音本来就对的词组不该被改动。
		{"重要", "zhòngyào", "zhòngyào", "zhy"},
		{"中国人", "zhōngguórén", "zhōngguórén", "zhgr"},
		{"中华人民共和国", "zhōnghuárénmíngònghéguó", "zhōnghuárénmíngònghéguó", "zhhrmghg"},
	}
	for _, tt := range tests {
		if got := Convert(tt.in, Tone); got != tt.tone {
			t.Errorf("Convert(%q, Tone) = %q, want %q", tt.in, got, tt.tone)
		}
		got := ConvertWith(tt.in, Options{Style: Tone, CharByChar: true})
		if got != tt.byRune {
			t.Errorf("ConvertWith(%q, CharByChar) = %q, want %q", tt.in, got, tt.byRune)
		}
		if got := Convert(tt.in, Initials); got != tt.initials {
			t.Errorf("Convert(%q, Initials) = %q, want %q", tt.in, got, tt.initials)
		}
	}
}

// TestPhraseNoPartialMatch 覆盖「词组不会被更短的词组从中间切开」。
//
// 最大匹配是逐字前进的：在某个位置命中词组后，就跳到词组末尾，绝不会从词组的第二个字
// 重新起匹配。因此「一下子」不会被更短的「下子 xià zǐ」带走，「隆重庆祝」里的「重庆」
// 也不会被当成 chóng qìng——它前面的「隆重」先匹配上，跳过了「重庆」的起点。
func TestPhraseNoPartialMatch(t *testing.T) {
	tests := []struct{ in, want string }{
		{"一下子", "yīxiàzi"},           // 会被「下子 xià zǐ」切开
		{"一举两得", "yījǔliǎngdé"},      // 会被「两得 liǎng de」切开
		{"一切事物", "yīqièshìwù"},       // 会被「切事 qiē shì」切开
		{"隆重庆祝", "lóngzhòngqìngzhù"}, // 会被「重庆 chóng qìng」切开
		{"隆重庆典", "lóngzhòngqìngdiǎn"},
		{"隆重庆贺", "lóngzhòngqìnghè"},
	}
	for _, tt := range tests {
		if got := Convert(tt.in, Tone); got != tt.want {
			t.Errorf("Convert(%q, Tone) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestPhraseStyles 校验词组在各风格下的输出与逐字转换保持一致的形式。
func TestPhraseStyles(t *testing.T) {
	if got := Convert("重庆", Normal); got != "chongqing" {
		t.Errorf("Normal = %q", got)
	}
	if got := Convert("重庆", FirstLetter); got != "cq" {
		t.Errorf("FirstLetter = %q", got)
	}
	if got := Convert("重庆", Tone2); got != "cho2ngqi4ng" {
		t.Errorf("Tone2 = %q", got)
	}
	if got := Convert("重庆", Tone3); got != "chong2qing4" {
		t.Errorf("Tone3 = %q", got)
	}
	if got := Convert("为什么", Tone3); got != "wei4shen2me" {
		t.Errorf("Tone3 = %q", got)
	}
}

// TestPhraseSeparator 校验词组内部的音节之间也插入分隔符，且只插一次。
func TestPhraseSeparator(t *testing.T) {
	o := Options{Style: Tone, Separator: "-"}
	if got := ConvertWith("重庆", o); got != "chóng-qìng" {
		t.Errorf("got %q, want %q", got, "chóng-qìng")
	}
	// 词组与后续单字之间、以及词组内部，分隔符的表现一致。
	if got := ConvertWith("去重庆", o); got != "qù-chóng-qìng" {
		t.Errorf("got %q, want %q", got, "qù-chóng-qìng")
	}
	// 非汉字不参与分隔。
	if got := ConvertWith("重庆a", Options{Style: Normal, Separator: "-"}); got != "chong-qing-a" {
		t.Errorf("got %q, want %q", got, "chong-qing-a")
	}
	// 读音与逐字默认读音一致的词组走的是另一条分支（按单字写出），分隔行为必须一样。
	if got := ConvertWith("一下子", o); got != "yī-xià-zi" {
		t.Errorf("got %q, want %q", got, "yī-xià-zi")
	}
	if got := ConvertWith("一下子a重庆", Options{Style: Normal, Separator: "-"}); got != "yi-xia-zi-a-chong-qing" {
		t.Errorf("got %q, want %q", got, "yi-xia-zi-a-chong-qing")
	}
}

// TestHeteronymSkipsPhrases 固化「多音字模式与词组消歧互斥」这一约定。
func TestHeteronymSkipsPhrases(t *testing.T) {
	o := Options{Style: Tone, Heteronym: true}
	want := "zhòng/chóng/tóngqìng"
	if got := ConvertWith("重庆", o); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// CharByChar 不影响多音字模式：两者都按单字输出。
	o.CharByChar = true
	if got := ConvertWith("重庆", o); got != want {
		t.Errorf("CharByChar 下 got %q, want %q", got, want)
	}
}

// phraseIndexForTest 建好索引并返回，供完整性测试直接检查内部结构。
func phraseIndexForTest(t *testing.T) *phraseIndex {
	t.Helper()
	phraseOnce.Do(func() { phraseIdx = buildPhraseIndex() })
	if phraseIdx == nil {
		t.Fatal("索引没有建起来")
	}
	return phraseIdx
}

// TestPhraseIndexIntegrity 校验惰性建出的索引与 phraseBlob 严格对应。
//
// 索引依赖 phraseBlob 已按「前两个字、字数降序」排好序：同一个前两字的词组必须连续，
// 同一个首字的桶必须相邻且按第二个字升序（二分查找要用），桶内按字数降序（最长匹配
// 要用）。生成器的排序一变，这些前提就会破，索引会静默给出错误结果。
func TestPhraseIndexIntegrity(t *testing.T) {
	ix := phraseIndexForTest(t)

	if ix.starts[0] != 0 {
		t.Fatalf("第一个桶的起点是 %d，应为 0", ix.starts[0])
	}
	if last := ix.starts[len(ix.starts)-1]; int(last) != len(phraseBlob) {
		t.Fatalf("哨兵是 %d，应为 phraseBlob 的长度 %d", last, len(phraseBlob))
	}

	// 每个首字的区间必须只含该首字的桶，且桶按第二个字严格升序。
	grouped := 0
	for r1, g := range ix.groups {
		if g.n == 0 {
			t.Fatalf("首字 %c 的区间是空的", r1)
		}
		if end := g.start + g.n; end >= uint32(len(ix.starts)) {
			t.Fatalf("首字 %c 的区间 [%d, %d) 越界", r1, g.start, end)
		}
		grouped += int(g.n)
		prev := rune(-1)
		for k := g.start; k < g.start+g.n; k++ {
			gotR1, gotR2 := headRunes(phraseBlob[ix.starts[k]:])
			if gotR1 != r1 {
				t.Fatalf("首字 %c 的区间里混进了首字 %c 的桶", r1, gotR1)
			}
			if gotR2 <= prev {
				t.Fatalf("首字 %c 的桶未按第二个字升序：%c 排在 %c 之后", r1, gotR2, prev)
			}
			prev = gotR2
		}
	}
	if n := len(ix.starts) - 1; grouped != n {
		t.Errorf("各首字区间共 %d 个桶，buckets 里有 %d 个", grouped, n)
	}

	// 每个桶内部必须同属一个前两字，且按字数降序。
	for k := 0; k+1 < len(ix.starts); k++ {
		i, end := int(ix.starts[k]), int(ix.starts[k+1])
		if i >= end {
			t.Fatalf("第 %d 个桶的区间 [%d, %d) 为空或倒置", k, i, end)
		}
		r1, r2 := headRunes(phraseBlob[i:])
		prev := 0
		for j := i; j < end; j = nextRecord(j) {
			p, _ := record(j)
			if a, b := headRunes(p); a != r1 || b != r2 {
				t.Fatalf("前两字 %c%c 的桶混入了 %q", r1, r2, p)
			}
			c := utf8.RuneCountInString(p)
			if prev != 0 && c > prev {
				t.Fatalf("前两字 %c%c 的桶内未按字数降序：%q 排在 %d 字之后", r1, r2, p, prev)
			}
			prev = c
		}
	}
}

// TestPhraseDictIntegrity 校验生成的数据表结构正确。
func TestPhraseDictIntegrity(t *testing.T) {
	n := 0
	for i := 0; i < len(phraseBlob); i = nextRecord(i) {
		n++
		phrase, readings := record(i)
		runes := []rune(phrase)
		if len(runes) < 2 {
			t.Fatalf("第 %d 条不足两字: %q", n, phrase)
		}
		if strings.ContainsAny(phrase, "|") || strings.ContainsAny(readings, "|") {
			t.Fatalf("第 %d 条 %q 含记录分隔符", n, phrase)
		}
		for _, r := range runes {
			if _, ok := lookup(r); !ok {
				t.Fatalf("第 %d 条 %q 含未收录字 %c", n, phrase, r)
			}
		}
		if readings == "" {
			// 读音留空表示逐字默认读音就是对的，运行时按单字补齐。
			continue
		}
		fields := strings.Fields(readings)
		if len(fields) != len(runes) {
			t.Fatalf("第 %d 条 %q 有 %d 个读音 %q，字数却是 %d",
				n, phrase, len(fields), readings, len(runes))
		}
		for _, p := range fields {
			if !syll.Valid(tone.Base(p)) {
				t.Fatalf("第 %d 条 %q 的读音 %q 不是规范音节", n, phrase, p)
			}
		}
	}
	if n < 400000 {
		t.Fatalf("词组条目过少: %d", n)
	}
}

// TestPhraseDictionaryAccuracy 把完整词组表逐条过一遍：
// 对每个词组做一次整词转换，结果必须与源表给出的读音一致。
//
// 源表里有极少数词组被重复收录且两条读音互相矛盾（例如「朝阳」既是 zhāo yáng 也是
// cháo yáng），无从判定哪个对，只能保留先出现的一条，另一个必然对不上。除这些之外
// 任何一条对不上，都说明匹配逻辑出了问题。
func TestPhraseDictionaryAccuracy(t *testing.T) {
	entries := readPhraseSource(t)

	conflicting := make(map[string]bool)
	seen := make(map[string]string, len(entries))
	for _, e := range entries {
		r := strings.Join(e.readings, " ")
		if prev, dup := seen[e.phrase]; dup && prev != r {
			conflicting[e.phrase] = true
		}
		seen[e.phrase] = r
	}
	if len(conflicting) > 5 {
		t.Errorf("源表中读音冲突的重复词组多达 %d 个，需要人工复核", len(conflicting))
	}

	checked, bad := 0, 0
	for _, e := range entries {
		if conflicting[e.phrase] {
			continue
		}
		checked++
		want := strings.Join(e.readings, "")
		if got := Convert(e.phrase, Tone); got != want {
			bad++
			if bad <= 10 {
				t.Errorf("Convert(%q, Tone) = %q, want %q", e.phrase, got, want)
			}
		}
	}
	if bad > 10 {
		t.Errorf("共 %d 条对不上（已列出前 10 条）", bad)
	}
	t.Logf("逐条核对 %d 个词组，%d 条与源表不一致", checked, bad)
}

type phraseSourceEntry struct {
	phrase   string
	readings []string
}

// readPhraseSource 读入 data/phrase_pinyin.txt，只保留生成器会收录的那些行，
// 过滤规则与 cmd/gendata 保持一致。
func readPhraseSource(t *testing.T) []phraseSourceEntry {
	t.Helper()
	f, err := os.Open("data/phrase_pinyin.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	var out []phraseSourceEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(line)
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		phrase := strings.TrimSpace(line[:colon])
		readings := strings.Fields(line[colon+1:])
		runes := []rune(phrase)
		if len(runes) < 2 || len(readings) != len(runes) {
			continue
		}
		ok := true
		for _, r := range runes {
			if _, hit := lookup(r); !hit {
				ok = false
				break
			}
		}
		if ok {
			for _, p := range readings {
				if !syll.Valid(tone.Base(p)) {
					ok = false
					break
				}
			}
		}
		if ok {
			out = append(out, phraseSourceEntry{phrase: phrase, readings: readings})
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(out) < 400000 {
		t.Fatalf("词组源数据只解析出 %d 条，文件可能不完整", len(out))
	}
	return out
}

// TestToneNormalizeMatchesParseSyllable 保证 internal/tone.Normalize 与核心库的
// parseSyllable 对「基本形式 + 声调」的判断完全一致。
//
// 生成器用 Normalize 判断词组读音是否等于逐字默认读音，核心库用 parseSyllable
// 渲染音节。两者一旦漂移，被判定为「与单字一致」而丢弃的词组就会悄悄漏掉。
func TestToneNormalizeMatchesParseSyllable(t *testing.T) {
	check := func(s string) {
		t.Helper()
		sy := parseSyllable(s)
		want := string(sy.base[:sy.n])
		if sy.tone != 0 {
			want += string(rune('0' + sy.tone))
		}
		if got := tone.Normalize(s); got != want {
			t.Errorf("tone.Normalize(%q) = %q, parseSyllable 得到 %q", s, got, want)
		}
	}
	for _, e := range dict {
		for _, p := range strings.Split(e.py, ",") {
			check(p)
		}
	}
	for i := 0; i < len(phraseBlob); i = nextRecord(i) {
		_, readings := record(i)
		for _, p := range strings.Fields(readings) {
			check(p)
		}
	}
	// 组合声调符号与 ü 的两种写法。
	for _, s := range []string{"ê̄", "ê̌", "m̀", "ḿ", "nǚ", "lǜ", "ü", "de", ""} {
		check(s)
	}
}

// TestPhraseMatchesFullTableFMM 把随机拼接的词组整句交给 Convert，与「对完整词组表做
// 正向最大匹配」的参照实现逐条对比。
//
// TestPhraseDictionaryAccuracy 只验证单个词组，这一条验证词组连排时的跳转：命中一个
// 词组后必须整词跳过，不能让更短的词组从中间插进来——「隆重庆祝」里的「重庆」就是
// 这类情况，只按单条核对是发现不了的。
func TestPhraseMatchesFullTableFMM(t *testing.T) {
	entries := readPhraseSource(t)

	table := make(map[string]string, len(entries))
	maxRunes := 0
	for _, e := range entries {
		if _, dup := table[e.phrase]; dup {
			continue // 重复词组保留先出现的一条，与生成器一致
		}
		table[e.phrase] = strings.Join(e.readings, " ")
		if n := utf8.RuneCountInString(e.phrase); n > maxRunes {
			maxRunes = n
		}
	}

	rng := rand.New(rand.NewSource(20261015))
	for i := 0; i < 20000; i++ {
		var sb strings.Builder
		for k := 0; k < 1+rng.Intn(6); k++ {
			sb.WriteString(entries[rng.Intn(len(entries))].phrase)
		}
		s := sb.String()
		if got, want := Convert(s, Tone), refConvert(s, table, maxRunes); got != want {
			t.Fatalf("Convert(%q) = %q, want %q", s, got, want)
		}
	}
}

// refConvert 是对完整词组表做正向最大匹配的参照实现，只求直观，不管效率。
// 调用方只喂源表里的词组，因此每个字符都是字典收录的汉字。
func refConvert(s string, table map[string]string, maxRunes int) string {
	offs := make([]int, 0, len(s)+1)
	for i := range s {
		offs = append(offs, i)
	}
	offs = append(offs, len(s))

	var b strings.Builder
	for i := 0; i < len(offs)-1; {
		matched := 0
		for n := maxRunes; n >= 2; n-- {
			if i+n >= len(offs) {
				continue
			}
			py, ok := table[s[offs[i]:offs[i+n]]]
			if !ok {
				continue
			}
			for _, p := range strings.Fields(py) {
				sy := parseSyllable(p)
				b.Write(sy.appendTo(nil, Tone))
			}
			matched = n
			break
		}
		if matched == 0 {
			r, _ := utf8.DecodeRuneInString(s[offs[i]:])
			py, _ := lookup(r)
			sy := parseSyllable(strings.Split(py, ",")[0])
			b.Write(sy.appendTo(nil, Tone))
			matched = 1
		}
		i += matched
	}
	return b.String()
}
