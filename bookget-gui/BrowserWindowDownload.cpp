#include "BrowserWindow.h"
#include "Config.h"


// 设置页面下载监听
void BrowserWindow::SetupWebViewListeners(wil::com_ptr<ICoreWebView2> &contentWebView)
{
    if (contentWebView)
    {
        wil::com_ptr<ICoreWebView2_9> webview9;
        if (SUCCEEDED(contentWebView->QueryInterface(IID_PPV_ARGS(&webview9))))
        {
            // 移除旧的下载监听器（如果存在）
            if (m_webResourceResponseReceivedToken.value != 0)
            {
                webview9->remove_WebResourceResponseReceived(m_webResourceResponseReceivedToken);
            }

            webview9->add_WebResourceResponseReceived(
                Callback<ICoreWebView2WebResourceResponseReceivedEventHandler>(
                    [this](ICoreWebView2* sender, ICoreWebView2WebResourceResponseReceivedEventArgs* args) -> HRESULT {
                         // 获取请求对象
                        wil::com_ptr<ICoreWebView2WebResourceRequest> request;
                        args->get_Request(&request);

                        // 1. 检查请求方法是否为 OPTIONS（预检请求）
                        LPWSTR method;
                        request->get_Method(&method);
                        if (wcscmp(method, L"OPTIONS") == 0) {
                            return S_OK; 
                        }

                        // 2. 获取请求头集合
                        wil::com_ptr<ICoreWebView2HttpRequestHeaders> headers;
                        request->get_Headers(&headers);

                        // 3. 检查 Accept 头
                        LPWSTR acceptHeader = nullptr;
                        headers->GetHeader(L"Accept", &acceptHeader);

                        if (acceptHeader != nullptr) {
                            std::wstring accept(acceptHeader); // 转换为 std::wstring 方便处理
                            CoTaskMemFree(acceptHeader);       // 释放内存
                            // 检查 Accept 类型
                            if (accept.find(L"application/json") != std::wstring::npos) {
                                return S_OK; 
                            }
                            if (accept.find(L"image/") != std::wstring::npos ||
                                     accept.find(L"application/octet-stream") != std::wstring::npos ||
                                     accept.find(L"application/pdf") != std::wstring::npos
                                ) {
                                 return HandleWebResourceResponseReceived(sender, args);
                            }
                        }
                        return S_OK; 
                    }).Get(), 
                &m_webResourceResponseReceivedToken);
        }
    }
    return;
}

// 资源响应处理
HRESULT BrowserWindow::HandleWebResourceResponseReceived(ICoreWebView2* sender, ICoreWebView2WebResourceResponseReceivedEventArgs* args)
{
    wil::com_ptr<ICoreWebView2WebResourceRequest> request;
    RETURN_IF_FAILED(args->get_Request(&request));

    wil::unique_cotaskmem_string uri;
    RETURN_IF_FAILED(request->get_Uri(&uri));

    if (!m_downloader.ShouldInterceptRequest(uri.get()))
        return S_OK;

    std::wstring sUrl(uri.get());

    // 获取响应
    wil::com_ptr<ICoreWebView2WebResourceResponseView> response;
    RETURN_IF_FAILED(args->get_Response(&response));
    if (response) {
         if (ShouldInterceptRequest(uri.get(), response.get())) {    
               response->GetContent(
                Callback<ICoreWebView2WebResourceResponseViewGetContentCompletedHandler>(
                    [this, sUrl](HRESULT errorCode, IStream* content) -> HRESULT {
                        if (SUCCEEDED(errorCode) && content)
                        {
                           this->DownloadFile(sUrl, content);
                        }
                        return S_OK;
                    }).Get());
         }
    }

    return S_OK;
}

// 请求拦截判断逻辑
bool BrowserWindow::ShouldInterceptRequest(const std::wstring& sUrl,  ICoreWebView2WebResourceResponseView* response)
{
    wil::com_ptr<ICoreWebView2HttpResponseHeaders> headers;
    if (FAILED(response->get_Headers(&headers)) || !headers) {
        return false;
    }
    

    wil::unique_cotaskmem_string contentType;
    wil::unique_cotaskmem_string contentLengthStr;
    wil::unique_cotaskmem_string transferEncoding;


    SUCCEEDED(headers->GetHeader(L"transfer-encoding",&transferEncoding));
    SUCCEEDED(headers->GetHeader(L"Content-Type", &contentType));
    SUCCEEDED(headers->GetHeader(L"Content-Length", &contentLengthStr));

    if (transferEncoding && contentType) {
        std::wstring s(transferEncoding.get());
        return s.compare(L"chunked") == 0;
    }

    if (contentType && contentLengthStr) {
        return m_downloader.ShouldInterceptResponse(contentType.get(), contentLengthStr.get());
    }

    return false;
}


bool BrowserWindow::DownloadFile(const std::wstring& sUrl, IStream *content) {

    std::wstring filePath = m_downloader.GetFilePath(sUrl);
    bool ret = Util::FileWrite(filePath, content);

    int sleepTime = Config::GetInstance().GetSleepTime();
    if (sleepTime > 0)
        std::this_thread::sleep_for(std::chrono::seconds(sleepTime)); // 3 秒延时

    int mode = m_downloader.GetDownloaderMode();
    if (mode == 0) {
        // 下载完成后继续下一个
        PostMessage(m_hWnd, WM_APP_DOWNLOAD_NEXT, 0, 0);
    }
    else if (mode == 1) {
        //执行 javascript 脚本
        std::wstring scriptPath;
        std::string narrow_url = Util::WideToUtf8(sUrl);
        for (const auto& site : Config::GetInstance().GetSiteConfigs()) {
            if (site.enabled && Util::matchUrlPattern(site.url, narrow_url) ) {
                scriptPath = GetFullPathFor(Util::Utf8ToWide(site.script).c_str());
                break;
            }
        }
        if(scriptPath.length() > 0) {
            this->ExecuteScriptFile(scriptPath, m_tabs.at(m_activeTabId)->m_contentWebView.get());
        }
    } 
    else if (mode == 2) {
        // 写入共享内存
        SharedMemory::GetInstance().WriteImagePath(filePath.c_str());
    }
    return ret;
}
