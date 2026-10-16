# go-pinyin

简体中文汉字转拼音的 Go 核心库。零依赖、纯查表、并发安全。

- 整句转写：带声调符号 / 声调数字 / 不带声调
- **词组消歧**：多音字按词组取音，「重庆」得到 `chóngqìng` 而不是 `zhòngqìng`
- 声母、首字母提取
- 多音字查询
- 非汉字字符默认**原样保留**，不静默丢弃
- 数据表内嵌，无运行时初始化开销

## 安装

```bash
go get github.com/xywf221/go-pinyin
```

模块路径是 `github.com/xywf221/go-pinyin`，包名固定为 `pinyin`。

## 快速开始

```go
import pinyin "github.com/xywf221/go-pinyin"

pinyin.Convert("中国人", pinyin.Normal)      // "zhongguoren"
pinyin.Convert("中国人", pinyin.Tone)        // "zhōngguórén"
pinyin.Convert("中国人", pinyin.Tone3)       // "zhong1guo2ren2"
pinyin.Convert("中国人", pinyin.Initials)    // "zhgr"
pinyin.Convert("中国人", pinyin.FirstLetter) // "zgr"

pinyin.Convert("重庆", pinyin.Tone)          // "chóngqìng"
pinyin.Convert("银行", pinyin.Tone)          // "yínháng"

pinyin.FirstLetters("中华人民共和国")        // "zhrmghg"

pinyin.Readings("重")                        // [["zhòng" "chóng" "tóng"]]
```

## API

| 函数 | 说明 |
| --- | --- |
| `Convert(s string, style Style) string` | 按风格转换，先做词组消歧，词组外取单字最常用读音 |
| `ConvertWith(s string, o Options) string` | 完整配置：分隔符、多音字、词组消歧开关、兜底策略 |
| `FirstLetters(s string) string` | 取每个汉字的首字母，等价于 `Convert(s, FirstLetter)` |
| `Readings(s string) [][]string` | 每个字符的全部读音，与输入的 rune 一一对应 |

### Style

| Style | 说明 | 例（中国人） |
| --- | --- | --- |
| `Normal` | 不带声调 | `zhongguoren` |
| `Tone` | 带声调符号 | `zhōngguórén` |
| `Tone2` | 声调数字紧跟承载声调的韵母 | `zho1ngguo2re2n` |
| `Tone3` | 声调数字在音节末尾 | `zhong1guo2ren2` |
| `Initials` | 声母 | `zhgr` |
| `FirstLetter` | 首字母 | `zgr` |

`Initials` 对零声母音节（a/o/e 开头）返回其首字母，所以「啊」得到 `a` 而不是空串。

### Options

```go
type Options struct {
	Style      Style               // 输出风格，默认 Normal
	Separator  string              // 相邻音节之间的分隔符，默认空
	Heteronym  bool                // true 时每个字输出全部读音，用 "/" 分隔
	Fallback   func(r rune) string // 处理查不到的字符；返回空串表示丢弃
	CharByChar bool                // true 时逐字转换，关闭词组消歧
}
```

```go
// 音节之间加连字符
pinyin.ConvertWith("中国人", pinyin.Options{Style: pinyin.Tone, Separator: "-"})
// "zhōng-guó-rén"

// 词组内部的音节之间同样会插入分隔符
pinyin.ConvertWith("重庆", pinyin.Options{Style: pinyin.Tone, Separator: "-"})
// "chóng-qìng"

// 逐字转换，不做词组消歧
pinyin.ConvertWith("重庆", pinyin.Options{Style: pinyin.Tone, CharByChar: true})
// "zhòngqìng"

// 多音字
pinyin.ConvertWith("重", pinyin.Options{Heteronym: true})
// "zhong/chong/tong"

// 丢弃非汉字字符
pinyin.ConvertWith("hello 中国!", pinyin.Options{Fallback: func(r rune) string { return "" }})
// "zhongguo"
```

