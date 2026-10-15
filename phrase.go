package pinyin

import (
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

// 词组消歧采用正向最大匹配：在当前位置找出以它开头的最长词组，整词取其读音。
//
// phraseBlob 收录了词组源表的整张表，结果因此与「对完整词组表做正向最大匹配」逐字
// 一致。词组有四十万条，直接拿整个词组做哈希键太费内存，索引改按前两个字分桶：
// 同一个前两字的词组在 phraseBlob 里连续存放，桶内按字数从多到少排列，于是第一个
// 前缀匹配上的就是最长匹配。前两字相同的词组平均只有 1.3 条，绝大多数位置一次比较
// 就能定下来。
//
// 索引只存「首字 → 桶区间」、每个桶的第二个字、以及每个桶在 phraseBlob 中的起点。
// 桶内条数不存：一个桶的末尾就是下一个桶的起点，末尾再补一个哨兵即可，
// 省下来的那张表比存下来的还大。

// phraseGroup 是同一个首字下全部桶在 phraseIndex.starts 中的区间。
type phraseGroup struct {
	start uint32
	n     uint32
}

// phraseIndex 是词组消歧的索引。
type phraseIndex struct {
	// groups 把词组的首字映射到它在下面两张表里的区间。
	groups map[rune]phraseGroup
	// second[k] 是第 k 个桶第二个字的码点，同一个首字的桶按它升序排列。
	// 单独存一份是为了让二分查找只碰整数：现从 phraseBlob 里解 rune 会慢一倍多。
	second []rune
	// starts[k] 是第 k 个桶在 phraseBlob 中的起始下标，末尾多一个哨兵 len(phraseBlob)，
	// 因此第 k 个桶的区间是 [starts[k], starts[k+1])。
	starts []uint32
}

var (
	phraseOnce sync.Once
	phraseIdx  *phraseIndex
)

// buildPhraseIndex 建出索引。它只被 matchPhrase 里的 sync.Once 调用一次。
//
// 只用逐字转换的调用方不会走到这里，也就不必为四十万条词组付出建表的时间与内存。
func buildPhraseIndex() *phraseIndex {
	// 一个桶平均对应十来字节的记录，按此估个容量，基本不会扩容。
	capacity := len(phraseBlob) / 12
	ix := &phraseIndex{
		groups: make(map[rune]phraseGroup, 1<<14),
		second: make([]rune, 0, capacity),
		starts: make([]uint32, 0, capacity),
	}
	for i := 0; i < len(phraseBlob); {
		r1, r2 := headRunes(phraseBlob[i:])
		g := ix.groups[r1]
		if g.n == 0 {
			g.start = uint32(len(ix.starts))
		}
		g.n++
		ix.groups[r1] = g
		ix.second = append(ix.second, r2)
		ix.starts = append(ix.starts, uint32(i))
		i, _ = bucketEnd(i)
	}
	ix.second = append(ix.second, 0)
	ix.starts = append(ix.starts, uint32(len(phraseBlob)))
	return ix
}

// headRunes 返回字符串的前两个字。词组至少两个字，调用方无需判长。
func headRunes(s string) (r1, r2 rune) {
	r1, sz := utf8.DecodeRuneInString(s)
	r2, _ = utf8.DecodeRuneInString(s[sz:])
	return r1, r2
}

// bucketEnd 返回从 i 开始、前两字与 i 处相同的连续记录的下一条记录下标，以及记录条数。
// i 必须落在一条记录的起点上。
func bucketEnd(i int) (end int, n uint32) {
	r1, r2 := headRunes(phraseBlob[i:])
	for end = i; end < len(phraseBlob); n++ {
		a, b := headRunes(phraseBlob[end:])
		if a != r1 || b != r2 {
			break
		}
		end = nextRecord(end)
	}
	return end, n
}

// record 返回 phraseBlob 中第 i 条记录的词组原文与读音。
//
// 一条记录是「词组|读音|」，两个字段都不会包含 '|'——生成器保证了这一点，
// 所以这里不必处理找不到分隔符的情况。
func record(i int) (phrase, readings string) {
	pEnd := i + strings.IndexByte(phraseBlob[i:], '|')
	rEnd := pEnd + 1 + strings.IndexByte(phraseBlob[pEnd+1:], '|')
	return phraseBlob[i:pEnd], phraseBlob[pEnd+1 : rEnd]
}

// nextRecord 返回 phraseBlob 中第 i 条记录的下一条记录的下标。
func nextRecord(i int) int {
	phrase, readings := record(i)
	return i + len(phrase) + len(readings) + 2
}

// matchPhrase 在 s[start:] 上做正向最大匹配，start 必须落在 rune 边界上。
//
// 返回匹配到的字节数与整词读音。读音为空表示该词组的读音与逐字默认读音一致，
// 调用方按单字输出即可——但它同样要占住这 n 个字节，不让更短的词组从中间插进来。
func matchPhrase(s string, start int) (int, string) {
	// 词组全部由字典收录的汉字组成，不可能以 ASCII 字符开头。
	if s[start] < utf8.RuneSelf {
		return 0, ""
	}
	rest := s[start:]
	_, sz := utf8.DecodeRuneInString(rest)
	if sz >= len(rest) {
		return 0, "" // 只剩一个字，不可能构成词组
	}
	phraseOnce.Do(func() { phraseIdx = buildPhraseIndex() })
	r1, r2 := headRunes(rest)
	g, ok := phraseIdx.groups[r1]
	if !ok {
		return 0, ""
	}

	// 同一个首字的桶按第二个字升序排列，二分定位到唯一可能的那个桶。
	lo, hi := int(g.start), int(g.start+g.n)
	k := lo + sort.Search(hi-lo, func(i int) bool { return phraseIdx.second[lo+i] >= r2 })
	if k == hi || phraseIdx.second[k] != r2 {
		return 0, ""
	}

	// 桶内按字数降序，第一个前缀匹配上的即最长匹配。
	// 这里直接由已取到的两个字段推出下一条记录的下标，省掉一次重复扫描分隔符。
	for i, end := int(phraseIdx.starts[k]), int(phraseIdx.starts[k+1]); i < end; {
		phrase, readings := record(i)
		if strings.HasPrefix(rest, phrase) {
			return len(phrase), readings
		}
		i += len(phrase) + len(readings) + 2
	}
	return 0, ""
}
