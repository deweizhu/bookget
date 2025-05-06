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
	"os"
	"path/filepath"
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

type Tile struct {
	X, Y  int
	Image image.Image
}

// IIIFXMLInfo 表示XML格式的IIIF信息
type IIIFXMLInfo struct {
	XMLName   xml.Name `xml:"Image"`
	TileSize  int      `xml:"TileSize,attr"`
	Overlap   int      `xml:"Overlap,attr"`
	Format    string   `xml:"Format,attr"`
	Width     int      `xml:"Size>Width"`
	Height    int      `xml:"Size>Height"`
	ServerURL string   // 需要从请求URL推断
}

func DezoomifyGo(ctx context.Context, infoURL string, outputPath string, args []string) error {
	// 转换为header
	headers, err := argsToHeaders(args)
	if err != nil {
		fmt.Printf("转换header失败: %v\n", err)
		return err
	}

	// 1. 获取图像信息
	info, err := getIIIFInfo(ctx, infoURL, headers)
	if err != nil {
		fmt.Printf("获取图像信息失败: %v\n", err)
		return err
	}

	// 2. 下载并合并拼图
	tileConfig := info.Tiles[0]
	cols := (info.Width + tileConfig.Width - 1) / tileConfig.Width
	rows := (info.Height + tileConfig.Height - 1) / tileConfig.Height

	finalImg, err := downloadAndMergeTiles(ctx, info, cols, rows, headers)
	if err != nil {
		fmt.Printf("处理拼图失败: %v\n", err)
		return err
	}

	// 3. 保存最终图像
	if err := saveImage(finalImg, outputPath); err != nil {
		fmt.Printf("保存图像失败: %v\n", err)
		return err
	}

	fmt.Printf("\n图像合并完成，已保存到 %s\n", outputPath)
	return nil
}

func getIIIFInfo(ctx context.Context, url string, headers http.Header) (*IIIFInfo, error) {
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

func getIIIFXMLInfo(ctx context.Context, url string, headers http.Header) (*IIIFXMLInfo, error) {
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

	// 从URL推断服务器基础路径
	info.ServerURL = url[:len(url)-len(filepath.Ext(url))]

	return &info, nil
}

func downloadAndMergeTiles(ctx context.Context, info *IIIFInfo, cols, rows int, headers http.Header) (image.Image, error) {
	tileConfig := info.Tiles[0]
	tileWidth := tileConfig.Width
	tileHeight := tileConfig.Height
	// 更安全的选择最高分辨率的方式
	level := 1
	//if len(tileConfig.ScaleFactors) > 0 {
	//	// scaleFactors通常按降序排列，第一个就是最高分辨率
	//	level = tileConfig.ScaleFactors[0]
	//}

	if len(tileConfig.ScaleFactors) > 0 {
		// 找到最小的缩放因子（最高分辨率）
		level = tileConfig.ScaleFactors[0]
		for _, sf := range tileConfig.ScaleFactors {
			if sf < level {
				level = sf
			}
		}
	}

	// 计算实际拼图尺寸（考虑缩放级别）
	actualTileWidth := tileWidth * level
	actualTileHeight := tileHeight * level
	// 创建最终图像（使用实际尺寸）
	finalImg := image.NewRGBA(image.Rect(0, 0, cols*actualTileWidth, rows*actualTileHeight))

	progressBar := progressbar.Default(int64(cols*rows), "IIIF")

	// 控制并发数的信号量
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, 1)

	// 下载并合并所有拼图
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			wg.Add(1)
			go func(x, y int) {
				defer wg.Done()

				// 获取信号量，控制并发数
				sem <- struct{}{}
				defer func() { <-sem }()

				// 下载拼图
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

				// 将拼图合并到最终图像
				mu.Lock()
				bounds := img.Bounds()
				for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
					for px := bounds.Min.X; px < bounds.Max.X; px++ {
						finalImg.Set(x*tileWidth+px, y*tileHeight+py, img.At(px, py))
					}
				}
				mu.Unlock()

				// 更新进度
				progressBar.Add(1)
			}(x, y)
		}
	}

	// 等待所有下载完成
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// 检查错误
	if err, ok := <-errChan; ok {
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
