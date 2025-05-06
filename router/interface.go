// router/interface.go
package router

import (
	"bookget/app"
	"bookget/config"
	"bookget/pkg/util"
	"errors"
	"strings"
	"sync"
)

type RouterInit interface {
	GetRouterInit(sUrl string) (map[string]interface{}, error)
}

var (
	Router = make(map[string]RouterInit)
	doInit sync.Once
)

// FactoryRouter 创建路由器的工厂函数
func FactoryRouter(siteID string, sUrl string) (map[string]interface{}, error) {
	// 自动检测逻辑
	if config.Conf.AutoDetect == 1 {
		siteID = "bookget"
	} else if config.Conf.AutoDetect == 2 || strings.Contains(sUrl, ".json") {
		siteID = "iiif.io"
	}

	if strings.Contains(sUrl, "tiles/infos.json") {
		siteID = "dzicnlib"
	}

	// 初始化路由器(线程安全)
	doInit.Do(func() {
		//[中国]国家图书馆
		Router["read.nlc.cn"] = app.NewChinaNlc()
		Router["mylib.nlc.cn"] = app.NewChinaNlc()
		Router["guji.nlc.cn"] = app.NewNlcGuji()

		//[中国]臺灣華文電子書庫
		Router["taiwanebook.ncl.edu.tw"] = app.NewHuawen()

		//[中国]香港中文大学图书馆
		Router["repository.lib.cuhk.edu.hk"] = app.NewCuhk()

		//[中国]香港科技大学图书馆
		Router["lbezone.hkust.edu.hk"] = app.NewUsthk()

		//[中国]洛阳市图书馆
		Router["111.7.82.29:8090"] = app.NewLuoyang()

		//[中国]温州市图书馆
		Router["oyjy.wzlib.cn"] = app.NewWzlib()
		Router["arcgxhpv7cw0.db.wzlib.cn"] = app.NewWzlib()

		//[中国]深圳市图书馆-古籍
		Router["yun.szlib.org.cn"] = app.NewSzLib()

		//[中国]广州大典
		Router["gzdd.gzlib.gov.cn"] = app.NewGzlib()
		Router["gzdd.gzlib.org.cn"] = app.NewGzlib()

		//[中国]天一阁博物院古籍数字化平台
		Router["gj.tianyige.com.cn"] = app.NewTianyige()

		//[中国]江苏高校珍贵古籍数字图书馆
		Router["jsgxgj.nju.edu.cn"] = app.NewNjuedu()

		//[中国]中华寻根网-国图
		Router["ouroots.nlc.cn"] = app.NewOuroots()

		//[中国]国家哲学社会科学文献中心
		Router["www.ncpssd.org"] = app.NewNcpssd()
		Router["www.ncpssd.cn"] = app.NewNcpssd()

		//[中国]山东中医药大学古籍数字图书馆
		Router["gjsztsg.sdutcm.edu.cn"] = app.NewSdutcm()

		//[中国]天津图书馆历史文献数字资源库
		Router["lswx.tjl.tj.cn:8001"] = app.NewTjlswx()

		//[中国]云南数字方志馆
		Router["dfz.yn.gov.cn"] = app.NewYndfz()

		//[中国]香港大学数字图书
		Router["digitalrepository.lib.hku.hk"] = app.NewHkulib()

		//[中国]山东省诸城市图书馆
		Router["124.134.220.209:8100"] = app.NewZhuCheng()
		//[中国]中央美术学院
		Router["dlibgate.cafa.edu.cn"] = app.NewCafaEdu()
		Router["dlib.cafa.edu.cn"] = app.NewCafaEdu()

		//抗日战争与中日关系文献数据平台
		Router["www.modernhistory.org.cn"] = app.NewWar1931()
		//}}} -----------------------------------------------------------------

		//---------------日本--------------------------------------------------
		//[日本]国立国会图书馆
		Router["dl.ndl.go.jp"] = app.NewNdlJP()

		//[日本]E国宝eMuseum
		Router["emuseum.nich.go.jp"] = app.NewEmuseum()

		//[日本]宫内厅书陵部（汉籍集览）
		Router["db2.sido.keio.ac.jp"] = app.NewKeio()

		//[日本]东京大学东洋文化研究所（汉籍善本资料库）
		Router["shanben.ioc.u-tokyo.ac.jp"] = app.NewUtokyo()

		//[日本]国立公文书馆（内阁文库）
		Router["www.digital.archives.go.jp"] = app.NewNationaljp()

		//[日本]东洋文库
		Router["dsr.nii.ac.jp"] = app.NewNiiac()

		//[日本]早稻田大学图书馆
		Router["archive.wul.waseda.ac.jp"] = app.NewWaseda()

		//[日本]国書数据库（古典籍）
		Router["kokusho.nijl.ac.jp"] = app.NewKokusho()

		//[日本]京都大学人文科学研究所 东方学数字图书博物馆
		Router["kanji.zinbun.kyoto-u.ac.jp"] = app.NewKyotou()

		//[日本]駒澤大学 电子贵重书库
		Router["repo.komazawa-u.ac.jp"] = app.NewIiifRouter()

		//[日本]关西大学图书馆
		Router["www.iiif.ku-orcas.kansai-u.ac.jp"] = app.NewIiifRouter()

		//[日本]庆应义塾大学图书馆
		Router["dcollections.lib.keio.ac.jp"] = app.NewIiifRouter()

		//[日本]国立历史民俗博物馆
		Router["khirin-a.rekihaku.ac.jp"] = app.NewKhirin()

		//[日本]市立米泽图书馆
		Router["www.library.yonezawa.yamagata.jp"] = app.NewYonezawa()
		Router["webarchives.tnm.jp"] = app.NewTnm()

		//[日本]龙谷大学
		Router["da.library.ryukoku.ac.jp"] = app.NewRyukoku()
		//}}} -----------------------------------------------------------------

		//{{{---------------美国、欧洲--------------------------------------------------
		//[美国]哈佛大学图书馆
		Router["iiif.lib.harvard.edu"] = app.NewHarvard()
		Router["listview.lib.harvard.edu"] = app.NewHarvard()
		Router["curiosity.lib.harvard.edu"] = app.NewHarvard()

		//[美国]hathitrust 数字图书馆
		Router["babel.hathitrust.org"] = app.NewHathitrust()

		//[美国]普林斯顿大学图书馆
		Router["catalog.princeton.edu"] = app.NewPrinceton()
		Router["dpul.princeton.edu"] = app.NewPrinceton()

		//[美国]国会图书馆
		Router["www.loc.gov"] = app.NewLoc()

		//[美国]斯坦福大学图书馆
		Router["searchworks.stanford.edu"] = app.NewStanford()

		//[美国]犹他州家谱
		Router["www.familysearch.org"] = app.NewFamilysearch()

		//[德国]柏林国立图书馆
		Router["digital.staatsbibliothek-berlin.de"] = app.NewOnbDigital()

		//[德国]巴伐利亞州立圖書館東亞數字資源庫
		Router["ostasien.digitale-sammlungen.de"] = app.NewSammlungen()
		Router["www.digitale-sammlungen.de"] = app.NewSammlungen()

		//[英国]牛津大学博德利图书馆
		Router["digital.bodleian.ox.ac.uk"] = app.NewOxacuk()

		//[英国]图书馆文本手稿
		Router["www.bl.uk"] = app.NewBluk()

		//Smithsonian Institution
		Router["ids.si.edu"] = app.NewSiEdu()
		Router["www.si.edu"] = app.NewSiEdu()
		Router["iiif.si.edu"] = app.NewSiEdu()
		Router["asia.si.edu"] = app.NewSiEdu()

		//[美國]柏克萊加州大學東亞圖書館
		Router["digicoll.lib.berkeley.edu"] = app.NewBerkeley()

		//奥地利国图
		Router["digital.onb.ac.at"] = app.NewOnbDigital()
		//}}} -----------------------------------------------------------------

		//{{{---------------其它--------------------------------------------------
		//國際敦煌項目
		Router["idp.nlc.cn"] = app.NewIdp()
		Router["idp.bl.uk"] = app.NewIdp()
		Router["idp.orientalstudies.ru"] = app.NewIdp()
		Router["idp.afc.ryukoku.ac.jp"] = app.NewIdp()
		Router["idp.bbaw.de"] = app.NewIdp()
		Router["idp.bnf.fr"] = app.NewIdp()
		Router["idp.korea.ac.kr"] = app.NewIdp()

		//[韩国]
		Router["kyudb.snu.ac.kr"] = app.NewKyudbSnu()
		Router["lod.nl.go.kr"] = app.NewLodNLGoKr()

		//高丽大学
		Router["kostma.korea.ac.kr"] = app.NewKorea()

		//俄罗斯图书馆
		Router["viewer.rsl.ru"] = app.NewRslRu()

		//越南汉喃古籍文献典藏数位计划
		Router["lib.nomfoundation.org"] = app.NewNomfoundation()

		//越南国家图书馆汉农图书馆
		Router["hannom.nlv.gov.vn"] = app.NewHannomNlv()
		//}}} -----------------------------------------------------------------

		Router["bookget"] = app.NewImageDownloader()
		Router["dzicnlib"] = app.NewDziCnLib()
		Router["iiif.io"] = app.NewIiifRouter()
	})

	// 检查路由器是否存在
	if _, ok := Router[siteID]; !ok {
		urlType := util.GetHeaderContentType(sUrl)
		if urlType == "json" {
			siteID = "iiif.io"
		} else if urlType == "bookget" {
			siteID = "bookget"
		}

		if _, ok := Router[siteID]; !ok {
			return nil, errors.New("unsupported URL: " + sUrl)
		}
	}

	return Router[siteID].GetRouterInit(sUrl)

}
