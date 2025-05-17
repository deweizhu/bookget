#include "BrowserWindow.h"
#include "Config.h"
#include <filesystem>

// 静态成员初始化
WCHAR BrowserWindow::s_windowClass[MAX_LOADSTRING] = {0};
WCHAR BrowserWindow::s_title[MAX_LOADSTRING] = {0};




BrowserWindow::~BrowserWindow() {
    StopBackgroundThread();
    SharedMemory::GetInstance().Cleanup();
      // 移除AJAX请求监听
    if (m_webResourceResponseReceivedToken.value != 0)
    {
        wil::com_ptr<ICoreWebView2_3> webview3;
        if (SUCCEEDED(m_contentEnv->QueryInterface(IID_PPV_ARGS(&webview3))))
        {
            webview3->remove_WebResourceRequested(m_webResourceResponseReceivedToken);
        }
    }
};

//  窗口类注册实现
//  FUNCTION: WndProcStatic(HWND, UINT, WPARAM, LPARAM)
//
//  PURPOSE: Redirect messages to approriate instance or call default proc
//
//  WM_COMMAND  - process the application menu
//  WM_PAINT    - Paint the main window
//  WM_DESTROY  - post a quit message and return
//
//
ATOM BrowserWindow::RegisterClass(_In_ HINSTANCE hInstance) {
    LoadStringW(hInstance, IDC_BOOKGETAPP, s_windowClass, MAX_LOADSTRING);
    WNDCLASSEXW wcex;

    wcex.cbSize = sizeof(WNDCLASSEX);

    wcex.style = CS_HREDRAW | CS_VREDRAW;
    wcex.lpfnWndProc = WndProcStatic;
    wcex.cbClsExtra = 0;
    wcex.cbWndExtra = 0;
    wcex.hInstance = hInstance;
    wcex.hIcon = LoadIcon(hInstance, MAKEINTRESOURCE(IDI_WEBVIEWBROWSERAPP));
    wcex.hCursor = LoadCursor(nullptr, IDC_ARROW);
    wcex.hbrBackground = (HBRUSH)(COLOR_WINDOW + 1);
    wcex.lpszMenuName = MAKEINTRESOURCEW(IDC_BOOKGETAPP);
    wcex.lpszClassName = s_windowClass;
    wcex.hIconSm = LoadIcon(wcex.hInstance, MAKEINTRESOURCE(IDI_SMALL));

    return RegisterClassExW(&wcex);
}


//   窗口初始化实现...
//   FUNCTION: InitInstance(HINSTANCE, int)
//
//   PURPOSE: Saves instance handle and creates main window
//
//   COMMENTS:
//
//        In this function, we save the instance handle in a global variable and
//        create and display the main program window.
//
BOOL BrowserWindow::InitInstance(HINSTANCE hInstance, int nCmdShow)
{
    m_hInst = hInstance; // Store app instance handle
    LoadStringW(m_hInst, IDS_APP_TITLE, s_title, MAX_LOADSTRING);

    SetUIMessageBroker();

    m_hWnd = CreateWindowW(s_windowClass, s_title, WS_OVERLAPPEDWINDOW,
        CW_USEDEFAULT, 0, CW_USEDEFAULT, 0, nullptr, nullptr, m_hInst, this);

    if (!m_hWnd)
    {
        return FALSE;
    }
    // Make the BrowserWindow instance ptr available through the hWnd
    SetWindowLongPtr(m_hWnd, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(this));

    UpdateMinWindowSize();
    ShowWindow(m_hWnd, nCmdShow);
    UpdateWindow(m_hWnd);

    // Get directory for user data. This will be kept separated from the
    // directory for the browser UI data.
    std::wstring userDataDirectory = GetUserDataDirectory();

    // Create WebView environment for web content requested by the user. All
    // tabs will be created from this environment and kept isolated from the
    // browser UI. This enviroment is created first so the UI can request new
    // tabs when it's ready.
    HRESULT hr = CreateCoreWebView2EnvironmentWithOptions(nullptr, userDataDirectory.c_str(),
        nullptr, Callback<ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler>(
            [this](HRESULT result, ICoreWebView2Environment* env) -> HRESULT
    {
        RETURN_IF_FAILED(result);

        m_contentEnv = env;
        HRESULT hr = InitUIWebViews();

        if (!SUCCEEDED(hr))
        {
            OutputDebugString(L"UI WebViews environment creation failed\n");
             WCHAR errorMsg[256];
            swprintf_s(errorMsg, L"Error code: 0x%08X", hr);
            OutputDebugString(errorMsg);
            return 0;
        }

        // 1. 加载 YAML 配置文件
        std::wstring configPath = GetFullPathFor(L"config.yaml");
        auto &conf = Config::GetInstance();
        if (!conf.Load(Util::WideToUtf8(configPath))) {
            OutputDebugString(L"Failed to load YAML config\n");
             return 0;
        }

      
        // 2. 初始化下载目录
        std::wstring downloadDir = Util::Utf8ToWide(conf.GetDownloadDir());
        if (!CreateDirectory(downloadDir.c_str(), NULL) && GetLastError() != ERROR_ALREADY_EXISTS) {
            OutputDebugString(L"Could not create downloads directory\n");
            return 0;
        }
        //if (!std::filesystem::exists(downloadDir)) {
        //    std::filesystem::create_directory(downloadDir);
        //}

        //3. 初始化共享内存
        if (SharedMemory::GetInstance().Init()) {
            StartBackgroundThread();
        }
        // 4. 待第一个tab页加载完 Tab::Init()末尾

        return hr;
    }).Get());

    if (!SUCCEEDED(hr))
    {
        OutputDebugString(L"Content WebViews environment creation failed\n");
        return FALSE;
    }
  
    return TRUE;
}

