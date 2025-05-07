package downloader

import (
	"bookget/pkg/progressbar"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"
)

/*
// 创建下载器
downloader := downloader.NewIIIFDownloader()

// 自定义 IIIF tileURL 格式
downloader.SetIIIFTileFormat("{{.ID}}/region/{{.X}},{{.Y}},{{.Width}},{{.Height}}/{{.Width}},{{.Height}}/0/default.{{.Format}}")

// 自定义 DeepZoom tileURL 格式
downloader.SetDeepZoomTileFormat("{{.ServerURL}}/tiles/{{.Level}}/{{.Y}}/{{.X}}.{{.Format}}")
*/
type IIIFInfo struct {
	// 公共字段
	Context  string `json:"@context"`
	Protocol string `json:"protocol,omitempty"` // v2专用
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Type     string `json:"type,omitempty"`    // v3专用
	Profile  string `json:"profile,omitempty"` // v3专用
	ID       string `json:"@id"`               // v2字段
	Id       string `json:"id"`                // v3字段

	// 兼容性字段
	Sizes []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"sizes,omitempty"`

	Tiles []struct {
		Width        int   `json:"width"`
		Height       int   `json:"height"`
		ScaleFactors []int `json:"scaleFactors"`
		Overlap      int   `json:"overlap,omitempty"`
	} `json:"tiles,omitempty"`

	// v3扩展字段
	ExtraQualities []string `json:"extraQualities,omitempty"`
	ExtraFormats   []string `json:"extraFormats,omitempty"`
	ExtraFeatures  []string `json:"extraFeatures,omitempty"`

	// 内部计算字段
	version int // 2或3
	baseURL string
}

type IIIFXMLInfo struct {
	XMLName  xml.Name `xml:"Image"`
	TileSize int      `xml:"TileSize,attr"`
	Overlap  int      `xml:"Overlap,attr"`
	Format   string   `xml:"Format,attr"`
	Size     struct {
		Width  int `xml:"Width,attr"`
		Height int `xml:"Height,attr"`
	} `xml:"Size"`
	ServerURL string `xml:"Url,attr"`
}

// TileURLFormat 定义 tileURL 的格式配置
type TileURLFormat struct {
	// 模板字符串，支持 Go 模板语法
	// 可用变量: .ServerURL, .ID, .Level, .X, .Y, .Format, .Width, .Height
	Template string

	// 预编译的模板
	compiledTemplate *template.Template
	// 固定值字段
	FixedValues map[string]interface{}
}

/*
模板变量说明
IIIF 格式可用变量:
.ID: 图像ID
.X: 拼图X坐标
.Y: 拼图Y坐标
.Width: 拼图宽度
.Height: 拼图高度
.Format: 图像格式

DeepZoom 格式可用变量:
.ServerURL: 服务器URL
.Level: 缩放级别
.X: 拼图X索引
.Y: 拼图Y索引
.Format: 图像格式
*/
type IIIFDownloader struct {
	client *http.Client
	// tileURL 格式配置
	iiifTileFormat     TileURLFormat // IIIF 格式的 tileURL
	DeepzoomTileFormat TileURLFormat // DeepZoom 格式的 tileURL
}

func NewIIIFDownloader() *IIIFDownloader {
	// 创建自定义 Transport 忽略 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, _ := cookiejar.New(nil)

	dl := &IIIFDownloader{
		client: &http.Client{Jar: jar, Transport: tr},
	}
	// 设置默认的 tileURL 格式
	//dl.SetIIIFTileFormat("{{.ID}}/{{.X}},{{.Y}},{{.Width}},{{.Height}}/{{.Width}},{{.Height}}/0/default.{{.Format}}")
	// 更新模板，在size部分支持 ^ 前缀
	dl.SetIIIFTileFormat("{{.ID}}/{{.X}},{{.Y}},{{.Width}},{{.Height}}/{{if .sizeUpscaling}}^{{end}}{{.Width}},{{.Height}}/0/default.{{.Format}}")

	dl.SetDeepZoomTileFormat("{{.ServerURL}}/image_files/{{.Level}}/{{.X}}_{{.Y}}.{{.Format}}")

	return dl
}

// SetIIIFTileFormat 设置 IIIF 格式的 tileURL 模板
func (d *IIIFDownloader) SetIIIFTileFormat(format string) error {
	tmpl, err := template.New("iiifTile").Parse(format)
	if err != nil {
		return fmt.Errorf("解析 IIIF tileURL 模板失败: %v", err)
	}
	d.iiifTileFormat = TileURLFormat{
		Template:         format,
		compiledTemplate: tmpl,
	}
	return nil
}

// SetDeepZoomTileFormat 设置 DeepZoom 格式的 tileURL 模板
func (d *IIIFDownloader) SetDeepZoomTileFormat(format string) error {
	tmpl, err := template.New("deepzoomTile").Parse(format)
	if err != nil {
		return fmt.Errorf("解析 DeepZoom tileURL 模板失败: %v", err)
	}
	d.DeepzoomTileFormat = TileURLFormat{
		Template:         format,
		compiledTemplate: tmpl,
	}
	return nil
}

// buildIIIFTileURL 根据模板构建 IIIF 格式的 tileURL
func (d *IIIFDownloader) buildIIIFTileURL(data map[string]interface{}) (string, error) {
	// 确保有基础URL
	if _, ok := data["ServerBaseURL"]; !ok {
		if id, ok := data["ID"].(string); ok {
			if u, err := url.Parse(id); err == nil {
				data["ServerBaseURL"] = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
			}
		}
	}

	// 合并固定值和传入数据
	mergedData := make(map[string]interface{})
	for k, v := range data {
		mergedData[k] = v
	}

	for k, v := range d.DeepzoomTileFormat.FixedValues {
		mergedData[k] = v
	}

	var buf bytes.Buffer
	err := d.iiifTileFormat.compiledTemplate.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	result := buf.String()
	if !strings.HasPrefix(result, "http") {
		if base, ok := data["ServerBaseURL"].(string); ok {
			result = strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(result, "/")
		}
	}

	return result, nil
}

// buildDeepZoomTileURL 根据模板构建 DeepZoom 格式的 tileURL
func (d *IIIFDownloader) buildDeepZoomTileURL(data map[string]interface{}) (string, error) {
	// 合并固定值和传入数据
	mergedData := make(map[string]interface{})
	for k, v := range data {
		mergedData[k] = v
	}

	for k, v := range d.DeepzoomTileFormat.FixedValues {
		mergedData[k] = v
	}

	var buf bytes.Buffer
	err := d.DeepzoomTileFormat.compiledTemplate.Execute(&buf, mergedData)
	if err != nil {
		return "", fmt.Errorf("执行 DeepZoom tileURL 模板失败: %v", err)
	}
	return buf.String(), nil
}

func (d *IIIFDownloader) Dezoomify(ctx context.Context, infoURL string, outputPath string, args []string) error {
	headers, err := d.argsToHeaders(args)
	if err != nil {
		return fmt.Errorf("转换header失败: %v", err)
	}

	info, err := d.getIIIFInfoByURL(ctx, "", infoURL, headers)
	if err != nil {
		return fmt.Errorf("获取图像信息失败: %v", err)
	}

	var finalImg image.Image
	switch v := info.(type) {
	case *IIIFInfo:
		tileConfig := v.Tiles[0]
		cols := (v.Width + tileConfig.Width - 1) / tileConfig.Width
		rows := (v.Height + tileConfig.Height - 1) / tileConfig.Height
		finalImg, err = d.downloadAndMergeTiles(ctx, v, cols, rows, headers)
	case *IIIFXMLInfo:
		finalImg, err = d.downloadAndMergeXMLTiles(ctx, v, headers)
	default:
		return fmt.Errorf("未知的图像信息格式")
	}

	if err != nil {
		return fmt.Errorf("处理拼图失败: %v", err)
	}

	if err := d.saveImage(finalImg, outputPath); err != nil {
		return fmt.Errorf("保存图像失败: %v", err)
	}

	fmt.Printf("\n图像合并完成，已保存到 %s\n", outputPath)
	return nil
}

func (d *IIIFDownloader) DezoomifyWithServer(ctx context.Context, serverBaseURL, xmlURL string, outputPath string, args []string) error {
	headers, err := d.argsToHeaders(args)
	if err != nil {
		return fmt.Errorf("转换header失败: %v", err)
	}

	info, err := d.getIIIFInfoByURL(ctx, serverBaseURL, xmlURL, headers)
	if err != nil {
		return fmt.Errorf("获取图像信息失败: %v", err)
	}

	var finalImg image.Image
	switch v := info.(type) {
	case *IIIFInfo:
		tileConfig := v.Tiles[0]
		cols := (v.Width + tileConfig.Width - 1) / tileConfig.Width
		rows := (v.Height + tileConfig.Height - 1) / tileConfig.Height
		finalImg, err = d.downloadAndMergeTiles(ctx, v, cols, rows, headers)
	case *IIIFXMLInfo:
		finalImg, err = d.downloadAndMergeXMLTiles(ctx, v, headers)
	default:
		return fmt.Errorf("未知的图像信息格式")
	}

	if err != nil {
		return fmt.Errorf("处理拼图失败: %v", err)
	}

	if err := d.saveImage(finalImg, outputPath); err != nil {
		return fmt.Errorf("保存图像失败: %v", err)
	}

	fmt.Printf("\n图像合并完成，已保存到 %s\n", outputPath)
	return nil
}

// DezoomifyWithContent 直接使用XML或JSON内容进行下载
func (d *IIIFDownloader) DezoomifyWithContent(ctx context.Context, content string, outputPath string, args []string) error {
	headers, err := d.argsToHeaders(args)
	if err != nil {
		return fmt.Errorf("转换header失败: %v", err)
	}

	// 尝试解析为JSON
	var jsonInfo IIIFInfo
	if err := json.Unmarshal([]byte(content), &jsonInfo); err == nil {
		// 成功解析为JSON
		if len(jsonInfo.Tiles) == 0 {
			return fmt.Errorf("JSON内容中未找到拼图配置信息")
		}

		tileConfig := jsonInfo.Tiles[0]
		cols := (jsonInfo.Width + tileConfig.Width - 1) / tileConfig.Width
		rows := (jsonInfo.Height + tileConfig.Height - 1) / tileConfig.Height

		finalImg, err := d.downloadAndMergeTiles(ctx, &jsonInfo, cols, rows, headers)
		if err != nil {
			return fmt.Errorf("处理拼图失败: %v", err)
		}

		return d.saveImage(finalImg, outputPath)
	}

	// 尝试解析为XML
	var xmlInfo IIIFXMLInfo
	if err := xml.Unmarshal([]byte(content), &xmlInfo); err == nil {
		finalImg, err := d.downloadAndMergeXMLTiles(ctx, &xmlInfo, headers)
		if err != nil {
			return fmt.Errorf("处理拼图失败: %v", err)
		}

		return d.saveImage(finalImg, outputPath)
	}

	return fmt.Errorf("内容既不是有效的JSON也不是有效的XML")
}

func (d *IIIFDownloader) downloadAndMergeTiles(ctx context.Context, info *IIIFInfo, cols, rows int, headers http.Header) (image.Image, error) {
	tileConfig := info.Tiles[0]
	tileWidth := tileConfig.Width
	tileHeight := tileConfig.Height

	// 从 JSON 中获取 overlap（默认为 0）
	overlap := 0
	if len(info.Tiles) > 0 {
		overlap = info.Tiles[0].Overlap
	}

	effectiveTileWidth := tileWidth - overlap*2
	effectiveTileHeight := tileHeight - overlap*2

	finalImg := image.NewRGBA(image.Rect(0, 0, cols*effectiveTileWidth, rows*effectiveTileHeight))
	progressBar := progressbar.Default(int64(cols*rows), "IIIF")

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, 1)

	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			wg.Add(1)
			go func(x, y int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				// 构建包含重叠区域的请求
				tileData := map[string]interface{}{
					"ID":            info.ID,
					"ServerBaseURL": info.baseURL,
					"X":             x * effectiveTileWidth,
					"Y":             y * effectiveTileHeight,
					"Width":         tileWidth,
					"Height":        tileHeight,
					"Format":        "jpg",
					"Version":       info.version, // 传递版本信息
					"sizeUpscaling": d.needsUpscale(info, tileWidth, tileHeight),
				}

				tileURL, err := d.buildIIIFTileURL(tileData)
				if err != nil {
					select {
					case errChan <- fmt.Errorf("构建 tileURL 失败: %v", err):
					default:
					}
					return
				}

				img, err := d.downloadImage(ctx, tileURL, headers)
				if err != nil {
					select {
					case errChan <- fmt.Errorf("下载拼图(%d,%d)失败: %v", x, y, err):
					default:
					}
					return
				}

				mu.Lock()
				// 计算目标位置（跳过重叠部分）
				destX := x * effectiveTileWidth
				destY := y * effectiveTileHeight
				if x > 0 {
					destX += overlap
				}
				if y > 0 {
					destY += overlap
				}

				// 复制有效像素区域
				bounds := img.Bounds()
				for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
					for px := bounds.Min.X; px < bounds.Max.X; px++ {
						targetX := destX + (px - bounds.Min.X)
						targetY := destY + (py - bounds.Min.Y)
						if targetX < info.Width && targetY < info.Height {
							finalImg.Set(targetX, targetY, img.At(px, py))
						}
					}
				}
				mu.Unlock()

				progressBar.Add(1)
			}(x, y)
		}
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	if err := <-errChan; err != nil {
		return nil, err
	}

	return finalImg, nil
}

func (d *IIIFDownloader) downloadAndMergeXMLTiles(ctx context.Context, info *IIIFXMLInfo, headers http.Header) (image.Image, error) {
	tileSize := info.TileSize
	overlap := info.Overlap

	// 计算有效瓦片尺寸（减去重叠部分）
	effectiveTileSize := tileSize - overlap*2

	// 调整行列数计算（考虑重叠）
	cols := (info.Size.Width + effectiveTileSize - 1) / effectiveTileSize
	rows := (info.Size.Height + effectiveTileSize - 1) / effectiveTileSize

	finalImg := image.NewRGBA(image.Rect(0, 0, info.Size.Width, info.Size.Height))
	progressBar := progressbar.Default(int64(cols*rows), "IIIF")

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, 1)

	maxLevel := d.getMaxZoomLevel(info.Size.Width, info.Size.Height, tileSize)

	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			wg.Add(1)
			go func(x, y int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				// 计算实际位置（考虑重叠）
				posX := x * effectiveTileSize
				posY := y * effectiveTileSize

				// 构建包含重叠区域的请求
				tileData := map[string]interface{}{
					"ServerURL": info.ServerURL,
					"Level":     maxLevel,
					"X":         x,
					"Y":         y,
					"Width":     tileSize, // 请求包含重叠的完整瓦片
					"Height":    tileSize,
					"Format":    info.Format,
				}

				tileURL, err := d.buildDeepZoomTileURL(tileData)
				if err != nil {
					select {
					case errChan <- fmt.Errorf("构建 tileURL 失败: %v", err):
					default:
					}
					return
				}

				img, err := d.downloadImage(ctx, tileURL, headers)
				if err != nil {
					select {
					case errChan <- fmt.Errorf("下载拼图(%d,%d)失败: %v", x, y, err):
					default:
					}
					return
				}

				mu.Lock()
				// 计算目标位置（跳过重叠部分）
				destX := posX
				destY := posY
				if x > 0 {
					destX += overlap
				} // 跳过左侧重叠
				if y > 0 {
					destY += overlap
				} // 跳过上方重叠

				// 复制有效像素区域
				bounds := img.Bounds()
				for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
					for px := bounds.Min.X; px < bounds.Max.X; px++ {
						targetX := destX + (px - bounds.Min.X)
						targetY := destY + (py - bounds.Min.Y)

						// 确保不超出图像边界
						if targetX < info.Size.Width && targetY < info.Size.Height {
							finalImg.Set(targetX, targetY, img.At(px, py))
						}
					}
				}
				mu.Unlock()

				progressBar.Add(1)
			}(x, y)
		}
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	if err := <-errChan; err != nil {
		return nil, err
	}

	return finalImg, nil
}

func (d *IIIFDownloader) getIIIFInfo(ctx context.Context, serverBaseURL, url string, headers http.Header) (*IIIFInfo, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", userAgent)
	}

	resp, err := d.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %v", err)
	}
	info, err := d.parseIIIFResponse(data)
	if err != nil {
		return nil, err
	}

	if len(info.Tiles) == 0 {
		return nil, fmt.Errorf("未找到拼图配置信息")
	}

	return info, nil
}

func (d *IIIFDownloader) getIIIFXMLInfo(ctx context.Context, serverBaseURL, url string, headers http.Header) (*IIIFXMLInfo, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", userAgent)
	}

	resp, err := d.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %v", err)
	}

	var info IIIFXMLInfo
	if err := xml.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("XML解析失败: %v", err)
	}

	imagePath, err := d.getImagePathFromXMLURL(url)
	if err != nil {
		return nil, err
	}
	info.ServerURL = serverBaseURL + "/" + imagePath

	return &info, nil
}

func (d *IIIFDownloader) downloadImage(ctx context.Context, url string, headers http.Header) (image.Image, error) {
	baseDelay := time.Second // 初始延迟1秒
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// 指数退避: 1s, 2s, 4s
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseDelay
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, fmt.Errorf("下载取消: %v", ctx.Err())
			}
		}

		// 创建新请求防止Body已关闭等问题
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			lastErr = fmt.Errorf("创建请求失败: %v", err)
			continue
		}

		// 设置headers
		for key, values := range headers {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", userAgent)
		}

		// 记录调试信息
		//fmt.Printf("尝试下载 (第%d次): %s\n", attempt+1, url)

		resp, err := d.client.Do(req.WithContext(ctx))
		if err != nil {
			lastErr = fmt.Errorf("请求失败: %v", err)
			if d.shouldRetry(err) {
				continue
			}
			return nil, lastErr
		}

		// 检查状态码
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("服务器返回错误状态码: %d\n响应体: %s", resp.StatusCode, string(body))

			// 404等错误不应重试
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return nil, lastErr
			}
			continue
		}

		// 成功读取图像
		imgData, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("读取图像数据失败: %v", err)
			continue
		}

		img, _, err := image.Decode(bytes.NewReader(imgData))
		if err != nil {
			lastErr = fmt.Errorf("解码图像失败: %v", err)
			// 可能是损坏的图像数据，重试可能有用
			continue
		}

		return img, nil
	}

	return nil, fmt.Errorf("下载失败(尝试%d次): %v", maxRetries, lastErr)
}

// 判断错误是否可重试
func (d *IIIFDownloader) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// 网络错误可以重试
	if _, ok := err.(net.Error); ok {
		return true
	}

	// 特定错误判断
	switch {
	case strings.Contains(err.Error(), "connection reset"),
		strings.Contains(err.Error(), "broken pipe"),
		strings.Contains(err.Error(), "timeout"):
		return true
	}

	return false
}

func (d *IIIFDownloader) downloadImage2(ctx context.Context, url string, headers http.Header) (image.Image, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", userAgent)
	}

	resp, err := d.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
	}

	imgData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取图像数据失败: %v", err)
	}

	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, fmt.Errorf("解码图像失败: %v", err)
	}

	return img, nil
}

func (d *IIIFDownloader) saveImage(img image.Image, path string) error {
	outFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer outFile.Close()

	switch ext := path[len(path)-4:]; ext {
	case ".jpg", "jpeg":
		return jpeg.Encode(outFile, img, &jpeg.Options{Quality: 90})
	case ".png":
		return png.Encode(outFile, img)
	default:
		return fmt.Errorf("不支持的图像格式: %s", ext)
	}
}

func (d *IIIFDownloader) argsToHeaders(args []string) (http.Header, error) {
	headers := make(http.Header)
	for i := 0; i < len(args); i++ {
		if args[i] == "-H" {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("缺少header值")
			}
			headerStr := args[i+1]
			i++
			parts := strings.SplitN(headerStr, ":", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("无效的header格式: %s", headerStr)
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			headers.Add(key, value)
		}
	}
	return headers, nil
}

func (d *IIIFDownloader) getIIIFInfoByURL(ctx context.Context, serverBaseURL, url string, headers http.Header) (interface{}, error) {
	ext := strings.ToLower(filepath.Ext(url))

	switch ext {
	case ".json":
		return d.getIIIFInfo(ctx, serverBaseURL, url, headers)
	case ".xml":
		return d.getIIIFXMLInfo(ctx, serverBaseURL, url, headers)
	default:
		info, err := d.getIIIFInfo(ctx, serverBaseURL, url, headers)
		if err == nil {
			return info, nil
		}
		return d.getIIIFXMLInfo(ctx, serverBaseURL, url, headers)
	}
}

func (d *IIIFDownloader) getImagePathFromXMLURL(xmlURL string) (string, error) {
	u, err := url.Parse(xmlURL)
	if err != nil {
		return "", err
	}

	path := strings.TrimSuffix(u.Path, "/image.xml")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		return "", fmt.Errorf("无法从URL提取图像路径")
	}

	return path, nil
}

func (d *IIIFDownloader) getMaxZoomLevel(width, height, tileSize int) int {
	maxDim := width
	if height > maxDim {
		maxDim = height
	}
	return 12
}

func (d *IIIFDownloader) calculateMaxZoomLevel(width, height, tileSize int) int {
	maxDim := width
	if height > maxDim {
		maxDim = height
	}

	level := 0
	for maxDim > tileSize {
		maxDim = (maxDim + 1) / 2
		level++
	}

	if level > 12 {
		return 12
	}
	return level
}

func (d *IIIFDownloader) parseIIIFResponse(data []byte) (*IIIFInfo, error) {
	var info IIIFInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	// 自动检测版本
	switch {
	case strings.Contains(info.Context, "iiif.io/api/image/2/"):
		info.version = 2
		info.baseURL = strings.TrimSuffix(info.ID, "/info.json")
		info.Id = info.ID // 兼容v3字段
	case strings.Contains(info.Context, "iiif.io/api/image/3/"):
		info.version = 3
		info.baseURL = strings.TrimSuffix(info.Id, "/info.json")
		info.ID = info.Id // 兼容v2字段
	default:
		return nil, fmt.Errorf("无法识别的IIIF版本: %s", info.Context)
	}

	return &info, nil
}

func (d *IIIFDownloader) needsUpscale(info *IIIFInfo, requestW, requestH int) bool {
	// 检查服务是否支持放大
	supportsUpscaling := false
	for _, feature := range info.ExtraFeatures {
		if strings.Contains(feature, "sizeUpscaling") {
			supportsUpscaling = true
			break
		}
	}

	// 当瓦片尺寸 < 原始尺寸时，表示需要放大
	return (requestW < info.Width || requestH < info.Height) && supportsUpscaling
}
