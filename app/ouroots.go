package app

import (
	"bookget/config"
	"bookget/model/ouroots"
	"bookget/pkg/gohttp"
	"bookget/pkg/progressbar"
	"bookget/pkg/util"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Ouroots struct {
	dt      *DownloadTask
	Counter int
	bar     *progressbar.ProgressBar
}

func NewOuroots() *Ouroots {
	return &Ouroots{
		// 初始化字段
		dt:      new(DownloadTask),
		Counter: 0,
	}
}

func (r *Ouroots) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *Ouroots) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r *Ouroots) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`(?i)\.html\?([A-z0-9]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}

func (r *Ouroots) download() (msg string, err error) {
	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)

	respVolume, err := r.getVolumes(r.dt.BookId)
	if err != nil || respVolume.StatusCode != "200" {
		fmt.Println(err)
		return "getVolumes", err
	}
	//不按卷下载，所有图片存一个目录
	r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")
	macCounter := 0
	for i, vol := range respVolume.Volume {
		if !config.VolumeRange(i) {
			continue
		}
		macCounter += vol.Pages
	}
	fmt.Println()
	r.bar = progressbar.Default(int64(macCounter), "downloading")
	for i, vol := range respVolume.Volume {
		if !config.VolumeRange(i) {
			continue
		}
		r.do(vol.Pages, vol.VolumeId)
	}
	return "", nil
}

func (r *Ouroots) do(pageTotal int, volumeId int) (msg string, err error) {
	token, err := r.getToken()
	if err != nil {
		r.bar.Clear()
		return "token not found.", err
	}
	for i := 1; i <= pageTotal; i++ {
		sortId := fmt.Sprintf("%s.jpg", fmt.Sprintf("%04d", r.Counter+1))
		dest := r.dt.SavePath + sortId
		if util.FileExist(dest) {
			r.Counter++
			r.bar.Add(1)
			time.Sleep(40 * time.Millisecond)
			continue
		}
		respImage, err := r.getBase64Image(r.dt.BookId, volumeId, i, "", token)
		if err != nil || respImage.StatusCode != "200" {
			continue
		}
		if pos := strings.Index(respImage.ImagePath, "data:image/jpeg;base64,"); pos != -1 {
			data := respImage.ImagePath[pos+len("data:image/jpeg;base64,"):]
			bs, err := base64.StdEncoding.DecodeString(data)
			if err != nil || bs == nil {
				//log.Println(err)
				continue
			}
			_ = os.WriteFile(dest, bs, os.ModePerm)
			r.Counter++
			r.bar.Add(1)
			time.Sleep(40 * time.Millisecond)
		}
	}
	return "", nil
}

func (r *Ouroots) getVolumes(catalogKey string) (ouroots.ResponseVolume, error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  r.dt.Jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
		},
		Query: map[string]interface{}{
			"catalogKey": catalogKey,
			"bookid":     "", //目录索引，不重要
		},
	})
	resp, err := cli.Get("http://dsnode.ouroots.nlc.cn/gtService/data/catalogVolume")
	bs, _ := resp.GetBody()
	if bs == nil {
		return ouroots.ResponseVolume{}, errors.New(resp.GetReasonPhrase())
	}

	var respVolume ouroots.ResponseVolume
	if err = json.Unmarshal(bs, &respVolume); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return respVolume, errors.New(resp.GetReasonPhrase())
	}
	return respVolume, nil
}

func (r *Ouroots) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *Ouroots) getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error) {
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

func (r *Ouroots) postBody(sUrl string, d []byte) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (r *Ouroots) getToken() (string, error) {
	bs, err := r.getBody("http://dsNode.ouroots.nlc.cn/loginAnonymousUser", r.dt.Jar)
	if err != nil {
		return "", err
	}
	var respLoginAnonymousUser ouroots.ResponseLoginAnonymousUser
	if err = json.Unmarshal(bs, &respLoginAnonymousUser); err != nil {
		return "", err
	}
	return respLoginAnonymousUser.Token, nil
}
func (r *Ouroots) getBase64Image(catalogKey string, volumeId, page int, userKey, token string) (respImage ouroots.ResponseCatalogImage, err error) {
	ctx := context.Background()
	cli := gohttp.NewClient(ctx, gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  r.dt.Jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
		},
		Query: map[string]interface{}{
			"catalogKey": catalogKey,
			"volumeId":   strconv.FormatInt(int64(volumeId), 10),
			"page":       strconv.FormatInt(int64(page), 10),
			"userKey":    userKey,
			"token":      token,
		},
	})
	resp, err := cli.Get("http://dsnode.ouroots.nlc.cn/data/catalogImage")
	bs, _ := resp.GetBody()
	if bs == nil {
		err = errors.New(resp.GetReasonPhrase())
		return
	}
	if err = json.Unmarshal(bs, &respImage); err != nil {
		return
	}
	return respImage, nil
}
