// Command audit 把本仓库发布的词组读音与若干外部来源交叉比对，输出差异清单。
//
// 上游 mozillazg/phrase-pinyin-data 的词组读音来自汉典与 CC-CEDICT 的合并，而合并时
// 让项目自己维护的 pinyin.txt / overwrite.txt 覆盖这两者（见上游 Makefile 的 merge）。
// 因此对本仓库真正独立的证人是汉典与 CC-CEDICT 的原始表，但它们推翻不了已被覆盖的
// 读数，只能作为「值得复核」的信号。这个工具就是把信号挖出来。
//
// 用法：
//
//	go run ./cmd/audit \
//	    -ref 汉典=_research/phrase/zdic_cibs.txt \
//	    -ref CC-CEDICT=_research/phrase/cc_cedict.txt
//
// 参考数据不随仓库分发（CC-CEDICT 是 CC BY-SA 4.0，汉典数据授权不明），所以这个工具
// 不在 CI 里跑，需要人工准备参考文件。
//
// 输出按四类归并，与 data/phrase_overrides.txt 头部的取舍标准一一对应：
//
//	A 声母或韵母不同 —— 真正的错误候选
//	B 仅声调不同     —— 错误候选，但异读与方言里常见
//	C 轻声标注差异   —— 标注惯例，通常不算错
//	D 一/不 的变调   —— 标注惯例；本库按变调输出，不是错
//
// 只有 A、B 两类值得动手改数据，C、D 只作参考。
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/xywf221/go-pinyin/internal/tone"
)

// source 是一份「词组: 读音」形式的读音表。
type source struct {
	name  string
	table map[string][]string
}

// refFlag 收集可重复的 -ref 名称=路径 参数。
type refFlag []string

func (r *refFlag) String() string { return strings.Join(*r, ",") }

func (r *refFlag) Set(v string) error {
	*r = append(*r, v)
	return nil
}

// row 是一条待复核的差异：我们给出的读音，与各参考来源给出的读音。
type row struct {
	phrase string
	ours   []string
	// votes 把参考读音（空格连接）映射到给出该读音的来源名。
	votes map[string][]string
}

// category 返回这条差异的类别；0 表示我们与所有参考来源都不冲突。
func (r row) category() byte {
	rival, ok := r.rival()
	if !ok {
		return 0
	}
	return diffCategory(r.phrase, r.ours, strings.Fields(rival))
}

// rival 返回得票最多的那个「非我们」读音。票数相同时取字典序小的，保证输出稳定。
func (r row) rival() (string, bool) {
	best, bestN := "", 0
	for v, by := range r.votes {
		if v == strings.Join(r.ours, " ") {
			continue
		}
		if len(by) > bestN || (len(by) == bestN && v < best) {
			best, bestN = v, len(by)
		}
	}
	return best, bestN > 0
}

// peak 返回投给同一个「非我们」读音的来源数的最大值。
func (r row) peak() int {
	_, ok := r.rival()
	if !ok {
		return 0
	}
	best := 0
	for v, by := range r.votes {
		if v != strings.Join(r.ours, " ") && len(by) > best {
			best = len(by)
		}
	}
	return best
}

// vote 是一条参考读音及其支持者。
type vote struct {
	value string
	by    []string
}

// sortedVotes 按支持者数量降序返回参考读音，票数相同时按读音排序。
func (r row) sortedVotes() []vote {
	out := make([]vote, 0, len(r.votes))
	for v, by := range r.votes {
		sort.Strings(by)
		out = append(out, vote{value: v, by: by})
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].by) != len(out[j].by) {
			return len(out[i].by) > len(out[j].by)
		}
		return out[i].value < out[j].value
	})
	return out
}

func main() {
	oursPath := flag.String("ours", "data/phrase_pinyin.txt", "本仓库的词组源表")
	overridePath := flag.String("override", "data/phrase_overrides.txt", "人工校订表，留空表示不套用")
	minRunes := flag.Int("min-runes", 2, "比对的最小词长")
	maxRunes := flag.Int("max-runes", 6, "比对的最大词长")
	minAgree := flag.Int("min-agree", 2, "至少多少个来源一致才列出来")
	category := flag.String("category", "", "只列该类别（A/B/C/D），留空表示全部")
	var refs refFlag
	flag.Var(&refs, "ref", "参考来源，形如 名称=路径，可重复")
	flag.Parse()

	if len(refs) == 0 {
		fmt.Fprintln(os.Stderr, "audit: 至少要有一个 -ref 参考来源")
		flag.Usage()
		os.Exit(2)
	}

	ours, err := loadSource("本仓库", *oursPath)
	if err != nil {
		fatal(err)
	}
	if *overridePath != "" {
		n, err := applyOverrides(ours, *overridePath)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("已套用 %d 条人工校订（%s）\n", n, *overridePath)
	}

	others := make([]*source, 0, len(refs))
	for _, spec := range refs {
		name, path, ok := strings.Cut(spec, "=")
		if !ok {
			fatal(fmt.Errorf("-ref 参数应为 名称=路径，得到 %q", spec))
		}
		s, err := loadSource(name, path)
		if err != nil {
			fatal(err)
		}
		others = append(others, s)
	}

	report(os.Stdout, ours, others, *minRunes, *maxRunes, *minAgree, *category)
}