## 行为说明

### 非汉字字符默认保留

汉字以外的字符（拉丁字母、数字、标点、空白、emoji）默认原样输出，位置不变：

```go
pinyin.Convert("hello 中国! 123", pinyin.Normal) // "hello zhongguo! 123"
```

这一点与 `mozillazg/go-pinyin` 相反——后者默认丢弃，也因此收到过保留原文的 issue。
需要丢弃时用 `Options.Fallback` 返回空串。

### ü 的写法

`ü` 系在 `Normal` / `Tone2` / `Tone3` / `Initials` / `FirstLetter` 下写作 `v`，只有 `Tone` 保留 `ǚ` 这样的带声调字符：

| | Normal | Tone | Tone2 | Tone3 |
| --- | --- | --- | --- | --- |
| 女 | `nv` | `nǚ` | `nv3` | `nv3` |
| 绿 | `lv` | `lǜ` | `lv4` | `lv4` |

`j/q/x/y` 后的 `u` 不是 `ü`，按实际写法输出：具 → `ju`，去 → `qu`，鱼 → `yu`。

### 多音字与词组消歧

单字默认读音取《通用规范汉字表》(2013) 校准过的最常用读音：

| 字 | 默认读音 | 全部读音 |
| --- | --- | --- |
| 重 | zhòng | zhòng, chóng, tóng |
| 长 | zhǎng | zhǎng, cháng |
| 行 | xíng | xíng, háng, héng, xìng, hàng |
| 的 | de | de, dī, dí, dì |

`Heteronym` 模式下，不同读音去掉声调后可能重合（「中」的 zhōng / zhòng 在 `Normal` 下都是 `zhong`），会自动去重。

单字读音在多音字上必然出错，所以 `Convert` / `ConvertWith` 默认还会做**词组消歧**：
先对输入做正向最大匹配，命中词组时整词取该词组的读音。

| 词 | 词组消歧后 | 逐字转换 |
| --- | --- | --- |
| 重庆 | chóngqìng | zhòngqìng |
| 长沙 | chángshā | zhǎngshā |
| 厦门 | xiàmén | shàmén |
| 银行 | yínháng | yínxíng |
| 头发 | tóufà | tóufā |
| 西藏 | xīzàng | xīcáng |
| 主角 | zhǔjué | zhǔjiǎo |
| 便宜 | piányi | biànyí |
| 音乐 | yīnyuè | yīnlè |

匹配是逐字前进的：命中词组后就跳到词组末尾，不会从词组的第二个字重新起匹配。所以「隆重庆祝」得到 `lóngzhòngqìngzhù`——前面的「隆重」先匹配上，跳过了「重庆」的起点，不会读成 `chóng`。

`Heteronym` 与词组消歧互斥——多音字模式要的是每个字的全部读音，此时按单字输出。
需要逐字转换时把 `Options.CharByChar` 设为 `true`。

## 数据

### 来源与覆盖

