#include "BrowserWindow.h"

std::wstring BrowserWindow::GetAppDataDirectory()
{
    TCHAR path[MAX_PATH];
    std::wstring dataDirectory;
    HRESULT hr = SHGetFolderPath(nullptr, CSIDL_APPDATA, NULL, 0, path);
    if (SUCCEEDED(hr))
    {
        dataDirectory = std::wstring(path);
        dataDirectory.append(L"\\Bookget\\");
    }
    else
    {
        dataDirectory = std::wstring(L".\\");
    }

    //dataDirectory.append(s_title);
    return dataDirectory;
}

std::wstring BrowserWindow::GetFullPathFor(LPCWSTR relativePath)
{
    WCHAR path[MAX_PATH];
    GetModuleFileNameW(m_hInst, path, MAX_PATH);
    std::wstring pathName(path);

    std::size_t index = pathName.find_last_of(L"\\") + 1;
    pathName.replace(index, pathName.length(), relativePath);

    return pathName;
}

std::wstring BrowserWindow::GetFilePathAsURI(std::wstring fullPath)
{
    std::wstring fileURI;
    ComPtr<IUri> uri;
    DWORD uriFlags = Uri_CREATE_ALLOW_IMPLICIT_FILE_SCHEME;
    HRESULT hr = CreateUri(fullPath.c_str(), uriFlags, 0, &uri);

    if (SUCCEEDED(hr))
    {
        wil::unique_bstr absoluteUri;
        uri->GetAbsoluteUri(&absoluteUri);
        fileURI = std::wstring(absoluteUri.get());
    }

    return fileURI;
}

std::wstring BrowserWindow::GetUserDataDirectory() {
    std::wstring userDataDirectory = GetAppDataDirectory();
    userDataDirectory.append(L"\\User Data");
    return userDataDirectory;
}
