package app

import (
	"bookget/config"
	"bookget/model/iiif"
	"bookget/pkg/gohttp"
	"bookget/pkg/util"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"os"
	"sort"
	"strings"
)

//例如：
// 湖北古籍方志 http://gjpt.library.hb.cn:8991/f-medias/1840/tiles/infos.json
// 四川古籍 https://guji.sclib.org/medias/1122/tiles/infos.json
// 云南古籍 http://msq.ynlib.cn/medias2022/1001/tiles/infos.json

type DziCnLib struct {
	dt        *DownloadTask
	ServerUrl string
	Extention string
}

func NewDziCnLib() *DziCnLib {
	return &DziCnLib{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (d *DziCnLib) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := d.Run(sUrl)
	return map[string]interface{}{
		"type": "iiif",
		"url":  sUrl,
		"msg":  msg,
	}, err
}

// 自定义一个排序类型

func (r DziCnLib) Run(sUrl string) (msg string, err error) {

	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.BookId = getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r DziCnLib) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)

	r.ServerUrl = r.getServerUri()
	if r.ServerUrl == "" {
		return "requested URL was not found.", err
	}
	r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
	canvases, err := r.getCanvases(r.dt.Url, r.dt.Jar)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	log.Printf(" %d pages \n", len(canvases))
	return r.do(canvases)
}

func (r DziCnLib) do(dziUrls []string) (msg string, err error) {
	if dziUrls == nil {
		return "", err
	}
	storePath := r.dt.SavePath
	referer := url.QueryEscape(r.dt.Url)
	args := []string{"--dezoomer=deepzoom",
		"-H", "Origin:" + referer,
		"-H", "Referer:" + referer,
		"-H", "User-Agent:" + config.Conf.UserAgent,
	}
	size := len(dziUrls)
	for i, val := range dziUrls {
		if !config.PageRange(i, size) {
			continue
		}
		inputUri := storePath + val
		outfile := storePath + fmt.Sprintf("%04d", i+1) + r.Extention
		if FileExist(outfile) {
			continue
		}
		if ret := util.StartProcess(inputUri, outfile, args); ret == true {
			os.Remove(inputUri)
		}
		util.PrintSleepTime(config.Conf.Speed)
	}
	return "", err
}

func (r DziCnLib) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r DziCnLib) getCanvases(apiUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	//apiUrl := fmt.Sprintf("%s/tiles/infos.json", r.ServerHost)
	bs, err := r.getBody(apiUrl, jar)
	if err != nil {
		return
	}

	var result iiif.BaseResponse
	if err = json.Unmarshal(bs, &result); err != nil {
		return
	}
	if result.Tiles == nil {
		return
	}

	text := `{
    "Image": {
    "xmlns":    "http://schemas.microsoft.com/deepzoom/2009",
    "Url":      "%s",
    "Format":   "%s",
    "Overlap":  "1", 
	"MaxLevel": "0",
	"Separator": "/",
        "TileSize": "%d",
        "Size": {
            "Height": "%d",
            "Width":  "%d"
        }
    }
}
`
	// 有些不规范的JPG/jpg扩展名服务器，直接用配置文件指定
	ext := config.Conf.FileExt[1:]
	canvases = make([]string, 0, len(result.Tiles))
	for key, item := range result.Tiles {
		sortId := fmt.Sprintf("%s.json", key)
		dest := r.dt.SavePath + sortId
		serverUrl := fmt.Sprintf("%s/tiles/%s/", r.ServerUrl, key)
		// 有些不规范的JPG/jpg扩展名服务器
		// http://zggj.jslib.org.cn/medias/0118816-0002//tiles/infos.json
		// https://guji.sclib.cn/medias/557/tiles/infos.json
		//if r.Extention == "" {
		//	r.Extention = "." + strings.ToLower(item.Extension)
		//}

		jsonText := ""
		if item.TileSize.W == 0 {
			jsonText = fmt.Sprintf(text, serverUrl, ext, item.TileSize2.Width, item.Height, item.Width)
		} else {
			jsonText = fmt.Sprintf(text, serverUrl, ext, item.TileSize.W, item.Height, item.Width)
		}
		_ = os.WriteFile(dest, []byte(jsonText), os.ModePerm)
		canvases = append(canvases, sortId)
	}
	sort.Sort(util.SortByStr(canvases))
	return canvases, nil
}

func (r DziCnLib) getServerUri() string {
	return strings.Split(r.dt.Url, "/tiles/")[0]
	//m := regexp.MustCompile(`(?i)typeId=([A-z0-9_-]+)`).FindStringSubmatch(r.dt.UrlParsed.RawQuery)
	//typeId := 80
	//if m != nil {
	//	typeId, _ = strconv.Atoi(m[1])
	//}
	//match := regexp.MustCompile(`/([A-z0-9]+)/tiles/infos.json`).FindStringSubmatch(r.dt.Url)
	//if match == nil {
	//	return ""
	//}
	//bookId := match[1]
	//apiUrl := fmt.Sprintf("%s://%s/portal/book/view?bookId=%s&typeId=%d", r.dt.UrlParsed.Scheme,
	//	r.dt.UrlParsed.Host, bookId, typeId)
	//bs, err := r.getBody(apiUrl, r.dt.Jar)
	//if err != nil {
	//	return ""
	//}
	//var respServerBase iiif.ServerResponse
	//if err = json.Unmarshal(bs, &respServerBase); err != nil {
	//	return ""
	//}
	//return fmt.Sprintf("%s://%s%s", r.dt.UrlParsed.Scheme, r.dt.UrlParsed.Host, respServerBase.Data.ServerBase)
}

func (r DziCnLib) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
	referer := url.QueryEscape(apiUrl)
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
			"Referer":    referer,
		},
	})
	resp, err := cli.Get(apiUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	if resp.GetStatusCode() == 202 || bs == nil {
		return nil, errors.New(fmt.Sprintf("ErrCode:%d, %s", resp.GetStatusCode(), resp.GetReasonPhrase()))
	}
	return bs, nil
}
