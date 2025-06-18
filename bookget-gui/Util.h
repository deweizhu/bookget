// Copyright (C) Microsoft Corporation. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#pragma once

#include <ctime>

#include <iostream>
#include <fstream>
#include <sstream>
#include <string>

class Util
{
public:
    static std::wstring UnixEpochToDateTime(double value);

    static bool FileWrite(const std::wstring  &filePath, IStream *content);
    static bool ReadFileToString(const std::wstring& filePath, std::wstring& content);

    static bool fileWrite(std::wstring filename, std::wstring data);
    static bool fileAppend(std::wstring filename, std::wstring data);
    static std::wstring fileRead(std::wstring filename);
    static std::wstring GetCurrentExeDirectory();
    static std::wstring GetUserHomeDirectory();

    static std::wstring BoolToString(BOOL value);
    static std::wstring EncodeQuote(std::wstring raw);

    static std::wstring Utf8ToWide(const std::string& utf8);
    static std::string WideToUtf8(const std::wstring& wide);
   
    static std::string Utf16ToUtf8(const std::wstring& utf16);
    static std::wstring Utf8ToUtf16(const std::string& utf8);

    static bool FindProcessExist(const std::wstring);
    static void Trim(std::wstring& str);
    static bool CheckIfUrlsFileExists();

    static bool IsImageUrl(const std::wstring& url);

    static std::wstring GetFileNameFromUrl(const std::wstring& url);
    static bool IsImageContentType(const wchar_t* contentType);
    static std::wstring ParseJsonBool(const wchar_t* json);

    static std::string replace_fast(const std::string& str,
                        const std::string& from,
                        const std::string& to);

    static std::string replace(const std::string& str, 
                   const std::string& from, 
                   const std::string& to);

    static bool matchUrlPattern(const std::string& pattern, const std::string& url);
    static bool matchUrlPattern(const std::wstring& pattern, const std::wstring& url);
    static bool IsLocalUri(const std::wstring& url);
    static std::wstring GetFullPathFor(HINSTANCE hInst, LPCWSTR relativePath);

    
    static void DebugPrintException(const std::exception& e);

    static std::string removeDisableDevtoolJsCode(const std::string& input);
    static std::string ReadStreamToString(IStream* stream);

    std::vector<uint8_t> removeJPGHeader(const std::vector<uint8_t>& imageData, const std::string& headerMarker);
};