// 主消息处理实现
//
//  FUNCTION: WndProcStatic(HWND, UINT, WPARAM, LPARAM)
//
//  PURPOSE: Redirect messages to approriate instance or call default proc
//
//  WM_COMMAND  - process the application menu
//  WM_PAINT    - Paint the main window
//  WM_DESTROY  - post a quit message and return
//
//
LRESULT CALLBACK BrowserWindow::WndProcStatic(HWND hWnd, UINT message, WPARAM wParam, LPARAM lParam)
{
    // Get the ptr to the BrowserWindow instance who created this hWnd.
    // The pointer was set when the hWnd was created during InitInstance.
    BrowserWindow* browser_window = reinterpret_cast<BrowserWindow*>(GetWindowLongPtr(hWnd, GWLP_USERDATA));

    if (message == WM_CREATE)
    {
        CREATESTRUCT* pCreate = reinterpret_cast<CREATESTRUCT*>(lParam);
        browser_window = reinterpret_cast<BrowserWindow*>(pCreate->lpCreateParams);
        SetWindowLongPtr(hWnd, GWLP_USERDATA, reinterpret_cast<LONG_PTR>(browser_window));
    }

    if (browser_window != nullptr)
    {
        return browser_window->WndProc(hWnd, message, wParam, lParam);  // Forward message to instance-aware WndProc
    }
    else
    {
        return DefWindowProc(hWnd, message, wParam, lParam);
    }
}


