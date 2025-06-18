// Copyright (C) Microsoft Corporation. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

#include "CheckFailure.h"
#include "Util.h"
#include <codecvt>
#include <regex>
#include <Windows.h>
#include <filesystem>
#include <cwctype>
#include <string>    
#include <tlhelp32.h>
#include <shlobj.h> // For SHGetFolderPath
#include <algorithm>
#include <wincrypt.h>
#include <vector>

std::wstring Util::UnixEpochToDateTime(double value)
{
    WCHAR rawResult[32] = {};
    std::time_t rawTime = std::time_t(value / 1000);
    struct tm timeStruct = {};
    gmtime_s(&timeStruct, &rawTime);
    _wasctime_s(rawResult, 32, &timeStruct);
    std::wstring result(rawResult);
    return result;
}

//zhudw

bool Util::FileWrite(const std::wstring  &filePath, IStream *content)
{
    // 使用 CreateFileW 直接写入文件
    HANDLE hFile = CreateFileW(
        filePath.c_str(),
        GENERIC_WRITE,
        0,
        NULL,
        CREATE_ALWAYS,
        FILE_ATTRIBUTE_NORMAL,
        NULL
    );

    if (hFile == INVALID_HANDLE_VALUE)
    {
        OutputDebugString(L"Failed to create file\n");
        return false;
    }

    // 获取数据大小
    ULARGE_INTEGER fileSize = { 0 };
    if (FAILED(IStream_Size(content, &fileSize)) || fileSize.QuadPart == 0)
    {
        CloseHandle(hFile);
        OutputDebugString(L"Failed to get content size\n");
        return false;
    }

    // 重置流指针到起始位置
    LARGE_INTEGER seekPos = { 0 };
    content->Seek(seekPos, STREAM_SEEK_SET, nullptr);

    // 缓冲区（4KB 或更大，取决于性能需求）
    const DWORD bufferSize = 4096;
    BYTE buffer[bufferSize];
    DWORD bytesRead = 0;
    DWORD bytesWritten = 0;

    // 逐块读取并写入文件
    while (SUCCEEDED(content->Read(buffer, bufferSize, &bytesRead)) && bytesRead > 0)
    {
        if (!WriteFile(hFile, buffer, bytesRead, &bytesWritten, NULL) || bytesWritten != bytesRead)
        {
            OutputDebugString(L"Failed to write file\n");
            break;
        }
    }

    CloseHandle(hFile);
    return true;
}

bool Util::ReadFileToString(const std::wstring& filePath, std::wstring& content)
{
    std::wifstream file(filePath);
    if (!file.is_open())
    {
        return false;
    }

    std::wstringstream buffer;
    buffer << file.rdbuf();
    content = buffer.str();
    return true;
}

bool Util::fileWrite(const std::wstring  filename, std::wstring data)
{
    std::ofstream outfile(filename, std::ios::binary | std::ios::out | std::ios::trunc);
	if (!outfile.is_open()) {
		return 0;
	}

    /// 将 UTF-16 字符串转换为 UTF-8 字符串
    int length = ::WideCharToMultiByte(CP_UTF8, 0, data.c_str(), -1, nullptr, 0, nullptr, nullptr);
    if (length == 0) {
        return 0;
    }

    std::string utf8String(length, '\0');
    if (!::WideCharToMultiByte(CP_UTF8, 0, data.c_str(), -1, &utf8String[0], length, nullptr, nullptr)) {
        return 0;
    }
    outfile.write(utf8String.c_str(), utf8String.length());
	outfile.close();
	return true;
}

std::wstring Util::fileRead(const std::wstring  filename)
{
	std::ifstream infile(filename, std::ios::binary);
	if (!infile.is_open()) {
		return L"";
	}

    // 读取文件内容并转换为 UTF-16 字符串
    std::string utf8String;
    std::wstring result;
    while (std::getline(infile, utf8String)) {
         // 将 UTF-8 字符串转换为 UTF-16 字符串
        int length = ::MultiByteToWideChar(CP_UTF8, 0, utf8String.c_str(), -1, nullptr, 0);
        if (length == 0) {
            break;
        }
        std::wstring utf16String(length, L'\0');
        if (!::MultiByteToWideChar(CP_UTF8, 0, utf8String.c_str(), -1, &utf16String[0], length)) {
          break;
        }
        result += utf16String;
    }

	infile.close();
    return result;
}

