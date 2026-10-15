// Command gendata 由 data/ 下的原始数据生成根目录的 dict_data.go 与 phrase_data.go。
//
// 数据源（均来自 github.com/mozillazg，MIT 协议）：
//
//	data/pinyin.txt          合并表，含多音字的全部读音
//	data/kMandarin_8105.txt  《通用规范汉字表》(2013) 8105 字的最常用读音
//	data/phrase_pinyin.txt   词组读音表，用于词组消歧
//
// 生成规则：
//  1. 收录范围：CJK 扩展 A + 统一表意文字（U+3400–U+9FFF），外加《通用规范汉字表》
//     中位于 BMP 之外的少量汉字，以及 〇（U+3007）。
//  2. 每个字保留全部读音，逗号分隔，首个读音即默认读音。
//  3. 若该字在《通用规范汉字表》中有校准读音，则把它调整到首位。
//  4. 词组表整表收录，运行时对完整词组表做正向最大匹配。只有读音与逐字默认读音
//     不同的词组才写入读音，其余留空，由运行时按单字读音补齐。
//
// 用法：
//
//	go run ./cmd/gendata
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/format"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xywf221/go-pinyin/internal/syllable"
	"github.com/xywf221/go-pinyin/internal/tone"
)

const (
	minRune      = 0x3400 // CJK 扩展 A 起始
	maxRune      = 0x9FFF // 统一表意文字结束
	extraRune    = 0x3007 // 〇
	outFile      = "dict_data.go"
	phraseOut    = "phrase_data.go"
	phraseSource = "data/phrase_pinyin.txt"

	// unknownVersion 是数据源头部缺少 "# version:" 时的占位值。
	unknownVersion = "unknown"

	// maxPhraseRunes 是收录词组允许的最大字数。源表中最长 19 字，这里留出余量，
	// 超过上限的一律跳过，避免异常数据把数据表撑大。
	maxPhraseRunes = 32

	// phraseChunkRecords 是 phraseBlob 字面量每行打包的记录条数。
	// 整表拼成的字符串近 5MB，拆成多行既便于 diff，也避免出现十几万字符的超长单行。
	phraseChunkRecords = 1024
)

func main() {
	all, err := parseReadings("data/pinyin.txt")
	if err != nil {
		fatal(err)
	}
	common, err := parseReadings("data/kMandarin_8105.txt")
	if err != nil {
		fatal(err)
	}

	merged := make(map[rune][]string, len(all))
	for r, pys := range all {
		if inScope(r, common) {
			merged[r] = pys
		}
	}
	// 《通用规范汉字表》中 BMP 之外、或 pinyin.txt 未收录的字，一并补上。
	for r, pys := range common {
		if _, ok := merged[r]; !ok {
			merged[r] = pys
		}
	}

	// 用《通用规范汉字表》校准默认读音。
	calibrated := 0
	for r, pys := range common {
		cur, ok := merged[r]
		if !ok || len(cur) == 0 || len(pys) == 0 {
			continue
		}
		if cur[0] == pys[0] {
			continue
		}
		merged[r] = putFirst(cur, pys[0])
		calibrated++
	}

	overridden, err := applyOverrides(merged, "data/overrides.txt")
	if err != nil {
		fatal(err)
	}
	if err := validate(merged); err != nil {
		fatal(err)
	}
	version := sourceVersion("data/pinyin.txt")
	if err := emit(merged, version, overridden); err != nil {
		fatal(err)
	}

	fmt.Printf("已生成 %s\n", outFile)
	fmt.Printf("  数据源版本    : %s\n", version)
	fmt.Printf("  收录汉字      : %d\n", len(merged))
	fmt.Printf("  多音字        : %d\n", countPolyphonic(merged))
	fmt.Printf("  校准默认读音  : %d\n", calibrated)
	fmt.Printf("  人工校订      : %d\n", overridden)
	fmt.Printf("  规范音节数    : %d\n", syllable.Count())

	phrases, st, err := buildPhrases(merged, phraseSource)
	if err != nil {
		fatal(err)
	}
	if err := emitPhrases(phrases, sourceVersion(phraseSource)); err != nil {
		fatal(err)
	}
	phraseVersion := sourceVersion(phraseSource)
	fmt.Printf("已生成 %s\n", phraseOut)
	fmt.Printf("  数据源版本    : %s\n", phraseVersion)
	fmt.Printf("  源词组        : %d\n", st.total)
	fmt.Printf("  收录词组      : %d（最长 %d 字）\n", st.kept, st.maxRunes)
	fmt.Printf("    其中消歧例外: %d\n", st.excepted)
	fmt.Printf("    读音同单字  : %d\n", st.same)
	fmt.Printf("  跳过          : %d\n", st.skipped)
	st.report()
}

