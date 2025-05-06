package app

import (
	"bookget/config"
	"bookget/pkg/downloader"
	"bookget/pkg/gohttp"
	"bookget/pkg/util"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ChinaNlc struct {
	dm     *downloader.DownloadManager
	ctx    context.Context
	cancel context.CancelFunc
	client *http.Client
	jar    *cookiejar.Jar

	rawUrl    string
	parsedUrl *url.URL
	savePath  string
	bookId    string

	body        []byte
	dataType    int //0=pdf,1=pic
	aid         string
	vectorBooks []string
}

func NewChinaNlc() *ChinaNlc {
	ctx, cancel := context.WithCancel(context.Background())
	dm := downloader.NewDownloadManager(ctx, cancel, config.Conf.MaxConcurrent)

	// 创建自定义 Transport 忽略 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, _ := cookiejar.New(nil)

	return &ChinaNlc{
		// 初始化字段
		dm:     dm,
		client: &http.Client{Timeout: config.Conf.Timeout * time.Second, Jar: jar, Transport: tr},
		ctx:    ctx,
		cancel: cancel,
		jar:    jar,
	}
}

func (r *ChinaNlc) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	r.rawUrl = sUrl
	r.parsedUrl, _ = url.Parse(sUrl)
	msg, err := r.Run()
	return map[string]interface{}{
		"type": "dzicnlib",
		"url":  sUrl,
		"msg":  msg,
	}, err
}

func (r *ChinaNlc) Run() (msg string, err error) {
	if strings.Contains(r.rawUrl, "OutOpenBook/Open") {
		r.body, _ = r.getBody(r.rawUrl)
		r.bookId = r.getBookId(string(r.body))
	} else {
		r.bookId = r.getBookId(r.rawUrl)
	}
	if r.bookId == "" {
		return "requested URL was not found.", err
	}
	return r.download()
}

func (r *ChinaNlc) getBookId(sUrl string) (bookId string) {
	var (
		// 预编译正则表达式
		identifierRegex = regexp.MustCompile(`identifier\s*=\s*["']([^"']+)["']`)
		fidRegex        = regexp.MustCompile(`fid=([A-Za-z0-9]+)`)
	)

	// 尝试第一种匹配模式
	if matches := identifierRegex.FindStringSubmatch(sUrl); matches != nil {
		return matches[1]
	}

	// 尝试第二种匹配模式
	if matches := fidRegex.FindStringSubmatch(sUrl); matches != nil {
		return matches[1]
	}

	// 默认返回空字符串
	return ""
}

func (r *ChinaNlc) download() (msg string, err error) {
	//单册PDF
	if strings.Contains(r.rawUrl, "OutOpenBook/OpenObjectBook") {
		//PDF
		r.savePath = CreateDirectory(r.parsedUrl.Host, r.bookId, "")
		v, _ := r.identifier(r.rawUrl)
		filename := v.Get("bid") + ".pdf"
		err = r.doPdfUrl(r.rawUrl, filename)
		return "", err
	}
	//单张图
	if strings.Contains(r.rawUrl, "OutOpenBook/OpenObjectPic") {
		r.savePath = CreateDirectory(r.parsedUrl.Host, r.bookId, "")
		canvases, err := r.getCanvases()
		if err != nil || canvases == nil {
			return "", err
		}
		log.Printf("  %d pages \n", len(canvases))
		r.do(canvases)
		return "", err
	}
	//对照阅读单册
	if strings.Contains(r.rawUrl, "OpenTwoObjectBook") {
		r.savePath = CreateDirectory(r.parsedUrl.Host, r.bookId, "")
		v, _ := r.identifier(r.rawUrl)
		filename := v.Get("bid") + ".pdf"
		pageUrl := fmt.Sprintf("%s://%s/OutOpenBook/OpenObjectBook?aid=%s&bid=%s", r.parsedUrl.Scheme, r.parsedUrl.Host,
			v.Get("aid"), v.Get("bid"))
		err = r.doPdfUrl(pageUrl, filename)

		filename = v.Get("cid") + ".pdf"
		pageUrl = fmt.Sprintf("%s://%s/OutOpenBook/OpenObjectBook?aid=%s&bid=%s", r.parsedUrl.Scheme, r.parsedUrl.Host,
			v.Get("aid"), v.Get("cid"))
		err = r.doPdfUrl(pageUrl, filename)
		return "", err
	}
	//多册/多图
	err = r.downloadForPDFs()
	if err != nil {
		fmt.Println(err)
		return "getVolumes", err
	}
	//矢量多册PDF
	r.downloadForOCR()
	return "", nil
}