bool Util::fileAppend(const std::wstring  filename, std::wstring data)
{
    std::ofstream outfile(filename, std::ios::binary | std::ios::out | std::ios::app);
    if (!outfile.is_open()) {
        return 0;
    }

    /// 将 UTF-16 字符串转换为 UTF-8 字符串
    int length = ::WideCharToMultiByte(CP_UTF8, 0, data.c_str(), -1, nullptr, 0, nullptr, nullptr);
    if (length == 0) {
        return 0;
    }
    std::string utf8String(length, '\0');
    if (!::WideCharToMultiByte(CP_UTF8, 0, data.c_str(), -1, &utf8String[0], length, nullptr, nullptr)) {
        return 0;
    }
    outfile.write(utf8String.c_str(), utf8String.length());
	outfile.close();
	return true;
}


std::wstring Util::BoolToString(BOOL value)
{
    return value ? L"true" : L"false";
}

std::wstring Util::EncodeQuote(std::wstring raw)
{
    return L"\"" + std::regex_replace(raw, std::wregex(L"\""), L"\\\"") + L"\"";
}


std::wstring Util::GetCurrentExeDirectory()
{
    TCHAR buffer[MAX_PATH] = {0};
    GetModuleFileName(NULL, buffer, MAX_PATH);
    std::filesystem::path path(buffer);
    std::wstring filePath = path.parent_path().lexically_normal();
    return filePath;
}



std::wstring Util::GetUserHomeDirectory()
{
    wchar_t path[MAX_PATH];

    // CSIDL_PROFILE 表示用户的主目录（通常是 C:\Users\Username）
    if (SUCCEEDED(SHGetFolderPathW(nullptr, CSIDL_PROFILE, nullptr, 0, path)))
    {
        return path;
    }

    // 如果 SHGetFolderPathW 失败，尝试从环境变量获取
    wchar_t userProfile[MAX_PATH];
    size_t requiredSize = 0;
    if (_wgetenv_s(&requiredSize, userProfile, MAX_PATH, L"USERPROFILE") == 0 && requiredSize > 0)
    {
        return userProfile;
    }

    // 如果还是失败，尝试组合 HOMEDRIVE 和 HOMEPATH
    wchar_t homeDrive[MAX_PATH];
    wchar_t homePath[MAX_PATH];
    if (_wgetenv_s(&requiredSize, homeDrive, MAX_PATH, L"HOMEDRIVE") == 0 && requiredSize > 0 &&
        _wgetenv_s(&requiredSize, homePath, MAX_PATH, L"HOMEPATH") == 0 && requiredSize > 0)
    {
        return std::wstring(homeDrive) + homePath;
    }

    // 所有方法都失败，返回空字符串
    return L"";
}

std::wstring Util::Utf8ToWide(const std::string& utf8) {
    if (utf8.empty()) return L"";

    int size_needed = MultiByteToWideChar(CP_UTF8, 0, utf8.c_str(), (int)utf8.size(), NULL, 0);
    std::wstring wide(size_needed, 0);
    MultiByteToWideChar(CP_UTF8, 0, utf8.c_str(), (int)utf8.size(), &wide[0], size_needed);
    return wide;
}


std::string Util::WideToUtf8(const std::wstring& wide) {
    if (wide.empty()) return "";

    int size_needed = WideCharToMultiByte(CP_UTF8, 0, wide.c_str(), (int)wide.size(), NULL, 0, NULL, NULL);
    std::string utf8(size_needed, 0);
    WideCharToMultiByte(CP_UTF8, 0, wide.c_str(), (int)wide.size(), &utf8[0], size_needed, NULL, NULL);
    return utf8;
}

