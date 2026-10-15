package pinyin

import (
	"sync"
	"testing"
	"unicode/utf8"
)

var allStyles = []Style{Normal, Tone, Tone2, Tone3, Initials, FirstLetter}

// FuzzConvert 保证任意输入都不会 panic，且输出始终是合法 UTF-8。
func FuzzConvert(f *testing.F) {
	for _, s := range []string{
		"", "中国人", "重庆", "hello 中国! 123", "🎉",
		"女绿具去鱼", "嗯呣欸", "中华人民共和国成立于一九四九年。",
		"�", "\U00020000", "ā", "ê̄ê̌m̀ḿ",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, st := range allStyles {
			out := Convert(s, st)
			if !utf8.ValidString(out) {
				t.Fatalf("Convert(%q, %s) 输出非法 UTF-8: %q", s, st, out)
			}
			hetero := ConvertWith(s, Options{Style: st, Heteronym: true, Separator: "-"})
			if !utf8.ValidString(hetero) {
				t.Fatalf("多音字模式输出非法 UTF-8: %q", hetero)
			}
		}
		if out := FirstLetters(s); !utf8.ValidString(out) {
			t.Fatalf("FirstLetters(%q) 输出非法 UTF-8: %q", s, out)
		}
		// Readings 与输入的 rune 一一对应。
		if got, want := len(Readings(s)), utf8.RuneCountInString(s); got != want {
			t.Fatalf("Readings(%q) 长度 = %d, want %d", s, got, want)
		}
	})
}

// TestConcurrentUse 在 -race 下验证并发调用安全。
func TestConcurrentUse(t *testing.T) {
	const goroutines, iterations = 8, 500
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				for _, st := range allStyles {
					_ = Convert(benchText, st)
				}
				_ = ConvertWith(benchText, Options{Style: Tone, Heteronym: true, Separator: "-"})
				_ = FirstLetters(benchText)
				_ = Readings(benchText)
			}
		}()
	}
	wg.Wait()
}

// TestConvertAllocations 锁定「整句转换只分配两次」这一性能契约。
func TestConvertAllocations(t *testing.T) {
	for _, st := range allStyles {
		n := testing.AllocsPerRun(200, func() { _ = Convert(benchText, st) })
		if n > 2 {
			t.Errorf("Convert(_, %s) 每次分配 %.1f 次，期望 ≤ 2", st, n)
		}
	}
}

// TestOptionsZeroValue 固化零值语义：Normal 风格、不分隔、保留非汉字。
func TestOptionsZeroValue(t *testing.T) {
	if got := ConvertWith("a中国", Options{}); got != "azhongguo" {
		t.Errorf("Options{} 零值行为 = %q, want %q", got, "azhongguo")
	}
}
