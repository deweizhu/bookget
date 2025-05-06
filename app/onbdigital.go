package app

import (
	"bookget/config"
	"bookget/model/onbdigital"
	"bookget/pkg/gohttp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"sync"
)

type OnbDigital struct {
	dt *DownloadTask
}

func NewOnbDigital() *OnbDigital {
	return &OnbDigital{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *OnbDigital) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *OnbDigital) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r *OnbDigital) getBookId(sUrl string) (bookId string) {
	if m := regexp.MustCompile(`doc=([^&]+)`).FindStringSubmatch(sUrl); m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *OnbDigital) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)
	respVolume, err := r.getVolumes(r.dt.Url, r.dt.Jar)
	if err != nil {
		fmt.Println(err)
		return "getVolumes", err
	}
	r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
	for i, vol := range respVolume {
		if !config.VolumeRange(i) {
			continue
		}
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

func (r *OnbDigital) do(imgUrls []string) (msg string, err error) {
	if imgUrls == nil {
		return "", nil
	}
	size := len(imgUrls)
	fmt.Println()
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
		fmt.Println()
		log.Printf("Get %d/%d  %s\n", i+1, size, imgUrl)
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
				},
			}
			gohttp.FastGet(ctx, imgUrl, opts)
			fmt.Println()
		})
	}
	wg.Wait()
	fmt.Println()
	return "", err
}

func (r *OnbDigital) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	//刷新cookie
	_, err = r.getBody(sUrl, jar)
	if err != nil {
		return
	}
	volumes = append(volumes, sUrl)
	return volumes, nil
}

func (r *OnbDigital) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	apiUrl := "https://" + r.dt.UrlParsed.Host + "/OnbViewer/service/viewer/imageData?doc=" + r.dt.BookId + "&from=1&to=3000"
	bs, err := r.getBody(apiUrl, jar)
	if err != nil {
		return
	}
	var result = new(onbdigital.Response)
	if err = json.Unmarshal(bs, result); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	serverUrl := "https://" + r.dt.UrlParsed.Host + "/OnbViewer/image?"
	for _, m := range result.ImageData {
		imgUrl := serverUrl + m.QueryArgs + "&w=2400&q=70"
		canvases = append(canvases, imgUrl)
	}
	return canvases, err
}

func (r *OnbDigital) getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
		},
	})
	resp, err := cli.Get(sUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	if bs == nil {
		return nil, errors.New(fmt.Sprintf("ErrCode:%d, %s", resp.GetStatusCode(), resp.GetReasonPhrase()))
	}
	return bs, nil
}

func (r *OnbDigital) postBody(sUrl string, d []byte) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}
