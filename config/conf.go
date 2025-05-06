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
	DezoomifyPath string //dezoomify-rs 本地目录位置
	DezoomifyRs   string //dezoomify-rs 参数
	UseDziRs      bool   //启用DezoomifyRs下载IIIF
	FileExt       string //指定下载的扩展名
	Threads       int
	MaxConcurrent int
	Retry         int           //重试次数
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
	flag.BoolVar(&Conf.UseDziRs, "dezoomify-rs", iniConf.UseDziRs, "使用dezoomify-rs下载，仅对支持iiif的网站生效。")
	flag.StringVar(&Conf.CookieFile, "cookie", iniConf.CookieFile, "指定cookie.txt文件路径")
	flag.StringVar(&Conf.LocalStorage, "local-storage", iniConf.LocalStorage, "指定localStorage.txt文件路径")
	flag.StringVar(&Conf.FileExt, "extension", iniConf.FileExt, "指定文件扩展名[.jpg|.tif|.png]等")
	flag.IntVar(&Conf.Threads, "threads", iniConf.Threads, "最大线程数")
	flag.IntVar(&Conf.MaxConcurrent, "concurrent", iniConf.MaxConcurrent, "最大并发任务数")
	flag.IntVar(&Conf.Speed, "speed", iniConf.Speed, "下载限速 N 秒/任务，cuhk推荐5-60")
	flag.IntVar(&Conf.Retry, "retry", iniConf.Retry, "下载重试次数")
	flag.DurationVar(&Conf.Timeout, "timeout", iniConf.Timeout, "下载重试次数")
	flag.IntVar(&Conf.AutoDetect, "auto-detect", iniConf.AutoDetect, "自动检测下载URL。可选值[0|1|2]，;0=默认;\n1=通用批量下载（类似IDM、迅雷）;\n2= IIIF manifest.json 自动检测下载图片")
	flag.BoolVar(&Conf.Help, "help", false, "显示帮助")
	flag.BoolVar(&Conf.Version, "version", false, "显示版本 -v")
	flag.StringVar(&Conf.DezoomifyRs, "dezoomify-rs-args", iniConf.DezoomifyRs, "dezoomify-rs 参数")
	Conf.DezoomifyPath = iniConf.DezoomifyPath
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

func initINI() (io Input, err error) {
	dir, _ := os.Getwd()
	fPath, _ := os.Executable()
	binDir := filepath.Dir(fPath)
	var configPath string
	fi, err := os.Stat("/etc/bookget/config.ini")
	if string(os.PathSeparator) == "/" && err == nil && fi.Size() > 0 {
		configPath = "/etc/bookget/config.ini"
	} else {
		configPath = binDir + string(os.PathSeparator) + "config.ini"
	}

	if err := CreateConfigIfNotExists(configPath); err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}

	cFile := dir + string(os.PathSeparator) + "cookie.txt"
	urls := dir + string(os.PathSeparator) + "urls.txt"
	localStorage := dir + string(os.PathSeparator) + "localStorage.txt"
	c := runtime.NumCPU() * 2

	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/118.0"
	format := "full/full/0/default.jpg"
	io = Input{
		DUrl:          "",
		UrlsFile:      urls,
		CookieFile:    cFile,
		LocalStorage:  localStorage,
		Seq:           "",
		SeqStart:      0,
		SeqEnd:        0,
		Volume:        "",
		VolStart:      0,
		VolEnd:        0,
		Speed:         0,
		SaveFolder:    dir,
		Format:        format,
		UserAgent:     ua,
		AutoDetect:    0,
		DezoomifyPath: "",
		DezoomifyRs:   "-l --compression 20",
		UseDziRs:      false,
		FileExt:       ".jpg",
		Threads:       1,
		MaxConcurrent: c,
		Retry:         3,
		Bookmark:      false,
		Help:          false,
		Version:       false,
	}

	if os.PathSeparator == '\\' {
		io.DezoomifyPath = "dezoomify-rs.exe"
		if fi, err := os.Stat(dir + "\\dezoomify-rs.exe"); err == nil && fi.Size() > 0 {
			io.DezoomifyPath = dir + "\\dezoomify-rs.exe"
		}
	} else {
		io.DezoomifyPath = "dezoomify-rs"
		if fi, err := os.Stat(dir + "/dezoomify-rs"); err == nil && fi.Size() > 0 {
			io.DezoomifyPath = dir + "/dezoomify-rs"
		}
	}

	cfg, err := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, configPath)
	if err != nil {
		return
	}

	// 读取自动检测模式
	io.AutoDetect = cfg.Section("").Key("auto-detect").MustInt(0)

	// 读取输出目录设置
	io.SaveFolder = cfg.Section("paths").Key("output").String()
	if io.SaveFolder == "" {
		io.SaveFolder = dir
	}

	// 读取cookie和localStorage路径
	io.CookieFile = cfg.Section("paths").Key("cookie").String()
	io.LocalStorage = cfg.Section("paths").Key("local-storage").String()

	// 读取下载相关设置
	secDown := cfg.Section("download")
	io.FileExt = secDown.Key("extension").String()
	io.Threads = secDown.Key("threads").MustInt(c)
	if io.Threads == 0 {
		io.Threads = c
	}
	io.MaxConcurrent = secDown.Key("concurrent").MustInt(c)
	if io.MaxConcurrent == 0 {
		io.MaxConcurrent = c
	}
	io.Speed = secDown.Key("speed").MustInt(c)
	io.Retry = secDown.Key("retry").MustInt(3)            // 默认重试3次
	io.Timeout = secDown.Key("timeout").MustDuration(300) // 默认重试300秒

	// 读取自定义设置
	secCus := cfg.Section("custom")
	io.Seq = secCus.Key("sequence").String()
	io.Volume = secCus.Key("volume").String()
	io.Bookmark = secCus.Key("bookmark").MustBool(false)
	io.UserAgent = secCus.Key("user-agent").MustString(ua)
	io.UrlsFile = secCus.Key("input").String() // 读取URLs文件路径

	// 读取dzi相关设置
	secDzi := cfg.Section("dzi")
	io.UseDziRs = secDzi.Key("dezoomify-rs").MustBool(false)
	io.DezoomifyRs = secDzi.Key("dezoomify-rs-args").String()
	io.Format = secDzi.Key("format").MustString(format)

	if !strings.Contains(io.DezoomifyRs, "-n") && !strings.Contains(io.DezoomifyRs, "--parallelism") {
		io.DezoomifyRs += fmt.Sprintf(" -n=%d", io.Threads)
	}
	return io, nil
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
