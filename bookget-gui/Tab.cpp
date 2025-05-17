// Copyright (C) Microsoft Corporation. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#include "BrowserWindow.h"
#include "CheckFailure.h"
#include "Util.h"
#include "env.h"

using namespace Microsoft::WRL;


std::unique_ptr<Tab> Tab::CreateNewTab(HWND hWnd, ICoreWebView2Environment* env, size_t id, bool shouldBeActive)
{
    std::unique_ptr<Tab> tab = std::make_unique<Tab>();
    tab->m_parentHWnd = hWnd;
    tab->m_tabId = id;
    tab->SetMessageBroker();
    tab->Init(env, shouldBeActive);

    return tab;
}

HRESULT Tab::Init(ICoreWebView2Environment* env, bool shouldBeActive)
{
    return env->CreateCoreWebView2Controller(m_parentHWnd, Callback<ICoreWebView2CreateCoreWebView2ControllerCompletedHandler>(
        [this, shouldBeActive](HRESULT result, ICoreWebView2Controller* host) -> HRESULT {
        if (!SUCCEEDED(result))
        {
            OutputDebugString(L"Tab WebView creation failed\n");
            return result;
        }
        m_contentController = host;
        BrowserWindow::CheckFailure(m_contentController->get_CoreWebView2(&m_contentWebView), L"");
        BrowserWindow* browserWindow = reinterpret_cast<BrowserWindow*>(GetWindowLongPtr(m_parentHWnd, GWLP_USERDATA));
        RETURN_IF_FAILED(m_contentWebView->add_WebMessageReceived(m_messageBroker.get(), &m_messageBrokerToken));

        // Register event handler for history change
        RETURN_IF_FAILED(m_contentWebView->add_HistoryChanged(Callback<ICoreWebView2HistoryChangedEventHandler>(
            [this, browserWindow](ICoreWebView2* webview, IUnknown* args) -> HRESULT
        {
            BrowserWindow::CheckFailure(browserWindow->HandleTabHistoryUpdate(m_tabId, webview), L"Can't update go back/forward buttons.");

            return S_OK;
        }).Get(), &m_historyUpdateForwarderToken));

        // Register event handler for source change
        RETURN_IF_FAILED(m_contentWebView->add_SourceChanged(Callback<ICoreWebView2SourceChangedEventHandler>(
            [this, browserWindow](ICoreWebView2* webview, ICoreWebView2SourceChangedEventArgs* args) -> HRESULT
        {
            BrowserWindow::CheckFailure(browserWindow->HandleTabURIUpdate(m_tabId, webview), L"Can't update address bar");

            return S_OK;
        }).Get(), &m_uriUpdateForwarderToken));

        RETURN_IF_FAILED(m_contentWebView->add_NavigationStarting(Callback<ICoreWebView2NavigationStartingEventHandler>(
            [this, browserWindow](ICoreWebView2* webview, ICoreWebView2NavigationStartingEventArgs* args) -> HRESULT
        {
            BrowserWindow::CheckFailure(browserWindow->HandleTabNavStarting(m_tabId, webview), L"Can't update reload button");

            return S_OK;
        }).Get(), &m_navStartingToken));

        RETURN_IF_FAILED(m_contentWebView->add_NavigationCompleted(Callback<ICoreWebView2NavigationCompletedEventHandler>(
            [this, browserWindow](ICoreWebView2* webview, ICoreWebView2NavigationCompletedEventArgs* args) -> HRESULT
        {
            BrowserWindow::CheckFailure(browserWindow->HandleTabNavCompleted(m_tabId, webview, args), L"Can't udpate reload button");

            return S_OK;
        }).Get(), &m_navCompletedToken));

        // Enable listening for security events to update secure icon
        RETURN_IF_FAILED(m_contentWebView->CallDevToolsProtocolMethod(L"Security.enable", L"{}", nullptr));

        BrowserWindow::CheckFailure(m_contentWebView->GetDevToolsProtocolEventReceiver(L"Security.securityStateChanged", &m_securityStateChangedReceiver), L"");

        // Forward security status updates to browser
        RETURN_IF_FAILED(m_securityStateChangedReceiver->add_DevToolsProtocolEventReceived(Callback<ICoreWebView2DevToolsProtocolEventReceivedEventHandler>(
            [this, browserWindow](ICoreWebView2* webview, ICoreWebView2DevToolsProtocolEventReceivedEventArgs* args) -> HRESULT
        {
            BrowserWindow::CheckFailure(browserWindow->HandleTabSecurityUpdate(m_tabId, webview, args), L"Can't udpate security icon");
            return S_OK;
        }).Get(), &m_securityUpdateToken));

      
        
   
        browserWindow->HandleTabCreated(m_tabId, shouldBeActive);

        //! [CookieManager]
        auto webview2_2 = m_contentWebView.try_query<ICoreWebView2_2>();
        if (webview2_2) {
            webview2_2->get_CookieManager(&m_cookieManager);
        }
        //! [CookieManager]
        //! 
      
        wil::com_ptr<ICoreWebView2Settings> settings;
        CHECK_FAILURE(m_contentWebView->get_Settings(&settings));
        CHECK_FAILURE(settings->put_AreDefaultScriptDialogsEnabled(FALSE));
        CHECK_FAILURE(m_contentWebView->add_NewWindowRequested(
            Callback<ICoreWebView2NewWindowRequestedEventHandler>(
                [this](ICoreWebView2* sender, ICoreWebView2NewWindowRequestedEventArgs* args) -> HRESULT
        {
            wil::unique_cotaskmem_string uri;
            args->get_Uri(&uri);
            m_contentWebView->Navigate(uri.get());
       
            args->put_Handled(TRUE);
            return S_OK;
        }).Get(), &m_newWindowRequestedToken));
         // 设置监听下载器
         browserWindow->SetupWebViewListeners(m_contentWebView);

        return S_OK;
    }).Get());
}

