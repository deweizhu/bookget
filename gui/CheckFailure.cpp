// Copyright (C) Microsoft Corporation. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#include "framework.h"
#include <winrt/base.h>
#include <winrt/Windows.Foundation.h>

#include <iomanip>
#include <sstream>
// Notify the user of a failure with a message box.
void ShowFailure(HRESULT hr, const std::wstring& message)
{
    std::wstringstream formattedMessage;
    formattedMessage << message << ": 0x" << std::hex << std::setw(8) << hr << " ("
                     << winrt::hresult_error(hr).message().c_str() << ")";
    MessageBox(nullptr, formattedMessage.str().c_str(), nullptr, MB_OK);
}

// If something failed, show the error code and fail fast.
void CheckFailure(HRESULT hr, const std::wstring& message)
{
    if (FAILED(hr))
    {
        ShowFailure(hr, message);
        FAIL_FAST();
    }
}

void FeatureNotAvailable()
{
    MessageBox(nullptr,
        L"This feature is not available in the browser version currently being used.",
        L"Feature Not Available", MB_OK);
}
