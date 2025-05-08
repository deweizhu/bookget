package downloader

import (
	"bookget/config"
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
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"
)

// TileSizeFormat defines how tile sizes should be formatted in URLs

type IIIFInfo struct {
	Context  string `json:"@context"`
	Protocol string `json:"protocol,omitempty"` // v2
	Type     string `json:"type,omitempty"`     // v3
	ID       string `json:"@id"`                // v2
	Id       string `json:"id"`                 // v3
	Width    int    `json:"width"`
	Height   int    `json:"height"`

	// 使用自定义类型处理profile字段
	// Profile can be string, object, or array
	Profile json.RawMessage `json:"profile"`

	Qualities []string `json:"qualities,omitempty"`
	Formats   []string `json:"formats,omitempty"`

	// 兼容性字段
	Sizes []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"sizes,omitempty"`

	// v2 tiles
	Tiles []struct {
		Width        int   `json:"width"`
		Height       int   `json:"height,omitempty"`
		ScaleFactors []int `json:"scaleFactors"`
		Overlap      int   `json:"overlap,omitempty"`
	} `json:"tiles,omitempty"`

	// 内部计算字段
	// Computed fields
	version  int    // 2 or 3
	baseURL  string // base URL without info.json
	maxArea  int64  // from profile
	maxWidth int    // from profile
}

type ProfileInfo struct {
	Formats   []string `json:"formats,omitempty"`
	Qualities []string `json:"qualities,omitempty"`
	Supports  []string `json:"supports,omitempty"`
	MaxWidth  int      `json:"maxWidth,omitempty"`
	MaxHeight int      `json:"maxHeight,omitempty"`
	MaxArea   int64    `json:"maxArea,omitempty"`
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

	//config.ini传过来
	userAgent     string
	maxRetries    int
	jpgQuality    int
	maxConcurrent int
}

func NewIIIFDownloader(c *config.Input) *IIIFDownloader {
	// 创建自定义 Transport 忽略 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, _ := cookiejar.New(nil)

	dl := &IIIFDownloader{
		client:        &http.Client{Jar: jar, Transport: tr},
		userAgent:     c.UserAgent,
		maxRetries:    c.Retries,
		jpgQuality:    c.Quality,
		maxConcurrent: c.MaxConcurrent,
	}
	// 设置 v2 模板（支持简写尺寸和旧版字段名）
	//dl.SetIIIFTileFormat("{{.ID}}/{{.X}},{{.Y}},{{.Width}},{{.Height}}/{{.Width}},/0/default.{{.Format}}")

	// 或更完整的 v2 格式（包含协议声明）
	//dl.SetIIIFTileFormat("{{.ID}}/{{.X}},{{.Y}},{{.Width}},{{.Height}}/full/{{.Width}},/0/default.{{.Format}}")

	// 设置 v3 模板（严格尺寸和新版字段名）
	//dl.SetIIIFTileFormat("{{.ID}}/{{.X}},{{.Y}},{{.Width}},{{.Height}}/{{.Width}},{{.Height}}/0/default.{{.Format}}")

	// 或带区域参数的 v3 格式
	//dl.SetIIIFTileFormat("{{.ID}}/{{.X}},{{.Y}},{{.Width}},{{.Height}}/max/{{.Width}},{{.Height}}/0/default.{{.Format}}")

	//dl.SetIIIFTileFormat("{{.ID}}/{{.X}},{{.Y}},{{.Width}},{{.Height}}/{{if .sizeUpscaling}}^{{end}}{{.Width}},{{.Height}}/0/default.{{.Format}}")

	dl.SetDeepZoomTileFormat("{{.ServerURL}}/image_files/{{.Level}}/{{.X}}_{{.Y}}.{{.Format}}")

	return dl
}

func NewIIIFDownloaderDefault() *IIIFDownloader {
	// 创建自定义 Transport 忽略 SSL 验证
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	jar, _ := cookiejar.New(nil)

	dl := &IIIFDownloader{
		client:        &http.Client{Jar: jar, Transport: tr},
		userAgent:     userAgent,
		maxRetries:    maxRetries,
		jpgQuality:    JPGQuality,
		maxConcurrent: maxConcurrent,
	}
	// 设置 v2 模板（支持简写尺寸和旧版字段名）
	//dl.SetIIIFTileFormat("{{.ID}}/{{.X}},{{.Y}},{{.Width}},{{.Height}}/{{.Width}},/0/default.{{.Format}}")

	// 或更完整的 v2 格式（包含协议声明）
	//dl.SetIIIFTileFormat("{{.ID}}/{{.X}},{{.Y}},{{.Width}},{{.Height}}/full/{{.Width}},/0/default.{{.Format}}")

	// 设置 v3 模板（严格尺寸和新版字段名）
	//dl.SetIIIFTileFormat("{{.ID}}/{{.X}},{{.Y}},{{.Width}},{{.Height}}/{{.Width}},{{.Height}}/0/default.{{.Format}}")

	// 或带区域参数的 v3 格式
	//dl.SetIIIFTileFormat("{{.ID}}/{{.X}},{{.Y}},{{.Width}},{{.Height}}/max/{{.Width}},{{.Height}}/0/default.{{.Format}}")

	// 更新模板，在size部分支持 ^ 前缀
	//dl.SetIIIFTileFormat("{{.ID}}/{{.X}},{{.Y}},{{.Width}},{{.Height}}/{{if .sizeUpscaling}}^{{end}}{{.Width}},{{.Height}}/0/default.{{.Format}}")

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
		finalImg, err = d.downloadAndMergeTiles(ctx, v, headers)
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
		finalImg, err = d.downloadAndMergeTiles(ctx, v, headers)
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

		finalImg, err := d.downloadAndMergeTiles(ctx, &jsonInfo, headers)
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

func (d *IIIFDownloader) downloadAndMergeTiles(ctx context.Context, info *IIIFInfo, headers http.Header) (image.Image, error) {
	if len(info.Tiles) == 0 {
		return nil, fmt.Errorf("no tile configuration found")
	}

	tileConfig := info.Tiles[0]
	tileSize := Vec2d{
		x: tileConfig.Width,
		y: tileConfig.Height,
	}

	// Apply size constraints
	tileSize = d.cropTileSize(info, tileSize)

	// Calculate grid
	cols := int(math.Ceil(float64(info.Width) / float64(tileSize.x)))
	rows := int(math.Ceil(float64(info.Height) / float64(tileSize.y)))

	finalImg := image.NewRGBA(image.Rect(0, 0, info.Width, info.Height))
	progressBar := progressbar.Default(int64(cols*rows), "downloading tiles")

	sem := make(chan struct{}, d.maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, 1)

	quality := d.bestQuality(info)
	format := d.bestFormat(info)
	sizeFormat := d.preferredSizeFormat(info)

	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			wg.Add(1)

			go func(x, y int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				posX := x * tileSize.x
				posY := y * tileSize.y
				width := min(tileSize.x, info.Width-posX)
				height := min(tileSize.y, info.Height-posY)

				// 8构建瓦片URL参数
				tileData := map[string]interface{}{
					"ID":         info.ID,
					"X":          posX,
					"Y":          posY,
					"Width":      width,
					"Height":     height,
					"Format":     format,
					"Quality":    quality,
					"SizeFormat": sizeFormat,
					//"sizeUpscaling": d.needsUpscale(info, width, height),
				}

				// 构建完整的瓦片URL
				var tileURL string
				var err error
				if info.version == 3 {
					tileURL, err = d.buildIIIFv3TileURL(tileData)
				} else {
					tileURL, err = d.buildIIIFv2TileURL(tileData)
				}
				if err != nil {
					select {
					case errChan <- fmt.Errorf("build tile URL error: %v", err):
					default:
					}
					return
				}

				// 下载瓦片图像
				img, err := d.downloadImageWithRetry(ctx, tileURL, headers, d.maxRetries)
				if err != nil {
					select {
					case errChan <- fmt.Errorf("download tile(%d,%d) error: %v", x, y, err):
					default:
					}
					return
				}

				mu.Lock()
				bounds := img.Bounds()
				for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
					for px := bounds.Min.X; px < bounds.Max.X; px++ {
						targetX := posX + (px - bounds.Min.X)
						targetY := posY + (py - bounds.Min.Y)
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
	progressBar := progressbar.Default(int64(cols*rows), "downloading tiles")

	sem := make(chan struct{}, d.maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, 1)

	maxLevel := d.getMaxZoomLevel(info.Size.Width, info.Size.Height)

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
		req.Header.Set("User-Agent", d.userAgent)
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
		req.Header.Set("User-Agent", d.userAgent)
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
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header = headers.Clone()

	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", d.userAgent)
	}

	resp, err := d.client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		//404 || 500
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusInternalServerError {
			return nil, nil
		}
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
		return jpeg.Encode(outFile, img, &jpeg.Options{Quality: d.jpgQuality})
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

func (d *IIIFDownloader) getMaxZoomLevel(width, height int) int {
	// 1. 取最长边
	maxDim := width
	if height > maxDim {
		maxDim = height
	}

	// 2. 计算 log2(maxDim) 并向上取整
	level := 0
	for dim := 1; dim < maxDim; dim *= 2 {
		level++
	}

	// 3. 确保 level 最小为 0（即使是 1x1 图像）
	if level < 0 {
		level = 0
	}

	return level
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

//func (d *IIIFDownloader) needsUpscale(info *IIIFInfo, requestW, requestH int) bool {
//	// 检查服务是否支持放大
//	supportsUpscaling := false
//	for _, feature := range info.ExtraFeatures {
//		if strings.Contains(feature, "sizeUpscaling") {
//			supportsUpscaling = true
//			break
//		}
//	}
//
//	// 当瓦片尺寸 < 原始尺寸时，表示需要放大
//	return (requestW < info.Width || requestH < info.Height) && supportsUpscaling
//}

// 新增
func (d *IIIFDownloader) parseProfile(profile json.RawMessage) (*ProfileInfo, error) {
	var info ProfileInfo

	// Try to parse as string (profile reference)
	var profileRef string
	if err := json.Unmarshal(profile, &profileRef); err == nil {
		// TODO: Lookup predefined profile info
		return &info, nil
	}

	// Try to parse as ProfileInfo object
	if err := json.Unmarshal(profile, &info); err == nil {
		return &info, nil
	}

	// Try to parse as array of profiles
	var profiles []json.RawMessage
	if err := json.Unmarshal(profile, &profiles); err == nil {
		for _, p := range profiles {
			pi, err := d.parseProfile(p)
			if err != nil {
				continue
			}
			// Merge profile info
			info.Formats = append(info.Formats, pi.Formats...)
			info.Qualities = append(info.Qualities, pi.Qualities...)
			info.Supports = append(info.Supports, pi.Supports...)
			if pi.MaxWidth > 0 && (info.MaxWidth == 0 || pi.MaxWidth < info.MaxWidth) {
				info.MaxWidth = pi.MaxWidth
			}
			if pi.MaxHeight > 0 && (info.MaxHeight == 0 || pi.MaxHeight < info.MaxHeight) {
				info.MaxHeight = pi.MaxHeight
			}
			if pi.MaxArea > 0 && (info.MaxArea == 0 || pi.MaxArea < info.MaxArea) {
				info.MaxArea = pi.MaxArea
			}
		}
		return &info, nil
	}

	return &info, nil
}

func (d *IIIFDownloader) parseIIIFResponse(data []byte) (*IIIFInfo, error) {
	var info IIIFInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	// Detect version and set base URL
	switch {
	case strings.Contains(info.Context, "iiif.io/api/image/2/"):
		info.version = 2
		info.baseURL = strings.TrimSuffix(info.ID, "/info.json")
		info.Id = info.ID // Set v3 field
	case strings.Contains(info.Context, "iiif.io/api/image/3/"):
		info.version = 3
		info.baseURL = strings.TrimSuffix(info.Id, "/info.json")
		info.ID = info.Id // Set v2 field
	default:
		// Fallback to v2 if no context
		if info.Context == "" && info.Protocol == "http://iiif.io/api/image" {
			info.version = 2
			info.baseURL = strings.TrimSuffix(info.ID, "/info.json")
			info.Id = info.ID
		} else {
			return nil, fmt.Errorf("unsupported IIIF version")
		}
	}

	// Parse profile information
	if len(info.Profile) > 0 {
		profile, err := d.parseProfile(info.Profile)
		if err == nil {
			info.maxArea = profile.MaxArea
			info.maxWidth = profile.MaxWidth
		}
	}

	// Remove test IDs (like example.com)
	if matched, _ := regexp.MatchString(`^https?://((www\.)?example\.|localhost)`, info.ID); matched {
		info.ID = ""
	}

	return &info, nil
}