std::string Util::Utf16ToUtf8(const std::wstring& utf16) {

    std::string utf8; // Result
     if (utf16.empty()) {
        return utf8;
     }

    if (utf16.length() > static_cast<size_t>((std::numeric_limits<int>::max)()))
    {
      throw std::overflow_error(
        "Input string too long: size_t-length doesn't fit into int.");
    }

    // Safely convert from size_t (STL string's length)
    // to int (for Win32 APIs)
    const int utf16Length = static_cast<int>(utf16.length());

    // Safely fails if an invalid UTF-8 character
    // is encountered in the input string
    //constexpr DWORD kFlags = MB_ERR_INVALID_CHARS;
    constexpr DWORD kFlags = 0;

    const int utf8Length = ::WideCharToMultiByte(
                              CP_UTF8,       // convert to UTF-8
                              kFlags,        // Conversion flags
                              utf16.data(),   // Source UTF-16 string pointer
                              utf16Length,    // Length of the source UTF-8 string, in chars
                              nullptr,       // Unused - no conversion done in this step
                              0,              // Request size of destination buffer, in wchar_ts
                              nullptr, nullptr
                            );

    if (utf8Length == 0)
    {
      // Conversion error: capture error code and throw
      const DWORD error = ::GetLastError();
      throw std::overflow_error(
        "Cannot get result string length when converting " \
        "from UTF-16 to UTF-8 (WideCharToMultiByte failed)." +
        error);
    }

    utf8.resize(utf8Length);

    // Convert from UTF-16 to UTF-8
    int result = ::WideCharToMultiByte(
      CP_UTF8,       // convert to UTF-8
      kFlags,        // Conversion flags
      utf16.data(),   // Source UTF-16 string pointer
      utf16Length,    // Length of source UTF-16 string, in chars
      &utf8[0],     // Pointer to destination buffer
      utf8Length,    // Size of destination buffer, in wchar_ts
      nullptr, nullptr
    );

    if (result == 0)
    {
      // Conversion error: capture error code and throw
      const DWORD error = ::GetLastError();
      throw std::overflow_error(
        "Cannot convert from UTF-16 to UTF-8 "\
        "(WideCharToMultiByte failed)." +
        error);
    }

    return utf8;
} // End of Utf16ToUtf8


std::wstring Util::Utf8ToUtf16(const std::string& utf8) {
     std::wstring utf16; // Result
     if (utf8.empty()) {
        return utf16;
     }

    if (utf8.length() > static_cast<size_t>((std::numeric_limits<int>::max)()))
    {
      throw std::overflow_error(
        "Input string too long: size_t-length doesn't fit into int.");
    }

    // Safely convert from size_t (STL string's length)
    // to int (for Win32 APIs)
    const int utf8Length = static_cast<int>(utf8.length());

    // Safely fails if an invalid UTF-8 character
    // is encountered in the input string
    //constexpr DWORD kFlags = MB_ERR_INVALID_CHARS;
    constexpr DWORD kFlags = 0;

    const int utf16Length = ::MultiByteToWideChar(
                              CP_UTF8,       // Source string is in UTF-8
                              kFlags,        // Conversion flags
                              utf8.data(),   // Source UTF-8 string pointer
                              utf8Length,    // Length of the source UTF-8 string, in chars
                              nullptr,       // Unused - no conversion done in this step
                              0              // Request size of destination buffer, in wchar_ts
                            );
    if (utf16Length == 0)
    {
      // Conversion error: capture error code and throw
      const DWORD error = ::GetLastError();
      throw std::overflow_error(
        "Cannot get result string length when converting " \
        "from UTF-8 to UTF-16 (MultiByteToWideChar failed)." +
        error);
    }

    utf16.resize(utf16Length);

    // Convert from UTF-8 to UTF-16
    int result = ::MultiByteToWideChar(
      CP_UTF8,       // Source string is in UTF-8
      kFlags,        // Conversion flags
      utf8.data(),   // Source UTF-8 string pointer
      utf8Length,    // Length of source UTF-8 string, in chars
      &utf16[0],     // Pointer to destination buffer
      utf16Length    // Size of destination buffer, in wchar_ts
    );

    if (result == 0)
    {
      // Conversion error: capture error code and throw
      const DWORD error = ::GetLastError();
      throw std::overflow_error(
        "Cannot convert from UTF-8 to UTF-16 "\
        "(MultiByteToWideChar failed)." +
        error);
    }

    return utf16;
} // End of Utf8ToUtf16

