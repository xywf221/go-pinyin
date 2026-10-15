// Package syllable 维护普通话无调音节的规范清单，供数据生成器与测试共同校验拼音数据。
//
// 该清单是数据质量的守门人：cmd/gendata 在生成时会用它拒绝任何非法音节，
// 测试则用它校验已生成的数据表。
package syllable

import "strings"

// standard 是普通话的全部无调音节（约 410 个）。
const standard = `a ai an ang ao ba bai ban bang bao bei ben beng bi bian biao bie bin bing bo bu ` +
	`ca cai can cang cao ce cen ceng cha chai chan chang chao che chen cheng chi chong chou chu chua ` +
	`chuai chuan chuang chui chun chuo ci cong cou cu cuan cui cun cuo ` +
	`da dai dan dang dao de dei deng di dia dian diao die ding diu dong dou du duan dui dun duo ` +
	`e ei en eng er fa fan fang fei fen feng fo fou fu ` +
	`ga gai gan gang gao ge gei gen geng gong gou gu gua guai guan guang gui gun guo ` +
	`ha hai han hang hao he hei hen heng hong hou hu hua huai huan huang hui hun huo ` +
	`ji jia jian jiang jiao jie jin jing jiong jiu ju juan jue jun ` +
	`ka kai kan kang kao ke ken keng kong kou ku kua kuai kuan kuang kui kun kuo ` +
	`la lai lan lang lao le lei leng li lia lian liang liao lie lin ling liu long lou lu luan lun luo lv lve ` +
	`ma mai man mang mao me mei men meng mi mian miao mie min ming miu mo mou mu ` +
	`na nai nan nang nao ne nei nen neng ni nian niang niao nie nin ning niu nong nou nu nuan nuo nv nve ` +
	`o ou pa pai pan pang pao pei pen peng pi pian piao pie pin ping po pou pu ` +
	`qi qia qian qiang qiao qie qin qing qiong qiu qu quan que qun ` +
	`ran rang rao re ren reng ri rong rou ru ruan rui run ruo ` +
	`sa sai san sang sao se sen seng sha shai shan shang shao she shei shen sheng shi shou shu shua shuai ` +
	`shuan shuang shui shun shuo si song sou su suan sui sun suo ` +
	`ta tai tan tang tao te teng ti tian tiao tie ting tong tou tu tuan tui tun tuo ` +
	`wa wai wan wang wei wen weng wo wu xi xia xian xiang xiao xie xin xing xiong xiu xu xuan xue xun ` +
	`ya yan yang yao ye yi yin ying yo yong you yu yuan yue yun ` +
	`za zai zan zang zao ze zei zen zeng zha zhai zhan zhang zhao zhe zhei zhen zheng zhi zhong zhou ` +
	`zhu zhua zhuai zhuan zhuang zhui zhun zhuo zi zong zou zu zuan zui zun zuo`

// rare 是非普通话标准音节，仅出现在方言、叹词或生僻读音中，但确实存在于拼音数据里。
const rare = `biang bong cei den din fiao hm hng kei len lo m n ng nia nun rua tei wong`

// valid 是 standard 与 rare 的并集。
var valid = func() map[string]bool {
	m := make(map[string]bool)
	for _, s := range strings.Fields(standard + " " + rare) {
		m[s] = true
	}
	return m
}()

// Valid 报告 base 是否是合法的无调音节。
//
// base 必须已经去掉声调、且 ü 已写成 v，例如 "zhong"、"nv"、"lve"。
func Valid(base string) bool {
	return valid[base]
}

// Count 返回规范清单中的音节总数。
func Count() int {
	return len(valid)
}