func (d *IIIFDownloader) bestQuality(info *IIIFInfo) string {
	profile, _ := d.parseProfile(info.Profile)
	allQualities := append(info.Qualities, profile.Qualities...)

	if len(allQualities) == 0 {
		return "default"
	}

	// Find the highest priority quality
	for _, q := range qualityOrder {
		for _, qual := range allQualities {
			if strings.EqualFold(qual, q) {
				return qual
			}
		}
	}

	return allQualities[0] // Fallback to first if none match
}

func (d *IIIFDownloader) bestFormat(info *IIIFInfo) string {
	profile, _ := d.parseProfile(info.Profile)
	allFormats := append(info.Formats, profile.Formats...)

	if len(allFormats) == 0 {
		return "jpg"
	}

	// Find the highest priority format
	for _, f := range formatOrder {
		for _, fmt := range allFormats {
			if strings.EqualFold(fmt, f) {
				return fmt
			}
		}
	}

	return allFormats[0] // Fallback to first if none match
}

func (d *IIIFDownloader) preferredSizeFormat(info *IIIFInfo) TileSizeFormat {
	profile, _ := d.parseProfile(info.Profile)
	for _, s := range profile.Supports {
		if s == "sizeByW" {
			return Width
		}
	}
	return WidthHeight
}

