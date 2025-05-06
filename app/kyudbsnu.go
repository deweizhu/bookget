package app

import (
	"bookget/config"
	"bookget/pkg/gohttp"
	"bookget/pkg/util"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type KyudbSnu struct {
	dt     *DownloadTask
	itemId string
	entry  string
}

func NewKyudbSnu() *KyudbSnu {
	return &KyudbSnu{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *KyudbSnu) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *KyudbSnu) Run(sUrl string) (msg string, err error) {

	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl

	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	r.entry = r.getEntryPage(sUrl)
	r.itemId = r.getItemId(sUrl)
	return r.download()
}

func (r *KyudbSnu) getEntryPage(sUrl string) (entry string) {
	if strings.Contains(sUrl, "book/view.do") {
		entry = "bookview"
	} else if strings.Contains(sUrl, "rendererImg.do") {
		entry = "renderer"
	}
	return entry
}

func (r *KyudbSnu) getItemId(sUrl string) (itemId string) {
	m := regexp.MustCompile(`item_cd=([A-z0-9_-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		itemId = m[1]
	}
	return itemId
}

func (r *KyudbSnu) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`(?i)book_cd=([A-z0-9_-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *KyudbSnu) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)
	bs, err := r.getBody(r.dt.Url, r.dt.Jar)
	if err != nil || bs == nil {
		return "requested URL was not found.", err
	}
	//PDF
	if bytes.Contains(bs, []byte("name=\"mfpdf_link\"")) {
		r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
		canvases, err := r.getPdfUrls(r.dt.Url)
		if err != nil || canvases == nil {
			return "requested URL was not found.", err
		}
		log.Printf(" %d volumes \n", len(canvases))
		r.doPdf(canvases)
		return "", nil
	}
	//图片
	if r.itemId == "" && r.entry == "renderer" {
		match := regexp.MustCompile(`item_cd=([A-z0-9_-]+)`).FindSubmatch(bs)
		if match == nil {
			return "requested URL was not found.", err
		}
		r.itemId = string(match[1])
	}
	respVolume, err := r.getVolumes(r.dt.Url, r.dt.Jar)
	if err != nil {
		return "getVolumes", err
	}
	for i, vol := range respVolume {
		if !config.VolumeRange(i) {
			continue
		}
		r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, vol)
		canvases, err := r.getCanvases(vol, r.dt.Jar)
		if err != nil || canvases == nil {
			continue
		}
		log.Printf(" %d/%d volume, %d pages \n", i+1, len(respVolume), len(canvases))
		r.do(canvases)
	}
	return "", nil
}

func (r *KyudbSnu) do(imgUrls []string) (msg string, err error) {
	if imgUrls == nil {
		return
	}
	fmt.Println()
	referer := fmt.Sprintf("%s://%s/pf01/rendererImg.do", r.dt.UrlParsed.Scheme, r.dt.UrlParsed.Host)
	size := len(imgUrls)
	ctx := context.Background()
	for i, uri := range imgUrls {
		if !config.PageRange(i, size) {
			continue
		}
		if uri == "" {
			continue
		}
		ext := util.FileExt(uri)
		sortId := fmt.Sprintf("%04d", i+1)
		log.Printf("Get %d/%d page, URL: %s\n", i+1, len(imgUrls), uri)
		filename := sortId + ext
		dest := r.dt.SavePath + filename
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
		_, err = gohttp.FastGet(ctx, uri, opts)
		if err != nil {
			fmt.Println(err)
			break
		}
	}
	fmt.Println()
	return "", err
}

func (r *KyudbSnu) doPdf(imgUrls []string) (msg string, err error) {
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
		filename := sortId + ".pdf"
		dest := r.dt.SavePath + filename
		if FileExist(dest) {
			continue
		}
		imgUrl := uri
		log.Printf("Get %d/%d, URL: %s\n", i+1, size, imgUrl)
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
			gohttp.FastGet(ctx, imgUrl, opts)
			util.PrintSleepTime(config.Conf.Speed)
			fmt.Println()
		})
	}
	wg.Wait()
	fmt.Println()
	return "", err
}

func (r *KyudbSnu) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	d := map[string]interface{}{
		"item_cd":       r.itemId,
		"book_cd":       r.dt.BookId,
		"vol_no":        "",
		"page_no":       "",
		"imgFileNm":     "",
		"tbl_conts_seq": "",
		"mokNm":         "",
		"add_page_no":   "",
	}
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent":   config.Conf.UserAgent,
			"Referer":      url.PathEscape(sUrl),
			"Content-Type": "application/x-www-form-urlencoded",
		},
		FormParams: d,
	})
	resp, err := cli.Post(fmt.Sprintf("%s://%s/pf01/rendererImg.do", r.dt.UrlParsed.Scheme, r.dt.UrlParsed.Host))
	bs, err := resp.GetBody()
	if bs == nil || err != nil {
		return nil, err
	}
	matches := regexp.MustCompile(`<option\s+value=["']([A-z0-9]+)["']`).FindAllSubmatch(bs, -1)
	if matches == nil {
		err = errors.New("requested URL was not found.")
		return nil, err
	}
	for _, m := range matches {
		volumes = append(volumes, string(m[1]))
	}
	return volumes, nil
}

