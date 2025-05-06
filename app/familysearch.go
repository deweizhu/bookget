package app

import (
	"bookget/config"
	"bookget/model/family"
	"bookget/pkg/gohttp"
	"bookget/pkg/util"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
)

type Familysearch struct {
	dt          *DownloadTask
	apiUrl      string
	urlType     int
	dziTemplate string
	userAgent   string
	baseUrl     string
	sgBaseUrl   string
}

func NewFamilysearch() *Familysearch {
	return &Familysearch{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Familysearch) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Familysearch) Run(sUrl string) (msg string, err error) {

	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl

	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.baseUrl, r.sgBaseUrl, _ = r.getBaseUrl(r.dt.Url)
	r.dt.Jar, _ = cookiejar.New(nil)
	//  "https://www.familysearch.org/search/filmdata/filmdatainfo"
	//r.apiUrl = r.dt.UrlParsed.Scheme + "://" + r.dt.UrlParsed.Host + "/search/filmdata/filmdatainfo"
	//https://www.familysearch.org/search/filmdatainfo/image-data
	r.apiUrl = r.dt.UrlParsed.Scheme + "://" + r.dt.UrlParsed.Host + "/search/filmdatainfo/image-data"
	WaitNewCookie()
	return r.download()
}

func (r *Familysearch) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`(?i)ark:/(?:[A-z0-9-_:]+)/([A-z\d-_:]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *Familysearch) getBaseUrl(sUrl string) (baseUrl, sgBaseUrl string, err error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  r.dt.Jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
			"Accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		},
	})
	resp, err := cli.Get(sUrl)
	if err != nil {
		return
	}
	bs, _ := resp.GetBody()

	//SERVER_DATA.baseUrl = "https://www.familysearch.org";
	m := regexp.MustCompile(`SERVER_DATA.baseUrl\s=\s"([^"]+)"`).FindSubmatch(bs)
	baseUrl = string(m[1])

	// SERVER_DATA.sgBaseUrl = "https://sg30p0.familysearch.org"
	m = regexp.MustCompile(`SERVER_DATA.sgBaseUrl\s=\s"([^"]+)"`).FindSubmatch(bs)
	sgBaseUrl = string(m[1])
	return
}

func (r *Familysearch) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)
	r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
	var canvases []string
	imageData, err := r.getImageData(r.dt.Url)
	if err != nil {
		return "", err
	}
	canvases, err = r.getFilmData(r.dt.Url, imageData)
	if err != nil {
		return "", err
	}
	size := len(canvases)
	log.Printf(" %d pages.\n", size)

	r.do(canvases)
	return "", nil
}

func (r *Familysearch) do(iiifUrls []string) (msg string, err error) {
	if iiifUrls == nil {
		return
	}
	referer := url.QueryEscape(r.dt.Url)

	cookies := gohttp.ReadCookieFile(config.Conf.CookieFile)

	sid := r.getSessionId()
	args := []string{"--dezoomer=deepzoom",
		"-H", "authority:www.familysearch.org",
		"-H", "Authorization:" + sid,
		"-H", "referer:" + referer,
		"-H", "User-Agent:" + config.Conf.UserAgent,
		"-H", "cookie:" + cookies,
	}
	size := len(iiifUrls)
	for i, uri := range iiifUrls {
		if uri == "" || !config.PageRange(i, size) {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		dest := r.dt.SavePath + sortId + config.Conf.FileExt
		if FileExist(dest) {
			continue
		}
		log.Printf("Get %d/%d  %s\n", i+1, size, uri)
		util.StartProcess(uri, dest, args)
		util.PrintSleepTime(config.Conf.Speed)
	}
	return "", err
}

func (r *Familysearch) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *Familysearch) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	panic("implement me")
}

func (r *Familysearch) getImageData(sUrl string) (imageData family.ImageData, err error) {
	type ReqData struct {
		Type string `json:"type"`
		Args struct {
			ImageURL string `json:"imageURL"`
			State    struct {
				ImageOrFilmUrl     string `json:"imageOrFilmUrl"`
				ViewMode           string `json:"viewMode"`
				SelectedImageIndex int    `json:"selectedImageIndex"`
			} `json:"state"`
			Locale string `json:"locale"`
		} `json:"args"`
	}

	type Response struct {
		ImageURL string `json:"imageURL"`
		ArkId    string `json:"arkId"`
		DgsNum   string `json:"dgsNum"`
		Meta     struct {
			SourceDescriptions []struct {
				Id     string `json:"id"`
				About  string `json:"about"`
				Titles []struct {
					Value string `json:"value"`
					Lang  string `json:"lang,omitempty"`
				} `json:"titles"`
				Identifiers struct {
					HttpGedcomxOrgPrimary []string `json:"http://gedcomx.org/Primary"`
				} `json:"identifiers"`
				Descriptor struct {
					Resource string `json:"resource"`
				} `json:"descriptor,omitempty"`
			} `json:"sourceDescriptions"`
		} `json:"meta"`
	}

	var data = ReqData{}
	data.Type = "image-data"
	data.Args.ImageURL = sUrl
	data.Args.State.ImageOrFilmUrl = ""
	data.Args.State.ViewMode = "i"
	data.Args.State.SelectedImageIndex = -1
	data.Args.Locale = "zh"

	bs, err := r.postJson(r.apiUrl, r)
	if err != nil {
		return
	}
	var resultError family.ResultError
	if err = json.Unmarshal(bs, &resultError); resultError.Error.StatusCode != 0 {
		msg := fmt.Sprintf("StatusCode: %d, Message: %s", resultError.Error.StatusCode, resultError.Error.Message)
		err = errors.New(msg)
		return
	}
	resp := Response{}
	if err = json.Unmarshal(bs, &resp); err != nil {
		return
	}
	imageData.DgsNum = resp.DgsNum
	imageData.ImageURL = resp.ImageURL
	for _, description := range resp.Meta.SourceDescriptions {
		if strings.Contains(description.About, "platform/records/waypoints") {
			imageData.WaypointURL = description.About
			break
		}
	}
	return imageData, nil
}

