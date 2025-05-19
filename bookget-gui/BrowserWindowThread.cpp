#include "BrowserWindow.h"
#include "SharedMemory.h"

void BrowserWindow::StartBackgroundThread() {
    std::lock_guard<std::mutex> lock(m_threadMutex);
    StopBackgroundThread();  // 确保之前的线程已停止

    m_sharedMemoryThread = std::thread([this]() {
        while (!m_stopThread.load(std::memory_order_relaxed)) {
            // 读取精简后的共享内存数据
            auto data = SharedMemory::GetInstance().Read();
            
             // 检查是否有新图片路径需要处理
            if (data.URLReady && data.ImageReady && data.PID != GetCurrentProcessId()) {
                auto* sharedData = new SharedMemoryDataMini();
                sharedData->ImageReady = data.ImageReady;
                sharedData->URLReady = data.URLReady;
                sharedData->PID = data.PID;
                wcsncpy_s(sharedData->ImagePath, _countof(sharedData->ImagePath), data.ImagePath, _TRUNCATE);
                wcsncpy_s(sharedData->URL, _countof(sharedData->URL), data.URL, _TRUNCATE);
                
                PostMessage(m_hWnd, WM_APP_UPDATE_UI, 0, reinterpret_cast<LPARAM>(sharedData));
            }
            // 检查是否有新URL需要处理
            else if (data.URLReady && data.PID != GetCurrentProcessId()) {
                // 仅传递必要数据到UI线程
                auto* sharedData = new SharedMemoryDataMini();
                sharedData->URLReady = data.URLReady;
                sharedData->PID = data.PID;
                wcsncpy_s(sharedData->URL, _countof(sharedData->URL), data.URL, _TRUNCATE);
                
                PostMessage(m_hWnd, WM_APP_UPDATE_UI, 0, reinterpret_cast<LPARAM>(sharedData));
            }
                   
            // 降低CPU使用率
            std::this_thread::sleep_for(std::chrono::milliseconds(200));
        }
    });

    m_downloader.Start(m_hWnd);
}

void BrowserWindow::StopBackgroundThread() {
    m_stopThread = true; // 先设置停止标志
    // 确保线程已完全停止
    if (m_sharedMemoryThread.joinable()) {
        m_sharedMemoryThread.join();
    }

    m_downloader.Stop();

    m_stopThread = false;  // 重置标志
}



void BrowserWindow::HandleSharedMemoryUpdate(LPARAM lParam) {
    // 获取传递过来的数据
    auto* data = reinterpret_cast<SharedMemoryData*>(lParam);

    auto* sharedData = SharedMemory::GetInstance().GetMutex();
    if (sharedData == nullptr) {
        delete data;
        return;
     }
    
    // 处理图片下载模式
    if (sharedData->URLReady && sharedData->ImageReady && m_tabs.find(m_activeTabId) != m_tabs.end() && sharedData->PID != GetCurrentProcessId()) {
        // 重置标志位
        sharedData->URLReady = false;
        m_downloader.Reset(sharedData->URL, 2);
        // 配置下载处理器
        m_tabs.at(m_activeTabId)->SetupWebViewListeners();
        // 导航到URL
        m_tabs.at(m_activeTabId)->m_contentWebView->Navigate(sharedData->URL);
    } 
    // 处理普通URL导航
    else if (sharedData->URLReady && m_tabs.find(m_activeTabId) != m_tabs.end()) {
        // 重置标志位
        sharedData->URLReady = false;
            
        // 导航到URL
        m_tabs.at(m_activeTabId)->m_contentWebView->Navigate(sharedData->URL);
    }
        
    
    // 清理内存
    delete data;
    SharedMemory::GetInstance().ReleaseMutex();

}