func (d *IIIFDownloader) cropTileSize(info *IIIFInfo, size Vec2d) Vec2d {
	profile, _ := d.parseProfile(info.Profile)
	if profile.MaxWidth > 0 {
		size.x = min(size.x, profile.MaxWidth)
		size.y = min(size.y, profile.MaxHeight)
	}
	if profile.MaxArea > 0 && int64(size.x*size.y) > profile.MaxArea {
		sqrt := int(math.Sqrt(float64(profile.MaxArea)))
		size.x = min(size.x, sqrt)
		size.y = min(size.y, sqrt)
	}
	return size
}

func (d *IIIFDownloader) downloadImageWithRetry(ctx context.Context, url string, headers http.Header, maxRetries int) (image.Image, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		img, err := d.downloadImage(ctx, url, headers)
		if err == nil {
			return img, nil
		}
		lastErr = err
		time.Sleep(time.Second * time.Duration(i+1))
	}
	return nil, lastErr
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

func (d *IIIFDownloader) buildIIIFv2TileURL(data map[string]interface{}) (string, error) {
	// 如果设置了自定义模板，优先使用模板
	if d.iiifTileFormat.compiledTemplate != nil {
		return d.buildIIIFTileURL(data)
	}

	// 默认的IIIF v2格式: {id}/{region}/{size}/{rotation}/{quality}.{format}
	size := fmt.Sprintf("%d,", data["Width"].(int))
	if format, ok := data["SizeFormat"].(TileSizeFormat); ok && format == WidthHeight {
		size = fmt.Sprintf("%d,%d", data["Width"].(int), data["Height"].(int))
	}

	// 确保有必要的字段
	if _, ok := data["Quality"]; !ok {
		data["Quality"] = "default"
	}
	if _, ok := data["Format"]; !ok {
		data["Format"] = "jpg"
	}

	return fmt.Sprintf("%s/%d,%d,%d,%d/%s/0/%s.%s",
		data["ID"].(string),
		data["X"].(int),
		data["Y"].(int),
		data["Width"].(int),
		data["Height"].(int),
		size,
		data["Quality"].(string),
		data["Format"].(string),
	), nil
}

