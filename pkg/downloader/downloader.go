package downloader

import (
	"bookget/pkg/progressbar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxConcurrent = 8 // 最大并发下载数
	userAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:139.0) Gecko/20100101 Firefox/139.0"
	minFileSize   = 1024 // 最小文件大小(1KB)
)

// DownloadTask 下载任务
type DownloadTask struct {
	URL          string            // 下载URL
	Method       string            // 请求方法(GET/POST)
	Headers      map[string]string // 请求头
	Body         []byte            // POST请求体
	SaveDir      string            // 保存目录
	Threads      int               // 线程数
	FileName     string            // 保存文件名(不含扩展名)
	ContentType  string            // 文件类型
	ContentSize  int64             // 文件大小
	Success      bool              // 是否成功
	ErrorMessage string            // 错误信息
	buffer       *bytes.Buffer     // 内存缓冲区
	mu           sync.Mutex        // 互斥锁

	supportsHEAD  bool // 是否支持HEAD请求
	supportsRange bool // 是否支持Range请求
	testedMethods bool // 是否已检测过支持的方法

	totalSize  int64 // 总文件大小
	downloaded int64 // 已下载字节数
}

type DownloadManager struct {
	tasks         []*DownloadTask
	maxConcurrent int
	successCount  int32
	failCount     int32
	totalTasks    int   // 总任务数
	completed     int32 // 已完成任务数
	wg            sync.WaitGroup
	sem           chan struct{}
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.Mutex

	started    bool      // 标记是否已开始
	showPrompt bool      // 是否显示开始提示
	allDone    bool      // 标记所有任务是否已完成
	startTime  time.Time // 记录开始时间

	bar        *progressbar.ProgressBar // 总进度条(基于任务数)
	totalSize  int64                    // 总文件大小
	downloaded int64                    // 已下载字节数
}

// NewDownloadManager 创建下载管理器
func NewDownloadManager(ctx context.Context, cancel context.CancelFunc, maxTasks int) *DownloadManager {
	//ctx, cancel := context.WithCancel(context.Background())
	if maxTasks < 1 {
		maxTasks = maxConcurrent
	}
	return &DownloadManager{
		maxConcurrent: maxTasks,
		sem:           make(chan struct{}, maxConcurrent),
		ctx:           ctx,
		cancel:        cancel,
		showPrompt:    true,
	}
}

// AddTask 添加下载任务（需要加锁）
func (dm *DownloadManager) AddTask(url, method string, headers map[string]string, body []byte, saveDir string, filename string, threads int) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if threads <= 0 {
		threads = 1 // 默认单线程
	} else if threads > maxConcurrent {
		threads = maxConcurrent
	}

	task := &DownloadTask{
		URL:      url,
		Method:   method,
		Headers:  headers,
		Body:     body,
		SaveDir:  saveDir,
		FileName: filename,
		Threads:  threads,
		buffer:   bytes.NewBuffer(nil),
	}

	dm.tasks = append(dm.tasks, task)
}

// SetBar 设置进度条
func (dm *DownloadManager) SetBar(maxTasks int) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.bar = progressbar.Default(int64(maxTasks), "downloading")
}

// Start 开始下载
func (dm *DownloadManager) Start() {
	dm.mu.Lock()
	dm.startTime = time.Now()

	// 初始化进度条
	if dm.bar == nil {
		dm.bar = progressbar.Default(int64(len(dm.tasks)), "downloading")
	}

	if dm.showPrompt {
		fmt.Printf("\n开始下载任务 (最大并发数: %d)...\n", dm.maxConcurrent)
		fmt.Printf("总任务数: %d\n", len(dm.tasks))
		dm.showPrompt = false
	}

	dm.mu.Unlock()

	for _, task := range dm.tasks {
		dm.wg.Add(1)
		go func(t *DownloadTask) {
			dm.sem <- struct{}{}
			defer func() {
				<-dm.sem
				dm.wg.Done()
			}()

			err := t.Download(dm.ctx, dm) // 传入dm以更新总进度

			dm.mu.Lock()
			if err != nil {
				atomic.AddInt32(&dm.failCount, 1)
				t.Success = false
				t.ErrorMessage = err.Error()
				fmt.Printf("下载失败: %s (%s)\n", t.FileName, err)
			} else {
				atomic.AddInt32(&dm.successCount, 1)
				t.Success = true
				dm.bar.Add(1) // 每个任务完成时进度条+1
			}
			dm.mu.Unlock()
		}(task)
	}

	dm.wg.Wait()

	elapsed := time.Since(dm.startTime)
	dm.mu.Lock()
	if dm.bar != nil {
		_ = dm.bar.Finish()
	}
	fmt.Printf("\n下载完成! 成功: %d, 失败: %d, 耗时: %v\n",
		dm.successCount, dm.failCount, elapsed.Round(time.Millisecond))
	dm.allDone = true
	dm.mu.Unlock()
}