func (r *ChinaNlc) do(imgUrls []string) (msg string, err error) {
	if imgUrls == nil {
		return
	}
	fmt.Println()
	referer := url.QueryEscape(r.rawUrl)
	size := len(imgUrls)
	var wg sync.WaitGroup
	q := QueueNew(int(config.Conf.Threads))
	for i, uri := range imgUrls {
		if uri == "" || !config.PageRange(i, size) {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		filename := sortId + config.Conf.FileExt
		dest := r.savePath + filename
		if FileExist(dest) {
			continue
		}
		imgUrl := uri
		log.Printf("Get %d/%d, URL: %s\n", i+1, size, imgUrl)
		wg.Add(1)
		q.Go(func() {
			defer wg.Done()
			opts := gohttp.Options{
				DestFile:    dest,
				Overwrite:   false,
				Concurrency: 1,
				CookieFile:  config.Conf.CookieFile,
				CookieJar:   r.jar,
				Headers: map[string]interface{}{
					"User-Agent": config.Conf.UserAgent,
					"Referer":    referer,
				},
			}
			gohttp.FastGet(r.ctx, imgUrl, opts)
			util.PrintSleepTime(config.Conf.Speed)
			fmt.Println()
		})
	}
	wg.Wait()
	fmt.Println()
	return "", err
}

func (r *ChinaNlc) downloadForPDFs() error {
	respVolume, err := r.getVolumes()
	if err != nil {
		return err
	}
	size := len(respVolume)
	for i, vol := range respVolume {
		if !config.VolumeRange(i) {
			continue
		}
		vid := fmt.Sprintf("%04d", i+1)
		//图片
		if strings.Contains(vol, "OpenObjectPic") {
			r.dataType = 1
			r.savePath = CreateDirectory(r.parsedUrl.Host, r.bookId, vid)
			canvases, err := r.getCanvases()
			if err != nil || canvases == nil {
				fmt.Println(err)
				continue
			}
			log.Printf(" %d/%d volume, %d pages \n", i+1, size, len(canvases))
			r.do(canvases)
		} else {
			//PDF
			r.savePath = CreateDirectory(r.parsedUrl.Host, r.bookId, "")
			log.Printf("Get %d/%d volume, URL: %s\n", i+1, size, vol)
			filename := vid + ".pdf"
			r.doPdfUrl(vol, filename)
		}
	}
	return nil
}

func (r *ChinaNlc) downloadForOCR() {
	if r.vectorBooks == nil {
		return
	}
	for i, vol := range r.vectorBooks {
		if !config.VolumeRange(i) {
			continue
		}
		vid := fmt.Sprintf("%04d", i+1)
		r.savePath = CreateDirectory(r.parsedUrl.Host, r.bookId, "ocr")
		log.Printf("Get %d/%d volume, URL: %s\n", i+1, len(r.vectorBooks), vol)
		filename := vid + ".pdf"
		r.doPdfUrl(vol, filename)
	}
	return
}

func (r *ChinaNlc) getVolumes() (volumes []string, err error) {
	r.body, err = r.getBody(r.rawUrl)
	if err != nil {
		return nil, err
	}
	text := util.SubText(string(r.body), "<div id=\"multiple\"", "id=\"catalogDiv\">")
	//取册数
	aUrls := regexp.MustCompile(`<a[^>]+class="a1"[^>].+href="([^"]+)OutOpenBook/([^"]+)"`).FindAllStringSubmatch(text, -1)
	for _, uri := range aUrls {
		pageUrl := fmt.Sprintf("%s://%s%sOutOpenBook/%s", r.parsedUrl.Scheme, r.parsedUrl.Host, uri[1], uri[2])
		volumes = append(volumes, pageUrl)
	}
	//
	aid := ""
	if volumes != nil {
		match := regexp.MustCompile(`aid=([^&]+)`).FindStringSubmatch(volumes[0])
		if match != nil {
			aid = match[1]
		}
	}

	//对照阅读
	twoUrls := regexp.MustCompile(`openTwoBookNew\('([^"']+)','([^"']+)'`).FindAllStringSubmatch(text, -1)
	if twoUrls != nil && aid != "" {
		r.vectorBooks = make([]string, 0, len(twoUrls))
		for _, uri := range twoUrls {
			pageUrl := fmt.Sprintf("%s://%s/OutOpenBook/OpenObjectBook?aid=%s&bid=%s", r.parsedUrl.Scheme, r.parsedUrl.Host, aid, uri[2])
			r.vectorBooks = append(r.vectorBooks, pageUrl)
		}
	}
	return volumes, err
}

func (r *ChinaNlc) doPdfUrl(sUrl, filename string) error {
	dest := r.savePath + filename
	if FileExist(dest) {
		return nil
	}
	v, err := r.identifier(sUrl)
	if err != nil {
		return err
	}
	tokenKey, timeKey, timeFlag := r.getToken(sUrl)

	//http://read.nlc.cn/menhu/OutOpenBook/getReaderNew
	//http://read.nlc.cn/menhu/OutOpenBook/getReaderRangeNew
	pdfUrl := fmt.Sprintf("%s://%s/menhu/OutOpenBook/getReaderNew?aid=%s&bid=%s&kime=%s&fime=%s",
		r.parsedUrl.Scheme, r.parsedUrl.Host, v.Get("aid"), v.Get("bid"), timeKey, timeFlag)

	opts := gohttp.Options{
		DestFile:    dest,
		Overwrite:   false,
		Concurrency: 1,
		CookieFile:  config.Conf.CookieFile,
		CookieJar:   r.jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
			"Referer":    "http://read.nlc.cn/static/webpdf/lib/WebPDFJRWorker.js",
			"Range":      "bytes=0-1",
			"myreader":   tokenKey,
		},
	}
	resp, err := gohttp.FastGet(r.ctx, pdfUrl, opts)
	if err != nil || resp.GetStatusCode() != 200 {
		fmt.Println(err)
	}
	util.PrintSleepTime(config.Conf.Speed)
	fmt.Println()
	return err
}

