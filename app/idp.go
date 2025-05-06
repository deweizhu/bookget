package app

import (
	"bookget/config"
	"bookget/pkg/gohttp"
	"bookget/pkg/progressbar"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"regexp"
)

type Idp struct {
	dt  *DownloadTask
	bar *progressbar.ProgressBar
}

func NewIdp() *Idp {
	return &Idp{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Idp) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Idp) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r *Idp) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`uid=([A-Za-z0-9]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *Idp) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)

	canvases, err := r.getCanvases(r.dt.BookId, r.dt.Jar)
	if err != nil || canvases == nil {
		fmt.Println(err)
		return "requested URL was not found.", err
	}
	//不按卷下载，所有图片存一个目录
	r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
	sizeCanvases := len(canvases)
	fmt.Println()
	ext := ".jpg"
	r.bar = progressbar.Default(int64(sizeCanvases), "downloading")
	ctx := context.Background()
	for i, imgUrl := range canvases {
		if !config.PageRange(i, sizeCanvases) || imgUrl == "" {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		dest := r.dt.SavePath + sortId + ext
		cli := gohttp.NewClient(ctx, gohttp.Options{
			DestFile:   dest,
			CookieJar:  r.dt.Jar,
			CookieFile: config.Conf.CookieFile,
			Headers: map[string]interface{}{
				"User-Agent": config.Conf.UserAgent,
			},
		})
		_, err = cli.Get(imgUrl)
		if err != nil {
			log.Println(err)
			break
		}
		r.bar.Add(1)
	}
	return "", nil
}

func (r *Idp) do(imgUrls []string) (msg string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *Idp) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *Idp) getCanvases(sUrl string, jar *cookiejar.Jar) ([]string, error) {
	bs, err := r.getBody(sUrl, jar)
	if err != nil {
		return nil, err
	}
	//imageUrls[0] = "/image_IDP.a4d?type=loadRotatedMainImage;recnum=31305;rotate=0;imageType=_M";
	//imageRecnum[0] = "31305";
	m := regexp.MustCompile(`imageRecnum\[\d+\][ \S]?=[ \S]?"(\d+)";`).FindAllSubmatch(bs, -1)
	if m == nil {
		return []string{}, nil
	}
	canvases := make([]string, 0, len(m))
	for _, v := range m {
		id := string(v[1])
		imgUrl := fmt.Sprintf("%s://%s/image_IDP.a4d?type=loadRotatedMainImage;recnum=%s;rotate=0;imageType=_L",
			r.dt.UrlParsed.Scheme, r.dt.UrlParsed.Host, id)
		canvases = append(canvases, imgUrl)
	}
	return canvases, nil
}

func (r *Idp) getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error) {
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
	if resp.GetStatusCode() != 200 || bs == nil {
		return nil, errors.New(fmt.Sprintf("ErrCode:%d, %s", resp.GetStatusCode(), resp.GetReasonPhrase()))
	}
	return bs, nil
}

func (r *Idp) postBody(sUrl string, d []byte) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}
