#include "Downloader.h"
#include "Util.h"
#include "Config.h"
#include "SharedMemory.h"

void Downloader::Start(HWND hWnd) {
    m_downloaderThread = std::thread([this, hWnd]() {
        {
            //睡眠 3 秒，等第一个tab页 Init 成功
            std::this_thread::sleep_for(std::chrono::seconds(3));

            std::lock_guard<std::mutex> lock(m_downloadCounterMutex); 
            // 使用 YAML 配置的最大下载次数
            m_downloadCounter = 0;
            // 使用 YAML 配置的站点
            m_downloaderMode = 1;

            auto& conf = Config::GetInstance();
            m_downloadDir = Util::Utf8ToWide(conf.GetDownloadDir());
            m_sleepTime = conf.GetSleepTime();
            m_maxDownloads = conf.GetMaxDownloads();

            for (const auto& site : conf.GetSiteConfigs()) {
                if (site.enabled) {
                    m_siteUrls.push_back(Util::Utf8ToWide(site.url));
                }
            }
            // 加载URL列表
            std::wstring sUrlsFilePath = Util::GetCurrentExeDirectory() + L"\\urls.txt";
            LoadImageUrlsFromFile(sUrlsFilePath);
            if (!m_targetUrls.empty())
            {
                //开始下载
                DownloadNextImage(hWnd);
            }
        }


        m_workerThreadId = GetCurrentThreadId(); // 保存线程 ID
            
        // 消息循环（必需用于接收 PostThreadMessage）
         while (!m_stopThread.load(std::memory_order_relaxed)) {
            MSG msg;
            // 设置100ms超时，避免长期阻塞
            if (PeekMessage(&msg, nullptr, 0, 0, PM_REMOVE)) {
                if (msg.message == WM_DOWNLOAD_URL) {
                    std::wstring* pUrl = reinterpret_cast<std::wstring*>(msg.lParam);
                    DownloadFile(*pUrl, *pUrl);
                    delete pUrl;
                }
            }
            std::this_thread::sleep_for(std::chrono::milliseconds(100));
        }
    });
}
Downloader::~Downloader(){
    Stop();
}

void Downloader::Stop() {
    m_stopThread = true;
    if (m_downloaderThread.joinable()) {
        m_downloaderThread.join();
    }
    m_stopThread = false;
}

void Downloader::RequestDownload(const std::wstring& url) {
    // 深拷贝数据并发送到工作线程
    std::wstring* pUrl = new std::wstring(url);
    ::PostThreadMessage(m_workerThreadId, WM_DOWNLOAD_URL, 0, reinterpret_cast<LPARAM>(pUrl));

}
bool Downloader::ShouldInterceptRequest(const std::wstring& sUrl){
    
    // 跳过本地路径（file://, http://localhost, 127.0.0.1等）
    if (Util::IsLocalUri(sUrl)) {
        return false; // 不处理本地URI
    }

    // 1. 检查URL是否匹配目标URL列表
    bool urlMatch = false;
    for (const auto& targetUrl : m_targetUrls)
    {
        if (sUrl.find(targetUrl) != std::wstring::npos)
        {
            urlMatch = true;
            break;
        }
    }
    // 2. 检查URL是否匹配 config.yaml URL
    for (const auto& targetUrl : m_siteUrls) {
        if (wcsstr(sUrl.c_str(), targetUrl.c_str()) != nullptr || Util::matchUrlPattern(targetUrl, sUrl.c_str())) {
            urlMatch = true;
            break;
        }
    }
    // 3. 检查URL扩展名
    //for (const auto& ext : m_targetExtensions) {
    //    if (sUrl.size() >= ext.size() && 
    //        _wcsicmp(sUrl.substr(sUrl.size() - ext.size()).c_str(), ext.c_str()) == 0) {
    //        urlMatch = true;
    //        break;
    //    }
    //}
    
    return urlMatch;
}

