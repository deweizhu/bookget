#include "BrowserWindow.h"

// UI WebViews初始化
HRESULT BrowserWindow::InitUIWebViews()
{
    // Get data directory for browser UI data
    std::wstring browserDataDirectory = GetAppDataDirectory();
    browserDataDirectory.append(L"\\Browser Data");

    // Create WebView environment for browser UI. A separate data directory is
    // used to isolate the browser UI from web content requested by the user.
    return CreateCoreWebView2EnvironmentWithOptions(nullptr, browserDataDirectory.c_str(),
        nullptr, Callback<ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler>(
            [this](HRESULT result, ICoreWebView2Environment* env) -> HRESULT
    {
        // Environment is ready, create the WebView
        m_uiEnv = env;

        RETURN_IF_FAILED(CreateBrowserControlsWebView());
        RETURN_IF_FAILED(CreateBrowserOptionsWebView());
       
        return S_OK;
    }).Get());
}

// 控制WebView创建实现
HRESULT BrowserWindow::CreateBrowserControlsWebView()
{
    return m_uiEnv->CreateCoreWebView2Controller(m_hWnd, Callback<ICoreWebView2CreateCoreWebView2ControllerCompletedHandler>(
        [this](HRESULT result, ICoreWebView2Controller* host) -> HRESULT
    {
        if (!SUCCEEDED(result))
        {
            OutputDebugString(L"Controls WebView creation failed\n");
            return result;
        }
        // WebView created
        m_controlsController = host;
        CheckFailure(m_controlsController->get_CoreWebView2(&m_controlsWebView), L"");

        wil::com_ptr<ICoreWebView2Settings> settings;
        RETURN_IF_FAILED(m_controlsWebView->get_Settings(&settings));
        RETURN_IF_FAILED(settings->put_AreDevToolsEnabled(FALSE));

        // 禁用弹窗
        RETURN_IF_FAILED(settings->put_AreDefaultScriptDialogsEnabled(FALSE));

        // 设置新窗口在当前标签页打开
        RETURN_IF_FAILED(m_controlsWebView->add_NewWindowRequested(
            Callback<ICoreWebView2NewWindowRequestedEventHandler>(
                [this](ICoreWebView2* sender, ICoreWebView2NewWindowRequestedEventArgs* args) -> HRESULT
        {
            // 获取请求的URI
            wil::unique_cotaskmem_string uri;
            args->get_Uri(&uri);
            
            // 在当前WebView中导航到该URI
            m_controlsWebView->Navigate(uri.get());

            // 取消默认的新窗口行为
            args->put_Handled(TRUE);
      
            
            return S_OK;
        }).Get(), &m_newWindowRequestedToken));

        RETURN_IF_FAILED(m_controlsController->add_ZoomFactorChanged(Callback<ICoreWebView2ZoomFactorChangedEventHandler>(
            [](ICoreWebView2Controller* host, IUnknown* args) -> HRESULT
        {
            host->put_ZoomFactor(1.0);
            return S_OK;
        }
        ).Get(), &m_controlsZoomToken));

        RETURN_IF_FAILED(m_controlsWebView->add_WebMessageReceived(m_uiMessageBroker.get(), &m_controlsUIMessageBrokerToken));
        RETURN_IF_FAILED(ResizeUIWebViews());

        std::wstring controlsPath = GetFullPathFor(L"gui\\controls_ui\\default.html");
        RETURN_IF_FAILED(m_controlsWebView->Navigate(controlsPath.c_str()));

        //AJAX 注册 AJAX 请求拦截器
        //SetupDownloadHandler(m_controlsWebView);
    
        return S_OK;
    }).Get());
}