func report(w io.Writer, ours *source, others []*source, minRunes, maxRunes, minAgree int, only string) {
	var rows []row
	counts := map[byte]int{}
	shared := 0
	for phrase, ov := range ours.table {
		if n := utf8.RuneCountInString(phrase); n < minRunes || n > maxRunes {
			continue
		}
		votes := map[string][]string{}
		for _, s := range others {
			rv, ok := s.table[phrase]
			if !ok {
				continue
			}
			key := strings.Join(rv, " ")
			votes[key] = append(votes[key], s.name)
		}
		if len(votes) == 0 {
			continue
		}
		shared++
		r := row{phrase: phrase, ours: ov, votes: votes}
		counts[r.category()]++
		if r.category() != 0 {
			rows = append(rows, r)
		}
	}

	writef(w, "\n=== 与参考来源共有 %d 个词组（词长 %d–%d）===\n", shared, minRunes, maxRunes)
	for _, c := range []byte{'A', 'B', 'C', 'D'} {
		writef(w, "  %c %s：%d\n", c, categoryName(c), counts[c])
	}
	writef(w, "  读音一致：%d\n", shared-counts['A']-counts['B']-counts['C']-counts['D'])

	var hi []row
	for _, r := range rows {
		if r.peak() < minAgree {
			continue
		}
		if only != "" && string(r.category()) != only {
			continue
		}
		hi = append(hi, r)
	}
	sort.Slice(hi, func(i, j int) bool {
		if c := severity(hi[i].category()) - severity(hi[j].category()); c != 0 {
			return c > 0
		}
		if hi[i].peak() != hi[j].peak() {
			return hi[i].peak() > hi[j].peak()
		}
		return hi[i].phrase < hi[j].phrase
	})

	writef(w, "\n=== 至少 %d 个来源一致、且与我们不同：%d 条 ===\n", minAgree, len(hi))
	for _, r := range hi {
		writef(w, "%c %-12s 我们=%s\n", r.category(), r.phrase, strings.Join(r.ours, " "))
		for _, v := range r.sortedVotes() {
			writef(w, "    %-26s [%s]\n", v.value, strings.Join(v.by, ","))
		}
	}
}

// categoryName 返回类别的中文名。
func categoryName(c byte) string {
	switch c {
	case 'A':
		return "声母韵母不同"
	case 'B':
		return "仅声调不同"
	case 'C':
		return "轻声标注差异"
	case 'D':
		return "一/不 变调"
	}
	return "一致"
}

// severity 给出类别的严重程度，数字越大越值得动手。
func severity(c byte) int {
	switch c {
	case 'A':
		return 4
	case 'B':
		return 3
	case 'C':
		return 2
	case 'D':
		return 1
	}
	return 0
}

// diffCategory 返回两种读音之间的差异类别，取所有字位里最严重的一档。
func diffCategory(phrase string, ours, rival []string) byte {
	runes := []rune(phrase)
	if len(ours) != len(runes) || len(rival) != len(runes) {
		// 音节数与字数不符的参考数据不可信，不参与判定。
		return 0
	}
	worst := byte(0)
	for i, r := range runes {
		if c := diffPosition(r, ours[i], rival[i]); severity(c) > severity(worst) {
			worst = c
		}
	}
	return worst
}

// diffPosition 判定单个字位上两种读音的差异类别。
func diffPosition(ch rune, ours, rival string) byte {
	ob, ot := splitTone(tone.Normalize(ours))
	rb, rt := splitTone(tone.Normalize(rival))
	if ob == rb && ot == rt {
		return 0
	}
	if ob != rb {
		return 'A'
	}
	// 声母韵母相同，只有声调不同。「一」「不」的变调是标注惯例，单列一类。
	if ch == '一' || ch == '不' {
		return 'D'
	}
	if ot == 0 || rt == 0 {
		return 'C'
	}
	return 'B'
}

// splitTone 把 tone.Normalize 的输出拆成基本形式与声调，声调 0 表示轻声。
func splitTone(norm string) (base string, t int) {
	if norm == "" {
		return "", 0
	}
	if last := norm[len(norm)-1]; last >= '1' && last <= '4' {
		return norm[:len(norm)-1], int(last - '0')
	}
	return norm, 0
}

// loadSource 读取「词组: 读音 读音」形式的读音表。
//
// 与上游 parse 一致：'#' 之后是注释，同一个词组重复出现时保留先出现的一条，
// 词组名里的 "_2" 后缀（上游用来标记多音词）会被去掉。
func loadSource(name, path string) (*source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	s := &source{name: name, table: make(map[string][]string)}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimPrefix(sc.Text(), "\ufeff")
		line, _, _ = strings.Cut(line, "#")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		phrase, rest, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("%s:%d 缺少冒号: %q", path, n, line)
		}
		phrase = strings.TrimSpace(phrase)
		if i := strings.Index(phrase, "_"); i >= 0 {
			phrase = phrase[:i]
		}
		readings := strings.Fields(rest)
		if phrase == "" || len(readings) == 0 {
			continue
		}
		if _, dup := s.table[phrase]; dup {
			continue
		}
		s.table[phrase] = readings
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(s.table) == 0 {
		return nil, fmt.Errorf("%s: 没有解析出任何读音", path)
	}
	return s, nil
}

// applyOverrides 把人工校订表套到读音表上，返回实际改动的条数。
func applyOverrides(s *source, path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	changed := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line, _, _ := strings.Cut(sc.Text(), "#")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		phrase, rest, ok := strings.Cut(line, ":")
		phrase = strings.TrimSpace(phrase)
		readings := strings.Fields(rest)
		if !ok || phrase == "" || len(readings) == 0 {
			return 0, fmt.Errorf("%s:%d 格式应为 \"词组: 读音\": %q", path, n, line)
		}
		if _, ok := s.table[phrase]; !ok {
			return 0, fmt.Errorf("%s:%d 词组 %q 不在读音表中", path, n, phrase)
		}
		s.table[phrase] = readings
		changed++
	}
	return changed, sc.Err()
}

// writef 是 fmt.Fprintf 的包装，忽略错误：报告只写标准输出，写不进去也无从补救。
func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "audit:", err)
	os.Exit(1)
}
