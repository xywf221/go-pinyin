package pinyin_test

import (
	"fmt"

	pinyin "github.com/xywf221/go-pinyin"
)

func ExampleConvert() {
	fmt.Println(pinyin.Convert("中国人", pinyin.Normal))
	fmt.Println(pinyin.Convert("中国人", pinyin.Tone))
	fmt.Println(pinyin.Convert("中国人", pinyin.Tone2))
	fmt.Println(pinyin.Convert("中国人", pinyin.Tone3))
	fmt.Println(pinyin.Convert("中国人", pinyin.Initials))
	fmt.Println(pinyin.Convert("中国人", pinyin.FirstLetter))
	// Output:
	// zhongguoren
	// zhōngguórén
	// zho1ngguo2re2n
	// zhong1guo2ren2
	// zhgr
	// zgr
}

func ExampleFirstLetters() {
	fmt.Println(pinyin.FirstLetters("中华人民共和国"))
	// Output: zhrmghg
}

func ExampleReadings() {
	for _, readings := range pinyin.Readings("重") {
		fmt.Println(readings)
	}
	// Output: [zhòng chóng tóng]
}

func ExampleConvertWith_separator() {
	o := pinyin.Options{Style: pinyin.Tone, Separator: "-"}
	fmt.Println(pinyin.ConvertWith("中国人", o))
	// Output: zhōng-guó-rén
}

func ExampleConvert_phraseDisambiguation() {
	// 多音字默认取最常用读音，但整句转换会先做词组消歧：
	// 重 单独念 zhòng，「重庆」里念 chóng。
	fmt.Println(pinyin.Convert("重庆", pinyin.Tone))
	fmt.Println(pinyin.Convert("银行", pinyin.Tone))
	// Output:
	// chóngqìng
	// yínháng
}

func ExampleConvertWith_charByChar() {
	// 关闭词组消歧，退回逐字转换。
	o := pinyin.Options{Style: pinyin.Tone, CharByChar: true}
	fmt.Println(pinyin.ConvertWith("重庆", o))
	// Output: zhòngqìng
}

func ExampleConvertWith_heteronym() {
	o := pinyin.Options{Heteronym: true}
	fmt.Println(pinyin.ConvertWith("重", o))
	// Output: zhong/chong/tong
}

func ExampleConvertWith_fallback() {
	// 丢弃非汉字字符，只保留汉字的拼音。
	o := pinyin.Options{Fallback: func(r rune) string { return "" }}
	fmt.Println(pinyin.ConvertWith("hello 中国!", o))
	// Output: zhongguo
}

func ExampleConvert_nonHanPreserved() {
	// 默认保留非汉字字符。
	fmt.Println(pinyin.Convert("hello 中国!", pinyin.Normal))
	// Output: hello zhongguo!
}