//
//  FUNCTION: WndProc(HWND, UINT, WPARAM, LPARAM)
//
//  PURPOSE: Processes messages for each browser window instance.
//
//  WM_COMMAND  - process the application menu
//  WM_PAINT    - Paint the main window
//  WM_DESTROY  - post a quit message and return
//
//
LRESULT CALLBACK BrowserWindow::WndProc(HWND hWnd, UINT message, WPARAM wParam, LPARAM lParam)
{
    BrowserWindow* pThis = reinterpret_cast<BrowserWindow*>(::GetWindowLongPtr(hWnd, GWLP_USERDATA));
    if (!pThis) return DefWindowProc(hWnd, message, wParam, lParam);

    switch (message)
    {
        case WM_APP_DOWNLOAD_NEXT:
        {
            pThis->m_downloader.DownloadNextImage(hWnd); 
        }
        break;
    
        case WM_GETMINMAXINFO:
        {
            MINMAXINFO* minmax = reinterpret_cast<MINMAXINFO*>(lParam);
            minmax->ptMinTrackSize.x = m_minWindowWidth;
            minmax->ptMinTrackSize.y = m_minWindowHeight;
        }
        break;
    
        case WM_DPICHANGED:
        {
            UpdateMinWindowSize();
        }
        break;
    
        case WM_SIZE:
        {
            ResizeUIWebViews();
            if (m_tabs.find(m_activeTabId) != m_tabs.end())
            {
                m_tabs.at(m_activeTabId)->ResizeWebView();
            }
        }
        break;
        case WM_CREATE:
        {
        }
        break;
        case WM_TIMER:
        {
            if (wParam == 1) {
                // 使用PostMessage将工作转移到线程池
                PostMessage(hWnd, WM_APP_DO_WORK, 0, 0);
            }
            break;
        }
        case WM_APP_DO_WORK: {
            // 在线程池中执行耗时操作
            std::thread([this]() {
                // 实际的共享内存读取操作
                auto data = SharedMemory::GetInstance().Read();
                // 需要更新UI时回主线程
                PostMessage(m_hWnd, WM_APP_UPDATE_UI, 0, (LPARAM)new auto(data));
            }).detach();

            break;
            }
        case WM_APP_UPDATE_UI: {
                HandleSharedMemoryUpdate(lParam);
            break;
        }
        case WM_DOWNLOAD_URL: {
            std::unique_ptr<std::wstring> pUrl(reinterpret_cast<std::wstring*>(lParam));
            if (pUrl) {
                LPCWSTR uri = pUrl->c_str();
                // 处理 URL
                if (m_tabs.find(m_activeTabId) != m_tabs.end() && m_tabs.at(m_activeTabId)->m_contentWebView) {
                    m_tabs.at(m_activeTabId)->m_contentWebView->Navigate(uri);
                }
            }
            break;
        }
        //case WM_DOWNLOAD_URL_COMPLETE: {
        //       std::unique_ptr<std::wstring> pUrl(reinterpret_cast<std::wstring*>(lParam));
        //        if (pUrl) {
        //            LPCWSTR uri = pUrl->c_str();
        //            ExecuteScriptFile(uri, m_tabs.at(m_activeTabId)->m_contentWebView.get());
        //        }
        //    break;
        //}
        case WM_CLOSE:
        {
            SharedMemory::GetInstance().Cleanup();

            web::json::value jsonObj = web::json::value::parse(L"{}");
            jsonObj[L"message"] = web::json::value(MG_CLOSE_WINDOW);
            jsonObj[L"args"] = web::json::value::parse(L"{}");

            CheckFailure(PostJsonToWebView(jsonObj, m_controlsWebView.get()), L"Try again.");
        }
        break;
    
        case WM_NCDESTROY:
        {
            SetWindowLongPtr(hWnd, GWLP_USERDATA, NULL);
            delete this;
            PostQuitMessage(0);
            return 0;  // 这里直接返回，不需要break
        }
    
        case WM_PAINT:
        {
            PAINTSTRUCT ps;
            HDC hdc = BeginPaint(hWnd, &ps);
            EndPaint(hWnd, &ps);
        }
        break;
    
        default:
        {
            return DefWindowProc(hWnd, message, wParam, lParam);
        }
    }
    return 0;
}


 // 窗口启动实现
BOOL BrowserWindow::LaunchWindow(_In_ HINSTANCE hInstance, _In_ int nCmdShow)
{
    // BrowserWindow keeps a reference to itself in its host window and will
    // delete itself when the window is destroyed.
    BrowserWindow* window = new BrowserWindow();
    if (!window->InitInstance(hInstance, nCmdShow))
    {
        delete window;
        return FALSE;
    }
    return TRUE;
}