// getTasksToProcess 获取待处理的任务
func (dm *DownloadManager) getTasksToProcess() []*DownloadTask {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 找出所有未处理的任务(Success为默认值false且ErrorMessage为空)
	var tasks []*DownloadTask
	for _, task := range dm.tasks {
		if !task.Success && task.ErrorMessage == "" {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// WaitAll 等待所有任务完成
func (dm *DownloadManager) WaitAll() {
	for {
		dm.mu.Lock()
		done := dm.allDone
		dm.mu.Unlock()

		if done {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Stop 停止所有下载
func (dm *DownloadManager) Stop() {
	dm.cancel()
}

// Download 执行下载任务
func (task *DownloadTask) Download(ctx context.Context, dm *DownloadManager) error {
	// 1. 获取文件信息
	if err := task.getFileInfo(ctx); err != nil {
		log.Printf("警告: %v", err)
		if task.FileName == "" {
			task.FileName = getFileNameFromURL(task.URL)
		}
	}

	// 2. 自动获取文件名
	if task.FileName == "" {
		task.FileName = getFileNameFromURL(task.URL)
		if task.FileName == "" {
			task.FileName = fmt.Sprintf("download_%d", time.Now().Unix())
			if task.ContentType != "" {
				ext := getExtensionFromMime(task.ContentType)
				if ext != "" {
					task.FileName += ext
				}
			}
		}
	}

	// 3. 多线程下载
	if task.ContentSize > int64(minFileSize)*10 && task.Threads > 1 && task.supportsRange {
		if err := task.multiThreadDownload(ctx, dm); err != nil {
			return err
		}
	} else {
		if err := task.singleThreadDownload(ctx, dm); err != nil {
			return err
		}
	}

	// 4. 保存文件
	if task.buffer.Len() > 0 {
		if err := os.MkdirAll(task.SaveDir, 0755); err != nil {
			return fmt.Errorf("创建目录失败: %v", err)
		}

		filePath := filepath.Join(task.SaveDir, task.FileName)
		if err := os.WriteFile(filePath, task.buffer.Bytes(), 0644); err != nil {
			return fmt.Errorf("写入文件失败: %v", err)
		}
	}

	return nil
}

// 多线程下载
func (task *DownloadTask) multiThreadDownload(ctx context.Context, dm *DownloadManager) error {
	// 确保文件大小已知且有效
	if task.ContentSize <= 0 {
		return fmt.Errorf("无法使用多线程下载: 文件大小未知")
	}

	chunkSize := task.ContentSize / int64(task.Threads)
	lastChunkSize := task.ContentSize % int64(task.Threads)

	var wg sync.WaitGroup
	wg.Add(task.Threads)

	var firstErr error
	var errOnce sync.Once

	for i := 0; i < task.Threads; i++ {
		go func(threadID int) {
			defer wg.Done()

			start := int64(threadID) * chunkSize
			end := start + chunkSize - 1

			if threadID == task.Threads-1 {
				end += lastChunkSize
			}

			req, err := http.NewRequest(task.Method, task.URL, nil)
			if err != nil {
				errOnce.Do(func() { firstErr = err })
				return
			}

			// 设置请求头
			for k, v := range task.Headers {
				req.Header.Set(k, v)
			}
			if req.Header.Get("User-Agent") == "" {
				req.Header.Set("User-Agent", userAgent)
			}

			// 设置Range头
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

			client := &http.Client{}
			resp, err := client.Do(req.WithContext(ctx))
			if err != nil {
				errOnce.Do(func() { firstErr = err })
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
				errOnce.Do(func() {
					firstErr = fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
				})
				return
			}

			// 读取数据到缓冲区
			buf := make([]byte, 32*1024) // 32KB缓冲区
			for {
				select {
				case <-ctx.Done():
					return
				default:
					n, readErr := resp.Body.Read(buf)
					if n > 0 {
						task.mu.Lock()
						task.buffer.Write(buf[:n])
						task.mu.Unlock()

						atomic.AddInt64(&dm.downloaded, int64(n))
					}

					if readErr != nil {
						if readErr != io.EOF {
							errOnce.Do(func() { firstErr = readErr })
						}
						return
					}
				}
			}
		}(i)
	}

	wg.Wait()
	return firstErr
}

// 单线程下载
func (task *DownloadTask) singleThreadDownload(ctx context.Context, dm *DownloadManager) error {
	req, err := http.NewRequest(task.Method, task.URL, bytes.NewReader(task.Body))
	if err != nil {
		return err
	}

	// 设置请求头
	for k, v := range task.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", userAgent)
	}

	client := &http.Client{}
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
	}

	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				task.mu.Lock()
				task.buffer.Write(buf[:n])
				task.mu.Unlock()

				atomic.AddInt64(&dm.downloaded, int64(n))
			}

			if readErr != nil {
				if readErr != io.EOF {
					return readErr
				}
				return nil
			}
		}
	}
}

// 获取文件信息
func (task *DownloadTask) getFileInfo(ctx context.Context) error {
	// 检测服务器支持的方法
	if err := task.detectSupportedMethods(ctx); err != nil {
		return err
	}

	var req *http.Request
	var err error

	// 根据支持的方法选择请求类型
	if task.supportsHEAD {
		req, err = http.NewRequest("HEAD", task.URL, nil)
	} else {
		req, err = http.NewRequest("GET", task.URL, nil)
		if task.supportsRange {
			req.Header.Set("Range", "bytes=0-0")
		}
	}

	if err != nil {
		return err
	}

	// 设置请求头
	for k, v := range task.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", userAgent)
	}

	client := &http.Client{}
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 检查响应状态
	expectedStatus := http.StatusOK
	if !task.supportsHEAD && task.supportsRange {
		expectedStatus = http.StatusPartialContent
	}

	if resp.StatusCode != expectedStatus {
		return fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
	}

	// 处理分块传输的情况
	isChunked := resp.TransferEncoding != nil &&
		len(resp.TransferEncoding) > 0 &&
		resp.TransferEncoding[0] == "chunked"

	// 获取文件大小
	if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
		// 从Content-Range解析总大小，格式如: bytes 0-0/102400
		parts := strings.Split(contentRange, "/")
		if len(parts) == 2 {
			size, err := strconv.ParseInt(parts[1], 10, 64)
			if err == nil {
				task.ContentSize = size
			}
		}
	} else if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		size, err := strconv.ParseInt(contentLength, 10, 64)
		if err != nil {
			return fmt.Errorf("解析文件大小失败: %v", err)
		}
		task.ContentSize = size
	} else if isChunked {
		// 分块传输且没有Content-Length，设置为0表示大小未知
		task.ContentSize = 0
	} else {
		// 既不是分块传输也没有Content-Length，可能是服务器错误
		return fmt.Errorf("无法确定文件大小: 没有Content-Length头且不是分块传输")
	}

	// 获取内容类型
	task.ContentType = resp.Header.Get("Content-Type")

	// 尝试从Content-Disposition获取文件名
	if disposition := resp.Header.Get("Content-Disposition"); disposition != "" {
		if filename := parseFilenameFromDisposition(disposition); filename != "" {
			task.FileName = filename
		}
	}

	return nil
}