bool Downloader::ShouldInterceptResponse(const std::wstring& contentType)
{
    bool isCanDownload = false;
    //  检查Content-Type
    for (const auto& ext : m_targetContentTypes) {
        if (contentType.size() >= ext.size() && 
            _wcsicmp(contentType.c_str(), ext.c_str()) == 0) {
            isCanDownload = true;
            break;
        }
    }
    return isCanDownload;
}

bool Downloader::ShouldInterceptResponse(const std::wstring& contentType,const std::wstring& contentLength)
{
    bool isCanDownload = ShouldInterceptResponse(contentType);

    // 检查Content-Length
    ULONGLONG length = 0;
    if (swscanf_s(contentLength.c_str(), L"%llu", &length) == 1) {
        // 设置合理的图片大小范围 (10KB - 20MB)
        isCanDownload = (length >= 10240 && length <= 20 * 1024 * 1024);
    }

    return isCanDownload;
}

std::wstring Downloader::GetFilePath(const std::wstring& sUrl)
{
    std::wstring filePath;

    bool isSharedDataURL = false;
    //读共享内存
    auto* sharedData = SharedMemory::GetInstance().GetMutex();
    if (sharedData != nullptr) {
        isSharedDataURL = sharedData->ImageReady && sharedData->ImagePath && sharedData->PID != GetCurrentProcessId();
        filePath.assign(sharedData->ImagePath);
        if (isSharedDataURL)
             m_downloaderMode = 2;
        SharedMemory::GetInstance().ReleaseMutex();
     }

    if (m_downloaderMode == 0 || m_downloaderMode == 1) {
         // 获取下一个序号
        std::lock_guard<std::mutex> lock(m_downloadCounterMutex);
        int currentCount = ++m_downloadCounter;

        std::wstringstream filename;
        filename << m_downloadDir << L"\\"  
            << std::setw(4) << std::setfill(L'0') << currentCount;
    
        // 尝试从URL获取文件扩展名
        size_t dotPos = sUrl.find_last_of(L'.');
        if (dotPos != std::wstring::npos)
        {
            std::wstring ext = sUrl.substr(dotPos);
            if (ext.length() <= 5) // 假设扩展名不超过5个字符
            {
                filename << ext;
            }
            else {
                filename << L".jpg";
            }
        }
        else {
            // 默认使用.jpg
            filename << L".jpg";
        }
        filePath.assign(filename.str());
    }
 
    if (m_downloadCounter >=  m_maxDownloads)
    {
        OutputDebugString(L"超出 config.yaml 设置的限制 max_downloads 次数\n");
        return L"";
    }
    return filePath;
}

bool Downloader::DownloadFile(const std::wstring& sUrl, const std::wstring& filePath) {

    return false;
}



// 2. 从文件加载URLs
void Downloader::LoadImageUrlsFromFile(const std::wstring& sUrlsFilePath)
{
    std::wifstream file;
    if (sUrlsFilePath.empty())
        return;

    file.open(sUrlsFilePath);
    if (!file.is_open())
    {
        OutputDebugString(L"Error: Could not open any urls file (global or local)\n");
        return;
    }

    m_downloaderMode = 0;

    m_targetUrls.clear();
    std::wstring line;
    while (std::getline(file, line))
    {
        if (!line.empty())
        {
            m_targetUrls.emplace_back(line);
        }
    }
}

//3. 下载下一页
void Downloader::DownloadNextImage(HWND hWnd)
{
    std::lock_guard<std::mutex> lock(m_downloadCounterMutex);
    int currentIndex = m_downloadCounter;

    if (currentIndex >= m_targetUrls.size() || currentIndex >=  m_maxDownloads)
    {
        OutputDebugString(L"All downloads completed\n");
        return;
    }
   

    try {
        std::unique_ptr<std::wstring> pUrl = std::make_unique<std::wstring>(m_targetUrls.at(currentIndex));
        ::PostMessage(
            hWnd,
            WM_NAVIGATE_URL,
            0,
            reinterpret_cast<LPARAM>(pUrl.release()) // 移交所有权
        );
    } catch (const std::out_of_range&) {
        //::PostMessage(m_hWnd, WM_ERR, 0, (LPARAM)L"Index out of range");
    }
    
}

