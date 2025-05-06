package app

import (
	"bookget/config"
	"bookget/model/war"
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
)

type War1931 struct {
	dt              *DownloadTask
	docType         string
	fileCode        string
	jsonUrlTemplate string
}

func NewWar1931() *War1931 {
	return &War1931{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *War1931) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *War1931) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.Jar, _ = cookiejar.New(nil)
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	return r.download()
}

func (r *War1931) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`fileCode=([A-z0-9_-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		return m[1]
	}
	return ""
}

func (r *War1931) mkdirAll(directory, vid string) (dirPath string) {
	switch r.docType {
	case "ts":
		r.dt.VolumeId = r.dt.UrlParsed.Host + "_" + r.dt.BookId + string(os.PathSeparator) + directory + "_vol." + vid
		break
	case "bz":
		r.dt.VolumeId = r.dt.UrlParsed.Host + "_" + r.dt.BookId + string(os.PathSeparator) + directory
		break
	case "qk":
		r.dt.VolumeId = r.dt.UrlParsed.Host + "_" + r.dt.BookId + string(os.PathSeparator) + directory + "_vol." + vid
		break
	default:
	}
	r.dt.SavePath = config.Conf.SaveFolder + string(os.PathSeparator) + r.dt.VolumeId
	_ = os.MkdirAll(r.dt.SavePath, os.ModePerm)
	return r.dt.SavePath
}

func (r *War1931) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)
	apiUrl := "https://" + r.dt.UrlParsed.Host + "/backend-prod/esBook/findDetailsInfo/" + r.dt.BookId
	partialVolumes, err := r.getVolumes(apiUrl, r.dt.Jar)
	if err != nil {
		fmt.Println(err)
		return "getVolumes", err
	}
	for k, parts := range partialVolumes {
		if !config.VolumeRange(k) {
			continue
		}
		log.Printf(" %d/%d, %d volumes \n", k+1, len(partialVolumes), len(parts.Volumes))
		for i, vol := range parts.Volumes {
			vid := fmt.Sprintf("%04d", i+1)
			r.mkdirAll(parts.Directory, vid)
			canvases, err := r.getCanvases(vol, r.dt.Jar)
			if err != nil || canvases == nil {
				fmt.Println(err)
				continue
			}
			log.Printf(" %d/%d volume, %d pages \n", i+1, len(parts.Volumes), len(canvases))
			r.do(canvases)
		}
	}
	return "", err
}

func (r *War1931) do(canvases []string) (msg string, err error) {
	if canvases == nil {
		return "", nil
	}
	referer := url.QueryEscape(r.dt.Url)
	args := []string{
		"-H", "Origin:" + referer,
		"-H", "Referer:" + referer,
		"-H", "User-Agent:" + config.Conf.UserAgent,
	}
	size := len(canvases)
	for i, uri := range canvases {
		if uri == "" || !config.PageRange(i, size) {
			continue
		}
		sortId := fmt.Sprintf("%04d", i+1)
		filename := sortId + config.Conf.FileExt
		inputUri := r.dt.SavePath + string(os.PathSeparator) + sortId + "_info.json"
		bs, err := r.getBody(uri, r.dt.Jar)
		if err != nil {
			continue
		}
		bsNew := regexp.MustCompile(`profile":\[([^{]+)\{"formats":([^\]]+)\],`).ReplaceAll(bs, []byte(`profile":[{"formats":["jpg"],`))
		err = os.WriteFile(inputUri, bsNew, os.ModePerm)
		if err != nil {
			return "", err
		}
		dest := r.dt.SavePath + string(os.PathSeparator) + filename
		if FileExist(dest) {
			continue
		}
		log.Printf("Get %s  %s\n", sortId, uri)
		if ret := util.StartProcess(inputUri, dest, args); ret == true {
			os.Remove(inputUri)
		}
	}
	return "", err
}

func (r *War1931) getVolumes(apiUrl string, jar *cookiejar.Jar) (volumes []war.PartialVolumes, err error) {
	bs, err := r.getBody(apiUrl, jar)
	if err != nil {
		return nil, err
	}
	var resp war.DetailsInfo
	if err = json.Unmarshal(bs, &resp); err != nil {
		return nil, err
	}
	r.docType = resp.Result.Info.DocType
	r.fileCode = resp.Result.Info.FileCode
	jsonUrl := resp.Result.Info.IiifObj.JsonUrl
	r.jsonUrlTemplate, _ = r.getJsonUrlTemplate(jsonUrl, r.fileCode, r.docType)
	switch r.docType {
	case "ts":
		partVol := war.PartialVolumes{
			Directory: r.fileCode,
			Title:     resp.Result.Info.Title,
			Volumes:   []string{jsonUrl},
		}
		volumes = append(volumes, partVol)
		break
	case "bz":
		volumes, err = r.getVolumesForBz(jsonUrl, r.dt.Jar)
		break
	case "qk":
		volumes, err = r.getVolumesForQk(jsonUrl, r.dt.Jar)
		break
	default:
	}
	return volumes, nil
}

func (r *War1931) getJsonUrlTemplate(jsonUrl, fileCode, docType string) (jsonUrlTemplate string, err error) {
	if jsonUrl == "" {
		return "", err
	}
	u, err := url.Parse(jsonUrl)
	if err != nil {
		return "", err
	}
	if docType == "ts" {
		jsonUrlTemplate = u.Scheme + "://" + u.Host + "/" + fileCode + "/%s.json"
	} else {
		jsonUrlTemplate = u.Scheme + "://" + u.Host + "/" + fileCode + "/%s/%s.json"
	}
	return jsonUrlTemplate, err
}

func (r *War1931) getVolumesForBz(sUrl string, jar *cookiejar.Jar) (volumes []war.PartialVolumes, err error) {
	years, err := r.findBzYear(r.fileCode)
	if err != nil {
		return nil, err
	}
	fmt.Println()
	for _, year := range years {
		months, err := r.findBzMonth(year)
		if err != nil {
			continue
		}
		for _, month := range months {
			if len(month) == 1 {
				fmt.Printf("Test %s-0%s\r", year, month)
			} else {
				fmt.Printf("Test %s-%s\r", year, month)
			}
			apiUrl := "https://" + r.dt.UrlParsed.Host + "/backend-prod/esBook/findDirectoryByMonth?fileCode=" + r.fileCode + "&year=" + year + "&month=" + month
			bs, err := r.getBody(apiUrl, jar)
			if err != nil {
				break
			}
			var resp = new(war.FindDirectoryByMonth)
			if err := json.Unmarshal(bs, resp); err != nil {
				log.Printf("json.Unmarshal failed: %s\n", err)
				break
			}
			for _, item := range resp.Result {
				partVol := war.PartialVolumes{
					Directory: year + "/" + item.Date,
					Title:     item.Date,
					Volumes:   []string{item.IiifObj.JsonUrl},
				}
				volumes = append(volumes, partVol)
			}
		}
	}
	fmt.Println()
	return volumes, err
}

func (r *War1931) findBzYear(fileCode string) (years []string, err error) {
	apiUrl := "https://" + r.dt.UrlParsed.Host + "/backend-prod/esBook/findYear/" + fileCode
	bs, err := r.getBody(apiUrl, r.dt.Jar)
	if err != nil {
		return nil, err
	}
	type Response struct {
		Code    string   `json:"code"`
		Message string   `json:"message"`
		Result  []string `json:"result"`
	}
	var resp = new(Response)
	if err = json.Unmarshal(bs, resp); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	return resp.Result, err
}

func (r *War1931) findBzMonth(year string) (years []string, err error) {
	apiUrl := "https://" + r.dt.UrlParsed.Host + "/backend-prod/esBook/findMonth?fileCode=" + r.fileCode + "&year=" + year
	bs, err := r.getBody(apiUrl, r.dt.Jar)
	if err != nil {
		return nil, err
	}
	type Response struct {
		Code    string   `json:"code"`
		Message string   `json:"message"`
		Result  []string `json:"result"`
	}
	var resp = new(Response)
	if err = json.Unmarshal(bs, resp); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	return resp.Result, err
}

func (r *War1931) getVolumesForQk(sUrl string, jar *cookiejar.Jar) (volumes []war.PartialVolumes, err error) {
	apiUrl := "https://" + r.dt.UrlParsed.Host + "/backend-prod/esBook/findDirectoryByYear/" + r.fileCode
	bs, err := r.getBody(apiUrl, jar)
	if err != nil {
		return nil, err
	}
	var resp = new(war.Qk)
	if err = json.Unmarshal(bs, resp); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	for _, items := range resp.Result {
		for _, item := range items.List {
			partVol := war.PartialVolumes{
				Directory: item.Year,
				Title:     items.Title,
				Volumes:   nil,
			}
			partVol.Volumes = make([]string, 0, len(item.DataList))
			for _, v := range item.DataList {
				jsonUrl := fmt.Sprintf(r.jsonUrlTemplate, v.Id, v.Id)
				partVol.Volumes = append(partVol.Volumes, jsonUrl)
			}
			volumes = append(volumes, partVol)
		}
	}
	return volumes, err
}

func (r *War1931) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	bs, err := r.getBody(sUrl, jar)
	if err != nil {
		return nil, err
	}
	var manifest = new(war.Manifest)
	if err = json.Unmarshal(bs, manifest); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	if len(manifest.Sequences) == 0 {
		return
	}
	size := len(manifest.Sequences[0].Canvases)
	canvases = make([]string, 0, size)
	for _, canvase := range manifest.Sequences[0].Canvases {
		for _, image := range canvase.Images {
			iiiInfo := fmt.Sprintf("%s/info.json", image.Resource.Service.Id)
			canvases = append(canvases, iiiInfo)
		}
	}
	return canvases, nil
}

func (r *War1931) getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent":      config.Conf.UserAgent,
			"Accept-Language": "zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2",
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
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