void Tab::SetMessageBroker()
{
    m_messageBroker = Callback<ICoreWebView2WebMessageReceivedEventHandler>(
        [this](ICoreWebView2* webview, ICoreWebView2WebMessageReceivedEventArgs* eventArgs) -> HRESULT
    {
        BrowserWindow* browserWindow = reinterpret_cast<BrowserWindow*>(GetWindowLongPtr(m_parentHWnd, GWLP_USERDATA));
        BrowserWindow::CheckFailure(browserWindow->HandleTabMessageReceived(m_tabId, webview, eventArgs), L"");

        return S_OK;
    });
}

HRESULT Tab::ResizeWebView()
{
    RECT bounds;
    GetClientRect(m_parentHWnd, &bounds);

    BrowserWindow* browserWindow = reinterpret_cast<BrowserWindow*>(GetWindowLongPtr(m_parentHWnd, GWLP_USERDATA));
    bounds.top += browserWindow->GetDPIAwareBound(BrowserWindow::c_uiBarHeight);

    return m_contentController->put_Bounds(bounds);
}


HRESULT Tab::GetCookies(std::wstring uri) {
        //! [CookieManager]
        //! 
    if (m_cookieManager)
    {
        BrowserWindow* browserWindow = reinterpret_cast<BrowserWindow*>(GetWindowLongPtr(m_parentHWnd, GWLP_USERDATA));

        CHECK_FAILURE(m_cookieManager->GetCookies(
            uri.c_str(),
            Callback<ICoreWebView2GetCookiesCompletedHandler>(
                [this, uri, browserWindow](HRESULT error_code, ICoreWebView2CookieList* list) -> HRESULT {
                    CHECK_FAILURE(error_code);

                    std::wstring result;
                    UINT cookie_list_size;
                    CHECK_FAILURE(list->get_Count(&cookie_list_size));

                    if (cookie_list_size == 0)
                    {
                        result += L"#No cookies found.";
                    }
                    else
                    {
                        result += L"#"+ std::to_wstring(cookie_list_size) + L" cookie(s) found";
                        if (!uri.empty()) {
                            result += L" on " + uri;
                        }
                        result += L"\n#domain\t  subdomains\t  path\t  HTTPS only\t  expires\t  name\t  value\t secure\t  Same site\n\n";
                        for (UINT i = 0; i < cookie_list_size; ++i)
                        {
                            wil::com_ptr<ICoreWebView2Cookie> cookie;
                            CHECK_FAILURE(list->GetValueAtIndex(i, &cookie));

                            if (cookie.get())
                            {
                                result += CookieToString(cookie.get());
                                if (i != cookie_list_size - 1)
                                {
                                    result += L"\n";
                                }
                            }
                        }
                        result += L"\n";
                    }
                    Util::fileWrite(Util::GetCurrentExeDirectory() + L"\\cookie.txt", result);
                    SharedMemory::GetInstance().WriteCookies(result);
                    return S_OK;
                }).Get()));


    }
    return 0;
}


