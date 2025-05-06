package app

import (
	"bookget/config"
	"bookget/model/princeton"
	"bookget/pkg/gohttp"
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

type Princeton struct {
	dt *DownloadTask
}

func NewPrinceton() *Princeton {
	return &Princeton{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *Princeton) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Princeton) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r *Princeton) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`catalog/([A-z\d]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *Princeton) download() (msg string, err error) {
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

func (r *Princeton) do(canvases []string) (msg string, err error) {
	if canvases == nil {
		return
	}
	fmt.Println()
	referer := r.dt.Url
	size := len(canvases)
	var wg sync.WaitGroup
	q := QueueNew(int(config.Conf.Threads))
	for i, uri := range canvases {
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
		log.Printf("Get %d/%d page, URL: %s\n", i+1, size, imgUrl)
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
			fmt.Println()
		})
	}
	wg.Wait()
	fmt.Println()
	return "", err
}

func (r *Princeton) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	var manifestUrl = ""
	//
	if strings.Contains(sUrl, "dpul.princeton.edu") {
		bs, err := r.getBody(sUrl, jar)
		if err != nil {
			return nil, err
		}
		//页面中有https://figgy.princeton.edu/viewer#?manifest=https%3A%2F%2Ffiggy.princeton.edu%2Fconcern%2Fscanned_resources%2F64ee594e-4735-4a8e-b712-73b8c00ec56b%2Fmanifest&config=https%3A%2F%2Ffiggy.princeton.edu%2Fviewer%2Fexhibit%2Fconfig%3Fmanifest%3Dhttps%3A%2F%2Ffiggy.princeton.edu%2Fconcern%2Fscanned_resources%2F64ee594e-4735-4a8e-b712-73b8c00ec56b%2Fmanifest

		m := regexp.MustCompile(`manifest=(.+?)&`).FindStringSubmatch(string(bs))
		if m == nil {
			return nil, err
		}
		manifestUrl, _ = url.QueryUnescape(m[1])
	}

	if strings.Contains(sUrl, "catalog.princeton.edu") {
		//Graphql 查询
		phql := new(princeton.Graphql)
		d := fmt.Sprintf(`{"operationName":"GetResourcesByOrangelightIds","variables":{"ids":["%s"]},"query":"query GetResourcesByOrangelightIds($ids: [String!]!) {\n  resourcesByOrangelightIds(ids: $ids) {\n    id\n    thumbnail {\n      iiifServiceUrl\n      thumbnailUrl\n      __typename\n    }\n    url\n    members {\n      id\n      __typename\n    }\n    ... on ScannedResource {\n      manifestUrl\n      orangelightId\n      __typename\n    }\n    ... on ScannedMap {\n      manifestUrl\n      orangelightId\n      __typename\n    }\n    ... on Coin {\n      manifestUrl\n      orangelightId\n      __typename\n    }\n    __typename\n  }\n}\n"}`,
			r.dt.BookId)
		bs, err := r.postBody("https://figgy.princeton.edu/graphql", []byte(d))
		if err != nil {
			return nil, err

		}
		if err = json.Unmarshal(bs, phql); err != nil {
			log.Printf("json.Unmarshal failed: %s\n", err)
			return nil, err
		}
		for _, v := range phql.Data.ResourcesByOrangelightIds {
			manifestUrl = v.ManifestUrl
		}

	}
	if manifestUrl == "" {
		return
	}

	//查全书分卷URL
	var manifest = new(princeton.ResponseManifest)
	body, err := r.getBody(manifestUrl, jar)
	if err != nil {
		return
	}
	if err = json.Unmarshal(body, manifest); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}

	if manifest.Manifests == nil {
		volumes = append(volumes, manifestUrl)
	} else {
		//分卷URL处理
		for _, vol := range manifest.Manifests {
			volumes = append(volumes, vol.Id)
		}
	}
	return volumes, nil
}

func (r *Princeton) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	var manifest2 = new(princeton.ResponseManifest2)
	body, err := r.getBody(sUrl, jar)
	if err != nil {
		return
	}
	if err = json.Unmarshal(body, manifest2); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	i := len(manifest2.Sequences[0].Canvases)
	canvases = make([]string, 0, i)

	//分卷URL处理
	for _, sequences := range manifest2.Sequences {
		for _, canvase := range sequences.Canvases {
			for _, image := range canvase.Images {
				//JPEG URL
				imgUrl := image.Resource.Service.Id + "/" + config.Conf.Format
				canvases = append(canvases, imgUrl)
			}
		}
	}

	return canvases, nil
}

func (r *Princeton) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
	referer := url.QueryEscape(apiUrl)
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
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

func (r *Princeton) postBody(sUrl string, d []byte) ([]byte, error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  r.dt.Jar,
		Headers: map[string]interface{}{
			"User-Agent":   config.Conf.UserAgent,
			"Content-Type": "application/json",
			"authority":    "figgy.princeton.edu",
			"referer":      r.dt.Url,
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