// override 是一条人工校订。
type override struct {
	cp   rune
	py   string
	char rune // 注释中的汉字，用于交叉校验码点
}

// applyOverrides 读取人工校订表并应用到字典，返回实际改动的条数。
//
// 校订表里的每条都会被严格校验：码点必须与注释中的汉字一致，读音必须已经存在于
// 该字的读音列表中。任何一条不满足都会终止生成，避免错误的校订悄悄生效。
func applyOverrides(dict map[rune][]string, path string) (int, error) {
	overrides, err := parseOverrides(path)
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, o := range overrides {
		pys, ok := dict[o.cp]
		if !ok {
			return 0, fmt.Errorf("%s: 校订的 %c (U+%04X) 不在收录范围内", path, o.char, o.cp)
		}
		if !contains(pys, o.py) {
			return 0, fmt.Errorf("%s: 读音 %q 不在 %c (U+%04X) 的读音列表 %v 中",
				path, o.py, o.char, o.cp, pys)
		}
		if pys[0] != o.py {
			dict[o.cp] = putFirst(pys, o.py)
			changed++
		}
	}
	return changed, nil
}

// parseOverrides 解析 "U+XXXX: 读音  # 汉字 —— 说明" 形式的校订行。
func parseOverrides(path string) ([]override, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var out []override
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		body, comment, _ := strings.Cut(line, "#")
		body = strings.TrimSpace(body)
		if body == "" {
			continue
		}
		cpStr, py, ok := strings.Cut(body, ":")
		if !ok {
			return nil, fmt.Errorf("%s:%d 缺少冒号: %q", path, n, line)
		}
		cpStr, py = strings.TrimSpace(cpStr), strings.TrimSpace(py)
		if !strings.HasPrefix(cpStr, "U+") || py == "" {
			return nil, fmt.Errorf("%s:%d 格式应为 \"U+XXXX: 读音  # 汉字\": %q", path, n, line)
		}
		v, err := strconv.ParseUint(cpStr[2:], 16, 32)
		if err != nil {
			return nil, fmt.Errorf("%s:%d 非法码点 %q", path, n, cpStr)
		}
		got := []rune(strings.TrimSpace(comment))
		if len(got) == 0 {
			return nil, fmt.Errorf("%s:%d 缺少汉字注释", path, n)
		}
		if got[0] != rune(v) {
			return nil, fmt.Errorf("%s:%d 码点 U+%04X 与注释中的汉字 %c 不一致",
				path, n, v, got[0])
		}
		out = append(out, override{cp: rune(v), py: py, char: got[0]})
	}
	return out, sc.Err()
}

// contains 报告 slice 中是否存在 s。
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// validate 在写文件前校验数据，任何异常都直接终止生成，避免脏数据进入仓库。
func validate(dict map[rune][]string) error {
	for r, pys := range dict {
		if len(pys) == 0 {
			return fmt.Errorf("U+%04X 没有任何读音", r)
		}
		seen := make(map[string]bool, len(pys))
		for _, p := range pys {
			if p == "" {
				return fmt.Errorf("U+%04X 含空读音", r)
			}
			if seen[p] {
				return fmt.Errorf("U+%04X 的读音 %q 重复", r, p)
			}
			seen[p] = true
			if base := tone.Base(p); !syllable.Valid(base) {
				return fmt.Errorf("U+%04X 的读音 %q 去掉声调后为 %q，不在规范音节清单中", r, p, base)
			}
		}
	}
	return nil
}

// sourceVersion 读取数据源文件头部的 "# version: x.y.z"。
func sourceVersion(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return unknownVersion
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "#") {
			break
		}
		if v, ok := strings.CutPrefix(line, "# version: "); ok {
			return strings.TrimSpace(v)
		}
	}
	return unknownVersion
}

// inScope 判断码点是否在收录范围内。
func inScope(r rune, common map[rune][]string) bool {
	if r == extraRune {
		return true
	}
	if r >= minRune && r <= maxRune {
		return true
	}
	// 《通用规范汉字表》里的字一律收录，即使位于 BMP 之外。
	_, ok := common[r]
	return ok
}