// 选项卡 WebView创建实现.
HRESULT BrowserWindow::CreateBrowserOptionsWebView()
{
    return m_uiEnv->CreateCoreWebView2Controller(m_hWnd, Callback<ICoreWebView2CreateCoreWebView2ControllerCompletedHandler>(
        [this](HRESULT result, ICoreWebView2Controller* host) -> HRESULT
    {
        if (!SUCCEEDED(result))
        {
            OutputDebugString(L"Options WebView creation failed\n");
            return result;
        }
        // WebView created
        m_optionsController = host;
        CheckFailure(m_optionsController->get_CoreWebView2(&m_optionsWebView), L"");

        wil::com_ptr<ICoreWebView2Settings> settings;
        RETURN_IF_FAILED(m_optionsWebView->get_Settings(&settings));
        RETURN_IF_FAILED(settings->put_AreDevToolsEnabled(FALSE));

         // 禁用弹窗
        RETURN_IF_FAILED(settings->put_AreDefaultScriptDialogsEnabled(FALSE));
        
        // 设置新窗口在当前标签页打开
        RETURN_IF_FAILED(m_optionsWebView->add_NewWindowRequested(
            Callback<ICoreWebView2NewWindowRequestedEventHandler>(
                [this](ICoreWebView2* sender, ICoreWebView2NewWindowRequestedEventArgs* args) -> HRESULT
        {
            // 获取请求的URI
            wil::unique_cotaskmem_string uri;
            args->get_Uri(&uri);

           // 在当前WebView中导航到该URI
            m_controlsWebView->Navigate(uri.get());

            // 取消默认的新窗口行为
            args->put_Handled(TRUE);
            
            return S_OK;
        }).Get(), &m_newWindowRequestedToken));

        RETURN_IF_FAILED(m_optionsController->add_ZoomFactorChanged(Callback<ICoreWebView2ZoomFactorChangedEventHandler>(
            [](ICoreWebView2Controller* host, IUnknown* args) -> HRESULT
        {
            host->put_ZoomFactor(1.0);
            return S_OK;
        }
        ).Get(), &m_optionsZoomToken));

        // Hide by default
        RETURN_IF_FAILED(m_optionsController->put_IsVisible(FALSE));
        RETURN_IF_FAILED(m_optionsWebView->add_WebMessageReceived(m_uiMessageBroker.get(), &m_optionsUIMessageBrokerToken));

        // Hide menu when focus is lost
        RETURN_IF_FAILED(m_optionsController->add_LostFocus(Callback<ICoreWebView2FocusChangedEventHandler>(
            [this](ICoreWebView2Controller* sender, IUnknown* args) -> HRESULT
        {
            web::json::value jsonObj = web::json::value::parse(L"{}");
            jsonObj[L"message"] = web::json::value(MG_OPTIONS_LOST_FOCUS);
            jsonObj[L"args"] = web::json::value::parse(L"{}");

            PostJsonToWebView(jsonObj, m_controlsWebView.get());

            return S_OK;
        }).Get(), &m_lostOptionsFocus));

        RETURN_IF_FAILED(ResizeUIWebViews());

        std::wstring optionsPath = GetFullPathFor(L"gui\\controls_ui\\options.html");
        RETURN_IF_FAILED(m_optionsWebView->Navigate(optionsPath.c_str()));

        return S_OK;
    }).Get());
}


HRESULT BrowserWindow::OpenWindowTab(wchar_t *webUrl, bool isTab)
{
    if (!m_controlsWebView) {
        OutputDebugString(L"Controls WebView not ready\n");
        return S_FALSE;
    }
    if (isTab) {
         m_tabs.at(m_activeTabId)->m_contentWebView->Navigate(g_HomeUrl.c_str());
         return S_OK;
    }
    // 创建新标签页
    web::json::value jsonObj = web::json::value::parse(L"{}");
    jsonObj[L"message"] = web::json::value(MG_CREATE_TAB);
    jsonObj[L"args"] = web::json::value::parse(L"{}");
    jsonObj[L"args"][L"tabId"] = web::json::value::number(m_tabs.size()); // 使用下一个可用ID
    jsonObj[L"args"][L"active"] = web::json::value::boolean(true);
    jsonObj[L"args"][L"uri"] = web::json::value(webUrl);
    
    // 发送消息创建新标签页并导航
    PostJsonToWebView(jsonObj, m_controlsWebView.get());

    return S_OK;
}


