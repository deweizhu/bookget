package app

import (
	"bookget/config"
	"bookget/pkg/gohttp"
	xhash "bookget/pkg/hash"
	"bookget/pkg/util"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type Downloader interface {
	Init(sUrl string) (msg string, err error)
	getBookId(sUrl string) (bookId string)
	download() (msg string, err error)
	do(imgUrls []string) (msg string, err error)
	getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error)
	getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error)
	getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error)
	postBody(sUrl string, d []byte) ([]byte, error)
}

type DownloadTask struct {
	Index     int
	Url       string
	UrlParsed *url.URL
	SavePath  string
	BookId    string
	Title     string
	VolumeId  string
	Param     map[string]interface{} //备用参数
	Jar       *cookiejar.Jar
}

type Volume struct {
	Title string
	Url   string
	Seq   int
}
type PartialVolumes struct {
	directory string
	Title     string
	volumes   []string
}

type PartialCanvases struct {
	directory string
	Title     string
	Canvases  []string
}

func getBookId(sUrl string) (bookId string) {
	if sUrl == "" {
		return ""
	}
	mh := xhash.NewMultiHasher()
	_, _ = io.Copy(mh, bytes.NewBuffer([]byte(sUrl)))
	bookId, _ = mh.SumString(xhash.QuickXorHash, false)
	return bookId
}

func getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error) {
	referer := url.QueryEscape(sUrl)
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
			"Referer":    referer,
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

func postBody(sUrl string, d []byte, jar *cookiejar.Jar) ([]byte, error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent":   config.Conf.UserAgent,
			"Content-Type": "application/x-www-form-urlencoded",
		},
		Body: d,
	})
	resp, err := cli.Post(sUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	return bs, err
}

func postJSON(sUrl string, d interface{}, jar *cookiejar.Jar) ([]byte, error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent":   config.Conf.UserAgent,
			"Content-Type": "application/json",
		},
		JSON: d,
	})
	resp, err := cli.Post(sUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	return bs, err
}

func FileExist(path string) bool {
	fi, err := os.Stat(path)
	if err == nil && fi.Size() > 0 {
		return true
	}
	return false
}

func CreateDirectory(domain, bookId, volumeId string) string {
	bookIdEncode := getBookId(bookId)
	domainNew := strings.ReplaceAll(domain, ":", "_")
	dirPath := config.Conf.SaveFolder + string(os.PathSeparator) + domainNew + "_" + bookIdEncode + string(os.PathSeparator)
	if volumeId != "" {
		dirPath += "vol." + volumeId + string(os.PathSeparator)
	}
	_ = os.MkdirAll(dirPath, os.ModePerm)
	return dirPath
}

func OpenWebBrowser(sUrl string, args []string) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		argv := []string{sUrl}
		if args != nil {
			argv = append(argv, args...)
		}
		if ret := util.OpenGUI(argv); ret == true {
			return
		}
	}()

	//WaitNewCookie
	fmt.Println("请使用 bookget-gui 浏览器，打开图书网址，完成「真人验证 / 登录用户」，然后 「刷新」 网页.")
	go func() {
		defer wg.Done()
		for i := 0; i < 3600*8; i++ {
			if FileExist(config.Conf.CookieFile) {
				break
			}
			time.Sleep(time.Second * 3)
		}
	}()

	wg.Wait()
}

func WaitNewCookie() {
	if FileExist(config.Conf.CookieFile) {
		return
	}
	var wg sync.WaitGroup
	wg.Add(1)
	fmt.Println("请使用 bookget-gui 浏览器，打开图书网址，完成「真人验证 / 登录用户」，然后 「刷新」 网页.")
	go func() {
		defer wg.Done()
		for i := 0; i < 3600*8; i++ {
			if FileExist(config.Conf.CookieFile) {
				break
			}
			util.PrintSleepTime(config.Conf.Speed)
		}
	}()
	wg.Wait()
}

func WaitNewCookieWithMsg(uri string) {
	_ = os.Remove(config.Conf.CookieFile)
	var wg sync.WaitGroup
	wg.Add(1)
	fmt.Println("请使用 bookget-gui 浏览器打开下面 URL，完成「真人验证 / 登录用户」，然后 「刷新」 网页.")
	fmt.Println(uri)
	go func() {
		defer wg.Done()
		for i := 0; i < 3600*8; i++ {
			if FileExist(config.Conf.CookieFile) {
				break
			}
			util.PrintSleepTime(config.Conf.Speed)
		}
	}()
	wg.Wait()
}

func IsChinaIP(jar *cookiejar.Jar) bool {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
			"Referer":    "http://ip-api.com/",
		},
	})
	resp, err := cli.Get("http://ip-api.com/json/?lang=zh-CN")
	if err != nil {
		return false
	}
	bs, _ := resp.GetBody()
	text := string(bs)
	if strings.Contains(text, "\"countryCode\":\"CN\"") {
		return true
	}
	return false
}
