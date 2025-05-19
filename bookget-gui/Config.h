#pragma once

#include <yaml-cpp/yaml.h>
#include <memory>

class Config
{
public:
    // 删除拷贝构造函数和赋值运算符
    Config(const Config&) = delete;
    Config& operator=(const Config&) = delete;
    
    // 获取单例实例
    static Config& GetInstance() {
        static Config instance;
        return instance;
    }

    // 配置管理
    struct SiteConfig {
        std::string url;
        std::string script;
        int intercept; 
        std::string ext;
        std::string description;
        int downloaderMode;
    };

    // 公共接口
    bool Load(const std::string& configPath);
    std::string GetDownloadDir();
    std::string GetDefaultExt();
    int GetMaxDownloads();
    int GetSleepTime();
    int GetDownloaderMode();
    const std::vector<SiteConfig>& GetSiteConfigs();

private:
    // PIMPL 实现
    struct ConfigImpl {
        // 全局设置
        std::string downloadDir = "downloads";
        int maxDownloads = 1000;
        int sleepTime = 3;
        int downloaderMode = 1;    //下载模式 0=urls.txt | 1=自动监听图片 | 2 = 共享内存URL
        std::string fileExt = ".jpg";
     
        std::vector<SiteConfig> siteConfigs;

        // 加载 YAML 配置文件
        bool Load(const std::string& configPath);
    };

    Config();  // 私有构造函数
    ~Config() = default;
    
    // PIMPL实现
    std::unique_ptr<ConfigImpl> pImpl;
};
