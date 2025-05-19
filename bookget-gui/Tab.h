// Copyright (C) Microsoft Corporation. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#pragma once

#include "framework.h"

class Tab
{
public:
       // WebView资源
    wil::com_ptr<ICoreWebView2Environment> m_contentEnv;

    wil::com_ptr<ICoreWebView2Controller> m_contentController;
    wil::com_ptr<ICoreWebView2> m_contentWebView;
    wil::com_ptr<ICoreWebView2DevToolsProtocolEventReceiver> m_securityStateChangedReceiver;
    wil::com_ptr<ICoreWebView2CookieManager> m_cookieManager;

    wil::com_ptr<ICoreWebView2_22> m_webview22;


    static std::unique_ptr<Tab> CreateNewTab(HWND hWnd, ICoreWebView2Environment* env, size_t id, bool shouldBeActive);
    HRESULT ResizeWebView();

    HRESULT GetCookies(std::wstring uri);
    static std::wstring CookieToString(ICoreWebView2Cookie* cookie);

protected:

    HWND m_parentHWnd = nullptr;
    size_t m_tabId = INVALID_TAB_ID;
    EventRegistrationToken m_historyUpdateForwarderToken = {0};
    EventRegistrationToken m_uriUpdateForwarderToken = {0};
    EventRegistrationToken m_navStartingToken = {0};
    EventRegistrationToken m_navCompletedToken = {0};
    EventRegistrationToken m_securityUpdateToken = {0};
    EventRegistrationToken m_messageBrokerToken = {0};  // Message broker for browser pages loaded in a tab
    wil::com_ptr<ICoreWebView2WebMessageReceivedEventHandler> m_messageBroker;


    HRESULT Init(ICoreWebView2Environment* env, bool shouldBeActive);
    void SetMessageBroker();

public:
    EventRegistrationToken m_newWindowRequestedToken = {0};
    EventRegistrationToken m_webResourceRequestedToken = {0};
    EventRegistrationToken m_webResourceResponseReceivedToken = {0};
    EventRegistrationToken m_navigationToken = {0};
 

    void SetupWebViewListeners();

};

