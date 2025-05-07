package config

import (
	"context"
	"flag"
	"fmt"
	"gopkg.in/ini.v1"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type Input struct {
	DUrl         string //单个输入URL
	UrlsFile     string //输入urls.txt
	CookieFile   string //输入cookie.txt
	LocalStorage string //localStorage.txt
	Seq          string //页面范围 4:434
	SeqStart     int
	SeqEnd       int
	Volume       string //册范围 4:434
	VolStart     int
	VolEnd       int

	Speed      int    //限速
	SaveFolder string //下载文件存放目录，默认为当前文件夹下 Downloads 目录下
	//;生成 dezoomify-rs 可用的文件(默认生成文件名 dezoomify-rs.urls.txt）
	// ;0 = 禁用，1=启用 （只对支持的图书馆有效）
	Format        string //;全高清图下载时，指定宽度像素（16开纸185mm*260mm，像素2185*3071）
	UserAgent     string //自定义UserAgent
	AutoDetect    int    //自动检测下载URL。可选值[0|1|2]，;0=默认;1=通用批量下载（类似IDM、迅雷）;2= IIIF manifest.json 自动检测下载图片
	UseDzi        bool   //启用Dezoomify下载IIIF
	FileExt       string //指定下载的扩展名
	Threads       int
	MaxConcurrent int
	Retries       int           //重试次数
	Quality       int           //JPG品质
	Timeout       time.Duration //超时秒数
	Bookmark      bool          //只下載書簽目錄（浙江寧波天一閣）

	Help    bool
	Version bool
}

func Init(ctx context.Context) bool {

	dir, _ := os.Getwd()

	//你们为什么没有良好的电脑使用习惯？中文虽好，但不适用于计算机。
	if os.PathSeparator == '\\' {
		matched, _ := regexp.MatchString(`([^A-z0-9_\\/\-:.]+)`, dir)
		if matched {
			fmt.Println("本软件存放目录，不能包含空格、中文等特殊符号。推荐：D:\\bookget")
			fmt.Println("按回车键终止程序。Press Enter to exit ...")
			endKey := make([]byte, 1)
			os.Stdin.Read(endKey)
			os.Exit(0)
		}
	}
	iniConf, _ := initINI()

	flag.StringVar(&Conf.UrlsFile, "input", iniConf.UrlsFile, "下载的URLs，指定任意本地文件，例如：urls.txt")
	flag.StringVar(&Conf.SaveFolder, "output", iniConf.SaveFolder, "下载保存到目录")
	flag.StringVar(&Conf.Seq, "sequence", iniConf.Seq, "页面范围，如4:434")
	flag.StringVar(&Conf.Volume, "volume", iniConf.Volume, "多册图书，如10:20册，只下载10至20册")
	flag.StringVar(&Conf.Format, "format", iniConf.Format, "IIIF 图像请求URI: full/full/0/default.jpg")
	flag.StringVar(&Conf.UserAgent, "user-agent", iniConf.UserAgent, "user-agent")
	flag.BoolVar(&Conf.Bookmark, "bookmark", iniConf.Bookmark, "只下载书签目录，可选值[0|1]。0=否，1=是。仅对 gj.tianyige.com.cn 有效。")
	flag.BoolVar(&Conf.UseDzi, "dzi", iniConf.UseDzi, "使用 IIIF/DeepZoom 拼图下载")
	flag.StringVar(&Conf.CookieFile, "cookie", iniConf.CookieFile, "指定cookie.txt文件路径")
	flag.StringVar(&Conf.LocalStorage, "local-storage", iniConf.LocalStorage, "指定localStorage.txt文件路径")
	flag.StringVar(&Conf.FileExt, "extension", iniConf.FileExt, "指定文件扩展名[.jpg|.tif|.png]等")
	flag.IntVar(&Conf.Threads, "threads", iniConf.Threads, "最大线程数")
	flag.IntVar(&Conf.MaxConcurrent, "concurrent", iniConf.MaxConcurrent, "最大并发任务数")
	flag.IntVar(&Conf.Speed, "speed", iniConf.Speed, "下载限速 N 秒/任务，cuhk推荐5-60")
	flag.IntVar(&Conf.Retries, "retries", iniConf.Retries, "下载重试次数")
	flag.DurationVar(&Conf.Timeout, "timeout", iniConf.Timeout, "下载重试次数")
	flag.IntVar(&Conf.AutoDetect, "auto-detect", iniConf.AutoDetect, "自动检测下载URL。可选值[0|1|2]，;0=默认;\n1=通用批量下载（类似IDM、迅雷）;\n2= IIIF manifest.json 自动检测下载图片")
	flag.BoolVar(&Conf.Help, "help", false, "显示帮助")
	flag.BoolVar(&Conf.Version, "version", false, "显示版本 -v")
	flag.Parse()

	k := len(os.Args)
	if k == 2 {
		if os.Args[1] == "-v" || os.Args[1] == "--version" {
			printVersion()
			return false
		}
		if os.Args[1] == "-h" || os.Args[1] == "--help" {
			printHelp()
			return false
		}
	}
	v := flag.Arg(0)
	if strings.HasPrefix(v, "http") {
		Conf.DUrl = v
	}
	if Conf.UrlsFile != "" && !strings.Contains(Conf.UrlsFile, string(os.PathSeparator)) {
		Conf.UrlsFile = dir + string(os.PathSeparator) + Conf.UrlsFile
	}
	//fmt.Printf("%+v", Conf)
	if Conf.Speed > 60 {
		Conf.Speed = 60
	}
	initSeqRange()
	initVolumeRange()
	//保存目录处理
	_ = os.Mkdir(Conf.SaveFolder, os.ModePerm)
	_ = os.Mkdir(CacheDir(), os.ModePerm)
	return true
}

func initINI() (Input, error) {

	// 获取路径相关配置
	dir, err := os.Getwd()
	if err != nil {
		return Input{}, fmt.Errorf("获取当前工作目录失败: %w", err)
	}

	fPath, err := os.Executable()
	if err != nil {
		return Input{}, fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	binDir := filepath.Dir(fPath)

	// 确定配置文件路径
	configPath, err := determineConfigPath(binDir)
	if err != nil {
		return Input{}, fmt.Errorf("确定配置文件路径失败: %w", err)
	}

	// 创建配置文件（如果不存在）
	if err := CreateConfigIfNotExists(configPath); err != nil {
		return Input{}, fmt.Errorf("创建配置文件失败: %w", err)
	}

	// 初始化默认输入结构
	c := runtime.NumCPU() * 2
	io := Input{
		DUrl:          "",
		UrlsFile:      filepath.Join(dir, "urls.txt"),
		CookieFile:    filepath.Join(dir, "cookie.txt"),
		LocalStorage:  filepath.Join(dir, "localStorage.txt"),
		Seq:           "",
		SeqStart:      0,
		SeqEnd:        0,
		Volume:        "",
		VolStart:      0,
		VolEnd:        0,
		Speed:         0,
		SaveFolder:    dir,
		Format:        defaultFormat,
		UserAgent:     defaultUserAgent,
		AutoDetect:    0,
		UseDzi:        true,
		FileExt:       defaultFileExtension,
		Threads:       1,
		MaxConcurrent: c,
		Retries:       defaultRetry,
		Timeout:       defaultTimeout,
		Bookmark:      false,
		Help:          false,
		Version:       false,
	}

	// 加载并解析配置文件
	cfg, err := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, configPath)
	if err != nil {
		return Input{}, fmt.Errorf("加载配置文件失败: %w", err)
	}

	// 从配置文件更新配置
	updateConfigFromINI(cfg, &io, dir, c)

	return io, nil
}

// updateConfigFromINI 从INI文件更新配置
func updateConfigFromINI(cfg *ini.File, io *Input, defaultDir string, defaultConcurrency int) {
	// 自动检测模式
	io.AutoDetect = cfg.Section("").Key("auto-detect").MustInt(0)

	// 路径相关配置
	pathsSection := cfg.Section("paths")
	io.SaveFolder = pathsSection.Key("output").MustString(defaultDir)
	io.CookieFile = pathsSection.Key("cookie").MustString(io.CookieFile)
	io.LocalStorage = pathsSection.Key("local-storage").MustString(io.LocalStorage)
	io.UrlsFile = pathsSection.Key("input").MustString(io.UrlsFile)

	// 下载相关配置
	secDown := cfg.Section("download")
	io.FileExt = secDown.Key("extension").MustString(io.FileExt)
	io.Threads = secDown.Key("threads").MustInt(defaultConcurrency)
	io.MaxConcurrent = secDown.Key("concurrent").MustInt(defaultConcurrency)
	io.Speed = secDown.Key("speed").MustInt(io.Speed)
	io.Retries = secDown.Key("retries").MustInt(io.Retries)
	io.Timeout = secDown.Key("timeout").MustDuration(io.Timeout)

	// 自定义配置
	secCus := cfg.Section("custom")
	io.Seq = secCus.Key("sequence").String()
	io.Volume = secCus.Key("volume").String()
	io.Bookmark = secCus.Key("bookmark").MustBool(io.Bookmark)
	io.UserAgent = secCus.Key("user-agent").MustString(io.UserAgent)

	// DZI相关配置
	secDzi := cfg.Section("dzi")
	io.UseDzi = secDzi.Key("dzi").MustBool(io.UseDzi)
	io.Quality = secDown.Key("quality").MustInt(defaultQuality)
	io.Format = secDzi.Key("format").MustString(io.Format)
}

// determineConfigPath 确定配置文件路径（跨平台兼容）
func determineConfigPath(binDir string) (string, error) {
	// 定义可能的配置文件位置（按优先级排序）
	var possiblePaths []string

	// 1. 当前目录下的 config.ini
	currentDir, err := os.Getwd()
	if err == nil {
		possiblePaths = append(possiblePaths, filepath.Join(currentDir, "config.ini"))
	}

	// 2. 用户主目录下的配置文件
	if home, err := os.UserHomeDir(); err == nil {
		// Unix-like: ~/.config/bookget/config.ini
		// Windows: ~\bookget\config.ini
		configDir := filepath.Join(home, ".config", "bookget")
		if string(os.PathSeparator) == "\\" { // Windows
			configDir = filepath.Join(home, "bookget")
		}
		possiblePaths = append(possiblePaths, filepath.Join(configDir, "config.ini"))
	}

	// 3. 系统级配置文件
	if string(os.PathSeparator) == "/" { // Unix-like
		possiblePaths = append(possiblePaths, filepath.Join("/", "etc", "bookget", "config.ini"))
	} else { // Windows
		if appData := os.Getenv("APPDATA"); appData != "" {
			possiblePaths = append(possiblePaths, filepath.Join(appData, "bookget", "config.ini"))
		}
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			possiblePaths = append(possiblePaths, filepath.Join(localAppData, "bookget", "config.ini"))
		}
	}

	// 4. 二进制文件所在目录的配置文件
	possiblePaths = append(possiblePaths, filepath.Join(binDir, "config.ini"))

	// 检查这些路径中哪个配置文件存在且不为空
	for _, path := range possiblePaths {
		if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
			return path, nil
		}
	}

	// 如果没有找到现有配置文件，返回用户主目录的配置文件路径（将在此处创建）
	// 如果无法获取用户主目录，则返回二进制目录的路径
	if home, err := os.UserHomeDir(); err == nil {
		configDir := filepath.Join(home, ".config", "bookget")
		if string(os.PathSeparator) == "\\" {
			configDir = filepath.Join(home, "bookget")
		}

		// 确保目录存在
		if err := os.MkdirAll(configDir, 0755); err == nil {
			return filepath.Join(configDir, "config.ini"), nil
		}
	}

	return filepath.Join(binDir, "config.ini"), nil
}

func printHelp() {
	printVersion()
	fmt.Println(`Usage: bookget [OPTION]... [URL]...`)
	flag.PrintDefaults()
	fmt.Println("Originally written by zhudw <zhudwi@outlook.com>.")
	fmt.Println("https://github.com/deweizhu/bookget/")
}

func printVersion() {
	fmt.Printf("bookget v%s\n", Version)
}
