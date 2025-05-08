package downloader

const (
	maxConcurrent = 16 // 最大并发下载数
	userAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:139.0) Gecko/20100101 Firefox/139.0"
	minFileSize   = 1024 // 最小文件大小(1KB)

	maxRetries = 3
	JPGQuality = 90
)

// 在 downloader.go 或相关文件中添加
type Vec2d struct {
	x int // 或 float64 根据需求
	y int // 或 float64
}

// 可选：添加构造函数
func NewVec2d(x, y int) Vec2d {
	return Vec2d{x: x, y: y}
}

// 可选：添加常用方法
func (v Vec2d) Width() int  { return v.x }
func (v Vec2d) Height() int { return v.y }

type TileSizeFormat int

const (
	WidthHeight TileSizeFormat = iota // "width,height"
	Width                             // "width,"
)

// Quality preference order (least to most preferred)
var qualityOrder = []string{"default", "native"}

// Format preference order (least to most preferred)
var formatOrder = []string{"jpg", "png"}