func (r *ChinaNlc) getCanvases() (canvases []string, err error) {
	v, err := r.identifier(r.rawUrl)
	if err != nil {
		return nil, err
	}
	bid, _ := strconv.ParseFloat(v.Get("bid"), 32)
	iBid := int(bid)
	//图片类型检测
	var pageUrl string
	aid := v.Get("aid")
	if aid == "495" || aid == "952" || aid == "467" || aid == "1080" {
		pageUrl = fmt.Sprintf("%s://%s/allSearch/openBookPic?id=%d&l_id=%s&indexName=data_%s",
			r.parsedUrl.Scheme, r.parsedUrl.Host, iBid, v.Get("lid"), aid)
	} else if aid == "022" {
		//中国记忆库图片 不用登录可以查看
		pageUrl = fmt.Sprintf("%s://%s/allSearch/openPic_noUser?id=%d&identifier=%s&indexName=data_%s",
			r.parsedUrl.Scheme, r.parsedUrl.Host, iBid, v.Get("did"), aid)
	} else {
		pageUrl = fmt.Sprintf("%s://%s/allSearch/openPic?id=%d&identifier=%s&indexName=data_%s",
			r.parsedUrl.Scheme, r.parsedUrl.Host, iBid, v.Get("did"), aid)
	}
	//
	bs, err := r.getBody(pageUrl)
	if err != nil {
		return
	}
	matches := regexp.MustCompile(`<img\s+src="(http|https)://(read|mylib).nlc.cn([^"]+)"`).FindAllSubmatch(bs, -1)
	for _, m := range matches {
		imgUrl := r.parsedUrl.Scheme + "://" + r.parsedUrl.Host + string(m[3])
		canvases = append(canvases, imgUrl)
	}
	return canvases, nil
}

func (r *ChinaNlc) identifier(sUrl string) (v url.Values, err error) {
	u, err := url.Parse(sUrl)
	if err != nil {
		return
	}
	m, _ := url.ParseQuery(u.RawQuery)
	if m["aid"] == nil || m["bid"] == nil {
		return nil, errors.New("error aid/bid")
	}
	return m, nil
}

func (r *ChinaNlc) getBody(apiUrl string) ([]byte, error) {
	referer := url.QueryEscape(apiUrl)
	cli := gohttp.NewClient(r.ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  r.jar,
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
	if resp.GetStatusCode() != 200 || bs == nil {
		return nil, errors.New(fmt.Sprintf("ErrCode:%d, %s", resp.GetStatusCode(), resp.GetReasonPhrase()))
	}
	return bs, nil
}

func (r *ChinaNlc) getToken(uri string) (tokenKey, timeKey, timeFlag string) {
	body, err := r.getBody(uri)
	if err != nil {
		log.Printf("Server unavailable: %s", err.Error())
		return
	}
	//<iframe id="myframe" name="myframe" src="" width="100%" height="100%" scrolling="no" frameborder="0" tokenKey="4ADAD4B379874C10864990817734A2BA" timeKey="1648363906519" timeFlag="1648363906519" sflag=""></iframe>
	params := regexp.MustCompile(`(tokenKey|timeKey|timeFlag)="([a-zA-Z0-9]+)"`).FindAllStringSubmatch(string(body), -1)
	//tokenKey := ""
	//timeKey := ""
	//timeFlag := ""
	for _, v := range params {
		if v[1] == "tokenKey" {
			tokenKey = v[2]
		} else if v[1] == "timeKey" {
			timeKey = v[2]
		} else if v[1] == "timeFlag" {
			timeFlag = v[2]
		}
	}
	return
}
