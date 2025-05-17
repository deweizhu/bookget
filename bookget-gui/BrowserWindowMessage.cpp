#include "BrowserWindow.h"


HRESULT BrowserWindow::ClearContentCache()
{
    return m_tabs.at(m_activeTabId)->m_contentWebView->CallDevToolsProtocolMethod(L"Network.clearBrowserCache", L"{}", nullptr);
}

HRESULT BrowserWindow::ClearControlsCache()
{
    return m_controlsWebView->CallDevToolsProtocolMethod(L"Network.clearBrowserCache", L"{}", nullptr);
}

HRESULT BrowserWindow::ClearContentCookies()
{
    return m_tabs.at(m_activeTabId)->m_contentWebView->CallDevToolsProtocolMethod(L"Network.clearBrowserCookies", L"{}", nullptr);
}

HRESULT BrowserWindow::ClearControlsCookies()
{
    return m_controlsWebView->CallDevToolsProtocolMethod(L"Network.clearBrowserCookies", L"{}", nullptr);
}


HRESULT BrowserWindow::PostJsonToWebView(web::json::value jsonObj, ICoreWebView2* webview)
{
    utility::stringstream_t stream;
    jsonObj.serialize(stream);

    return webview->PostWebMessageAsJson(stream.str().c_str());
}
