package syllable

import (
	"strings"
	"testing"
)

func TestCount(t *testing.T) {
	if got := Count(); got < 420 || got > 440 {
		t.Errorf("音节总数 %d 超出预期区间 [420, 440]", got)
	}
}

func TestValid(t *testing.T) {
	for _, s := range strings.Fields(`zhong guo ren nv lve er ai o ng m hm hng biang`) {
		if !Valid(s) {
			t.Errorf("Valid(%q) = false, 期望 true", s)
		}
	}
	for _, s := range strings.Fields(`zhongg nv2 ZHO 中文 "" 1x a1`) {
		if Valid(s) {
			t.Errorf("Valid(%q) = true, 期望 false", s)
		}
	}
	if Valid("") {
		t.Error(`Valid("") = true, 期望 false`)
	}
}

func TestNoDuplicateOrBlank(t *testing.T) {
	seen := make(map[string]bool)
	for _, s := range strings.Fields(standard + " " + rare) {
		if s == "" {
			t.Fatal("清单中存在空音节")
		}
		if seen[s] {
			t.Errorf("音节 %q 重复出现", s)
		}
		seen[s] = true
	}
}