// UI调整大小
HRESULT BrowserWindow::ResizeUIWebViews()
{
    if (m_controlsWebView != nullptr)
    {
        RECT bounds;
        GetClientRect(m_hWnd, &bounds);
        bounds.bottom = bounds.top + GetDPIAwareBound(c_uiBarHeight);
        bounds.bottom += 1;

        RETURN_IF_FAILED(m_controlsController->put_Bounds(bounds));
    }

    if (m_optionsWebView != nullptr)
    {
        RECT bounds;
        GetClientRect(m_hWnd, &bounds);
        bounds.top = GetDPIAwareBound(c_uiBarHeight);
        bounds.bottom = bounds.top + GetDPIAwareBound(c_optionsDropdownHeight);
        bounds.left = bounds.right - GetDPIAwareBound(c_optionsDropdownWidth);

        RETURN_IF_FAILED(m_optionsController->put_Bounds(bounds));
    }

    // Workaround for black controls WebView issue in Windows 7
    HWND wvWindow = GetWindow(m_hWnd, GW_CHILD);
    while (wvWindow != nullptr)
    {
        UpdateWindow(wvWindow);
        wvWindow = GetWindow(wvWindow, GW_HWNDNEXT);
    }

    return S_OK;
}

// 最小化窗口
void BrowserWindow::UpdateMinWindowSize()
{
    RECT clientRect;
    RECT windowRect;

    GetClientRect(m_hWnd, &clientRect);
    GetWindowRect(m_hWnd, &windowRect);

    int bordersWidth = (windowRect.right - windowRect.left) - clientRect.right;
    int bordersHeight = (windowRect.bottom - windowRect.top) - clientRect.bottom;

    m_minWindowWidth = GetDPIAwareBound(MIN_WINDOW_WIDTH) + bordersWidth;
    m_minWindowHeight = GetDPIAwareBound(MIN_WINDOW_HEIGHT) + bordersHeight;
}


int BrowserWindow::GetDPIAwareBound(int bound)
{
    // Remove the GetDpiForWindow call when using Windows 7 or any version
    // below 1607 (Windows 10). You will also have to make sure the build
    // directory is clean before building again.
    return (bound * GetDpiForWindow(m_hWnd) / DEFAULT_DPI);
}