// 检测服务器支持的请求方法并缓存结果
func (task *DownloadTask) detectSupportedMethods(ctx context.Context) error {
	if task.testedMethods {
		return nil // 已经检测过，直接返回
	}

	// 先尝试HEAD请求
	headReq, err := http.NewRequest("HEAD", task.URL, nil)
	if err != nil {
		return err
	}

	// 设置请求头
	for k, v := range task.Headers {
		headReq.Header.Set(k, v)
	}
	if headReq.Header.Get("User-Agent") == "" {
		headReq.Header.Set("User-Agent", userAgent)
	}

	client := &http.Client{}
	resp, err := client.Do(headReq.WithContext(ctx))

	if err == nil && resp.StatusCode == http.StatusOK {
		task.supportsHEAD = true
		resp.Body.Close()
	} else {
		// HEAD请求失败，尝试Range请求
		getReq, err := http.NewRequest("GET", task.URL, nil)
		if err != nil {
			return err
		}

		// 设置请求头
		for k, v := range task.Headers {
			getReq.Header.Set(k, v)
		}
		if getReq.Header.Get("User-Agent") == "" {
			getReq.Header.Set("User-Agent", userAgent)
		}

		// 只请求前1个字节
		getReq.Header.Set("Range", "bytes=0-0")

		resp, err = client.Do(getReq.WithContext(ctx))
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		task.supportsRange = resp.StatusCode == http.StatusPartialContent
	}

	task.testedMethods = true
	return nil
}

// 辅助函数: 从URL获取文件名
func getFileNameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	path := u.Path
	if len(path) == 0 {
		return ""
	}

	// 获取最后一个斜杠后的部分
	parts := strings.Split(path, "/")
	lastPart := parts[len(parts)-1]

	// 去除查询参数
	if idx := strings.Index(lastPart, "?"); idx != -1 {
		lastPart = lastPart[:idx]
	}

	return lastPart
}

// 辅助函数: 从Content-Disposition解析文件名
func parseFilenameFromDisposition(disposition string) string {
	// 示例: attachment; filename="example.zip"
	parts := strings.Split(disposition, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "filename=") {
			filename := part[len("filename="):]
			filename = strings.Trim(filename, `"`)
			return filename
		}
	}
	return ""
}

// 辅助函数: 从MIME类型获取扩展名
func getExtensionFromMime(mimeType string) string {
	switch mimeType {
	case "application/zip":
		return ".zip"
	case "application/pdf":
		return ".pdf"
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "text/plain":
		return ".txt"
	default:
		return ""
	}
}

// 辅助函数: 格式化字节大小
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// 辅助函数: 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