bool Util::FindProcessExist(const std::wstring processName)
{
    HANDLE hSnapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (hSnapshot == INVALID_HANDLE_VALUE) {
        return false;
    }

    PROCESSENTRY32 pe32{};
    pe32.dwSize = sizeof(PROCESSENTRY32);
    if (!Process32First(hSnapshot, &pe32)) {
        CloseHandle(hSnapshot);
        return false;
    }

    int iCount = 0;
    do {
        if (std::wstring(pe32.szExeFile) == processName) {
            iCount++;
            break;
        }
    } while (Process32Next(hSnapshot, &pe32));
    CloseHandle(hSnapshot);
    return iCount > 0;
}

void Util::Trim(std::wstring& str)
{
    str.erase(str.begin(), std::find_if(str.begin(), str.end(), [](wchar_t ch) {
        return !std::iswspace(ch);
    }));
    str.erase(std::find_if(str.rbegin(), str.rend(), [](wchar_t ch) {
        return !std::iswspace(ch);
    }).base(), str.end());
}

bool Util::CheckIfUrlsFileExists()
{
    // 获取当前可执行文件所在目录
    WCHAR exePath[MAX_PATH];
    GetModuleFileNameW(NULL, exePath, MAX_PATH);

    // 提取目录部分
    std::wstring exeDir(exePath);
    size_t lastSlash = exeDir.find_last_of(L"\\/");
    if (lastSlash != std::wstring::npos)
    {
        exeDir = exeDir.substr(0, lastSlash + 1);
    }

    // 构建完整文件路径
    std::wstring urlsFilePath = exeDir + L"urls.txt";

    // 检查文件是否存在
    DWORD fileAttributes = GetFileAttributesW(urlsFilePath.c_str());
    if (fileAttributes == INVALID_FILE_ATTRIBUTES)
    {
        // 文件不存在或无法访问
        return false;
    }

    // 检查是否是普通文件
    return !(fileAttributes & FILE_ATTRIBUTE_DIRECTORY);
}


bool Util::IsImageUrl(const std::wstring& url)
{
    std::wstring lowerUrl = url;
    std::transform(lowerUrl.begin(), lowerUrl.end(), lowerUrl.begin(), ::tolower);

    return (
        lowerUrl.ends_with(L".jpg") ||
        lowerUrl.ends_with(L".jpeg") ||
        lowerUrl.ends_with(L".png") ||
        lowerUrl.ends_with(L".gif") ||
        lowerUrl.ends_with(L".bmp") ||
        lowerUrl.ends_with(L".webp")
    );
}



std::wstring Util::GetFileNameFromUrl(const std::wstring& url)
{
    // 1. 提取路径部分（去掉协议、域名和查询参数）
    size_t path_start = url.find(L"://");
    if (path_start != std::wstring::npos) {
        path_start = url.find(L'/', path_start + 3);
    } else {
        path_start = url.find(L'/');
    }

    std::wstring path;
    if (path_start != std::wstring::npos) {
        size_t query_start = url.find(L'?', path_start);
        path = url.substr(path_start + 1,
                         (query_start != std::wstring::npos) ?
                         (query_start - path_start - 1) : std::wstring::npos);
    } else {
        path = url;
    }

    // 2. 解码百分号编码（如 %20 -> 空格）
    std::wstring decoded_path;
    for (size_t i = 0; i < path.size(); ++i) {
        if (path[i] == L'%' && i + 2 < path.size()) {
            int hex_val;
            std::wistringstream hex_stream(path.substr(i + 1, 2));
            if (hex_stream >> std::hex >> hex_val) {
                decoded_path += static_cast<wchar_t>(hex_val);
                i += 2;
                continue;
            }
        }
        decoded_path += path[i];
    }

    // 3. 提取最后一个斜杠后的内容作为文件名
    size_t last_slash = decoded_path.find_last_of(L"/\\");
    std::wstring filename = (last_slash != std::wstring::npos) ?
                           decoded_path.substr(last_slash + 1) : decoded_path;

    // 4. 如果文件名为空（以斜杠结尾的URL），使用默认名
    if (filename.empty()) {
        filename = L"download";
    }

    // 5. 移除非法文件名字符（Windows系统）
    static const std::wstring illegal_chars = L"<>:\"/\\|?*";
    filename.erase(std::remove_if(filename.begin(), filename.end(),
        [](wchar_t c) { return illegal_chars.find(c) != std::wstring::npos; }),
        filename.end());

    // 6. 如果处理后为空，再次使用默认名
    if (filename.empty()) {
        filename = L"download";
    }

    // 7. 截断过长的文件名（Windows最大255字符）
    const size_t max_length = 255;
    if (filename.length() > max_length) {
        size_t last_dot = filename.find_last_of(L'.');
        if (last_dot != std::wstring::npos && last_dot > max_length - 10) {
            // 保留扩展名
            std::wstring ext = filename.substr(last_dot);
            filename = filename.substr(0, max_length - ext.length()) + ext;
        } else {
            filename = filename.substr(0, max_length);
        }
    }

    return filename;
}