func (d *IIIFDownloader) buildIIIFv3TileURL(data map[string]interface{}) (string, error) {
	// 如果设置了自定义模板，优先使用模板
	if d.iiifTileFormat.compiledTemplate != nil {
		return d.buildIIIFTileURL(data)
	}

	// 默认的IIIF v3格式: {id}/{region}/{size}/{rotation}/{quality}.{format}
	// 确保有必要的字段
	if _, ok := data["Quality"]; !ok {
		data["Quality"] = "default"
	}
	if _, ok := data["Format"]; !ok {
		data["Format"] = "jpg"
	}

	size := fmt.Sprintf("%d,%d", data["Width"].(int), data["Height"].(int))
	return fmt.Sprintf("%s/%d,%d,%d,%d/%s/0/%s.%s",
		data["ID"].(string),
		data["X"].(int),
		data["Y"].(int),
		data["Width"].(int),
		data["Height"].(int),
		size,
		data["Quality"].(string),
		data["Format"].(string),
	), nil
}

// 通用的模板构建函数（供v2和v3共用）
func (d *IIIFDownloader) buildIIIFTileURL(data map[string]interface{}) (string, error) {
	// 自动填充基础URL（如果未提供）
	if _, ok := data["ServerBaseURL"]; !ok {
		if id, ok := data["ID"].(string); ok {
			if u, err := url.Parse(id); err == nil {
				data["ServerBaseURL"] = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
			}
		}
	}

	// 确保有必要的字段
	if _, ok := data["Quality"]; !ok {
		data["Quality"] = "default"
	}
	if _, ok := data["Format"]; !ok {
		data["Format"] = "jpg"
	}

	var buf bytes.Buffer
	if err := d.iiifTileFormat.compiledTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("模板渲染失败: %v", err)
	}

	// 确保返回完整URL
	result := buf.String()
	if !strings.HasPrefix(result, "http") {
		if base, ok := data["ServerBaseURL"].(string); ok {
			result = strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(result, "/")
		}
	}

	return result, nil
}
