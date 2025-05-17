#pragma once
#include <mutex>
#include <windows.h>

// 共享内存结构
#include <cstdint>
#include <string>
#include <tchar.h>

#pragma pack(push, 1)  // 确保无填充字节
struct SharedMemoryData {
	uint32_t URLReady;
	uint32_t HTMLReady;
	uint32_t CookiesReady;
	uint32_t ImageReady;
	uint32_t PID;
	wchar_t URL[1024];                // 固定大小缓冲区
	wchar_t ImagePath[1024];          // MAX_PATH for image path
	wchar_t Cookies[4096];            // 4KB for cookies
	wchar_t HTML[1024 * 1024 * 8];    // 8MB for HTML
};
struct SharedMemoryDataMini {
	uint32_t URLReady;
	uint32_t HTMLReady;
	uint32_t CookiesReady;
	uint32_t ImageReady;
	uint32_t PID;
	wchar_t URL[1024];                // 固定大小缓冲区
	wchar_t ImagePath[1024];          // MAX_PATH for image path
};
#pragma pack(pop)  // 恢复默认对齐


class SharedMemory
{

public:
    // 获取单例实例
    static SharedMemory& GetInstance() {
        static SharedMemory instance;
        return instance;
    }


     bool Init();
     void Cleanup();
     void WriteHtml(const std::wstring& html);
     void WriteCookies(const std::wstring& cookies);
     void WriteImagePath(const std::wstring& imagePath);
     SharedMemoryDataMini Read();
     SharedMemoryData* Get();
     SharedMemoryData* GetMutex();
     void ReleaseMutex();


private:

     HANDLE m_hSharedMemory;        
     LPVOID m_pSharedMemory;          
     HANDLE m_hSharedMemoryMutex;      

    // 优化：用 constexpr 替代 static const
     const wchar_t* m_sharedMemoryName = L"Local\\WebView2SharedMemory";
     const wchar_t* m_sharedMemoryMutexName = L"Local\\WebView2SharedMemoryMutex";
     DWORD m_sharedMemorySize = sizeof(SharedMemoryData);
};