// 切换标签页
HRESULT BrowserWindow::SwitchToTab(size_t tabId)
{
    // 检查标签页是否存在
    if (m_tabs.find(tabId) == m_tabs.end())
    {
        return E_INVALIDARG;
    }

    size_t previousActiveTab = m_activeTabId;

    // 激活新标签页
    RETURN_IF_FAILED(m_tabs.at(tabId)->ResizeWebView());
    RETURN_IF_FAILED(m_tabs.at(tabId)->m_contentController->put_IsVisible(TRUE));
    m_activeTabId = tabId;

    // 隐藏之前的活动标签页
    if (previousActiveTab != INVALID_TAB_ID && previousActiveTab != m_activeTabId) 
    {
        auto previousTabIterator = m_tabs.find(previousActiveTab);
        if (previousTabIterator != m_tabs.end() && previousTabIterator->second &&
            previousTabIterator->second->m_contentController)
        {
            previousTabIterator->second->m_contentController->put_IsVisible(FALSE);
        }
    }

    return S_OK;
}


// 标签页URI更新处理
HRESULT BrowserWindow::HandleTabURIUpdate(size_t tabId, ICoreWebView2* webview)
{
    wil::unique_cotaskmem_string source;
    RETURN_IF_FAILED(webview->get_Source(&source));

    web::json::value jsonObj = web::json::value::parse(L"{}");
    jsonObj[L"message"] = web::json::value(MG_UPDATE_URI);
    jsonObj[L"args"] = web::json::value::parse(L"{}");
    jsonObj[L"args"][L"tabId"] = web::json::value::number(tabId);
    jsonObj[L"args"][L"uri"] = web::json::value(source.get());

    std::wstring uri(source.get());
    std::wstring favoritesURI = GetFilePathAsURI(GetFullPathFor(L"gui\\content_ui\\favorites.html"));
    std::wstring settingsURI = GetFilePathAsURI(GetFullPathFor(L"gui\\content_ui\\settings.html"));
    std::wstring historyURI = GetFilePathAsURI(GetFullPathFor(L"gui\\content_ui\\history.html"));

    if (uri.compare(favoritesURI) == 0)
    {
        jsonObj[L"args"][L"uriToShow"] = web::json::value(L"browser://favorites");
    }
    else if (uri.compare(settingsURI) == 0)
    {
        jsonObj[L"args"][L"uriToShow"] = web::json::value(L"browser://settings");
    }
    else if (uri.compare(historyURI) == 0)
    {
        jsonObj[L"args"][L"uriToShow"] = web::json::value(L"browser://history");
    }

    RETURN_IF_FAILED(PostJsonToWebView(jsonObj, m_controlsWebView.get()));
    return S_OK;
}
 // 标签页历史更新处理实现
HRESULT BrowserWindow::HandleTabHistoryUpdate(size_t tabId, ICoreWebView2* webview)
{
    wil::unique_cotaskmem_string source;
    RETURN_IF_FAILED(webview->get_Source(&source));
    
    web::json::value jsonObj = web::json::value::parse(L"{}");
    jsonObj[L"message"] = web::json::value(MG_UPDATE_URI);
    jsonObj[L"args"] = web::json::value::parse(L"{}");
    jsonObj[L"args"][L"tabId"] = web::json::value::number(tabId);
    jsonObj[L"args"][L"uri"] = web::json::value(source.get());

    BOOL canGoForward = FALSE;
    RETURN_IF_FAILED(webview->get_CanGoForward(&canGoForward));
    jsonObj[L"args"][L"canGoForward"] = web::json::value::boolean(canGoForward);

    BOOL canGoBack = FALSE;
    RETURN_IF_FAILED(webview->get_CanGoBack(&canGoBack));
    jsonObj[L"args"][L"canGoBack"] = web::json::value::boolean(canGoBack);

    RETURN_IF_FAILED(PostJsonToWebView(jsonObj, m_controlsWebView.get()));

    return S_OK;
}

