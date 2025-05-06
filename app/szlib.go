package app

import (
	"bookget/config"
	"bookget/model/szLib"
	"bookget/pkg/gohttp"
	"bookget/pkg/util"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

type SzLib struct {
	dt *DownloadTask
}

func NewSzLib() *SzLib {
	return &SzLib{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *SzLib) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *SzLib) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r *SzLib) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`book_id=([A-z\d]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *SzLib) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)

	respVolume, err := r.getVolumes(r.dt.Url)
	if err != nil {
		fmt.Println(err)
		return "getVolumes", err
	}
	sizeVol := len(respVolume.Volumes)
	for i, vol := range respVolume.Volumes {
		if !config.VolumeRange(i) {
			continue
		}
		fmt.Printf("\r Test volume %d ... ", i+1)
		if sizeVol == 1 {
			r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
		} else {
			vid := fmt.Sprintf("%04d", i+1)
			r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, vid)
		}

		canvases, err := r.getCanvases(vol)
		if err != nil || canvases == nil {
			fmt.Println(err)
			continue
		}
		fmt.Println()
		log.Printf(" %d/%d volume, %d pages \n", i+1, sizeVol, len(canvases))
		r.do(canvases)
	}
	return "", nil
}

func (r *SzLib) do(imgUrls []string) (msg string, err error) {
	if imgUrls == nil {
		return "图片URLs为空", errors.New("imgUrls is nil")
	}
	size := len(imgUrls)
	fmt.Println()
	var wg sync.WaitGroup
	q := QueueNew(int(config.Conf.Threads))
	for i, uri := range imgUrls {
		if uri == "" || !config.PageRange(i, size) {
			continue
		}
		ext := util.FileExt(uri)
		sortId := fmt.Sprintf("%04d", i+1)
		filename := sortId + ext
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
	return "", nil
}

func (r *SzLib) getVolumes(sUrl string) (*szLib.ResultVolumes, error) {
	apiUrl := fmt.Sprintf("https://%s/stgj2021/book_view/%s", r.dt.UrlParsed.Host, r.dt.BookId)
	bs, err := r.getBody(apiUrl)
	if err != nil {
		return nil, err
	}
	var rstVolumes = new(szLib.ResultVolumes)
	if err = json.Unmarshal(bs, rstVolumes); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return nil, err
	}
	return rstVolumes, err
}

func (r *SzLib) getCanvases(vol szLib.Directory) ([]string, error) {
	p1, err := r.getSinglePage(r.dt.BookId, vol.Volume, vol.Children[0].Page)
	pos := strings.LastIndex(p1, "/")
	urlPre := p1[:pos]
	ext := util.FileExt(p1)
	canvases := make([]string, 0, len(vol.Children))
	for _, child := range vol.Children {
		imgUrl := fmt.Sprintf("%s/%s%s", urlPre, child.Page, ext)
		canvases = append(canvases, imgUrl)
	}
	return canvases, err
}

func (r *SzLib) getBody(sUrl string) ([]byte, error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  r.dt.Jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
		},
	})
	resp, err := cli.Get(sUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	if resp.GetStatusCode() != 200 || bs == nil {
		return nil, errors.New(fmt.Sprintf("ErrCode:%d, %s", resp.GetStatusCode(), resp.GetReasonPhrase()))
	}
	return bs, nil
}

func (r *SzLib) postBody(sUrl string, d []byte) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (r *SzLib) getSinglePage(bookId string, volumeId string, page string) (string, error) {
	sUrl := fmt.Sprintf("https://%s/stgj2021/book_page/%s/%s/%s", r.dt.UrlParsed.Host, bookId, volumeId, page)
	bs, err := r.getBody(sUrl)
	if err != nil {
		return "", err
	}
	rstPage := new(szLib.ResultPage)
	if err = json.Unmarshal(bs, rstPage); err != nil {
		return "", err
	}
	return rstPage.BookImageUrl + rstPage.PicInfo.Path, nil
}