// parseReadings 解析 "U+4E2D: zhōng,zhòng  # 中" 形式的行。
// 行内 # 之后是注释（含 -> / ? 等标记），一律忽略。
func parseReadings(path string) (map[rune][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	out := make(map[rune][]string)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		cp := strings.TrimSpace(line[:colon])
		py := strings.TrimSpace(line[colon+1:])
		if !strings.HasPrefix(cp, "U+") || py == "" {
			continue
		}
		v, err := strconv.ParseUint(cp[2:], 16, 32)
		if err != nil {
			continue
		}
		parts := strings.Split(py, ",")
		pys := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				pys = append(pys, p)
			}
		}
		if len(pys) == 0 {
			continue
		}
		out[rune(v)] = pys
	}
	return out, sc.Err()
}

// putFirst 返回把 want 提到首位后的读音列表。
func putFirst(pys []string, want string) []string {
	out := make([]string, 0, len(pys)+1)
	out = append(out, want)
	for _, p := range pys {
		if p != want {
			out = append(out, p)
		}
	}
	return out
}

// emit 按码点升序写出字典。
func emit(dict map[rune][]string, sourceVersion string, overrideCount int) error {
	cps := make([]rune, 0, len(dict))
	for r := range dict {
		cps = append(cps, r)
	}
	sort.Slice(cps, func(i, j int) bool { return cps[i] < cps[j] })

	var b bytes.Buffer
	b.WriteString("// Code generated by cmd/gendata from data/pinyin.txt and data/kMandarin_8105.txt; DO NOT EDIT.\n")
	b.WriteString("//\n")
	fmt.Fprintf(&b, "// 数据来源：mozillazg/pinyin-data %s（MIT 协议）\n", sourceVersion)
	b.WriteString("// 收录范围：CJK 扩展 A 与统一表意文字 U+3400–U+9FFF，\n")
	b.WriteString("//           外加《通用规范汉字表》中位于 BMP 之外的汉字与 〇。\n")
	fmt.Fprintf(&b, "// 收录汉字：%d\n", len(dict))
	fmt.Fprintf(&b, "// 人工校订：%d 条，见 data/overrides.txt\n", overrideCount)
	b.WriteString("// 首个读音为该字最常用读音。\n\n")
	b.WriteString("package pinyin\n\n")
	b.WriteString("// dictEntry 是一个汉字的拼音条目。\n")
	b.WriteString("type dictEntry struct {\n")
	b.WriteString("\tcp rune   // Unicode 码点\n")
	b.WriteString("\tpy string // 逗号分隔的全部读音，首个为最常用读音\n")
	b.WriteString("}\n\n")
	b.WriteString("// dict 是按码点升序排列的拼音字典，供二分查找。\n")
	b.WriteString("var dict = [...]dictEntry{\n")
	for _, r := range cps {
		fmt.Fprintf(&b, "\t{0x%04X, %q}, // %s\n", r, strings.Join(dict[r], ","), string(r))
	}
	b.WriteString("}\n")

	src, err := format.Source(b.Bytes())
	if err != nil {
		return fmt.Errorf("格式化生成结果: %w", err)
	}
	return os.WriteFile(outFile, src, 0o644)
}

// phraseEntry 是一条词组记录。
type phraseEntry struct {
	phrase   string // 词组原文
	readings string // 空格分隔的带声调音节，与词组的字一一对应
	differs  bool   // 读音是否与逐字默认读音不同
}

// phraseStats 汇总词组表的处理情况，供生成日志与人工审计使用。
type phraseStats struct {
	total    int // 源文件中的数据行数
	kept     int // 写入 phrase_data.go 的词组数
	excepted int // 其中读音与逐字默认读音不同的
	same     int // 其中读音与逐字默认读音相同的
	skipped  int // 因数据异常被跳过的行数
	maxRunes int // 收录词组中的最大字数

	conflicts []string // 源表中重复出现且读音互相矛盾的行

	reasons  map[string]int    // 跳过原因 → 次数
	examples map[string]string // 跳过原因 → 首个例子
}

func newPhraseStats() *phraseStats {
	return &phraseStats{reasons: map[string]int{}, examples: map[string]string{}}
}

// skip 记一次跳过及其原因。
func (st *phraseStats) skip(reason, example string) {
	st.skipped++
	st.reasons[reason]++
	if _, ok := st.examples[reason]; !ok {
		st.examples[reason] = example
	}
}