func (r *KyudbSnu) getCanvases(vol string, jar *cookiejar.Jar) (canvases []string, err error) {
	sUrl := fmt.Sprintf("%s://%s/pf01/rendererImg.do", r.dt.UrlParsed.Scheme, r.dt.UrlParsed.Host)
	d := map[string]interface{}{
		"item_cd": r.itemId,
		"book_cd": r.dt.BookId,
		"vol_no":  vol,
		"page_no": "",
		"tool":    "1",
	}
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent":   config.Conf.UserAgent,
			"Referer":      url.PathEscape(sUrl),
			"Content-Type": "application/x-www-form-urlencoded",
		},
		FormParams: d,
	})
	resp, err := cli.Post(sUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	if bs == nil {
		err = errors.New(resp.GetReasonPhrase())
		return nil, err
	}
	var fromPage string
	m := regexp.MustCompile(`first_page_no\s+=\s+['"]([A-z0-9]+)['"];`).FindSubmatch(bs)
	if m != nil {
		fromPage = string(m[1])
	}
	var pageId string
	m = regexp.MustCompile(`imgFileNm\s+=\s+['"]([^"']+)['"]`).FindSubmatch(bs)
	if m != nil {
		pageId = string(m[1])
	}
	imgFileNm := filepath.Base(pageId)
	matches := regexp.MustCompile(`onclick="fn_goPageJumpWithMokIdxClear\('([A-z0-9]+)'\);">([A-z0-9]+)</a>`).FindAllSubmatch(bs, -1)
	_fromPage := vol + "_" + fromPage
	for _, match := range matches {
		_page := vol + "_" + string(match[1])
		_imgFileNm := strings.ReplaceAll(imgFileNm, _fromPage, _page)
		_pageId := strings.ReplaceAll(pageId, _fromPage, _page)
		//imgUrl := fmt.Sprintf("%s://%s/ImageDown.do?imgFileNm=%s&path=%s", r.dt.UrlParsed.Scheme, r.dt.UrlParsed.Host, _imgFileNm, _pageId)
		imgUrl := fmt.Sprintf("%s://%s/ImageServlet.do?imgFileNm=%s&path=%s", r.dt.UrlParsed.Scheme, r.dt.UrlParsed.Host, _imgFileNm, _pageId)
		canvases = append(canvases, imgUrl)
	}
	return canvases, nil
}

func (r *KyudbSnu) getPdfUrls(sUrl string) (canvases []string, err error) {
	type Response struct {
		RESULT  string `json:"RESULT"`
		VolList []struct {
			ISPDF      string      `json:"IS_PDF"`
			RELADDIMG  interface{} `json:"REL_ADD_IMG"`
			CALLNUM    string      `json:"CALL_NUM"`
			RELMAINIMG interface{} `json:"REL_MAIN_IMG"`
			TOTALCNT   int         `json:"TOTAL_CNT"`
			RNUM       int         `json:"RNUM"`
			ORITIT     string      `json:"ORI_TIT"`
			BOOKCD     string      `json:"BOOK_CD"`
			BOOKNM     interface{} `json:"BOOK_NM"`
			ITEMCD     string      `json:"ITEM_CD"`
			VOLNO      string      `json:"VOL_NO"`
		} `json:"volList"`
	}

	d := []byte("book_cd=" + r.dt.BookId)
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  r.dt.Jar,
		Headers: map[string]interface{}{
			"User-Agent":   config.Conf.UserAgent,
			"Content-Type": "application/x-www-form-urlencoded",
			"Referer":      sUrl,
		},
		Body: d,
	})
	resp, err := cli.Post("https://" + r.dt.UrlParsed.Host + "/ajax/book/mfPdfList.do")
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()

	var res Response
	if err = json.Unmarshal(bs, &res); err != nil {
		return nil, err
	}
	for _, v := range res.VolList {
		if v.ISPDF == "Y" {
			pdfUrl := fmt.Sprintf("https://%s/book/mfPdf.do?book_cd=%s&vol_no=%s", r.dt.UrlParsed.Host, v.BOOKCD, v.VOLNO)
			canvases = append(canvases, pdfUrl)
		}
	}
	return canvases, nil
}

func (r *KyudbSnu) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
		},
	})
	resp, err := cli.Get(apiUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	if bs == nil {
		err = errors.New(resp.GetReasonPhrase())
		return nil, err
	}
	return bs, nil
}