// Set the message broker for the UI webview. This will capture messages from ui web content.
// Lambda is used to capture the instance while satisfying Microsoft::WRL::Callback<T>()
void BrowserWindow::SetUIMessageBroker()
{
    m_uiMessageBroker = Callback<ICoreWebView2WebMessageReceivedEventHandler>(
        [this](ICoreWebView2* webview, ICoreWebView2WebMessageReceivedEventArgs* eventArgs) -> HRESULT
    {
        wil::unique_cotaskmem_string jsonString;
        CheckFailure(eventArgs->get_WebMessageAsJson(&jsonString), L"");  // Get the message from the UI WebView as JSON formatted string
        web::json::value jsonObj = web::json::value::parse(jsonString.get());

        if (!jsonObj.has_field(L"message"))
        {
            OutputDebugString(L"No message code provided\n");
            return S_OK;
        }

        if (!jsonObj.has_field(L"args"))
        {
            OutputDebugString(L"The message has no args field\n");
            return S_OK;
        }

        int message = jsonObj.at(L"message").as_integer();
        web::json::value args = jsonObj.at(L"args");

        switch (message)
        {
        case MG_CREATE_TAB:
        {
            size_t id = args.at(L"tabId").as_number().to_uint32();
            bool shouldBeActive = args.at(L"active").as_bool();
            std::unique_ptr<Tab> newTab = Tab::CreateNewTab(m_hWnd, m_contentEnv.get(), id, shouldBeActive);

            // 如果有提供URI，在新标签页中导航
            if (args.has_field(L"uri"))
            {
                std::wstring uri = args.at(L"uri").as_string();
                newTab->m_contentWebView->Navigate(uri.c_str());
            }

            std::map<size_t, std::unique_ptr<Tab>>::iterator it = m_tabs.find(id);
            if (it == m_tabs.end())
            {
                m_tabs.insert(std::pair<size_t,std::unique_ptr<Tab>>(id, std::move(newTab)));
            }
            else
            {
                m_tabs.at(id)->m_contentController->Close();
                it->second = std::move(newTab);
            }
        }
        break;
        case MG_NAVIGATE:
        {
            std::wstring uri(args.at(L"uri").as_string());
            std::wstring browserScheme(L"browser://");

            if (uri.substr(0, browserScheme.size()).compare(browserScheme) == 0)
            {
                // No encoded search URI
                std::wstring path = uri.substr(browserScheme.size());
                if (path.compare(L"favorites") == 0 ||
                    path.compare(L"settings") == 0 ||
                    path.compare(L"history") == 0)
                {
                    std::wstring filePath(L"gui\\content_ui\\");
                    filePath.append(path);
                    filePath.append(L".html");
                    std::wstring fullPath = GetFullPathFor(filePath.c_str());
                    CheckFailure(m_tabs.at(m_activeTabId)->m_contentWebView->Navigate(fullPath.c_str()), L"Can't navigate to browser page.");
                }
                else
                {
                    OutputDebugString(L"Requested unknown browser page\n");
                }
            }
            else if (!SUCCEEDED(m_tabs.at(m_activeTabId)->m_contentWebView->Navigate(uri.c_str())))
            {
                CheckFailure(m_tabs.at(m_activeTabId)->m_contentWebView->Navigate(args.at(L"encodedSearchURI").as_string().c_str()), L"Can't navigate to requested page.");
            }
        }
        break;
        case MG_GO_FORWARD:
        {
            CheckFailure(m_tabs.at(m_activeTabId)->m_contentWebView->GoForward(), L"");
        }
        break;
        case MG_GO_BACK:
        {
            CheckFailure(m_tabs.at(m_activeTabId)->m_contentWebView->GoBack(), L"");
        }
        break;
        case MG_RELOAD:
        {
            CheckFailure(m_tabs.at(m_activeTabId)->m_contentWebView->Reload(), L"");
             
            //! [CookieManager]
            wil::unique_cotaskmem_string source;
            RETURN_IF_FAILED(m_tabs.at(m_activeTabId)->m_contentWebView->get_Source(&source));
            std::wstring uri(source.get());
            CheckFailure(m_tabs.at(m_activeTabId)->GetCookies(uri), L"");
        }
        break;
        case MG_CANCEL:
        {
            CheckFailure(m_tabs.at(m_activeTabId)->m_contentWebView->CallDevToolsProtocolMethod(L"Page.stopLoading", L"{}", nullptr), L"");
        }
        break;
        case MG_SWITCH_TAB:
        {
            size_t tabId = args.at(L"tabId").as_number().to_uint32();

            SwitchToTab(tabId);
        }
        break;
        case MG_CLOSE_TAB:
        {
            size_t id = args.at(L"tabId").as_number().to_uint32();
            m_tabs.at(id)->m_contentController->Close();
            m_tabs.erase(id);
        }
        break;
        case MG_CLOSE_WINDOW:
        {
            DestroyWindow(m_hWnd);
        }
        break;
        case MG_SHOW_OPTIONS:
        {
            CheckFailure(m_optionsController->put_IsVisible(TRUE), L"");
            m_optionsController->MoveFocus(COREWEBVIEW2_MOVE_FOCUS_REASON_PROGRAMMATIC);
        }
        break;
        case MG_HIDE_OPTIONS:
        {
            CheckFailure(m_optionsController->put_IsVisible(FALSE), L"Something went wrong when trying to close the options dropdown.");
        }
        break;
        case MG_OPTION_SELECTED:
        {
            m_tabs.at(m_activeTabId)->m_contentController->MoveFocus(COREWEBVIEW2_MOVE_FOCUS_REASON_PROGRAMMATIC);
        }
        break;
        case MG_GET_FAVORITES:
        case MG_GET_SETTINGS:
        case MG_GET_HISTORY:
        {
            // Forward back to requesting tab
            size_t tabId = args.at(L"tabId").as_number().to_uint32();
            jsonObj[L"args"].erase(L"tabId");

            CheckFailure(PostJsonToWebView(jsonObj, m_tabs.at(tabId)->m_contentWebView.get()), L"Requesting history failed.");
        }
        break;
        default:
        {
            OutputDebugString(L"Unexpected message\n");
        }
        break;
        }

        return S_OK;
    });
}

void BrowserWindow::CheckFailure(HRESULT hr, LPCWSTR errorMessage)
{
    if (FAILED(hr))
    {
        std::wstring message;
        if (!errorMessage || !errorMessage[0])
        {
            message = std::wstring(L"Something went wrong.");
        }
        else
        {
            message = std::wstring(errorMessage);
        }

        MessageBoxW(nullptr, message.c_str(), nullptr, MB_OK);
    }
}
