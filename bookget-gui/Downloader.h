#pragma once
#include "framework.h"
#include <thread>
#include <string>

class Downloader {
public:
    ~Downloader();
    void Start(HWND hWnd);
    void Stop();

    void RequestDownload(const std::wstring& url);

    bool DownloadFile(const wchar_t* url, ICoreWebView2HttpRequestHeaders* headers);

    void LoadImageUrlsFromFile(const std::wstring& sUrlsFilePath);
    void DownloadNextImage(HWND hWnd);

    bool ShouldInterceptRequest(const std::wstring& sUrl);
    bool ShouldInterceptResponse(const std::wstring& contentType, const std::wstring& contentLength);
    bool ShouldInterceptResponse(const std::wstring& contentType);
    std::wstring GetFilePath(const std::wstring& sUrl);

    void Reset(std::wstring sUrl, int runMode) {
        m_targetUrls.clear();
        m_targetUrls.emplace_back(sUrl);
        m_downloaderMode = runMode;
    }
    int GetDownloaderMode(){return m_downloaderMode;};

private:
    std::thread m_downloaderThread;
    std::atomic<bool> m_stopThread{false};
    DWORD m_workerThreadId = 0;

    int m_sleepTime = 0;
    int m_maxDownloads = 0;
    std::wstring m_downloadDir;
    std::wstring m_filePath;

    std::vector<std::wstring> m_targetUrls;
    std::vector<std::wstring> m_siteUrls;
    std::atomic<int> m_downloadCounter = 0;
    std::mutex m_downloadCounterMutex;
    int m_downloaderMode = 1; // 0=urls.txt, 1=自动图片, 2=共享内存URL

    std::vector<std::wstring> m_targetExtensions = {
        L".jpg",
        L".jp2",
        L".png",
        L".pdf",
        L".jpeg",
        L".tif",
        L".tiff",
        L".bmp",
        L".webp"
    };

    std::vector<std::wstring> m_targetContentTypes = {
        L"image/",
        L"application/pdf",
        L"application/octet-stream",
    };


};

