package app

import (
	"bookget/config"
	"bookget/model/loc"
	"bookget/pkg/gohttp"
	"bookget/pkg/util"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

type Loc struct {
	dt         *DownloadTask
	xmlContent []byte
	tmpFile    string
}

func NewLoc() *Loc {
	return &Loc{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Loc) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Loc) Run(sUrl string) (msg string, err error) {

	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl

	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r *Loc) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`item/([A-Za-z0-9]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *Loc) download() (msg string, err error) {
	apiUrl := fmt.Sprintf("https://www.loc.gov/item/%s/?fo=json", r.dt.BookId)
	r.xmlContent, err = r.getBody(apiUrl, r.dt.Jar)
	if err != nil || r.xmlContent == nil {
		return "requested URL was not found.", err
	}
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)

	respVolume, err := r.getVolumes(r.dt.Url, r.dt.Jar)
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
	return "", nil
}

func (r *Loc) do(imgUrls []string) (msg string, err error) {
	if imgUrls == nil {
		return
	}
	fmt.Println()
	referer := url.QueryEscape(r.dt.Url)
	size := len(imgUrls)

	var wg sync.WaitGroup
	q := QueueNew(int(config.Conf.Threads))
	for i, uri := range imgUrls {
		if uri == "" || !config.PageRange(i, size) {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		filename := sortId + config.Conf.FileExt
		dest := r.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		imgUrl := uri
		log.Printf("Get %d/%d, %s\n", i+1, size, imgUrl)
		wg.Add(1)
		q.Go(func() {
			defer wg.Done()
			ctx := context.Background()
			opts := gohttp.Options{
				DestFile:    dest,
				Overwrite:   false,
				Concurrency: 1,
				CookieFile:  config.Conf.CookieFile,
				CookieJar:   r.dt.Jar,
				Headers: map[string]interface{}{
					"User-Agent": config.Conf.UserAgent,
					"Referer":    referer,
				},
			}
			for k := 0; k < config.Conf.Retry; k++ {
				_, err := gohttp.FastGet(ctx, imgUrl, opts)
				if err == nil {
					break
				}
			}
			util.PrintSleepTime(config.Conf.Speed)
			fmt.Println()
		})
	}
	wg.Wait()
	fmt.Println()
	return "", err
}

func (r *Loc) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	var manifests = new(loc.ManifestsJson)
	if err = json.Unmarshal(r.xmlContent, manifests); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	//一本书有N卷
	for _, resource := range manifests.Resources {
		volumes = append(volumes, resource.Url)
	}
	return volumes, nil
}

func (r *Loc) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	var manifests = new(loc.ManifestsJson)
	if err = json.Unmarshal(r.xmlContent, manifests); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}

	for _, resource := range manifests.Resources {
		if resource.Url != sUrl {
			continue
		}
		for _, file := range resource.Files {
			//每页有6种下载方式
			imgUrl, ok := r.getImagePage(file)
			if ok {
				canvases = append(canvases, imgUrl)
			}
		}
	}
	return canvases, nil
}
func (r *Loc) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
	referer := r.dt.Url
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
			"Referer":    referer,
			"authority":  "www.loc.gov",
			"origin":     "https://www.loc.gov",
		},
	})
	resp, err := cli.Get(apiUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	if resp.GetStatusCode() != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("ErrCode:%d, %s", resp.GetStatusCode(), resp.GetReasonPhrase()))
	}
	return bs, nil
}
func (r *Loc) getImagePage(fileUrls []loc.ImageFile) (downloadUrl string, ok bool) {
	for _, f := range fileUrls {
		if config.Conf.FileExt == ".jpg" && f.Mimetype == "image/jpeg" {
			if strings.Contains(f.Url, "full/pct:100/") {
				if config.Conf.Format != "" {
					downloadUrl = regexp.MustCompile(`full/pct:(.+)`).ReplaceAllString(f.Url, config.Conf.Format)
				} else {
					downloadUrl = f.Url
				}
				ok = true
				break
			}
		} else if f.Mimetype != "image/jpeg" {
			downloadUrl = f.Url
			ok = true
			break
		}
	}
	return
}