// report 按次数降序打印跳过原因，并逐条列出源表中读音互相矛盾的重复词组。
func (st *phraseStats) report() {
	reasons := make([]string, 0, len(st.reasons))
	for r := range st.reasons {
		reasons = append(reasons, r)
	}
	sort.Slice(reasons, func(i, j int) bool {
		if st.reasons[reasons[i]] != st.reasons[reasons[j]] {
			return st.reasons[reasons[i]] > st.reasons[reasons[j]]
		}
		return reasons[i] < reasons[j]
	})
	for _, r := range reasons {
		fmt.Printf("    %s：%d，例 %q\n", r, st.reasons[r], st.examples[r])
	}
	for _, c := range st.conflicts {
		fmt.Printf("    ! 源表读音冲突：%s\n", c)
	}
}

// buildPhrases 解析词组表，返回按「前两个字、字数从多到少」排好序的全部有效词组。
//
// 这里收录源表的整张表，而不是「读音与逐字默认读音不同」的例外集。原因是只收录例外集
// 无法与「对完整词组表做最大匹配」等价：「一下子」逐字读 yī xià zi 本身无需收录，
// 但源表里的「下子 xià zǐ」需要，最大匹配走到第二个字就会命中它，把「一下子」切成
// 「一 + 下子」；把这类「会被切开」的词组补进来之后，「隆重庆祝」里的「重庆」又会被
// 同样的问题带走——例外集必须一路补到几乎整张表才闭合。整表收录虽然让生成数据从
// 1.4MB 涨到约 5MB，但结果与完整词表的最大匹配逐字一致，不必再维护这套补集规则。
//
// 读音只对例外词组写入，其余留空，运行时按单字读音补齐，因此多出来的主要是词组原文。
//
// 任何一行不满足下列条件都会被跳过并计入统计，但不会中断生成：
//   - 词组至少 2 个字，不超过 maxPhraseRunes 个字，且音节数与字数一致
//   - 词组中的每个字都在字典收录范围内
//   - 每个音节去掉声调后都是规范音节
func buildPhrases(dict map[rune][]string, path string) ([]phraseEntry, *phraseStats, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()

	st := newPhraseStats()
	var all []phraseEntry
	seen := make(map[string]string)

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		st.total++

		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			st.skip("缺少冒号", line)
			continue
		}
		phrase := strings.TrimSpace(line[:colon])
		fields := strings.Fields(line[colon+1:])
		runes := []rune(phrase)
		if len(runes) < 2 {
			st.skip("不足两字", phrase)
			continue
		}
		if len(fields) != len(runes) {
			st.skip("音节数与字数不符", line)
			continue
		}
		if len(runes) > maxPhraseRunes {
			st.skip("词组过长", phrase)
			continue
		}
		if prev, dup := seen[phrase]; dup {
			// 读音相同时留一条即可；读音互相矛盾时无从自动判定哪个对，
			// 保留先出现的一条，并逐条列出来供人工核对。
			readings := strings.Join(fields, " ")
			if prev == readings {
				st.skip("重复词组（读音相同）", phrase)
			} else {
				st.skip("重复词组（读音冲突）", phrase)
				st.conflicts = append(st.conflicts, fmt.Sprintf("%s: %q 与 %q", phrase, prev, readings))
			}
			continue
		}
		seen[phrase] = strings.Join(fields, " ")

		bad := ""
		for _, r := range runes {
			if _, ok := dict[r]; !ok {
				bad = "含未收录字"
				break
			}
		}
		if bad == "" {
			for _, p := range fields {
				if !syllable.Valid(tone.Base(p)) {
					bad = "非规范音节"
					break
				}
			}
		}
		if bad != "" {
			st.skip(bad, line)
			continue
		}
		all = append(all, phraseEntry{
			phrase:   phrase,
			readings: strings.Join(fields, " "),
			differs:  phraseDiffers(runes, fields, dict),
		})
	}
	if err := sc.Err(); err != nil {
		return nil, nil, err
	}

	st.kept = len(all)
	if st.kept == 0 {
		return nil, nil, fmt.Errorf("%s: 没有解析出任何词组，数据文件格式可能已变化", path)
	}
	for _, e := range all {
		if e.differs {
			st.excepted++
		} else {
			st.same++
		}
		if n := utf8.RuneCountInString(e.phrase); n > st.maxRunes {
			st.maxRunes = n
		}
	}

	// 运行时的索引按「前两个字」分桶，要求同一个前两字的词组在字符串中连续，
	// 且桶内按字数降序排列。这里排好序，phraseIndex 才能只扫一遍就建出索引。
	sort.Slice(all, func(i, j int) bool {
		hi, hj := headKey(all[i].phrase), headKey(all[j].phrase)
		if hi != hj {
			return hi < hj
		}
		li, lj := utf8.RuneCountInString(all[i].phrase), utf8.RuneCountInString(all[j].phrase)
		if li != lj {
			return li > lj
		}
		return all[i].phrase < all[j].phrase
	})
	return all, st, nil
}

