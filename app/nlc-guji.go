package app

import (
	"bookget/config"
	"bookget/model/nlc"
	"bookget/pkg/cookie"
	"bookget/pkg/downloader"
	"bookget/pkg/util"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

type NlcGuji struct {
	dm     *downloader.DownloadManager
	ctx    context.Context
	cancel context.CancelFunc
	client *http.Client

	rawUrl    string
	parsedUrl *url.URL
	savePath  string
	bookId    string

	responseBody []byte
	urlsFile     string
	bufBuilder   strings.Builder
}

func NewNlcGuji() *NlcGuji {
	ctx, cancel := context.WithCancel(context.Background())
	dm := downloader.NewDownloadManager(ctx, cancel, config.Conf.MaxConcurrent)

	// 创建自定义 Transport 忽略 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, _ := cookiejar.New(nil)

	return &NlcGuji{
		// 初始化字段
		dm:     dm,
		client: &http.Client{Timeout: config.Conf.Timeout * time.Second, Jar: jar, Transport: tr},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *NlcGuji) GetRouterInit(sUrl string) (map[string]interface{}, error) {
	s.rawUrl = sUrl
	s.parsedUrl, _ = url.Parse(sUrl)
	s.Run()
	return map[string]interface{}{
		"type": "dzicnlib",
		"url":  sUrl,
	}, nil
}

func (s *NlcGuji) getBookId() (bookId string) {
	const (
		metadataIdPattern = `(?i)metadataId=([A-Za-z0-9_-]+)`
		idPattern         = `(?i)\?id=([A-Za-z0-9_-]+)`
	)

	// 预编译正则表达式
	var (
		metadataIdRe = regexp.MustCompile(metadataIdPattern)
		idRe         = regexp.MustCompile(idPattern)
	)

	// 优先尝试匹配 metadataId
	if matches := metadataIdRe.FindStringSubmatch(s.rawUrl); matches != nil && len(matches) > 1 {
		return matches[1]
	}

	// 然后尝试匹配 id
	if matches := idRe.FindStringSubmatch(s.rawUrl); matches != nil && len(matches) > 1 {
		return matches[1]
	}

	return "" // 明确返回空字符串表示未找到
}

func (s *NlcGuji) Run() (msg string, err error) {
	s.bookId = s.getBookId()
	if s.bookId == "" {
		return "[err=getBookId]", err
	}
	s.savePath = CreateDirectory(s.parsedUrl.Host, s.bookId, "")
	s.urlsFile = s.savePath + "urls.txt"
	//先生成书签目录
	s.buildCatalog(s.savePath + "catalog.txt")

	groupedVolumes, err := s.getVolumes()
	if err != nil || groupedVolumes == nil {
		return "[err=getVolumes]", err

	}

	if err != nil {
		fmt.Println(err)
		return "getVolumes", err
	}

	var i = 0
	for _, item := range groupedVolumes {
		if !config.VolumeRange(i) {
			continue
		}
		i++
		vid := fmt.Sprintf("%04d", i)
		s.savePath = CreateDirectory(s.parsedUrl.Host, s.bookId, vid)
		log.Printf(" %d/%d volume, %d pages \n", i, len(groupedVolumes), len(item.Items))
		s.letsGo(item.Items)
	}

	err = os.WriteFile(s.urlsFile, []byte(s.bufBuilder.String()), os.ModePerm)
	if err != nil {
		return "", err
	}
	fmt.Printf("\n已生成图片URLs文件[%s]\n 可复制到 bookget-gui.exe 目录下，或使用其它软件下载。\n", s.urlsFile)

	return "", nil
}

func (s *NlcGuji) letsGo(canvases []nlc.DataItem) (msg string, err error) {
	sizeVol := len(canvases)
	if sizeVol <= 0 {
		return "[err=letsGo]", err
	}
	imgServer := fmt.Sprintf("https://%s/api/common/jpgViewer?ftpId=1&filePathName=", s.parsedUrl.Host)

	counter := 0
	for i, item := range canvases {
		i++
		sortId := fmt.Sprintf("%04d", i)
		fileName := sortId + config.Conf.FileExt
		//跳过存在的文件
		if FileExist(s.savePath + fileName) {
			continue
		}
		//https://guji.nlc.cn/api/anc/ancImageAndContent?metadataId=1001165&structureId=1014544&imageId=2075393
		apiUrl := fmt.Sprintf("https://%s/api/anc/ancImageAndContent?metadataId=%s&structureId=%d&imageId=%s",
			s.parsedUrl.Host, s.bookId, item.StructureId, item.ImageId)
		//metadataId=1001165&structureId=1014544&imageId=2075393
		rawData := []byte(fmt.Sprintf("metadataId=%s&structureId=%d&imageId=%s", s.bookId, item.StructureId, item.ImageId))
		bs, err := s.postBody(apiUrl, rawData)
		if err != nil {
			return "[err=letsGo]", err
		}
		var resp nlc.ImageData
		if err = json.Unmarshal(bs, &resp); err != nil {
			return "[err=letsGo::Unmarshal]", err
		}
		encoded := url.QueryEscape(resp.Data.FilePath)
		imgUrl := imgServer + encoded
		fmt.Printf("准备中 %d/%d\r", i, sizeVol)

		s.bufBuilder.WriteString(imgUrl)
		s.bufBuilder.WriteString("\n")

		// 添加GET下载任务
		s.dm.AddTask(
			imgUrl,
			"GET",
			map[string]string{"User-Agent": config.Conf.UserAgent},
			nil,
			s.savePath,
			fileName,
			config.Conf.Threads,
		)
		counter++
	}
	fmt.Println()
	s.dm.SetBar(counter)
	s.dm.Start()
	return "", nil
}

func (s *NlcGuji) getCanvases() (canvases []nlc.DataItem, err error) {

	if s.responseBody == nil {
		apiUrl := fmt.Sprintf("https://%s/api/anc/ancImageIdListWithPageNum?metadataId=%s", s.parsedUrl.Host, s.bookId)
		rawData := []byte("metadataId=" + s.bookId)
		s.responseBody, err = s.postBody(apiUrl, rawData)
		if err != nil {
			return canvases, err
		}
	}
	resp := new(nlc.BaseResponse)
	if err = json.Unmarshal(s.responseBody, &resp); err != nil {
		return canvases, err
	}
	// 打印结果
	for _, item := range resp.Data.ImageIdList {
		canvases = append(canvases, item)
	}
	return canvases, nil
}

func (s *NlcGuji) getVolumes() (volumes []nlc.GroupedVolume, err error) {
	if s.responseBody == nil {
		apiUrl := fmt.Sprintf("https://%s/api/anc/ancImageIdListWithPageNum?metadataId=%s", s.parsedUrl.Host, s.bookId)
		rawData := []byte("metadataId=" + s.bookId)
		s.responseBody, err = s.postBody(apiUrl, rawData)
		if err != nil {
			return volumes, err
		}
	}
	resp := new(nlc.BaseResponse)
	if err = json.Unmarshal(s.responseBody, &resp); err != nil {
		return volumes, err
	}

	volumes = make([]nlc.GroupedVolume, 0)
	volId := 0
	lastStructureId := -1

	for _, item := range resp.Data.ImageIdList {
		if item.StructureId != lastStructureId {
			lastStructureId = item.StructureId
			volumes = append(volumes, nlc.GroupedVolume{VolID: volId, Items: []nlc.DataItem{}})
			volId++
		}
		// 添加到当前分组
		volumes[len(volumes)-1].Items = append(volumes[len(volumes)-1].Items, item)
	}
	return volumes, nil
}

func (s *NlcGuji) getBody(sUrl string) ([]byte, error) {
	req, err := http.NewRequest("GET", sUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.Conf.UserAgent)
	cookies := cookie.CookiesFromFile(config.Conf.CookieFile)
	if cookies != "" {
		req.Header.Set("Cookie", cookies)
	}
	resp, err := s.client.Do(req.WithContext(s.ctx))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("close body err=%v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		err = fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (s *NlcGuji) postBody(sUrl string, postData []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", sUrl, bytes.NewBuffer(postData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", config.Conf.UserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2")
	req.Header.Set("token", "")
	req.Header.Set("Origin", "https://"+s.parsedUrl.Host)
	req.Header.Set("Referer", s.rawUrl)
	req.Header.Set("Content-Type", "application/json")
	cookies := cookie.CookiesFromFile(config.Conf.CookieFile)
	if cookies != "" {
		req.Header.Set("Cookie", cookies)
	}
	resp, err := s.client.Do(req.WithContext(s.ctx))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("close body err=%v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		err = fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (s *NlcGuji) buildCatalog(outputPath string) {
	// 1. 获取目录结构数据
	//fmt.Println("正在获取目录结构数据...")

	apiUrl := fmt.Sprintf("https://%s/api/anc/ancStructureAndCatalogList?metadataId=%s", s.parsedUrl.Host, s.bookId)
	rawData := []byte("metadataId=" + s.bookId)

	structureData, err := s.postBody(apiUrl, rawData)
	if err != nil {
		fmt.Printf("获取目录结构失败: %v\n", err)
		return
	}

	var structureResp nlc.StructureResponse
	if err := json.Unmarshal(structureData, &structureResp); err != nil {
		fmt.Printf("解析目录结构失败: %v\n", err)
		return
	}

	// 2. 获取页码映射数据
	//fmt.Println("正在获取页码映射数据...")
	apiUrl = fmt.Sprintf("https://%s/api/anc/ancImageIdListWithPageNum?metadataId=%s", s.parsedUrl.Host, s.bookId)
	s.responseBody, err = s.postBody(apiUrl, rawData)
	if err != nil {
		fmt.Printf("获取页码映射失败: %v\n", err)
		return
	}

	var pageResp nlc.PageResponse
	if err := json.Unmarshal(s.responseBody, &pageResp); err != nil {
		fmt.Printf("解析页码映射失败: %v\n", err)
		return
	}

	// 创建imageID到pageNum的映射
	idToPage := make(map[int]string)
	for _, item := range pageResp.Data.ImageIDList {
		imageID, err := util.ToInt(item.ImageID)
		if err != nil || imageID == 0 {
			continue
		}

		pageNum, err := util.ToString(item.PageNum)
		if err != nil {
			continue
		}

		idToPage[imageID] = pageNum
	}

	//fmt.Printf("获取到 %d 条页码映射数据\n", len(idToPage))

	// 生成目录
	catalog := []string{config.CatalogVersionInfo}
	for _, volume := range structureResp.Data {
		for _, child := range volume.Children {
			processItem(&child, idToPage, &catalog, "")
		}
	}

	// 保存到文件
	content := strings.Join(catalog, "\n")
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		fmt.Printf("保存文件失败: %v\n", err)
		return
	}

	fmt.Printf("目录已成功保存到 %s\n", outputPath)
	//fmt.Printf("共生成 %d 条目录项）\n", len(catalog)-1)
}

func processItem(item *nlc.CatalogItem, idToPage map[int]string, catalog *[]string, prefix string) {
	if item.Title == "" || len(item.ImageIDs) == 0 {
		return
	}

	// 获取imageID
	imageID, err := util.ToInt(item.ImageIDs[0])
	if err != nil {
		*catalog = append(*catalog, fmt.Sprintf("%s%s ………… 未知", prefix, strings.TrimSpace(item.Title)))
	} else {
		pageNum, exists := idToPage[imageID]
		if !exists {
			pageNum = "未知"
		}
		*catalog = append(*catalog, fmt.Sprintf("%s%s ………… %s", prefix, strings.TrimSpace(item.Title), pageNum))
	}

	// 处理子项
	for _, child := range item.Children {
		processItem(&child, idToPage, catalog, prefix+"\t")
	}
}

// 按 StructureId 分组
func (s *NlcGuji) groupByStructureID(items []nlc.DataItem) map[int][]nlc.DataItem {
	groups := make(map[int][]nlc.DataItem)
	for _, item := range items {
		groups[item.StructureId] = append(groups[item.StructureId], item)
	}
	return groups
}

// 方案1：按 StructureId 升序排列
func (s *NlcGuji) sortGroupsByStructureID(groups map[int][]nlc.DataItem) []nlc.DataItem {
	// 提取所有 StructureId 并排序
	var structureIDs []int
	for id := range groups {
		structureIDs = append(structureIDs, id)
	}
	sort.Ints(structureIDs)

	// 按排序后的 StructureId 合并数据
	var sortedItems []nlc.DataItem
	for _, id := range structureIDs {
		sortedItems = append(sortedItems, groups[id]...)
	}
	return sortedItems
}

// 方案2：按 PageNum 升序排列（每组内部排序）
func (s *NlcGuji) sortGroupsByPageNum(groups map[int][]nlc.DataItem) []nlc.DataItem {
	var structureIDs []int
	for id := range groups {
		structureIDs = append(structureIDs, id)
	}
	sort.Ints(structureIDs)

	var sortedItems []nlc.DataItem
	for _, id := range structureIDs {
		group := groups[id]
		// 对每组按 PageNum 排序
		sort.Slice(group, func(i, j int) bool {
			return group[i].PageNum < group[j].PageNum
		})
		sortedItems = append(sortedItems, group...)
	}
	return sortedItems
}