// 导航URL开始
HRESULT BrowserWindow::HandleTabNavStarting(size_t tabId, ICoreWebView2* webview)
{
    web::json::value jsonObj = web::json::value::parse(L"{}");
    jsonObj[L"message"] = web::json::value(MG_NAV_STARTING);
    jsonObj[L"args"] = web::json::value::parse(L"{}");
    jsonObj[L"args"][L"tabId"] = web::json::value::number(tabId);

    return PostJsonToWebView(jsonObj, m_controlsWebView.get());
}

// 导航页面加载完成
HRESULT BrowserWindow::HandleTabNavCompleted(size_t tabId, ICoreWebView2* webview, ICoreWebView2NavigationCompletedEventArgs* args)
{
    std::wstring getTitleScript(
        // Look for a title tag
        L"(() => {"
        L"    const titleTag = document.getElementsByTagName('title')[0];"
        L"    if (titleTag) {"
        L"        return titleTag.innerHTML;"
        L"    }"
        // No title tag, look for the file name
        L"    pathname = window.location.pathname;"
        L"    var filename = pathname.split('/').pop();"
        L"    if (filename) {"
        L"        return filename;"
        L"    }"
        // No file name, look for the hostname
        L"    const hostname =  window.location.hostname;"
        L"    if (hostname) {"
        L"        return hostname;"
        L"    }"
        // Fallback: let the UI use a generic title
        L"    return '';"
        L"})();"
    );

    std::wstring getFaviconURI(
        L"(() => {"
        // Let the UI use a fallback favicon
        L"    let faviconURI = '';"
        L"    let links = document.getElementsByTagName('link');"
        // Test each link for a favicon
        L"    Array.from(links).map(element => {"
        L"        let rel = element.rel;"
        // Favicon is declared, try to get the href
        L"        if (rel && (rel == 'shortcut icon' || rel == 'icon')) {"
        L"            if (!element.href) {"
        L"                return;"
        L"            }"
        // href to icon found, check it's full URI
        L"            try {"
        L"                let urlParser = new URL(element.href);"
        L"                faviconURI = urlParser.href;"
        L"            } catch(e) {"
        // Try prepending origin
        L"                let origin = window.location.origin;"
        L"                let faviconLocation = `${origin}/${element.href}`;"
        L"                try {"
        L"                    urlParser = new URL(faviconLocation);"
        L"                    faviconURI = urlParser.href;"
        L"                } catch (e2) {"
        L"                    return;"
        L"                }"
        L"            }"
        L"        }"
        L"    });"
        L"    return faviconURI;"
        L"})();"
    );

    CheckFailure(webview->ExecuteScript(getTitleScript.c_str(), Callback<ICoreWebView2ExecuteScriptCompletedHandler>(
        [this, tabId](HRESULT error, PCWSTR result) -> HRESULT
    {
        RETURN_IF_FAILED(error);

        web::json::value jsonObj = web::json::value::parse(L"{}");
        jsonObj[L"message"] = web::json::value(MG_UPDATE_TAB);
        jsonObj[L"args"] = web::json::value::parse(L"{}");
        jsonObj[L"args"][L"title"] = web::json::value::parse(result);
        jsonObj[L"args"][L"tabId"] = web::json::value::number(tabId);

        CheckFailure(PostJsonToWebView(jsonObj, m_controlsWebView.get()), L"Can't update title.");
        return S_OK;
    }).Get()), L"Can't update title.");

    CheckFailure(webview->ExecuteScript(getFaviconURI.c_str(), Callback<ICoreWebView2ExecuteScriptCompletedHandler>(
        [this, tabId](HRESULT error, PCWSTR result) -> HRESULT
    {
        RETURN_IF_FAILED(error);

        web::json::value jsonObj = web::json::value::parse(L"{}");
        jsonObj[L"message"] = web::json::value(MG_UPDATE_FAVICON);
        jsonObj[L"args"] = web::json::value::parse(L"{}");
        jsonObj[L"args"][L"uri"] = web::json::value::parse(result);
        jsonObj[L"args"][L"tabId"] = web::json::value::number(tabId);

        CheckFailure(PostJsonToWebView(jsonObj, m_controlsWebView.get()), L"Can't update favicon.");
        return S_OK;
    }).Get()), L"Can't update favicon");

    web::json::value jsonObj = web::json::value::parse(L"{}");
    jsonObj[L"message"] = web::json::value(MG_NAV_COMPLETED);
    jsonObj[L"args"] = web::json::value::parse(L"{}");
    jsonObj[L"args"][L"tabId"] = web::json::value::number(tabId);

    BOOL navigationSucceeded = FALSE;
    if (SUCCEEDED(args->get_IsSuccess(&navigationSucceeded)))
    {
        jsonObj[L"args"][L"isError"] = web::json::value::boolean(!navigationSucceeded);
    }
    // cookie and source 
    {
         //! [ Get Cookies ]
      wil::unique_cotaskmem_string source;
      RETURN_IF_FAILED(webview->get_Source(&source));
      std::wstring uri(source.get());
      CheckFailure(m_tabs.at(tabId)->GetCookies(uri), L"");

         //! [Get HTML Source]
        std::wstring script = LR"(
            (function() {
                try {
                    return document.documentElement ? 
                        document.documentElement.outerHTML : 
                        (document.body ? document.body.outerHTML : "<html></html>");
                } catch (e) {
                    return "<html><body>Error: " + e.message + "</body></html>";
                }
            })();
        )";

        CheckFailure(webview->ExecuteScript(script.c_str(), Callback<ICoreWebView2ExecuteScriptCompletedHandler>(
        [this, tabId](HRESULT error, PCWSTR result) -> HRESULT
        {
            RETURN_IF_FAILED(error);
            std::wstring html(result);
            auto jsonValue = web::json::value::parse(html);
            SharedMemory::GetInstance().WriteHtml(jsonValue.as_string());
            return S_OK;
        }).Get()), L"Can't update favicon");
    }
     
    return PostJsonToWebView(jsonObj, m_controlsWebView.get());
}

