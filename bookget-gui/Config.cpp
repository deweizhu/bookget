#include "Config.h"
#include <windows.h>

// 构造函数初始化PIMPL
Config::Config() : pImpl(std::make_unique<ConfigImpl>()) {}

// 加载 YAML 配置文件
bool Config::ConfigImpl::Load(const std::string& configPath) {
    try {
        YAML::Node config = YAML::LoadFile(configPath);

        // 解析全局设置
        if (config["global_settings"]) {
            auto global = config["global_settings"];
            if (global["download_dir"]) downloadDir = global["download_dir"].as<std::string>();
            if (global["max_downloads"]) maxDownloads = global["max_downloads"].as<int>();
            if (global["sleep_time"]) sleepTime = global["sleep_time"].as<int>();
            if (global["downloader_mode"]) downloaderMode = global["downloader_mode"].as<int>();
            if (global["ext"]) fileExt = global["ext"].as<std::string>();
        }

        // 解析站点配置
        if (config["sites"]) {
            for (const auto& site : config["sites"]) {
                SiteConfig siteConfig;
                if (site["url"]) siteConfig.url = site["url"].as<std::string>();
                if (site["script"]) siteConfig.script = site["script"].as<std::string>();

                siteConfig.ext = site["ext"] ? site["ext"].as<std::string>() : fileExt;
                
                if (site["intercept"]) siteConfig.intercept = site["intercept"].as<int>();
                if (site["description"]) siteConfig.description = site["description"].as<std::string>();

                //if (site["metadata"] && site["metadata"]["description"]) {
                    //siteConfig.description = site["metadata"]["description"].as<std::string>();
                //}
                siteConfigs.push_back(siteConfig);
            }
        }

        return true;
    } catch (...) {
        OutputDebugString(L"Failed to parse YAML config file\n");
        return false;
    }
}

bool Config::Load(const std::string& configPath) {
    return pImpl->Load(configPath);
}

std::string Config::GetDownloadDir() { 
    return pImpl->downloadDir; 
}

std::string Config::GetDefaultExt() { 
    return pImpl->fileExt; 
}

int Config::GetMaxDownloads() { 
    return pImpl->maxDownloads; 
}

int Config::GetSleepTime() { 
    return pImpl->sleepTime; 
}
int Config::GetDownloaderMode() { 
    return pImpl->downloaderMode; 
}

const std::vector<Config::SiteConfig>& Config::GetSiteConfigs() {
    return pImpl->siteConfigs;
}