bool Util::IsImageContentType(const wchar_t* contentType) {
    static const std::vector<std::wstring> imageTypes = {
        L"image/jpeg", L"image/png", L"image/gif",
        L"image/webp", L"image/svg+xml", L"image/bmp"
    };
    std::wstring type(contentType);
    return std::any_of(imageTypes.begin(), imageTypes.end(),
        [&type](const std::wstring& imgType) {
            return type.find(imgType) != std::wstring::npos;
        });
}

// 解析JSON布尔返回值
std::wstring Util::ParseJsonBool(const wchar_t* json) {
    std::wstring str(json);
    if (str.find(L"true") != std::wstring::npos) return L"true";
    if (str.find(L"false") != std::wstring::npos) return L"false";
    return L"";
}


std::string Util::replace_fast(const std::string& str,
                        const std::string& from,
                        const std::string& to) {
    if (from.empty()) return str;

    std::vector<size_t> positions;
    size_t pos = 0;

    // 先收集所有匹配位置
    while ((pos = str.find(from, pos)) != std::string::npos) {
        positions.push_back(pos);
        pos += from.length();
    }

    if (positions.empty()) return str;

    // 高性能版本（预分配内存）预分配结果字符串内存
    std::string result;
    result.reserve(str.length() + positions.size() * (to.length() - from.length()));

    // 构建结果字符串
    size_t prev = 0;
    for (size_t p : positions) {
        result.append(str, prev, p - prev);
        result += to;
        prev = p + from.length();
    }
    result.append(str, prev, str.length() - prev);

    return result;
}

std::string Util::replace(const std::string& str,
                   const std::string& from,
                   const std::string& to) {
    std::string result = str;
    size_t pos = 0;
    while ((pos = result.find(from, pos)) != std::string::npos) {
        result.replace(pos, from.length(), to);
        pos += to.length(); // 避免无限循环替换
    }
    return result;
}


bool Util::matchUrlPattern(const std::string& pattern, const std::string& url) {
    // 将通配符模式转换为正则表达式
    // 将 * 替换为 .*
    // 需要转义其他特殊字符
    std::string regexPattern;
    for (char c : pattern) {
        switch (c) {
            case '*':
                regexPattern += ".*";
                break;
            case '?':
                regexPattern += '.';
                break;
            case '.':
            case '^':
            case '$':
            case '+':
            case '(':
            case ')':
            case '[':
            case ']':
            case '{':
            case '}':
            case '|':
            case '\\':
                regexPattern += '\\';
                regexPattern += c;
                break;
            default:
                regexPattern += c;
        }
    }

    std::regex re(regexPattern, std::regex_constants::icase);
    return std::regex_match(url, re);
}

