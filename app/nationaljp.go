package app

import (
	"bookget/config"
	"bookget/pkg/gohttp"
	"context"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"regexp"
)

type Nationaljp struct {
	dt    *DownloadTask
	extId string
}

func NewNationaljp() *Nationaljp {
	return &Nationaljp{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Nationaljp) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Nationaljp) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	r.extId = "jp2"
	return r.download()
}

func (r *Nationaljp) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`(?i)BID=([A-z0-9_-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		return m[1]
	}
	return ""
}

func (r *Nationaljp) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", (r.dt.Index))
	log.Printf("Get %s  %s\n", name, r.dt.Url)

	respVolume, err := r.getVolumes()
	if err != nil {
		fmt.Println(err)
		return "getVolumes", err
	}
	r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
	for i, vol := range respVolume {
		if !config.VolumeRange(i) {
			continue
		}
		vid := fmt.Sprintf("%04d", i+1)
		fileName := vid + ".zip"
		dest := r.dt.SavePath + fileName
		if FileExist(dest) {
			continue
		}
		log.Printf(" %d/%d volume, %s\n", i+1, len(respVolume), r.extId)
		r.do(i+1, vol, dest)
		fmt.Println()
	}
	return msg, err
}

func (r *Nationaljp) do(index int, id, dest string) (msg string, err error) {
	apiUrl := "https://" + r.dt.UrlParsed.Host + "/acv/auto_conversion/download"
	data := fmt.Sprintf("DL_TYPE=%s&id_%d=%s", r.extId, index, id)
	ctx := context.Background()
	opts := gohttp.Options{
		DestFile:    dest,
		Overwrite:   false,
		Concurrency: 1,
		CookieFile:  config.Conf.CookieFile,
		CookieJar:   r.dt.Jar,
		Headers: map[string]interface{}{
			"User-Agent":   config.Conf.UserAgent,
			"Content-Type": "application/x-www-form-urlencoded",
		},
		Body: []byte(data),
	}
	_, err = gohttp.Post(ctx, apiUrl, opts)
	return "", err
}

func (r *Nationaljp) getVolumes() (volumes []string, err error) {
	apiUrl := fmt.Sprintf("https://%s/DAS/meta/listPhoto?LANG=default&BID=%s&ID=&NO=&TYPE=dljpeg&DL_TYPE=jpeg", r.dt.UrlParsed.Host, r.dt.BookId)
	bs, err := getBody(apiUrl, nil)
	if err != nil {
		return
	}
	//<input type="checkbox" class="check" name="id_2" posi="2" value="M2016092111023960474"
	//取册数
	matches := regexp.MustCompile(`<input[^>]+posi=["']([0-9]+)["'][^>]+value=["']([A-Za-z0-9]+)["']`).FindAllStringSubmatch(string(bs), -1)
	if matches == nil {
		return
	}
	iLen := len(matches)
	for _, match := range matches {
		//跳过全选复选框
		if iLen > 1 && (match[1] == "0" || match[2] == "") {
			continue
		}
		volumes = append(volumes, match[2])
	}
	return volumes, nil
}
