package pinyin

import (
	"strings"
	"testing"

	syll "github.com/xywf221/go-pinyin/internal/syllable"
)

// TestAllReadingsAreValidSyllables 断言数据表里的每个读音都是合法音节。
// 这是发现上游数据源引入脏数据的第一道防线。
func TestAllReadingsAreValidSyllables(t *testing.T) {
	bad := map[string]rune{}
	for _, e := range dict {
		for _, p := range strings.Split(e.py, ",") {
			sy := parseSyllable(p)
			b := string(sy.base[:sy.n])
			if !syll.Valid(b) {
				if _, seen := bad[b]; !seen {
					bad[b] = e.cp
				}
			}
		}
	}
	for s, cp := range bad {
		t.Errorf("非法音节 %q（出现在 %c U+%04X）", s, cp, cp)
	}
}

// uncoveredInURO 是 U+4E00–U+9FA5 区间内数据源中没有拼音的字。
//
// 这 10 个字都不属于 GB2312（经 CP936 映射表核对），而是 GBK 扩展区的度量衡单位字
// 和日制汉字，在《通用规范汉字表》《现代汉语词典》等权威字表中均无普通话读音记录，
// 因此保持空缺而不是猜测读音。
const uncoveredInURO = "兙兡嗧桛烪瓧瓰瓱瓼甅"

// TestGB2312Coverage 断言 GB2312 的 6763 个汉字全部有拼音。
//
// U+4E00–U+9FA5 区间比 GB2312 更宽，覆盖它就必然覆盖 GB2312，
// 因此这里用「区间全覆盖减去已知空缺」来断言。
func TestGB2312Coverage(t *testing.T) {
	exempt := make(map[rune]bool)
	for _, r := range uncoveredInURO {
		exempt[r] = true
	}
	if len(exempt) != 10 {
		t.Fatalf("豁免表应有 10 个字，实际 %d 个", len(exempt))
	}

	var missing []rune
	for r := rune(0x4E00); r <= 0x9FA5; r++ {
		if exempt[r] {
			continue
		}
		if _, ok := lookup(r); !ok {
			missing = append(missing, r)
		}
	}
	for _, r := range missing {
		t.Errorf("汉字 %c (U+%04X) 缺少拼音", r, r)
	}
}

// TestCuratedCorrections 固化 data/overrides.txt 里人工校订的结果。
//
// 上游数据源更新后，如果这些字被改回错误读音，此测试会失败。
// 例词都按整词转换，因此读音同时受词组消歧影响——「颇为」的 为 取词组的 wéi，
// 而不是单字默认的 wèi。
func TestCuratedCorrections(t *testing.T) {
	tests := []struct{ word, want string }{
		{"鹿茸", "lùróng"},
		{"颇为", "pōwéi"},
		{"豆豉", "dòuchǐ"},
		{"砝码", "fǎmǎ"},
		{"川芎", "chuānxiōng"},
		{"嗲", "diǎ"},
		{"菹", "zū"},
		{"蓇葖", "gūtū"},
		{"谝", "piǎn"},
		{"迕", "wǔ"},
		{"聒噪", "guōzào"},
		{"缅甸", "miǎndiàn"},
		{"佛教", "fójiào"},
	}
	for _, tt := range tests {
		if got := Convert(tt.word, Tone); got != tt.want {
			t.Errorf("Convert(%q, Tone) = %q, want %q", tt.word, got, tt.want)
		}
	}
}

// TestCommonCharactersCovered 抽查高频汉字，防止数据表出现空洞。
func TestCommonCharactersCovered(t *testing.T) {
	const common = "的一是了我不人在他有这个上们来到时大地为子中你说生国年着就那和要她出也得里后自以会家可下而过天去能对小多然于心学么之都好看起发当没成只如事把还用第样道想作种开美总从无情己面最女但现前些所同日手又行意动方期它头经长儿回位分爱老因很给名法间斯知世什两次使身者被高已亲其进此话常与活正感"
	for _, r := range common {
		if _, ok := lookup(r); !ok {
			t.Errorf("高频字 %c (U+%04X) 缺少拼音", r, r)
		}
	}
}
