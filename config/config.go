package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const configContent = `# 自动检测下载URL。可选值[0|1|2]，
# 0=默认 ，只下载支持的图书馆
# 1=图片专用交互式批量下载
# 2=IIIF 标准类型网站自动检测
auto-detect = 0

[paths]
# 下载文件存放目录，空值是当前目录
output = ""

# 指定cookie.txt文件路径
cookie = ""

# 指定localStorage.txt文件路径
local-storage = ""

[download]
# 指定文件扩展名[.jpg|.tif|.png|.pdf]等
extension = ".jpg"

# 最大线程数，0=自动识别CPU核数*2
threads = 1

# 最大并发连接数，0=自动识别CPU核数*2
concurrent = 8

# 下载限速 N 秒/任务，cuhk推荐5-60
speed = 0

# 下载重试次数
retries = 3

[custom]
# 页面范围，如4:434
sequence = ""

# 多册图书，只下第N册，或 3:6 即是3至6冊
volume = ""


# User-Agent
user-agent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:139.0) Gecko/20100101 Firefox/139.0"

# 只下载书签目录，可选值[0|1]。0=否，1=是。仅对 gj.tianyige.com.cn 有效。
bookmark = 0

# 下载的URLs，指定任意本地文件，例如：urls.txt
input = ""

[dzi]
# 使用 IIIF/DeepZoom 拼图下载
# 0 = 禁用，1=启用
dzi = 1

#JPG质量
quality = 80

# IIIF 图像请求 URI: {size}/{rotation}/{quality}.{format}
format = "full/full/0/default.jpg"
`

// CreateConfigIfNotExists 检查并创建配置文件
func CreateConfigIfNotExists(configPath string) error {
	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		dir := filepath.Dir(configPath)
		if dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}
		}

		// 文件不存在，创建并写入内容
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			return fmt.Errorf("创建配置文件失败: %w", err)
		}
		fmt.Printf("配置文件已创建: %s\n", configPath)
	} else if err != nil {
		// 其他错误
		return fmt.Errorf("检查配置文件失败: %w", err)
	} else {
		fmt.Printf("配置文件在: %s\n", configPath)
	}
	return nil
}
