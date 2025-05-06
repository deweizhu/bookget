package app

import (
	"bookget/config"
	"bookget/model/iiif"
	xcrypt "bookget/pkg/crypt"
	"bookget/pkg/util"
	"fmt"
	"log"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	//故宫名画 minghuaji.dpm.org.cn
	//来源 https://minghuaji.dpm.org.cn/js/gve.js
	// array[3] 是Key, array[5] 是IV
	MINGHUAJI_KEY = "ucv4uHn5bynSi42c"
	MINGHUAJI_IV  = "CGnpTaoTS5sIG5SK"

	//数字文物 digicol.dpm.org.cn
	//来源 https://digicol.dpm.org.cn/js/gve.js
	// array[3] 是Key, array[5] 是IV
	DIGICOL_KEY = "XxHgrFq2IzqOgORm"
	DIGICOL_IV  = "3HhRveOYbpEBrwqF"
)

type DpmBj struct {
	dt *DownloadTask
}

func NewDpmBj() *DpmBj {
	return &DpmBj{
		// 初始化字段
		dt: new(DownloadTask),
	}
}

func (r *DpmBj) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	msg, err := r.Run(sUrl)
	return map[string]interface{}{
		"url": sUrl,
		"msg": msg,
	}, err
}

func (r *DpmBj) Run(sUrl string) (msg string, err error) {
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.VolumeId = getBookId(r.dt.Url)
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r *DpmBj) getBookId(sUrl string) (bookId string) {
	m := regexp.MustCompile(`id=([A-z0-9_-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	return bookId
}
func (r *DpmBj) getTitle(bs []byte) string {
	//<title>赵孟頫水村图卷-故宫名画记</title>
	m := regexp.MustCompile(`<title>([^<]+)</title>`).FindSubmatch(bs)
	if m == nil {
		return ""
	}
	title := regexp.MustCompile("([|/\\:+\\?]+)").ReplaceAll(m[1], nil)
	return strings.Replace(string(title), "-故宫名画记", "", -1)
}

func (r *DpmBj) getCipherText(bs []byte) []byte {
	//gv.init("",...)
	m := regexp.MustCompile(`gv.init(?:[ \r\n\t\f]*)\("([^"]+)"`).FindSubmatch(bs)
	if m == nil {
		return nil
	}
	return m[1]
}

func (r *DpmBj) download() (msg string, err error) {
	bs, err := getBody(r.dt.Url, r.dt.Jar)
	if err != nil {
		return "Error:", err
	}
	cipherText := r.getCipherText(bs)
	r.dt.Title = r.getTitle(bs)

	name := fmt.Sprintf("%04d", r.dt.Index)
	log.Printf("Get %s %s %s\n", name, r.dt.Title, r.dt.Url)

	if cipherText == nil || len(cipherText) == 0 {
		return "cipherText not found", err
	}

	r.dt.SavePath = CreateDirectory(r.dt.UrlParsed.Host, r.dt.BookId, "")

	dziJson, dziFormat := r.getDziJson(r.dt.UrlParsed.Host, cipherText)
	filename := fmt.Sprintf("%s.json", r.dt.VolumeId)
	dest := r.dt.SavePath + filename
	os.WriteFile(dest, []byte(dziJson), os.ModePerm)
	return r.do(dest, dziFormat)
}

func (r *DpmBj) do(dest string, dziFormat iiif.DziFormat) (msg string, err error) {
	referer := fmt.Sprintf("https://%s", r.dt.UrlParsed.Host)
	args := []string{"--dezoomer=deepzoom",
		"-H", "Origin:" + referer,
		"-H", "Referer:" + referer,
		"-H", "User-Agent:" + config.Conf.UserAgent,
	}
	storePath := r.dt.SavePath
	ext := "." + dziFormat.Format
	outfile := storePath + r.dt.VolumeId + ext
	if util.FileExist(outfile) {
		return "", nil
	}
	if ret := util.StartProcess(dest, outfile, args); ret == true {
		os.Remove(dest)
	}
	return "", err
}

func (r *DpmBj) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *DpmBj) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *DpmBj) getBody(sUrl string, jar *cookiejar.Jar) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (r *DpmBj) postBody(sUrl string, d []byte) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (r *DpmBj) getDziJson(host string, text []byte) (dziJson string, dzi iiif.DziFormat) {
	template := `{
  "xmlns": "http://schemas.microsoft.com/deepzoom/2009",
  "Url": "%s",
  "Overlap": "%d",
  "TileSize": "%d",
  "Format": "%s",
  "Size": {
    "Width": "%d",
    "Height": "%d"
  }
}
`
	var recoveredPt []byte
	var err error
	if host == "digicol.dpm.org.cn" {
		recoveredPt, err = xcrypt.DecryptByAes(string(text), []byte(DIGICOL_KEY), []byte(DIGICOL_IV))
	} else {
		recoveredPt, err = xcrypt.DecryptByAes(string(text), []byte(MINGHUAJI_KEY), []byte(MINGHUAJI_IV))
	}
	if err != nil {
		return
	}
	m := strings.Split(string(recoveredPt), "^")
	if m == nil || len(m) != 6 {
		return
	}
	//fmt.Printf("Split plaintext: %+v\n", m)
	dzi.Url = m[0]
	dzi.Format = m[1]
	dzi.TileSize, _ = strconv.Atoi(m[4])
	dzi.Overlap, _ = strconv.Atoi(m[5])
	if strings.Contains(m[2], ".") {
		w, _ := strconv.ParseFloat(m[2], 32)
		dzi.Size.Width = int(w)
	} else {
		dzi.Size.Width, _ = strconv.Atoi(m[2])
	}
	if strings.Contains(m[3], ".") {
		h, _ := strconv.ParseFloat(m[3], 32)
		dzi.Size.Height = int(h)
	} else {
		dzi.Size.Height, _ = strconv.Atoi(m[3])
	}
	dziJson = fmt.Sprintf(template, dzi.Url, dzi.Overlap, dzi.TileSize, dzi.Format, dzi.Size.Width, dzi.Size.Height)
	return
}
