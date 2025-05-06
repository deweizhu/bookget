package app

import (
	"bookget/config"
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"bookget/pkg/progressbar"
)

const (
	maxConcurrent = 8
	userAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:139.0) Gecko/20100101 Firefox/139.0"
	minFileSize   = 1024 // 最小文件大小(1KB)
)

type ImageDownloader struct {
	client            *http.Client
	reader            *bufio.Reader
	hasVolPlaceholder bool   // 是否包含[VOL]或[PAGE]占位符
	hasABPlaceholder  bool   // 是否包含[AB]或[ab]占位符
	abPlaceholder     string // 存储实际的占位符形式（"[AB]"或"[ab]"）
	abIsLowercase     bool   // 占位符是否为小写
	maxConcurrent     int

	ctx context.Context
}

func NewImageDownloader() *ImageDownloader {
	maxConcurrent_ := maxConcurrent
	if config.Conf.MaxConcurrent > 0 {
		maxConcurrent_ = config.Conf.MaxConcurrent
	}

	// 创建自定义 Transport 忽略 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, _ := cookiejar.New(nil)

	return &ImageDownloader{
		// 初始化字段
		client:            &http.Client{Timeout: config.Conf.Timeout * time.Second, Jar: jar, Transport: tr},
		reader:            bufio.NewReader(os.Stdin),
		hasVolPlaceholder: false,
		maxConcurrent:     maxConcurrent_,
		ctx:               context.Background(),
	}
}

func (i *ImageDownloader) GetRouterInit(rawUrl string) (map[string]interface{}, error) {
	// 实现具体逻辑
	i.Run(rawUrl)

	return map[string]interface{}{
		"type": "bookget",
		"url":  rawUrl,
	}, nil
}

func (i *ImageDownloader) Run(rawUrl string) {
	for {
		fmt.Println("\n=== 当前模式：图片批量下载 ===")
		fmt.Println("输入 'exit' 退出程序")

		// 1. 获取URL模板
		urlTemplate, err := i.getInput("请输入图片URL模板（必须包含[PAGE]，可选[VOL]和[AB]）: ")
		if err != nil || strings.ToLower(urlTemplate) == "exit" {
			break
		}

		// 检查必须包含[PAGE]占位符
		if !strings.Contains(urlTemplate, "[PAGE]") {
			fmt.Println("错误: URL模板必须包含[PAGE]占位符")
			continue
		}

		// 检查占位符
		i.hasVolPlaceholder = strings.Contains(urlTemplate, "[VOL]")
		i.hasABPlaceholder = strings.Contains(urlTemplate, "[AB]") || strings.Contains(urlTemplate, "[ab]")

		// 存储实际的占位符形式
		if strings.Contains(urlTemplate, "[AB]") {
			i.abPlaceholder = "[AB]"
			i.abIsLowercase = false
		} else if strings.Contains(urlTemplate, "[ab]") {
			i.abPlaceholder = "[ab]"
			i.abIsLowercase = true
		}

		// 2. 获取页码格式化位数
		pageFormat, err := i.getInput("请输入页码格式化位数（如04表示0001，03表示001）: ")
		if err != nil || pageFormat == "" {
			fmt.Println("输入错误: 必须指定页码格式化位数")
			continue
		}

		// 3. 获取扩展名（从URL模板中提取或用户指定）
		ext := filepath.Ext(urlTemplate)
		if ext == "" {
			ext, err = i.getInput("无法从URL中识别扩展名，请手动输入（如.jpg、.png）: ")
			if err != nil || ext == "" {
				fmt.Println("输入错误: 必须指定文件扩展名")
				continue
			}
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
		}

		var startVol, endVol int
		if i.hasVolPlaceholder {
			// 4. 获取册数范围
			startVol, endVol, err = i.getVolumeRange()
			if err != nil {
				fmt.Println("输入错误:", err)
				continue
			}
		} else {
			startVol, endVol = 1, 1
		}

		// 5. 获取总页数
		totalPages, err := i.getInputInt("请输入全部册数的总页数: ")
		if err != nil || totalPages <= 0 {
			fmt.Println("输入错误: 总页数必须大于0")
			continue
		}

		// 6. 确认并开始下载
		if i.hasVolPlaceholder {
			fmt.Printf("\n即将开始下载:\nURL模板: %s\n册数范围: %04d-%04d\n总页数: %d\n页码格式: %%0%sd\n扩展名: %s\n",
				urlTemplate, startVol, endVol, totalPages, pageFormat, ext)
		} else {
			fmt.Printf("\n即将开始下载:\nURL模板: %s\n总页数: %d\n页码格式: %%0%sd\n扩展名: %s\n",
				urlTemplate, totalPages, pageFormat, ext)
		}
		confirm, _ := i.getInput("确认开始下载？(y/n): ")
		if strings.ToLower(confirm) != "y" {
			continue
		}

		// 7. 执行下载
		i.downloadAll(urlTemplate, startVol, endVol, totalPages, pageFormat, ext)

		// 8. 询问是否继续
		cont, _ := i.getInput("\n下载完成！是否继续下载其他URL模板？(y/n): ")
		if strings.ToLower(cont) != "y" {
			break
		}
	}

	fmt.Println("程序退出")
}

