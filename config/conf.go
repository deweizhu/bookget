package config

import (
	"context"
	"fmt"
	"github.com/spf13/pflag"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

type Input struct {
	DownloaderMode int  //自动检测下载URL。可选值[0|1|2]，;0=默认;1=通用批量下载（类似IDM、迅雷）;2= IIIF manifest.json 自动检测下载图片
	UseDzi         bool //启用Dezoomify下载IIIF

	DUrl       string
	UrlsFile   string //已废弃
	CookieFile string //输入 chttp.txt
	HeaderFile string //输入 header.txt

	Seq      string //页面范围 4:434
	SeqStart int
	SeqEnd   int
	Volume   string //册范围 4:434
	VolStart int
	VolEnd   int

	Sleep     int    //限速
	Directory string //下载文件存放目录，默认为当前文件夹下 Downloads 目录下
	Format    string //;全高清图下载时，指定宽度像素（16开纸185mm*260mm，像素2185*3071）
	UserAgent string //自定义UserAgent

	Threads       int
	MaxConcurrent int
	Timeout       time.Duration //超时秒数
	Retries       int           //重试次数

	FileExt string //指定下载的扩展名
	Quality int    //JPG品质

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

	pflag.StringVarP(&Conf.DUrl, "input", "i", "", "下载 URL")
	pflag.StringVarP(&Conf.UrlsFile, "input-file", "I", "", "下载 URLs")
	pflag.StringVarP(&Conf.Directory, "dir", "O", path.Join(dir, "downloads"), "保存文件到目录")

	pflag.StringVarP(&Conf.Seq, "sequence", "p", "", "页面范围，如4:434")
	pflag.StringVarP(&Conf.Volume, "volume", "v", "", "多册图书，如10:20册，只下载10至20册")

	pflag.StringVar(&Conf.Format, "format", "full/full/0/default.jpg", "IIIF 图像请求URI")

	pflag.StringVarP(&Conf.UserAgent, "user-agent", "U", defaultUserAgent, "http头信息 user-agent")

	pflag.BoolVarP(&Conf.UseDzi, "dzi", "d", true, "使用 IIIF/DeepZoom 拼图下载")

	pflag.StringVarP(&Conf.CookieFile, "cookies", "C", path.Join(dir, "cookie.txt"), "cookie 文件")
	pflag.StringVarP(&Conf.HeaderFile, "headers", "H", path.Join(dir, "header.txt"), "header 文件")

	pflag.IntVarP(&Conf.Threads, "threads", "n", 1, "每任务最大线程数")
	pflag.IntVarP(&Conf.MaxConcurrent, "concurrent", "c", 16, "最大并发任务数")

	pflag.IntVar(&Conf.Quality, "quality", 80, "JPG品质，默认80")
	pflag.StringVar(&Conf.FileExt, "ext", ".jpg", "指定文件扩展名[.jpg|.tif|.png]等")

	pflag.IntVar(&Conf.Retries, "retries", 3, "下载重试次数")

	pflag.DurationVarP(&Conf.Timeout, "timeout", "T", 300, "网络超时（秒)")
	pflag.IntVar(&Conf.Sleep, "sleep", 3, "间隔睡眠几秒，一般情况 3-20")

	pflag.IntVarP(&Conf.DownloaderMode, "downloader_mode", "m", 0, "下载模式。可选值[0|1|2]，;0=默认;\n1=通用批量下载（类似IDM、迅雷）;\n2= IIIF manifest.json 自动检测下载图片")

	pflag.BoolVarP(&Conf.Help, "help", "h", false, "显示帮助")
	pflag.BoolVarP(&Conf.Version, "version", "V", false, "显示版本 -v")
	pflag.Parse()

	k := len(os.Args)
	if k == 2 {
		if Conf.Version {
			printVersion()
			return false
		}
		if Conf.Help {
			printHelp()
			return false
		}
	}
	v := pflag.Arg(0)
	if strings.HasPrefix(v, "http") {
		Conf.DUrl = v
	}
	if Conf.UrlsFile != "" && !strings.Contains(Conf.UrlsFile, string(os.PathSeparator)) {
		Conf.UrlsFile = path.Join(dir, Conf.UrlsFile)
	}
	initSeqRange()
	initVolumeRange()
	//保存目录处理
	_ = os.Mkdir(Conf.Directory, os.ModePerm)
	//_ = os.Mkdir(CacheDir(), os.ModePerm)
	return true
}

func printHelp() {
	printVersion()
	fmt.Println(`Usage: bookget [OPTION]... [URL]...`)
	pflag.PrintDefaults()
	fmt.Println()
	fmt.Println("Originally written by zhudw <zhudwi@outlook.com>.")
	fmt.Println("https://github.com/deweizhu/bookget/")
}

func printVersion() {
	fmt.Printf("bookget v%s\n", Version)
}
