// Copyright (C) Microsoft Corporation. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#pragma once

#include "framework.h"
#include "Tab.h"
#include <chrono>
#include <thread>
#include "SharedMemory.h"
#include <mutex>
#include <atomic>
#include <memory>
#include <vector>
#include <string>
#include <wrl.h>
#include <wil/com.h>
#include <cpprest/json.h>
#include "Util.h"

#include <yaml-cpp/yaml.h>
#include <filesystem>

#include "shlobj.h"
#include <Urlmon.h>
#pragma comment (lib, "Urlmon.lib")
#include "env.h"

#include <shlwapi.h> // for PathCombine
#include "CheckFailure.h"
#pragma comment(lib, "shlwapi.lib")

#include "Downloader.h"


using namespace Microsoft::WRL;
using Microsoft::WRL::Callback;


class BrowserWindow
{
public:
    BrowserWindow(){};
    ~BrowserWindow();

    // 窗口尺寸常量
    static const int c_uiBarHeight = 70;
    static const int c_optionsDropdownHeight = 108;
    static const int c_optionsDropdownWidth = 200;

    // 窗口管理
    static ATOM RegisterClass(_In_ HINSTANCE hInstance);
    static LRESULT CALLBACK WndProcStatic(HWND hWnd, UINT message, WPARAM wParam, LPARAM lParam);
    LRESULT CALLBACK WndProc(HWND hWnd, UINT message, WPARAM wParam, LPARAM lParam);
    static BOOL LaunchWindow(_In_ HINSTANCE hInstance, _In_ int nCmdShow);



    // WebView管理
    HRESULT OpenWindowTab(wchar_t *webUrl, bool isTab = false);
    HRESULT ExecuteScriptFile(const std::wstring& scriptPath, ICoreWebView2* webview);

    // 标签页管理
    HRESULT HandleTabURIUpdate(size_t tabId, ICoreWebView2* webview);
    HRESULT HandleTabHistoryUpdate(size_t tabId, ICoreWebView2* webview);
    HRESULT HandleTabNavStarting(size_t tabId, ICoreWebView2* webview);
    HRESULT HandleTabNavCompleted(size_t tabId, ICoreWebView2* webview, ICoreWebView2NavigationCompletedEventArgs* args);
    HRESULT HandleTabSecurityUpdate(size_t tabId, ICoreWebView2* webview, ICoreWebView2DevToolsProtocolEventReceivedEventArgs* args);
    void HandleTabCreated(size_t tabId, bool shouldBeActive);
    HRESULT HandleTabMessageReceived(size_t tabId, ICoreWebView2* webview, ICoreWebView2WebMessageReceivedEventArgs* eventArgs);
    void HandleSharedMemoryUpdate(LPARAM lParam);

    // 下载管理
    bool DownloadFile(const std::wstring& sUrl, IStream *content);
 
    // 工作线程
    void StartBackgroundThread();
    void StopBackgroundThread();

    // 实用工具
    static std::wstring GetAppDataDirectory();
    std::wstring GetFullPathFor(LPCWSTR relativePath);
    std::wstring GetFilePathAsURI(std::wstring fullPath);
    static std::wstring GetUserDataDirectory();
    int GetDPIAwareBound(int bound);
    static void CheckFailure(HRESULT hr, LPCWSTR errorMessage = L"");

private:
    //工作线程
    std::mutex m_threadMutex;
    std::thread m_sharedMemoryThread;
    std::atomic<bool> m_stopThread{false};

protected:
    // 窗口资源
    HINSTANCE m_hInst = nullptr;
    HWND m_hWnd = nullptr;
    static WCHAR s_windowClass[MAX_LOADSTRING];
    static WCHAR s_title[MAX_LOADSTRING];
    int m_minWindowWidth = 0;
    int m_minWindowHeight = 0;

    // WebView资源
    wil::com_ptr<ICoreWebView2Environment> m_uiEnv;
    wil::com_ptr<ICoreWebView2Environment> m_contentEnv;
    wil::com_ptr<ICoreWebView2Controller> m_controlsController;
    wil::com_ptr<ICoreWebView2Controller> m_optionsController;
    wil::com_ptr<ICoreWebView2> m_controlsWebView;
    wil::com_ptr<ICoreWebView2> m_optionsWebView;
    std::map<size_t, std::unique_ptr<Tab>> m_tabs;
    size_t m_activeTabId = 0;

    // WebView事件处理
    EventRegistrationToken m_controlsUIMessageBrokerToken;
    EventRegistrationToken m_optionsUIMessageBrokerToken;
    EventRegistrationToken m_controlsZoomToken;
    EventRegistrationToken m_optionsZoomToken;
    EventRegistrationToken m_lostOptionsFocus;
    EventRegistrationToken m_newWindowRequestedToken;
    wil::com_ptr<ICoreWebView2WebMessageReceivedEventHandler> m_uiMessageBroker;

    // 下载管理
    Downloader m_downloader;
    EventRegistrationToken m_webResourceResponseReceivedToken;
 

private:



public:
    // 初始化方法
    BOOL InitInstance(HINSTANCE hInstance, int nCmdShow);
    HRESULT InitUIWebViews();
    HRESULT CreateBrowserControlsWebView();
    HRESULT CreateBrowserOptionsWebView();
    void SetUIMessageBroker();
    HRESULT ResizeUIWebViews();
    void UpdateMinWindowSize();
    HRESULT PostJsonToWebView(web::json::value jsonObj, ICoreWebView2* webview);
    HRESULT SwitchToTab(size_t tabId);


    // 缓存和Cookie管理
    HRESULT ClearContentCache();
    HRESULT ClearControlsCache();
    HRESULT ClearContentCookies();
    HRESULT ClearControlsCookies();

    // 下载处理
    void SetupWebViewListeners(wil::com_ptr<ICoreWebView2>& contentWebView);
    HRESULT HandleWebResourceResponseReceived(ICoreWebView2* sender, 
        ICoreWebView2WebResourceResponseReceivedEventArgs* args);
    bool ShouldInterceptRequest(const std::wstring& sUrl, 
        ICoreWebView2WebResourceResponseView* response);
    bool DownloadFile(const std::wstring& sUrl, const std::wstring& filePath, IStream *content);

};
