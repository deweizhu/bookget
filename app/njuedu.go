package app

import (
	"bookget/config"
	"bookget/model/njuedu"
	"bookget/pkg/util"
	"encoding/json"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
)

type Njuedu struct {
	dt     *DownloadTask
	typeId int
}

func NewNjuedu() *Njuedu {
	return &Njuedu{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Njuedu) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Njuedu) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r *Njuedu) getBookId(sUrl string) (bookId string) {
	if m := regexp.MustCompile(`(?i)bookId=([A-z0-9_-]+)`).FindStringSubmatch(sUrl); m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *Njuedu) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)
	r.typeId, err = r.getDetail(r.dt.BookId, r.dt.Jar)
	if err != nil {
		fmt.Println(err)
		return "getDetail", err
	}
	respVolume, err := r.getVolumes(r.dt.BookId, r.dt.Jar)
	if err != nil {
		fmt.Println(err)
		return "getVolumes", err
	}
	for i, vol := range respVolume {
		if !config.VolumeRange(i) {
			continue
		}
		vid := fmt.Sprintf("%04d", i+1)
		r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, vid)
		canvases, err := r.getCanvases(vol, r.dt.Jar)
		if err != nil || canvases == nil {
			fmt.Println(err)
			continue
		}
		log.Printf(" %d/%d volume, %d pages \n", i+1, len(respVolume), len(canvases))
		r.do(canvases)
	}
	return msg, err
}

func (r *Njuedu) do(dziUrls []string) (msg string, err error) {
	if dziUrls == nil {
		return "", err
	}
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
		fileName := fmt.Sprintf("%04d", i+1) + config.Conf.FileExt
		inputUri := r.dt.SavePath + val
		outfile := r.dt.SavePath + fileName
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

func (r *Njuedu) getDetail(bookId string, jar *cookiejar.Jar) (typeId int, err error) {
	apiUrl := "https://" + r.dt.UrlParsed.Host + "/portal/book/getBookById?bookId=" + bookId
	bs, err := getBody(apiUrl, jar)
	if err != nil {
		return 0, err
	}
	var result njuedu.Detail
	if err = json.Unmarshal(bs, &result); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	for _, v := range result.Data {
		if v.TypeId > 0 {
			typeId = v.TypeId
			break
		}
	}
	return typeId, err
}

func (r *Njuedu) getVolumes(bookId string, jar *cookiejar.Jar) (volumes []string, err error) {
	apiUrl := fmt.Sprintf("https://%s/portal/book/getMasterSlaveCatalogue?typeId=%d&bookId=%s", r.dt.UrlParsed.Host, r.typeId, bookId)
	bs, err := getBody(apiUrl, jar)
	if err != nil {
		return nil, err
	}
	var result njuedu.Catalog
	if err = json.Unmarshal(bs, &result); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	for _, d := range result.Data {
		volUrl := fmt.Sprintf("https://%s/portal/book/view?bookId=%s&typeId=%d", r.dt.UrlParsed.Host, d.BookId, r.typeId)
		volumes = append(volumes, volUrl)
	}
	return volumes, err

}

func (r *Njuedu) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	bs, err := getBody(sUrl, jar)
	if err != nil {
		return nil, err
	}
	var result njuedu.Response
	if err = json.Unmarshal(bs, &result); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	for _, id := range result.Data.Images {
		sortId := fmt.Sprintf("%s.json", id)
		canvases = append(canvases, sortId)
	}

	serverBase := "https://" + r.dt.UrlParsed.Host + result.Data.ServerBase
	jsonUrl := serverBase + "/tiles/infos.json"
	text := `{
    "Image": {
    "xmlns":    "https://schemas.microsoft.com/deepzoom/2009",
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
	bs, err = getBody(jsonUrl, jar)
	if err != nil {
		return nil, err
	}
	var resp njuedu.ResponseTiles
	if err = json.Unmarshal(bs, &resp); err != nil {
		return
	}
	if resp.Tiles == nil {
		return
	}
	ext := config.Conf.FileExt[1:]
	for key, item := range resp.Tiles {
		sortId := fmt.Sprintf("%s.json", key)
		dest := r.dt.SavePath + sortId
		serverUrl := fmt.Sprintf("%s/tiles/%s/", serverBase, key)
		jsonText := ""
		//ext := strings.ToLower(item.Extension)
		if item.TileSize.W == 0 {
			jsonText = fmt.Sprintf(text, serverUrl, ext, item.TileSize2.Width, item.Height, item.Width)
		} else {
			jsonText = fmt.Sprintf(text, serverUrl, ext, item.TileSize.W, item.Height, item.Width)
		}
		_ = os.WriteFile(dest, []byte(jsonText), os.ModePerm)
	}
	return canvases, nil
}