func (r *Familysearch) getFilmData(sUrl string, imageData family.ImageData) (canvases []string, err error) {
	type ReqData struct {
		Type string `json:"type"`
		Args struct {
			DgsNum string `json:"dgsNum"`
			State  struct {
				I                  string `json:"i"`
				Cat                string `json:"cat"`
				ImageOrFilmUrl     string `json:"imageOrFilmUrl"`
				CatalogContext     string `json:"catalogContext"`
				ViewMode           string `json:"viewMode"`
				SelectedImageIndex int    `json:"selectedImageIndex"`
			} `json:"state"`
			Locale    string `json:"locale"`
			SessionId string `json:"sessionId"`
			LoggedIn  bool   `json:"loggedIn"`
		} `json:"args"`
	}

	u, err := url.Parse(imageData.ImageURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	var data = ReqData{}
	data.Type = "film-data"
	data.Args.DgsNum = imageData.DgsNum
	data.Args.State.CatalogContext = q.Get("cat")
	data.Args.State.Cat = q.Get("cat")
	data.Args.State.ImageOrFilmUrl = u.Path
	data.Args.State.ViewMode = "i"
	data.Args.State.SelectedImageIndex = -1
	data.Args.Locale = "zh"
	data.Args.LoggedIn = true
	data.Args.SessionId = r.getSessionId()

	type Response struct {
		DgsNum             string      `json:"dgsNum"`
		Images             []string    `json:"images"`
		PreferredCatalogId string      `json:"preferredCatalogId"`
		Type               string      `json:"type"`
		WaypointCrumbs     interface{} `json:"waypointCrumbs"`
		WaypointURL        interface{} `json:"waypointURL"`
		Templates          struct {
			DasTemplate string `json:"dasTemplate"`
			DzTemplate  string `json:"dzTemplate"`
		} `json:"templates"`
	}
	bs, err := r.postJson(r.apiUrl, r)
	if err != nil {
		return
	}
	var resultError family.ResultError
	if err = json.Unmarshal(bs, &resultError); resultError.Error.StatusCode != 0 {
		msg := fmt.Sprintf("StatusCode: %d, Message: %s", resultError.Error.StatusCode, resultError.Error.Message)
		err = errors.New(msg)
		return
	}
	resp := Response{}
	if err = json.Unmarshal(bs, &resp); err != nil {
		return
	}
	//https://sg30p0.familysearch.org/service/records/storage/deepzoomcloud/dz/v1/{id}/{image}
	r.dziTemplate = regexp.MustCompile(`\{[A-z]+\}`).ReplaceAllString(resp.Templates.DzTemplate, "%s")
	r.dziTemplate = regexp.MustCompile(`https://([A-z0-9\./]+)/service/records/storage/deepzoomcloud`).ReplaceAllString(r.dziTemplate, r.baseUrl)
	for _, image := range resp.Images {
		//https://familysearch.org/ark:/61903/3:1:3QSQ-G9MC-ZSQ7-3/image.xml
		m := regexp.MustCompile(`(?i)ark:/(?:[A-z0-9-_:]+)/([A-z\d-_:]+)/image.xml`).FindStringSubmatch(image)
		if m == nil {
			continue
		}
		xmlUrl := fmt.Sprintf(r.dziTemplate, m[1], "image.xml")
		canvases = append(canvases, xmlUrl)

	}
	return canvases, err
}

func (r *Familysearch) postJson(sUrl string, d interface{}) ([]byte, error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  r.dt.Jar,
		Headers: map[string]interface{}{
			"User-Agent":   config.Conf.UserAgent,
			"Content-Type": "application/json",
			"authority":    "www.familysearch.org",
			"origin":       r.baseUrl,
			"referer":      r.dt.Url,
		},
		JSON: r,
	})
	resp, err := cli.Post(sUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	return bs, err
}

func (r *Familysearch) getSessionId() string {
	bs, _ := os.ReadFile(config.Conf.CookieFile)
	cookies := string(bs)
	//fssessionid=e10ce618-f7f7-45de-b2c3-d1a31d080d58-prod;
	m := regexp.MustCompile(`fssessionid=([^;]+);`).FindStringSubmatch(cookies)
	if m != nil {
		return "bearer " + m[1]
	}
	return ""
}

func (r *Familysearch) postBody(sUrl string, data []byte) ([]byte, error) {
	sid := r.getSessionId()
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  r.dt.Jar,
		Headers: map[string]interface{}{
			"User-Agent":    config.Conf.UserAgent,
			"Content-Type":  "application/json",
			"authority":     "www.familysearch.org",
			"origin":        r.baseUrl,
			"authorization": sid,
			"referer":       r.dt.Url,
		},
		Body: data,
	})
	resp, err := cli.Post(sUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	return bs, err
}

func (r *Familysearch) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent":   config.Conf.UserAgent,
			"Content-Type": "application/json",
			"authority":    "www.familysearch.org",
			"origin":       r.baseUrl,
			"referer":      r.dt.Url,
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
