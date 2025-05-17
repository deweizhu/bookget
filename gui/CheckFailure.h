// Copyright (C) Microsoft Corporation. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
#ifndef CHECK_FAILURE_H
#define	CHECK_FAILURE_H

#include "framework.h"

#include <string>
// Notify the user of a failure with a message box.
void ShowFailure(HRESULT hr, const std::wstring& message = L"Error");

// If something failed, show the error code and fail fast.
void CheckFailure(HRESULT hr, const std::wstring& message = L"Error");

// Needs to be a separate macro because the preprocessor is weird
#define CHECK_FAILURE_STRINGIFY(arg) #arg

// If we use a function-like macro to wrap a function call, the macro expansion covers the
// entire function call, and if that function call contains a lambda which spans many lines,
// it makes error messages, the __LINE__ macro, and debuggers less accurate.  Instead,
// we make it a term-like macro which generates a partially-applied function.  In effect,
//     CHECK_FAILURE(MultiLineFunctionCall(...));
// becomes
//     ([](HRESULT hr){ CheckFailure(hr, "error message"); })(MultiLineFunctionCall(...));
// so that MultiLineFunctionCall(...) doesn't have to be part of the macro expansion.
#define CHECK_FAILURE_FILE_LINE(file, line) ([](HRESULT hr){ CheckFailure(hr, L"Failure at " CHECK_FAILURE_STRINGIFY(file) L"(" CHECK_FAILURE_STRINGIFY(line) L")"); })
#define CHECK_FAILURE CHECK_FAILURE_FILE_LINE(__FILE__, __LINE__)
#define CHECK_FAILURE_BOOL(value) CHECK_FAILURE((value) ? S_OK : E_UNEXPECTED)

// Show a message box indicating that an interface isn't available in this browser version.
// Only call this in direct response to a specific user action.
void FeatureNotAvailable();

// Wraps the above in a conditional.
#define CHECK_FEATURE_RETURN(feature) { if (!feature) { FeatureNotAvailable(); return true; } }

// Returns nothing, which is different from CHECK_FEATURE_RETURN
#define CHECK_FEATURE_RETURN_EMPTY(feature) { if (!feature) { FeatureNotAvailable(); return; } }

#endif // !CHECK_FAILURE_H