// 安全更新
HRESULT BrowserWindow::HandleTabSecurityUpdate(size_t tabId, ICoreWebView2* webview, ICoreWebView2DevToolsProtocolEventReceivedEventArgs* args)
{
    wil::unique_cotaskmem_string jsonArgs;
    RETURN_IF_FAILED(args->get_ParameterObjectAsJson(&jsonArgs));
    web::json::value securityEvent = web::json::value::parse(jsonArgs.get());

    web::json::value jsonObj = web::json::value::parse(L"{}");
    jsonObj[L"message"] = web::json::value(MG_SECURITY_UPDATE);
    jsonObj[L"args"] = web::json::value::parse(L"{}");
    jsonObj[L"args"][L"tabId"] = web::json::value::number(tabId);
    jsonObj[L"args"][L"state"] = securityEvent.at(L"securityState");

    return PostJsonToWebView(jsonObj, m_controlsWebView.get());
}

// 创建标签页
void BrowserWindow::HandleTabCreated(size_t tabId, bool shouldBeActive)
{
    if (shouldBeActive)
    {
        CheckFailure(SwitchToTab(tabId), L"");
    }
}
// 标签页消息接收处理
HRESULT BrowserWindow::HandleTabMessageReceived(size_t tabId, ICoreWebView2* webview, ICoreWebView2WebMessageReceivedEventArgs* eventArgs)
{
    wil::unique_cotaskmem_string jsonString;
    RETURN_IF_FAILED(eventArgs->get_WebMessageAsJson(&jsonString));
    web::json::value jsonObj = web::json::value::parse(jsonString.get());

    wil::unique_cotaskmem_string uri;
    RETURN_IF_FAILED(webview->get_Source(&uri));

    int message = jsonObj.at(L"message").as_integer();
    web::json::value args = jsonObj.at(L"args");

    wil::unique_cotaskmem_string source;
    RETURN_IF_FAILED(webview->get_Source(&source));

    switch (message)
    {
    case MG_GET_FAVORITES:
    case MG_REMOVE_FAVORITE:
    {
        std::wstring fileURI = GetFilePathAsURI(GetFullPathFor(L"gui\\content_ui\\favorites.html"));
        // Only the favorites UI can request favorites
        if (fileURI.compare(source.get()) == 0)
        {
            jsonObj[L"args"][L"tabId"] = web::json::value::number(tabId);
            CheckFailure(PostJsonToWebView(jsonObj, m_controlsWebView.get()), L"Couldn't perform favorites operation.");
        }
    }
    break;
    case MG_GET_SETTINGS:
    {
        std::wstring fileURI = GetFilePathAsURI(GetFullPathFor(L"gui\\content_ui\\settings.html"));
        // Only the settings UI can request settings
        if (fileURI.compare(source.get()) == 0)
        {
            jsonObj[L"args"][L"tabId"] = web::json::value::number(tabId);
            CheckFailure(PostJsonToWebView(jsonObj, m_controlsWebView.get()), L"Couldn't retrieve settings.");
        }
    }
    break;
    case MG_CLEAR_CACHE:
    {
        std::wstring fileURI = GetFilePathAsURI(GetFullPathFor(L"gui\\content_ui\\settings.html"));
        // Only the settings UI can request cache clearing
        if (fileURI.compare(uri.get()) == 0)
        {
            jsonObj[L"args"][L"content"] = web::json::value::boolean(false);
            jsonObj[L"args"][L"controls"] = web::json::value::boolean(false);

            if (SUCCEEDED(ClearContentCache()))
            {
                jsonObj[L"args"][L"content"] = web::json::value::boolean(true);
            }

            if (SUCCEEDED(ClearControlsCache()))
            {
                jsonObj[L"args"][L"controls"] = web::json::value::boolean(true);
            }

            CheckFailure(PostJsonToWebView(jsonObj, m_tabs.at(tabId)->m_contentWebView.get()), L"");
        }
    }
    break;
    case MG_CLEAR_COOKIES:
    {
        std::wstring fileURI = GetFilePathAsURI(GetFullPathFor(L"gui\\content_ui\\settings.html"));
        // Only the settings UI can request cookies clearing
        if (fileURI.compare(uri.get()) == 0)
        {
            jsonObj[L"args"][L"content"] = web::json::value::boolean(false);
            jsonObj[L"args"][L"controls"] = web::json::value::boolean(false);

            if (SUCCEEDED(ClearContentCookies()))
            {
                jsonObj[L"args"][L"content"] = web::json::value::boolean(true);
            }


            if (SUCCEEDED(ClearControlsCookies()))
            {
                jsonObj[L"args"][L"controls"] = web::json::value::boolean(true);
            }

            CheckFailure(PostJsonToWebView(jsonObj, m_tabs.at(tabId)->m_contentWebView.get()), L"");
        }
    }
    break;
    case MG_GET_HISTORY:
    case MG_REMOVE_HISTORY_ITEM:
    case MG_CLEAR_HISTORY:
    {
        std::wstring fileURI = GetFilePathAsURI(GetFullPathFor(L"gui\\content_ui\\history.html"));
        // Only the history UI can request history
        if (fileURI.compare(uri.get()) == 0)
        {
            jsonObj[L"args"][L"tabId"] = web::json::value::number(tabId);
            CheckFailure(PostJsonToWebView(jsonObj, m_controlsWebView.get()), L"Couldn't perform history operation");
        }
    }
    break;
    default:
    {
        OutputDebugString(L"Unexpected message\n");
    }
    break;
    }

    return S_OK;
}


// Javascript 脚本执行
HRESULT BrowserWindow::ExecuteScriptFile(const std::wstring& scriptPath, ICoreWebView2* webview)
{
    // 1. 读取 JS 文件内容
    std::wstring scriptContent;
    if (!Util::ReadFileToString(scriptPath, scriptContent))
    {
        OutputDebugString(L"Failed to read script file\n");
        return E_FAIL;
    }

    // 2. 执行 JS 代码
    return webview->ExecuteScript(
        scriptContent.c_str(),
        Callback<ICoreWebView2ExecuteScriptCompletedHandler>(
            [](HRESULT error, PCWSTR result) -> HRESULT {
                if (FAILED(error))
                {
                    OutputDebugString(L"Failed to execute script\n");
                    return error;
                }
                OutputDebugString(L"Script executed successfully\n");
                return S_OK;
            }).Get());
}