std::wstring Tab::CookieToString(ICoreWebView2Cookie* cookie)
{
    //! [CookieObject]
    wil::unique_cotaskmem_string name;
    CHECK_FAILURE(cookie->get_Name(&name));
    wil::unique_cotaskmem_string value;
    CHECK_FAILURE(cookie->get_Value(&value));
    wil::unique_cotaskmem_string domain;
    CHECK_FAILURE(cookie->get_Domain(&domain));
    wil::unique_cotaskmem_string path;
    CHECK_FAILURE(cookie->get_Path(&path));
    double expires;
    CHECK_FAILURE(cookie->get_Expires(&expires));
    BOOL isHttpOnly = FALSE;
    CHECK_FAILURE(cookie->get_IsHttpOnly(&isHttpOnly));
    COREWEBVIEW2_COOKIE_SAME_SITE_KIND same_site;
    std::wstring same_site_as_string;
    CHECK_FAILURE(cookie->get_SameSite(&same_site));
    switch (same_site)
    {
    case COREWEBVIEW2_COOKIE_SAME_SITE_KIND_NONE:
        same_site_as_string = L"None";
        break;
    case COREWEBVIEW2_COOKIE_SAME_SITE_KIND_LAX:
        same_site_as_string = L"Lax";
        break;
    case COREWEBVIEW2_COOKIE_SAME_SITE_KIND_STRICT:
        same_site_as_string = L"Strict";
        break;
    }
    BOOL isSecure = FALSE;
    CHECK_FAILURE(cookie->get_IsSecure(&isSecure));
    BOOL isSession = FALSE;
    CHECK_FAILURE(cookie->get_IsSession(&isSession));

    //see https://curl.se/docs/http-cookies.html
    //Field number, what type and example data and the meaning of it:
    //0. string example.com - the domain name
    //1. boolean FALSE - include subdomains
    //2. string /foobar/ - path
    //3. boolean TRUE - send/receive over HTTPS only
    //4. number 1462299217 - expires at - seconds since Jan 1st 1970, or 0
    //5. string person - name of the cookie
    //6. string daniel - value of the cookie
    //7. boolean FALSE - isSecure 
    //8. string None - Same site

    std::wstring result = L"";
    std::wstring sDomain = Util::EncodeQuote(domain.get());
    result +=  sDomain + L"\t";
    if (sDomain.starts_with(L"\".")) {
         result +=  L"true\t";
    }
    else {
        result +=  L"false\t"; 
    }
    result += Util::EncodeQuote(path.get()) + L"\t";
    result += Util::BoolToString(isHttpOnly) + L"\t";  
    if (!!isSession)
    {
        result += L"#HttpOnly_\t";
    }
    else
    {
        result += std::to_wstring(expires)+ L"\t";  
    }
    result += Util::EncodeQuote(name.get()) + L"\t";
    result += Util::EncodeQuote(value.get()) + L"\t"; 
    result += Util::BoolToString(isSecure) + L"\t";
    result += Util::EncodeQuote(same_site_as_string) + L"\t";

    return result;
    //! [CookieObject]
}


