// Package pinyin 提供简体中文汉字转拼音的核心能力：整句转写、声母/首字母提取、
// 多音字查询。
//
// # 快速上手
//
//	pinyin.Convert("中国人", pinyin.Normal)      // "zhongguoren"
//	pinyin.Convert("中国人", pinyin.Tone)        // "zhōngguórén"
//	pinyin.Convert("中国人", pinyin.Tone2)       // "zho1ngguo2re2n"
//	pinyin.Convert("中国人", pinyin.Tone3)       // "zhong1guo2ren2"
//	pinyin.Convert("中国人", pinyin.Initials)    // "zhgr"
//	pinyin.Convert("中国人", pinyin.FirstLetter) // "zgr"
//	pinyin.FirstLetters("中华人民共和国")        // "zhrmghg"
//
// 需要更细的控制时用 ConvertWith 配合 Options：
//
//	o := pinyin.Options{Style: pinyin.Tone, Separator: "-"}
//	pinyin.ConvertWith("中国人", o) // "zhōng-guó-rén"
//
// # 非汉字字符
//
// 汉字以外的字符（拉丁字母、数字、标点、空白、emoji）默认原样保留、位置不变。
// 需要丢弃或改写时，用 Options.Fallback 返回空串或替换文本。
//
// # 多音字与词组消歧
//
// 单字默认取最常用读音，Readings 返回每个字的全部读音，Options.Heteronym 让整句
// 按「全部读音」输出。
//
// Convert 与 ConvertWith 默认还会做词组消歧：先对输入做正向最大匹配，命中词组时
// 整词取其读音，因此
//
//	pinyin.Convert("重庆", pinyin.Tone) // "chóngqìng"
//
// 而逐字转换会得到 "zhòngqìng"。词组消歧的数据来自 mozillazg/phrase-pinyin-data，
// 收录源表的整张词组表，因此结果与「对完整词组表做正向最大匹配」逐字一致。
//
// 需要逐字转换时把 Options.CharByChar 设为 true。Heteronym 与词组消歧互斥：
// 多音字模式要的是每个字的全部读音，此时按单字输出。
//
// # 线程安全
//
// 本包全部导出函数都是只读的，可被任意多个 goroutine 并发调用，无需加锁。
//
// 词组索引是惰性构建的：第一次用到词组消歧时才建表，之后只读。CharByChar 为 true
// 的调用方完全不会为它付出内存和时间代价。
//
// # 性能
//
// 单字查表采用按码点排序的静态数组 + 二分查找，没有运行时初始化开销。词组消歧在
// 静态词组表上按前两个字分桶：先用首字定位到一组桶，再按第二个字二分，命中的桶内
// 候选按字数降序排列，第一个前缀匹配上的就是最长匹配。
//
// 单个汉字的转换不产生堆分配；整句转换只在输出缓冲和最终字符串上各分配一次。
//
// # 数据来源与覆盖
//
// 拼音数据来自 github.com/mozillazg/pinyin-data（MIT 协议），词组数据来自
// github.com/mozillazg/phrase-pinyin-data（MIT 协议），均由 cmd/gendata 生成，
// 不引入任何运行时依赖。
//
// 收录 CJK 扩展 A 与统一表意文字（U+3400–U+9FFF），以及《通用规范汉字表》中位于
// BMP 之外的 196 个汉字，共 26908 字。其中：
//
//   - GB2312 的 6763 个汉字：100% 覆盖
//   - 《通用规范汉字表》(2013) 的 8105 字：100% 覆盖
//
// CJK 扩展 B 及以后的生僻字不在收录范围内。
//
// # 已知限制
//
// 词组消歧在词组表上做正向最大匹配，不做分词。词组表之外的组合仍按单字默认读音
// 处理，例如生造词、人名地名、以及词组表中没有的新词。
package pinyin

//go:generate go run ./cmd/gendata
