package app

import (
	"bookget/config"
	"bookget/model/iiif"
	"bookget/pkg/downloader"
	"bookget/pkg/gohttp"
	"bookget/pkg/util"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"strconv"
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
	ctx       context.Context

	Canvases map[int]string
}

func NewDziCnLib() *DziCnLib {
	return &DziCnLib{
		// 初始化字段
		dt:  new(DownloadTask),
		ctx: context.Background(),
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
	log.Printf("Get %s\n", r.dt.Url)

	r.ServerUrl = r.getServerUri()
	if r.ServerUrl == "" {
		return "requested URL was not found.", err
	}
	r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
	r.Canvases, err = r.getCanvases(r.dt.Url, r.dt.Jar)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	return r.dezoomify()
}

func (r DziCnLib) dezoomify() (msg string, err error) {

	storePath := r.dt.SavePath
	referer := url.QueryEscape(r.dt.Url)
	args := []string{"-H", "Origin:" + referer,
		"-H", "Referer:" + referer,
		"-H", "User-Agent:" + config.Conf.UserAgent,
	}
	size := len(r.Canvases)
	downloader := downloader.NewIIIFDownloader(&config.Conf)
	err = downloader.SetDeepZoomTileFormat("{{.ServerURL}}/{{.Level}}/{{.X}}/{{.Y}}.{{.Format}}")
	if err != nil {
		return "[err=SetDeepZoomTileFormat]", err
	}
	// 有些不规范的JPG/jpg扩展名服务器，直接用配置文件指定
	ext := config.Conf.FileExt[1:]
	// 设置固定值
	downloader.DeepzoomTileFormat.FixedValues = map[string]interface{}{
		"Level":  0,
		"Format": ext,
	}

	for i, xml := range r.Canvases {
		if !config.PageRange(i, size) {
			continue
		}
		outputPath := storePath + fmt.Sprintf("%04d", i+1) + config.Conf.FileExt
		if FileExist(outputPath) {
			continue
		}

		err = downloader.DezoomifyWithContent(r.ctx, xml, outputPath, args)
		if err != nil {
			return "[err=downloader.Dezoomify]", err
		}
		util.PrintSleepTime(config.Conf.Speed)
	}
	return "", err
}

func (r DziCnLib) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r DziCnLib) getCanvases(apiUrl string, jar *cookiejar.Jar) (map[int]string, error) {
	//apiUrl := fmt.Sprintf("%s/tiles/infos.json", r.ServerHost)
	bs, err := r.getBody(apiUrl, jar)
	if err != nil {
		return nil, err
	}

	var result iiif.BaseResponse
	if err = json.Unmarshal(bs, &result); err != nil {
		return nil, err
	}
	if result.Tiles == nil {
		return nil, err
	}

	text := `
	<?xml version="1.0" encoding="UTF-8"?>
	<Image xmlns="http://schemas.microsoft.com/deepzoom/2009"
	  Url="%s"
	  Format="%s"
	  Overlap="1"
	  TileSize="%d"
	  >
	  <Size 
		Height="%d"
		Width="%d"
	  />
	</Image>
	`
	// 有些不规范的JPG/jpg扩展名服务器，直接用配置文件指定
	ext := config.Conf.FileExt[1:]
	r.Canvases = make(map[int]string, len(result.Tiles))
	for key, item := range result.Tiles {
		id, err := strconv.Atoi(key)
		if err != nil {
			return nil, err
		}
		serverUrl := fmt.Sprintf("%s/tiles/%s/", r.ServerUrl, key)
		// 有些不规范的JPG/jpg扩展名服务器
		// http://zggj.jslib.org.cn/medias/0118816-0002//tiles/infos.json
		// https://guji.sclib.cn/medias/557/tiles/infos.json
		//if r.Extention == "" {
		//	r.Extention = "." + strings.ToLower(item.Extension)
		//}

		if item.TileSize.W == 0 {
			r.Canvases[id] = fmt.Sprintf(text, serverUrl, ext, item.TileSize2.Width, item.Height, item.Width)
		} else {
			r.Canvases[id] = fmt.Sprintf(text, serverUrl, ext, item.TileSize.W, item.Height, item.Width)
		}
	}
	return r.Canvases, nil
}

func (r DziCnLib) getServerUri() string {
	return strings.Split(r.dt.Url, "/tiles/")[0]
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