func (i *ImageDownloader) getInput(prompt string) (string, error) {
	fmt.Print(prompt)
	input, err := i.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func (i *ImageDownloader) getInputInt(prompt string) (int, error) {
	input, err := i.getInput(prompt)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(input)
}

func (i *ImageDownloader) getVolumeRange() (int, int, error) {
	startVol, err := i.getInputInt("请输入起始册号: ")
	if err != nil {
		return 0, 0, err
	}

	endVol, err := i.getInputInt("请输入结束册号: ")
	if err != nil {
		return 0, 0, err
	}

	if startVol > endVol {
		return 0, 0, fmt.Errorf("起始册号不能大于结束册号")
	}

	return startVol, endVol, nil
}

func (i *ImageDownloader) downloadAll(urlTemplate string, startVol, endVol, totalPages int, pageFormat, ext string) {
	totalVolumes := endVol - startVol + 1
	//totalExpected := totalPages * 2 // 假设每页都有A/B两面

	var totalDownloaded int64
	globalBar := progressbar.NewOptions64(
		int64(totalPages),
		progressbar.OptionSetDescription("总下载进度"),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(50),
		progressbar.OptionOnCompletion(func() {
			fmt.Printf("\n下载完成！共成功下载 %d 个文件\n", atomic.LoadInt64(&totalDownloaded))
		}),
	)

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, i.maxConcurrent)

	// 计算每册页数（简单平均分配）
	pagesPerVol := totalPages / totalVolumes
	remainingPages := totalPages % totalVolumes

	for vol := startVol; vol <= endVol; vol++ {
		wg.Add(1)
		semaphore <- struct{}{}

		// 当前册的页数
		currentPages := pagesPerVol
		if vol <= startVol+remainingPages-1 {
			currentPages++
		}

		go func(volume, pagesThisVol int) {
			defer wg.Done()
			defer func() { <-semaphore }()

			volStr := fmt.Sprintf("%04d", volume)
			var dirPath string
			if i.hasVolPlaceholder {
				dirPath = filepath.Join(config.Conf.SaveFolder, "downloads", volStr)
			} else {
				dirPath = filepath.Join(config.Conf.SaveFolder, "downloads")
			}

			if err := os.MkdirAll(dirPath, 0755); err != nil {
				fmt.Printf("\n创建目录 %s 失败: %v\n", dirPath, err)
				return
			}

			for page := 1; page <= pagesThisVol; page++ {
				i.downloadPageSmart(urlTemplate, volStr, page, dirPath, pageFormat, ext, globalBar, &totalDownloaded)
			}
		}(vol, currentPages)
	}

	wg.Wait()
	globalBar.Finish()
}

func (i *ImageDownloader) downloadPageSmart(urlTemplate, volStr string, page int, dirPath, pageFormat, ext string, globalBar *progressbar.ProgressBar, totalDownloaded *int64) {
	// 构建页码格式
	pageNum := fmt.Sprintf("%0"+pageFormat+"d", page)

	// 构建基础URL
	url := urlTemplate
	if i.hasVolPlaceholder {
		url = strings.Replace(url, "[VOL]", volStr, 1)
	}

	if i.hasABPlaceholder {
		// 智能处理AB面（只检测一次）
		abSuffix := "A" // 默认使用大写
		if i.abIsLowercase {
			abSuffix = "a"
		}

		// 构建A面URL
		urlA := strings.Replace(url, "[PAGE]", pageNum, 1)
		urlA = strings.Replace(urlA, i.abPlaceholder, abSuffix, 1)
		err := i.downloadAndValidate(urlA, filepath.Join(dirPath, fmt.Sprintf("%s%s%s", pageNum, abSuffix, ext)), globalBar, totalDownloaded)

		if err == nil {
			// 如果A面存在，下载B面
			abSuffix = "B" // 默认使用大写
			if i.abIsLowercase {
				abSuffix = "b"
			}

			urlB := strings.Replace(url, "[PAGE]", pageNum, 1)
			urlB = strings.Replace(urlB, i.abPlaceholder, abSuffix, 1)
			err = i.downloadAndValidate(urlB, filepath.Join(dirPath, fmt.Sprintf("%s%s%s", pageNum, abSuffix, ext)), globalBar, totalDownloaded)
			if err != nil {
				fmt.Printf("[err=downloadAndValidate]+%v\n", err)
				return
			}
		} else {
			urlPlain := strings.Replace(url, "[PAGE]", pageNum, 1)
			urlPlain = strings.Replace(urlPlain, i.abPlaceholder, "", 1)
			s := filepath.Join(dirPath, fmt.Sprintf("%s%s", pageNum, ext))
			err = i.downloadAndValidate(urlPlain, s, globalBar, totalDownloaded)
			if err != nil {
				fmt.Printf("[err=downloadAndValidate]+%v\n", err)
				return
			}
		}

	} else {
		urlPlain := strings.Replace(url, "[PAGE]", pageNum, 1)
		err := i.downloadAndValidate(urlPlain, filepath.Join(dirPath, fmt.Sprintf("%s%s", pageNum, ext)), globalBar, totalDownloaded)
		if err != nil {
			fmt.Printf("[err=downloadAndValidate]+%v\n", err)
			return
		}
	}
}

func (i *ImageDownloader) downloadAndValidate(url, filePath string, globalBar *progressbar.ProgressBar, totalDownloaded *int64) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := i.client.Do(req.WithContext(i.ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return err
	}

	buf := bytes.NewBuffer(make([]byte, 0, 10*1024*1024))
	if _, err := io.CopyBuffer(buf, resp.Body, make([]byte, 32*1024)); err != nil { // 32KB缓冲区
		return err
	}
	if buf.Len() < minFileSize {
		return errors.New("发现0字节文件")
	}

	//file, err := os.Create(filePath)
	//if err != nil {
	//	return err
	//}
	//defer file.Close()
	//
	//
	//if _, err := buf.WriteTo(file); err != nil {
	//	os.Remove(filePath)
	//	return err
	//}

	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	atomic.AddInt64(totalDownloaded, 1)
	globalBar.Add(1)

	return nil
}
