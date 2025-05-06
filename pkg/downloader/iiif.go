package downloader

import (
	"bookget/pkg/progressbar"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type IIIFInfo struct {
	Context  string `json:"@context"`
	ID       string `json:"@id"`
	Protocol string `json:"protocol"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Sizes    []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"sizes"`
	Tiles []struct {
		Width        int   `json:"width"`
		Height       int   `json:"height"`
		ScaleFactors []int `json:"scaleFactors"`
	} `json:"tiles"`
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
	ServerURL string
}

type Tile struct {
	X, Y  int
	Image image.Image
}

func DezoomifyGo(ctx context.Context, infoURL string, outputPath string, args []string) error {
	// 转换为header
	headers, err := argsToHeaders(args)
	if err != nil {
		return fmt.Errorf("转换header失败: %v", err)
	}

	// 1. 根据URL后缀获取图像信息
	info, err := getIIIFInfoByURL(ctx, "", infoURL, headers)
	if err != nil {
		return fmt.Errorf("获取图像信息失败: %v", err)
	}

	// 2. 下载并合并拼图
	var finalImg image.Image
	switch v := info.(type) {
	case *IIIFInfo:
		tileConfig := v.Tiles[0]
		cols := (v.Width + tileConfig.Width - 1) / tileConfig.Width
		rows := (v.Height + tileConfig.Height - 1) / tileConfig.Height
		finalImg, err = downloadAndMergeTiles(ctx, v, cols, rows, headers)
	case *IIIFXMLInfo:
		finalImg, err = downloadAndMergeXMLTiles(ctx, v, headers)
	default:
		return fmt.Errorf("未知的图像信息格式")
	}

	if err != nil {
		return fmt.Errorf("处理拼图失败: %v", err)
	}

	// 3. 保存最终图像
	if err := saveImage(finalImg, outputPath); err != nil {
		return fmt.Errorf("保存图像失败: %v", err)
	}

	fmt.Printf("\n图像合并完成，已保存到 %s\n", outputPath)
	return nil
}

func DezoomifyGo2(ctx context.Context, serverBaseURL, xmlURL string, outputPath string, args []string) error {
	// 转换为header
	headers, err := argsToHeaders(args)
	if err != nil {
		return fmt.Errorf("转换header失败: %v", err)
	}

	// 1. 根据URL后缀获取图像信息
	info, err := getIIIFInfoByURL(ctx, serverBaseURL, xmlURL, headers)
	if err != nil {
		return fmt.Errorf("获取图像信息失败: %v", err)
	}

	// 2. 下载并合并拼图
	var finalImg image.Image
	switch v := info.(type) {
	case *IIIFInfo:
		tileConfig := v.Tiles[0]
		cols := (v.Width + tileConfig.Width - 1) / tileConfig.Width
		rows := (v.Height + tileConfig.Height - 1) / tileConfig.Height
		finalImg, err = downloadAndMergeTiles(ctx, v, cols, rows, headers)
	case *IIIFXMLInfo:
		finalImg, err = downloadAndMergeXMLTiles(ctx, v, headers)
	default:
		return fmt.Errorf("未知的图像信息格式")
	}

	if err != nil {
		return fmt.Errorf("处理拼图失败: %v", err)
	}

	// 3. 保存最终图像
	if err := saveImage(finalImg, outputPath); err != nil {
		return fmt.Errorf("保存图像失败: %v", err)
	}

	fmt.Printf("\n图像合并完成，已保存到 %s\n", outputPath)
	return nil
}

func downloadAndMergeXMLTiles(ctx context.Context, info *IIIFXMLInfo, headers http.Header) (image.Image, error) {
	tileSize := info.TileSize
	cols := (info.Size.Width + tileSize - 1) / tileSize
	rows := (info.Size.Height + tileSize - 1) / tileSize

	finalImg := image.NewRGBA(image.Rect(0, 0, info.Size.Width, info.Size.Height))
	progressBar := progressbar.Default(int64(cols*rows), "IIIF")

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, 1)

	// 使用固定最大级别12，或使用calculateMaxZoomLevel计算结果
	maxLevel := 12 // 或 calculateMaxZoomLevel(info.Size.Width, info.Size.Height, tileSize)

	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			wg.Add(1)
			go func(x, y int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				// 使用新的URL构建方式
				tileURL := buildTileURL(info.ServerURL, maxLevel, x, y, info.Format)

				img, err := downloadImage(ctx, tileURL, headers)
				if err != nil {
					select {
					case errChan <- fmt.Errorf("下载拼图(%d,%d)失败: %v", x, y, err):
					default:
					}
					return
				}

				mu.Lock()
				bounds := img.Bounds()
				for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
					for px := bounds.Min.X; px < bounds.Max.X; px++ {
						finalImg.Set(x*tileSize+px, y*tileSize+py, img.At(px, py))
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

func getIIIFInfo(ctx context.Context, serverBaseURL, url string, headers http.Header) (*IIIFInfo, error) {
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

	client := &http.Client{}
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务器返回错误状态码: %d", resp.StatusCode)
	}

	var info IIIFInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %v", err)
	}

	if len(info.Tiles) == 0 {
		return nil, fmt.Errorf("未找到拼图配置信息")
	}

	return &info, nil
}

func getIIIFXMLInfo(ctx context.Context, serverBaseURL, url string, headers http.Header) (*IIIFXMLInfo, error) {
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

	client := &http.Client{}
	resp, err := client.Do(req.WithContext(ctx))
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

	// 使用传入的serverBaseURL
	imagePath, err := getImagePathFromXMLURL(url)
	if err != nil {
	}
	info.ServerURL = serverBaseURL + "/" + imagePath

	return &info, nil
}

func downloadAndMergeTiles(ctx context.Context, info *IIIFInfo, cols, rows int, headers http.Header) (image.Image, error) {
	tileConfig := info.Tiles[0]
	tileWidth := tileConfig.Width
	tileHeight := tileConfig.Height

	// 选择最高分辨率
	level := 1
	if len(tileConfig.ScaleFactors) > 0 {
		level = tileConfig.ScaleFactors[0]
		for _, sf := range tileConfig.ScaleFactors {
			if sf < level {
				level = sf
			}
		}
	}

	actualTileWidth := tileWidth * level
	actualTileHeight := tileHeight * level
	finalImg := image.NewRGBA(image.Rect(0, 0, cols*actualTileWidth, rows*actualTileHeight))
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

				tileURL := fmt.Sprintf("%s/%d,%d,%d,%d/%d,%d/0/default.jpg",
					info.ID,
					x*tileWidth,
					y*tileHeight,
					tileWidth,
					tileHeight,
					tileWidth,
					tileHeight,
				)

				img, err := downloadImage(ctx, tileURL, headers)
				if err != nil {
					select {
					case errChan <- fmt.Errorf("下载拼图(%d,%d)失败: %v", x, y, err):
					default:
					}
					return
				}

				mu.Lock()
				bounds := img.Bounds()
				for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
					for px := bounds.Min.X; px < bounds.Max.X; px++ {
						finalImg.Set(x*tileWidth+px, y*tileHeight+py, img.At(px, py))
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

func downloadImage(ctx context.Context, url string, headers http.Header) (image.Image, error) {
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

	client := &http.Client{}
	resp, err := client.Do(req.WithContext(ctx))
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

func saveImage(img image.Image, path string) error {
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

func argsToHeaders(args []string) (http.Header, error) {
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

// 根据URL后缀自动选择解析器
func getIIIFInfoByURL(ctx context.Context, serverBaseURL, url string, headers http.Header) (interface{}, error) {
	// 获取URL后缀
	ext := strings.ToLower(filepath.Ext(url))

	switch ext {
	case ".json":
		return getIIIFInfo(ctx, serverBaseURL, url, headers)
	case ".xml":
		return getIIIFXMLInfo(ctx, serverBaseURL, url, headers)
	default:
		// 默认先尝试JSON，失败后再尝试XML
		info, err := getIIIFInfo(ctx, serverBaseURL, url, headers)
		if err == nil {
			return info, nil
		}
		return getIIIFXMLInfo(ctx, serverBaseURL, url, headers)
	}
}

func buildTileURL(serverURL string, level, x, y int, format string) string {
	// 示例:
	// serverURL = "https://sg30p0.familysearch.org/service/records/storage/deepzoomcloud/dz/v1/3:1:3QS7-89DL-LK41"
	// 结果: https://sg30p0.familysearch.org/service/records/storage/deepzoomcloud/dz/v1/3:1:3QS7-89DL-LK41/image_files/12/1_3.jpg

	return fmt.Sprintf("%s/image_files/%d/%d_%d.%s",
		serverURL,
		level,
		x, y,
		format)
}

func getImagePathFromXMLURL(xmlURL string) (string, error) {
	// 从XML URL中提取路径部分
	// 示例: https://www.familysearch.org/service/records/storage/deepzoomcloud/dz/v1/3:1:3QS7-L9DL-LK1X/image.xml
	// 返回: service/records/storage/deepzoomcloud/dz/v1/3:1:3QS7-L9DL-LK1X

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

// 修改后的getZoomLevel函数
func getMaxZoomLevel(width, height, tileSize int) int {
	maxDim := width
	if height > maxDim {
		maxDim = height
	}

	// Deep Zoom通常使用固定级别，最高级别为12
	// 即使计算值小于12，也可能有更高级别
	return 12
}

// 或者更精确的计算方法（如果需要）
func calculateMaxZoomLevel(width, height, tileSize int) int {
	maxDim := width
	if height > maxDim {
		maxDim = height
	}

	level := 0
	for maxDim > tileSize {
		maxDim = (maxDim + 1) / 2 // Deep Zoom是每次缩小一半
		level++
	}

	// 确保不超过服务器支持的最大级别
	if level > 12 {
		return 12
	}
	return level
}
