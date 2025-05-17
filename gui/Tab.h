// Copyright (C) Microsoft Corporation. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#pragma once

#include "framework.h"

class Tab
{
public:
    wil::com_ptr<ICoreWebView2Controller> m_contentController;
    wil::com_ptr<ICoreWebView2> m_contentWebView;
    wil::com_ptr<ICoreWebView2DevToolsProtocolEventReceiver> m_securityStateChangedReceiver;
    wil::com_ptr<ICoreWebView2CookieManager> m_cookieManager;


    static std::unique_ptr<Tab> CreateNewTab(HWND hWnd, ICoreWebView2Environment* env, size_t id, bool shouldBeActive);
    HRESULT ResizeWebView();

    HRESULT GetCookies(std::wstring uri);
    static std::wstring CookieToString(ICoreWebView2Cookie* cookie);

protected:

    HWND m_parentHWnd = nullptr;
    size_t m_tabId = INVALID_TAB_ID;
    EventRegistrationToken m_historyUpdateForwarderToken = {};
    EventRegistrationToken m_uriUpdateForwarderToken = {};
    EventRegistrationToken m_navStartingToken = {};
    EventRegistrationToken m_navCompletedToken = {};
    EventRegistrationToken m_securityUpdateToken = {};
    EventRegistrationToken m_messageBrokerToken = {};  // Message broker for browser pages loaded in a tab
    wil::com_ptr<ICoreWebView2WebMessageReceivedEventHandler> m_messageBroker;

    HRESULT Init(ICoreWebView2Environment* env, bool shouldBeActive);
    void SetMessageBroker();

private:
    EventRegistrationToken m_newWindowRequestedToken;

protected:
    EventRegistrationToken m_webResourceResponseReceivedToken = {};

};