// headKey 返回词组的前两个字，与运行时 phrase.go 中的 headKey 排序等价。
// 词组至少两个字，调用方无需判长。
func headKey(phrase string) string {
	_, sz := utf8.DecodeRuneInString(phrase)
	_, sz2 := utf8.DecodeRuneInString(phrase[sz:])
	return phrase[:sz+sz2]
}

// phraseDiffers 报告词组的读音是否与逐字默认读音（每个字的首个读音）不同。
func phraseDiffers(runes []rune, fields []string, dict map[rune][]string) bool {
	for i, r := range runes {
		if tone.Normalize(fields[i]) != tone.Normalize(dict[r][0]) {
			return true
		}
	}
	return false
}

// emitPhrases 写出词组表。
//
// 全部词组拼成一个字符串，记录之间用 "|" 分隔，形如「词组|读音|」，读音为空表示
// 逐字默认读音就是对的。用字符串常量而不是结构体数组，是为了省掉每条记录两个
// 字符串头（32 字节）——四十万条累计下来要多出 13MB，比数据本身还大。
//
// 记录按「前两个字、字数从多到少」排列，运行时 phraseIndex 依赖这个顺序把同一个
// 前两字的词组收进同一个桶。
func emitPhrases(entries []phraseEntry, sourceVersion string) error {
	var b bytes.Buffer
	b.WriteString("// Code generated by cmd/gendata from data/phrase_pinyin.txt; DO NOT EDIT.\n")
	b.WriteString("//\n")
	fmt.Fprintf(&b, "// 数据来源：mozillazg/phrase-pinyin-data %s（MIT 协议）\n", sourceVersion)
	b.WriteString("// 收录源表整表，因此与「对完整词组表做正向最大匹配」逐字一致。\n")
	b.WriteString("// 每条记录是「词组|读音|」，读音为空表示逐字默认读音就是对的。\n")
	fmt.Fprintf(&b, "// 词组数：%d，最长 %d 字。\n", len(entries), maxRunesOf(entries))
	b.WriteString("// 按「前两个字、字数从多到少」排列，顺序与 phraseIndex 的分桶方式对应。\n\n")
	b.WriteString("package pinyin\n\n")

	b.WriteString("// phraseBlob 是全部词组的拼接，供 phraseIndex 建表、matchPhrase 做最大匹配。\n")
	b.WriteString("var phraseBlob = \"\"")
	for i := 0; i < len(entries); i += phraseChunkRecords {
		j := min(i+phraseChunkRecords, len(entries))
		var chunk bytes.Buffer
		for _, e := range entries[i:j] {
			chunk.WriteString(e.phrase)
			chunk.WriteByte('|')
			// 读音与逐字默认读音相同的词组不写读音，省下近 1MB。
			if e.differs {
				chunk.WriteString(e.readings)
			}
			chunk.WriteByte('|')
		}
		fmt.Fprintf(&b, " +\n\t%q", chunk.String())
	}
	b.WriteString("\n")

	src, err := format.Source(b.Bytes())
	if err != nil {
		return fmt.Errorf("格式化生成结果: %w", err)
	}
	return os.WriteFile(phraseOut, src, 0o644)
}

// maxRunesOf 返回条目中最长的词组字数。
func maxRunesOf(entries []phraseEntry) int {
	n := 0
	for _, e := range entries {
		if c := utf8.RuneCountInString(e.phrase); c > n {
			n = c
		}
	}
	return n
}

// countPolyphonic 统计多音字数量。
func countPolyphonic(dict map[rune][]string) int {
	n := 0
	for _, pys := range dict {
		if len(pys) > 1 {
			n++
		}
	}
	return n
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gendata:", err)
	os.Exit(1)
}