拼音数据来自 [mozillazg/pinyin-data](https://github.com/mozillazg/pinyin-data)（MIT 协议），词组数据来自 [mozillazg/phrase-pinyin-data](https://github.com/mozillazg/phrase-pinyin-data)（MIT 协议），由 `cmd/gendata` 生成到 `dict_data.go` 与 `phrase_data.go`，不引入任何运行时依赖。

收录 CJK 扩展 A 与统一表意文字（U+3400–U+9FFF），以及《通用规范汉字表》中位于 BMP 之外的 196 个汉字，共 **26908** 字。覆盖率由 `data_test.go` 中的测试断言：

| 字集 | 覆盖 |
| --- | --- |
| GB2312（6763 字） | 100% |
| 《通用规范汉字表》(2013)（8105 字） | 100% |

CJK 扩展 B 及以后的生僻字不在收录范围内。另有 10 个位于 U+4E00–U+9FA5 但**不属于 GB2312** 的 GBK 扩展区度量衡单位字与日制汉字，因权威字表中没有普通话读音记录而保持空缺。

### 词组表

词组源表有 411958 条，生成器**整表收录** 411946 条（另 12 条源数据本身有瑕疵，见下），因此结果与「对完整词组表做正向最大匹配」逐字一致。其中只有 **40150** 条读音与逐字默认读音不同，其余 371796 条在数据里只存词组原文、读音留空，运行时按单字读音补齐。

为什么不只收录那 40150 条例外？因为正向最大匹配是逐字前进的，一个没被收录的词组会被更短的词组从中间切开，而「被切开」的判定又依赖另一条词组是否被收录，例外集必须一路补到几乎整张表才闭合：

- 「下子 `xià zǐ`」需要收录，于是「一下子 `yī xià zi`」会被从中间切开，必须一并收录；
- 「重庆 `chóng qìng`」需要收录，于是「隆重庆祝」「隆重庆典」「隆重庆贺」…… 里的「重庆」都会被带走，全部必须收录。

实测这条闭包覆盖完整词表的 98.4%，省下的空间不到 2%，却要额外维护一套容易出错的补集规则。整表收录的代价是生成数据从 1.4MB 涨到 4.5MB，换来的是与完整词表逐字一致、不需要任何特例。

`phrase_test.go` 里的 `TestPhraseDictionaryAccuracy` 会拿完整词组表的 411944 条逐一核对。

### 数据校验

生成器 `cmd/gendata` 在写文件前会拒绝任何异常数据，任何一条不满足都会终止生成：

- 读音不能为空、不能重复
- 每个读音去掉声调后必须是规范音节（425 个规范音节的闭集校验，见 `internal/syllable`）
- 单字校订的码点必须与注释中的汉字一致
- 单字校订的读音必须已经存在于该字的读音列表中
- 词组校订的词组必须存在于源表中，音节数必须与字数一致，音节必须规范
- 词组校订后的读音必须与源表不同（源表升级修好该读音后，这条校订会失败，逼着人删掉它）

词组表里不合格的行（音节数与字数不符、含未收录字、非规范音节等）会被跳过并在生成日志里按原因分类统计，源表中读音互相矛盾的重复词组会逐条列出。当前源表 411958 行中有 12 行属于此类。

### 人工校订

上游数据源的默认读音取自 Unihan `kMandarin` 字段，绝大多数情况下正确，但存在少量明显错误——通常是把罕见读音排在了常用读音前面。

`data/overrides.txt` 收录了 13 条人工校订，每条都满足：**汉典 zdic.net、现代汉语词典(kXHC1983)、通用规范汉字字典(kTGHZ2013) 三方一致**，且能在常用词中验证。例如：

| 字 | 原默认音 | 校订为 | 例词 |
| --- | --- | --- | --- |
| 茸 | rōng | róng | 鹿茸 lùróng |
| 颇 | pǒ | pō | 颇为 pōwéi |
| 豉 | shì | chǐ | 豆豉 dòuchǐ |
| 砝 | fá | fǎ | 砝码 fǎmǎ |
| 甸 | diān | diàn | 缅甸 miǎndiàn |
| 佛 | fú | fó | 佛教 fójiào |

被**有意排除**的争议字：`重`、`长`、`行` 等各来源本身分歧，且词频数据支持当前默认音；`万`、`且`、`区` 等现汉把姓氏或生僻义排在前面，当前默认音才是常用音。它们的问题是词组层面的，由词组消歧负责。

### 词组人工校订

单字读音有《通用规范汉字表》兜底，词组读音则完全依赖上游源表，而源表对多音字的词义区分有成片的遗漏。`data/phrase_overrides.txt` 收录 **89** 条词组级校订。

第一批是整族错误：

| 词 | 原读音 | 校订为 | 说明 |
| --- | --- | --- | --- |
| 古朴 · 简朴 · 纯朴 · 质朴 …（8 条） | gǔ piáo | gǔ pǔ | 朴的 `piáo` 只用于姓氏，源表把它当成了默认 |
| 温差 · 时差 · 落差 · 逆差 …（17 条） | wēn chà | wēn chā | 差数的 `chā` 被整体误作「差劲」的 `chà` |
| 骨头架子 · 皮包骨头 · 软骨头 …（6 条） | gú tou | gǔ tou | 骨的口语变读，非规范读音 |
| 重结晶 · 重设 · 重楼 | zhòng … | chóng … | 「再次」义的重 |
| 漯河 | tà hé | luò hé | 与同一张表的「漯河市 luò hé shì」自相矛盾 |
| 秦桧 | qín guì | qín huì | 桧的 `guì` 只用于桧树 |
| 标识 | biāo shí | biāo zhì | |
| 说客 | shuō kè | shuì kè | |
| 半间不界 | bàn jiān bù jiè | bàn gān bù gà | 同「尴尬」 |

第二批是 `cmd/audit` 交叉比对挖出来的 14 条：汉典与 CC-CEDICT **两个来源都独立给出同一个读音，而上游基准表给的是另一个**（上游自己维护的 `overwrite.txt` 里没有这些条目，也就是说上游的读数在这里没有任何依据）。例如：

| 词 | 原读音 | 校订为 |
| --- | --- | --- |
| 哀乐 | āi lè | āi yuè |
| 心惊胆颤 | xīn jīng dǎn zhàn | xīn jīng dǎn chàn |
| 坑蒙拐骗 | kēng méng … | kēng mēng … |
| 曲肱而枕 | qǔ gōng … | qū gōng … |
| 舍车保帅 | shè jū … | shě jū … |
| 红彤彤 | hóng tóng tóng | hóng tōng tōng |
| 芥蓝 | jiè lán | gài lán |
| 荤粥 | xūn zhōu | xūn yù |

### 交叉比对

`cmd/audit` 把本仓库发布的读音与外部来源逐条比对，按差异性质分成四类：

```bash
go run ./cmd/audit \
    -ref 汉典=_research/phrase/zdic_cibs.txt \
    -ref CC-CEDICT=_research/phrase/cc_cedict.txt
```

| 类别 | 含义 | 是否动手 |
| --- | --- | --- |
| A 声母韵母不同 | 读音完全不同 | 错误候选 |
| B 仅声调不同 | | 错误候选，异读里常见 |
| C 轻声标注差异 | 玻璃 bō lí / bō li | 惯例，通常不算错 |
| D 一/不 变调 | 一刀两断 yì dāo / yī dāo | 惯例，本库按变调输出 |

> **上游的合并规则很重要。** `phrase-pinyin-data` 用 `merge.py zdic_cibs zdic_cybs cc_cedict di pinyin overwrite` 生成词表，**后出现的文件覆盖先出现的**。所以上游自己维护的 `pinyin.txt` 与 `overwrite.txt` 会盖掉汉典与 CC-CEDICT。这意味着：**汉典、CC-CEDICT 对本仓库都不是「真值」，只是证人**——它们能指出可疑之处，但不能证明我们错。工具里 `-min-agree 2` 找的正是「两个证人都反对我们」的部分。

参考数据不随仓库分发（CC-CEDICT 是 CC BY-SA 4.0，汉典数据授权不明），所以 `cmd/audit` 不在 CI 里跑。

**有意不收录**的几类：

- **一/不 的变调**（一刀两断 yì dāo、不对 bú duì）。源表按变调标注，本库也按变调输出，两边都对。与 CC-CEDICT「一 一律标 yī」的差异属标注惯例，不是错误。
- **真正的异读**（上调 shàngdiào/shàngtiáo、供养 gòngyǎng/gōngyǎng、斗牛 dòuniú/dǒuniú）。各词典分歧，保持源表原样。
- **轻声的书写差异**（豆腐 dòu fu 写成 dòu fǔ）。属标注惯例。
- **姓氏**（单 shàn、仇 qiú、解 xiè）。源表基本都按常用音处理，本库不做姓氏识别。

校订表由 `cmd/gendata` 严格校验：词组必须在源表中存在、音节数与字数一致、音节规范、且校订后的读音确实与源表不同；任何一条不满足都终止生成。根目录的 `TestPhraseOverridesApplied` 再逐条比对 `Convert` 的输出，锁住端到端结果——包括那些校订回单字默认读音、在生成数据里看不出痕迹的条目。

## 已知限制

**词组消歧在词组表上做正向最大匹配，不做分词。** 词组表之外的组合仍按单字默认读音处理，例如生造词、人名地名、词组表里没有的新词。

正向最大匹配也会在词组表自身有歧义时做出取舍，例如源表同时把「朝阳」标成 `zhāo yáng` 和 `cháo yáng`，生成器保留先出现的一条。

## 质量保证

| 项目 | 状态 |
| --- | --- |
| 核心包语句覆盖率 | 100% |
| `internal/syllable`、`internal/tone` 覆盖率 | 100% |
| 模糊测试 | `FuzzConvert`，任意输入不 panic、输出始终是合法 UTF-8 |
| 竞态检测 | `go test -race` 全绿，全部导出函数并发安全 |
| 分配契约 | `TestConvertAllocations` 锁定整句转换 ≤ 2 次分配 |
| 词组数据 | `TestPhraseDictionaryAccuracy` 拿完整词组表 411944 条逐条核对 |
| 词组匹配 | `TestPhraseMatchesFullTableFMM` 拿随机拼接的句子与完整词表的最大匹配参照实现对比 |
| 词组索引 | `TestPhraseIndexIntegrity` 校验索引与数据表逐桶对应 |
| 词组校订 | `TestPhraseOverridesApplied` 逐条比对 `Convert` 的输出；`TestPhraseOverridesAreLoadBearing` 确认没有死条目 |
| CI | 构建、vet、race、覆盖率、golangci-lint、模糊测试、生成数据一致性 |

## 性能

在 AMD Ryzen 5 5600X 上，对 26 字的句子（`中华人民共和国成立于一九四九年，首都是北京。`）整句转换：

| 基准 | 耗时 | 分配 |
| --- | --- | --- |
| `BenchmarkConvert` | ~2.79 µs | 2 次 |
| `BenchmarkConvertCharByChar` | ~1.30 µs | 2 次 |
| `BenchmarkConvertPhraseHeavy`（39 字，多音字密集） | ~4.04 µs | 2 次 |
| `BenchmarkFirstLetters` | ~2.77 µs | 2 次 |
| `BenchmarkConvertMixed`（汉字与拉丁字母混排） | ~2.14 µs | 2 次 |

词组消歧的代价约 1.5 µs：每个字位先按首字查一次表定位到一组桶，再按第二个字二分，命中的桶内候选通常只有一两条，一次前缀比较就能定下来。

索引惰性构建，只用 `CharByChar` 的调用方不会付出这份代价；构建完成后常驻约 3.2MB 堆内存（四十万条词组的分桶索引）。

## 重新生成数据

```bash
go generate ./...
```

生成器会：

1. 合并全量表与《通用规范汉字表》校准读音 → 应用 `data/overrides.txt` → 校验 → 按码点排序 → 输出 `dict_data.go`
2. 解析词组表 → 校验并整表收录 → 应用 `data/phrase_overrides.txt` → 按「前两个字、字数从多到少」排序 → 输出 `phrase_data.go`

CI 中有一步 `generated-up-to-date` 会重新生成并 `git diff`，确保两个生成文件与 `data/` 下的源数据始终一致。

## 协议

MIT。拼音与词组数据版权归 [mozillazg/pinyin-data](https://github.com/mozillazg/pinyin-data) 与 [mozillazg/phrase-pinyin-data](https://github.com/mozillazg/phrase-pinyin-data) 所有，同为 MIT 协议。