bool Util::matchUrlPattern(const std::wstring& pattern, const std::wstring& url) {
    // 将通配符模式转换为正则表达式
    // 将 * 替换为 .*
    // 需要转义其他特殊字符
    std::wstring regexPattern;
    for (wchar_t c : pattern) {
        switch (c) {
            case L'*':
                regexPattern += L".*";
                break;
            case L'?':
                regexPattern += L'.';
                break;
            case L'.':
            case L'^':
            case L'$':
            case L'+':
            case L'(':
            case L')':
            case L'[':
            case L']':
            case L'{':
            case L'}':
            case L'|':
            case L'\\':
                regexPattern += L'\\';
                regexPattern += c;
                break;
            default:
                regexPattern += c;
        }
    }

    std::wregex re(regexPattern, std::regex_constants::icase);
    return std::regex_match(url, re);
}

bool Util::IsLocalUri(const std::wstring& url) {
    // 转换为小写方便比较
    std::wstring lowerUrl = url;
    std::transform(lowerUrl.begin(), lowerUrl.end(), lowerUrl.begin(), ::towlower);

    // 检查常见本地URI模式
    if (lowerUrl.find(L"file://") == 0 ||
        lowerUrl.find(L"http://localhost") == 0 ||
        lowerUrl.find(L"https://localhost") == 0 ||
        lowerUrl.find(L"http://127.0.0.1") == 0 ||
        lowerUrl.find(L"https://127.0.0.1") == 0 ||
        lowerUrl.find(L"http://[::1]") == 0 ||
        lowerUrl.find(L"https://[::1]") == 0) {
        return true;
    }

    // 可以添加更多本地URI模式...
    return false;
}

// 声明为静态函数，显式传入模块句柄
std::wstring Util::GetFullPathFor(HINSTANCE hInst, LPCWSTR relativePath) {
    WCHAR path[MAX_PATH];
    GetModuleFileNameW(hInst, path, MAX_PATH);  // 使用参数 hInst
    std::wstring pathName(path);

    std::size_t index = pathName.find_last_of(L"\\") + 1;
    pathName.replace(index, pathName.length(), relativePath);

    return pathName;
}

void Util::DebugPrintException(const std::exception& e) {
    try {
        std::string u8_What = e.what();

        int size_needed = MultiByteToWideChar(CP_UTF8, 0, u8_What.c_str(), (int)u8_What.size(), NULL, 0);
        std::wstring wide(size_needed, 0);
        MultiByteToWideChar(CP_UTF8, 0, u8_What.c_str(), (int)u8_What.size(), &wide[0], size_needed);
        
        // 添加异常类型和换行
        std::wstring message = L"Exception: " + wide + L"\n";
        OutputDebugStringW(message.c_str());
    } catch (...) {
        // 如果转换失败，回退到ANSI
        OutputDebugStringA("Failed to convert exception message\n");
    }
}

std::string  Util::removeDisableDevtoolJsCode(const std::string& input) {
    // 正则表达式模式，匹配包含 ondevtoolopen 和 ondevtoolclose 的 var xxx = {...} 代码
    std::regex pattern(R"(var\s+[a-zA-Z_$][\w$]*\s*=\s*\{[^}]*ondevtoolopen[^}]*ondevtoolclose[^}]*\})");
    
    // 使用空字符串替换匹配的部分
    std::string result = std::regex_replace(input, pattern, "");
    
    return result;
}

std::string Util::ReadStreamToString(IStream* stream) {
    STATSTG stat;
    stream->Stat(&stat, STATFLAG_NONAME);
    
    std::string content(stat.cbSize.QuadPart, '\0');
    ULONG read = 0;
    stream->Seek({0}, STREAM_SEEK_SET, nullptr);
    stream->Read(&content[0], static_cast<ULONG>(content.size()), &read);
    
    return content;
}

std::vector<uint8_t> Util::removeJPGHeader(const std::vector<uint8_t>& imageData, const std::string& headerMarker) {
    if (imageData.empty() || headerMarker.size() > imageData.size()) {
        return {};
    }

    if (memcmp(imageData.data(), headerMarker.data(), headerMarker.size()) != 0) {
        return imageData;
    }

    std::vector<uint8_t> result(imageData.size() - headerMarker.size());
    memcpy(result.data(), imageData.data() + headerMarker.size(), result.size());
    return result;
}
