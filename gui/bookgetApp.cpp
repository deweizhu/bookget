// Copyright (C) Microsoft Corporation. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// bookgetApp.cpp : Defines the entry point for the application.
//

#include "BrowserWindow.h"
#include "bookgetApp.h"
#include "env.h"
#include "Util.h"

using namespace Microsoft::WRL;

void tryLaunchWindow(HINSTANCE hInstance, int nCmdShow);

int APIENTRY wWinMain(_In_ HINSTANCE hInstance,
                      _In_opt_ HINSTANCE hPrevInstance,
                      _In_ LPWSTR    lpCmdLine,
                      _In_ int       nCmdShow)
{
    UNREFERENCED_PARAMETER(hPrevInstance);
    UNREFERENCED_PARAMETER(lpCmdLine);

    // 解析命令行参数
    LPWSTR argumentStart = GetCommandLine();
    //LPWSTR argumentEnd;
    int cArgs;
    LPWSTR* arguments = CommandLineToArgvW(argumentStart, &cArgs);

    for (int i = 0; i < cArgs; i++) {
       if (i == 0) continue;
       std::wstring cmd;
       cmd = arguments[i];
       if (cmd == L"-i") {  
           std::wstring sUrl = arguments[i+1];
           std::error_code ec;
           if (std::filesystem::exists(sUrl, ec)) {
               g_urlsFile = sUrl;
           }
           else {
               g_HomeUrl = sUrl;
           }
           g_arguments.push_back(std::make_pair(cmd, sUrl));
           i++;
       }
    }
    LocalFree(arguments);

    // Call SetProcessDPIAware() instead when using Windows 7 or any version
    // below 1703 (Windows 10).
    SetProcessDpiAwarenessContext(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2);

    BrowserWindow::RegisterClass(hInstance);

    tryLaunchWindow(hInstance, nCmdShow);

    HACCEL hAccelTable = LoadAccelerators(hInstance, MAKEINTRESOURCE(IDC_BOOKGETAPP));

    MSG msg;

    // Main message loop:
    while (GetMessage(&msg, nullptr, 0, 0))
    {
        if (!TranslateAccelerator(msg.hwnd, hAccelTable, &msg))
        {
            TranslateMessage(&msg);
            DispatchMessage(&msg);
        }
    }
    return (int) msg.wParam;
}

void tryLaunchWindow(HINSTANCE hInstance, int nCmdShow)
{
    BOOL launched = BrowserWindow::LaunchWindow(hInstance, nCmdShow);
    if (!launched)
    {
        DWORD err = GetLastError();
      
        std::wstringstream fmtMessage;
        fmtMessage << "Could not launch the browser [ " << err << " ].";
        int msgboxID = MessageBox(NULL, fmtMessage.str().c_str(), L"Error", MB_RETRYCANCEL);

        switch (msgboxID)
        {
        case IDRETRY:
            tryLaunchWindow(hInstance, nCmdShow);
            break;
        case IDCANCEL:
        default:
            PostQuitMessage(0);
        }
    }
}

