# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概览

简体中文汉字转拼音的 Go 核心库。零依赖、纯查表、并发安全。

模块路径 `github.com/xywf221/go-pinyin`，但**包名是 `pinyin`**，所以 import 必须带别名：

```go
import pinyin "github.com/xywf221/go-pinyin"
```

代码注释、文档、提交信息一律用中文。所有导出标识符都要有 doc comment，且以句号结尾（godot linter 会检查）。

## 常用命令

```bash
go build ./...
go vet ./...
go test ./...                     # 全量测试（约 2s）
go test -run TestPhraseIndexIntegrity -v .   # 跑单个测试
go test -race ./...
go test -cover .
go test -run XXX -bench . -benchmem .        # 基准测试
go test -run XXX -fuzz FuzzConvert -fuzztime 60s .
golangci-lint run                 # 配置见 .golangci.yml
go generate ./...                 # 重新生成 dict_data.go 与 phrase_data.go
```

CI（`.github/workflows/ci.yml`）会跑：build、vet、`-race`、覆盖率、golangci-lint、60s 模糊测试，以及一步 `generated-up-to-date`——重新 `go generate` 后 `git diff --exit-code -- dict_data.go phrase_data.go`。

## 架构

### 生成数据与源数据

仓库里有两个**生成文件**，都在根目录，不要手改：

| 文件 | 大小 | 来源 | 生成逻辑 |
| --- | --- | --- | --- |
| `dict_data.go` | 1.3MB | `data/pinyin.txt` + `data/kMandarin_8105.txt` + `data/overrides.txt` | `cmd/gendata` 的 `emit` |
| `phrase_data.go` | 4.5MB | `data/phrase_pinyin.txt` + `data/phrase_overrides.txt` | `cmd/gendata` 的 `buildPhrases` + `applyPhraseOverrides` + `emitPhrases` |

改数据或改生成逻辑后**必须**跑 `go generate ./...` 并提交结果，否则 CI 会红。生成是确定性的（测试断言过）。

### 规则共用：`internal/`

`internal/syllable`（约 425 个规范无调音节的闭集）和 `internal/tone`（带声调字符 → 基本字母 + 声调的对照表）被**生成器和运行时共用**，目的是防止两边对「什么算合法音节」的判断漂移。`TestToneNormalizeMatchesParseSyllable`（在 `phrase_test.go`）专门锁这一点：生成器用 `tone.Normalize` 判断词组读音是否等于逐字默认读音，运行时用 `parseSyllable` 渲染，两者一旦不一致，被判定为「与单字一致」的词组就会悄悄漏掉。

### 运行时核心

- `pinyin.go` — `ConvertWith` 主循环。每个位置先试词组匹配，不中再逐字。
- `phrase.go` — 词组消歧的索引与匹配。
- `tone.go` — `syllable` 结构体与各风格的渲染。`syllable` 内部用定长数组 `[8]byte` 存基本形式，**不产生堆分配**；`letterStr` 是单字母字符串表，让声母/首字母的返回也不分配。
- `style.go` — `Style` 枚举。

## 词组数据的三个不变量

这是最容易踩的地方。`phraseBlob` 是一个拼接的大字符串，每条记录形如 `词组|读音|`；**读音为空表示逐字默认读音就是对的**，运行时按单字补齐。

记录按 `(前两个字, 字数降序, 词组)` 排序。运行时索引**完全依赖**这个顺序，它同时满足三个不变量：

1. 同一个前两字的词组连续 → 构成一个「桶」，桶内区间由相邻桶的起点推出（末尾有哨兵，桶内条数不单独存）。
2. 同一个首字的桶相邻，且按第二个字**升序** → 二分查找靠它。
3. 桶内按字数**降序** → 第一个前缀匹配上的就是最长匹配。

生成器的排序一改，索引会静默给出错误结果。`TestPhraseIndexIntegrity` 逐桶校验这三条。

### 为什么收录整张表而不是例外集

`phraseBlob` 收了源表全部 411946 条，其中只有 40141 条读音与逐字默认读音不同。看起来只存这 40141 条能省 90% 空间，但不行：

正向最大匹配是逐字前进的，一个没被收录的词组会被更短的词组从中间切开，而「被切开」的判定又依赖另一条词组是否被收录，例外集必须一路补到几乎整张表才闭合——实测覆盖完整词表的 **98.4%**。省下的不到 2%，却要多维护一套容易出错的补集规则，而且「隆重庆祝」这类词边界巧合仍会出错。整表收录换来与「对完整词组表做最大匹配」逐字一致。

### `matchPhrase` 的返回值

```go
func matchPhrase(s string, start int) (n int, readings string)
```

`readings == ""` 表示「该词组读音与逐字默认读音一致」——调用方仍必须**消费掉这 n 个字节**（`pinyin.go` 里走 `appendRunes` 分支），否则更短的词组会从中间插进来。这是整表方案能压缩掉 90% 读音字段的前提。

## 数据质量闸门

`cmd/gendata` 写文件前会拒绝异常数据并终止生成：空读音、重复读音、非规范音节、人工校订的码点与注释不符、单字校订的读音不在该字读音列表里、词组校订的词组不在源表中、词组校订的音节数与字数不符、词组校订的读音与源表相同（说明源表已经修好，这条校订该删了）。

词组表里不合格的行会被跳过并按原因分类统计（生成日志里会打印），源表中读音互相矛盾的重复词组（如「朝阳」既是 `zhāo yáng` 又是 `cháo yáng`）逐条列出。当前源表 411958 行里有 12 行属于此类。

`data/overrides.txt` 是单字校订表，准入标准严格：**汉典 zdic.net、现代汉语词典(kXHC1983)、通用规范汉字字典(kTGHZ2013) 三方一致**，且能在常用词中验证。`重`、`长`、`行` 这类各来源本身分歧的字被**有意排除**——它们的问题是词组层面的，交给词组消歧。

`data/phrase_overrides.txt` 是词组校订表，修的是上游词组表对多音字词义区分的成片遗漏（`朴` 在 8 个词里被当成姓氏音 piáo，`差` 在 17 个词里被当成 chà，`骨`、`重` 同理）。它比单字表宽松——不要求读音出现在某个字的读音列表里，「秦桧」的 `huì`、「半间不界」的 `gà` 都是整词层面的读法——但结构校验一样严：词组必须在源表中存在、音节数与字数一致、音节规范、且校订后的读音确实与源表不同，否则终止生成。

词组校订有个陷阱：**校订回单字默认读音的条目在 `phrase_data.go` 里不留痕迹**（读音字段被清空，运行时按单字补齐），光看生成数据分辨不出校订是生效了还是被丢弃了。根目录的 `TestPhraseOverridesApplied` 因此逐条比对 `Convert` 的输出，这才是唯一能锁住这类条目的地方；改 `applyPhraseOverrides` 时别把它删掉。

一/不 的变调、真异读、轻声书写差异、姓氏**有意不收**，理由写在 `phrase_overrides.txt` 头部与 README 的「词组人工校订」一节。

## 测试约定

- **核心包语句覆盖率要求 100%**，加分支时要一并补测试。
- **分配契约**：`TestConvertAllocations` 锁定整句转换 ≤ 2 次分配。动 `ConvertWith` 热路径时别引入新分配。
- `cmd/gendata` 的测试以 `cmd/gendata` 为工作目录运行，读源数据要用 `../../data/...`；根目录的测试用 `data/...`。
- 词组数据有两道独立验证，缺一不可：
  - `TestPhraseDictionaryAccuracy` 逐条核对单个词组（411944 条）。
  - `TestPhraseMatchesFullTableFMM` 拿随机拼接的句子与「对完整词表做最大匹配」的参照实现对比。单条核对**发现不了**词组连排时的跳转问题（「隆重庆祝」里的「重庆」），所以必须保留这道。
  - 这两道都拿源表当基准，因此都要先套上 `phraseOverrideReadings(t)`，否则 75 条被校订的词组会全部「对不上」。加新的源表比对测试时同理。
- 词组校订另有两道：`TestPhraseOverridesApplied`（端到端，唯一能锁住「校订回单字默认读音」的条目）与 `TestPhraseOverridesAreLoadBearing`（没有死条目）。
- `.golangci.yml` 对 `dict_data.go` / `phrase_data.go` 关掉了 dupl、lll、godot、goconst；`style.go` 关掉了 goconst（`styleNames` 本身就是风格名的唯一来源）。

## 性能现状

AMD Ryzen 5 5600X，26 字句子整句转换：`BenchmarkConvert` ~2.79µs，`BenchmarkConvertCharByChar` ~1.30µs，均为 2 次分配。

词组索引**惰性构建**（`sync.Once`），只用 `CharByChar` 的调用方零代价；建好后常驻约 3.2MB 堆。索引结构刻意做得紧凑：只存「首字→桶区间」「每桶第二个字」「每桶起点」三张表，桶内条数由相邻桶起点推出。早期版本用 `map[uint64]phraseBucket` 直接按前两字哈希是 8.98MB，别退回去。

README 里的性能数字改动后要同步更新。
