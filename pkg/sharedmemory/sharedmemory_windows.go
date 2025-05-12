//go:build windows

package sharedmemory

import (
	"fmt"
	"golang.org/x/sys/windows"
	"time"
	"unsafe"
)

const (
	MEM_NAME   = "Local\\WebView2SharedMemory"
	MUTEX_NAME = "Local\\WebView2SharedMemoryMutex"
)

// 确保与C++结构体完全一致的内存布局
type SharedMemoryData struct {
	URLReady       uint32 // Windows BOOL实际上是32位整数
	HTMLReady      uint32
	CookiesReady   uint32
	ImagePathReady uint32
	PID            uint32                  // 进程ID
	URL            [1024]uint16            // wchar_t[1024]
	ImagePath      [1024]uint16            // wchar_t[1024]
	Cookies        [4096]uint16            // 4KB
	HTML           [1024 * 1024 * 8]uint16 // 8MB
}

// 计算共享内存大小（转换为uint32）
func getSharedMemorySize() uint32 {
	// 计算结构体大小并确保不超过uint32最大值
	size := unsafe.Sizeof(SharedMemoryData{})
	if size > 0xFFFFFFFF {
		panic("Shared memory size exceeds maximum limit")
	}
	return uint32(size)
}

func WriteURLToSharedMemory(url string) error {
	// 1. 创建或打开互斥锁
	mutexNamePtr, err := windows.UTF16PtrFromString(MUTEX_NAME)
	if err != nil {
		return fmt.Errorf("UTF16PtrFromString failed: %v", err)
	}

	hMutex, err := windows.CreateMutex(nil, false, mutexNamePtr)
	defer windows.CloseHandle(hMutex)

	// 2. 等待获取互斥锁
	_, err = windows.WaitForSingleObject(hMutex, windows.INFINITE)
	if err != nil {
		return fmt.Errorf("WaitForSingleObject failed: %v", err)
	}
	defer windows.ReleaseMutex(hMutex)

	// 3. 创建或打开共享内存
	memSize := getSharedMemorySize()
	namePtr, err := windows.UTF16PtrFromString(MEM_NAME)
	if err != nil {
		return fmt.Errorf("UTF16PtrFromString failed: %v", err)
	}

	hMemory, err := windows.CreateFileMapping(
		windows.InvalidHandle,
		nil,
		windows.PAGE_READWRITE,
		0,       // 高32位大小
		memSize, // 低32位大小
		namePtr)
	if err != nil {
		// 如果已存在则打开
		hMemory, err = openFileMapping(
			windows.FILE_MAP_WRITE,
			false,
			namePtr)
		if err != nil {
			return fmt.Errorf("Create/OpenFileMapping failed: %v", err)
		}
	}
	defer windows.CloseHandle(hMemory)

	// 4. 映射到当前进程地址空间
	pMemory, err := windows.MapViewOfFile(
		hMemory,
		windows.FILE_MAP_WRITE,
		0,
		0,
		0)
	if err != nil {
		return fmt.Errorf("MapViewOfFile failed: %v", err)
	}
	defer windows.UnmapViewOfFile(pMemory)

	// 5. 写入数据
	sharedData := (*SharedMemoryData)(unsafe.Pointer(pMemory))
	sharedData.URLReady = 1
	sharedData.HTMLReady = 0
	sharedData.CookiesReady = 0
	sharedData.ImagePathReady = 0
	sharedData.PID = uint32(windows.GetCurrentProcessId())

	urlUTF16, err := windows.UTF16FromString(url)
	if err != nil {
		return fmt.Errorf("UTF16FromString failed: %v", err)
	}

	copySize := len(urlUTF16)
	if copySize > len(sharedData.URL)-1 {
		copySize = len(sharedData.URL) - 1
	}
	copy(sharedData.URL[:], urlUTF16[:copySize])
	sharedData.URL[copySize] = 0 // null终止

	return nil
}

func WriteURLImagePathToSharedMemory(url, imagePath string) error {
	// 1. 创建或打开互斥锁
	mutexNamePtr, err := windows.UTF16PtrFromString(MUTEX_NAME)
	if err != nil {
		return fmt.Errorf("UTF16PtrFromString failed: %v", err)
	}

	hMutex, err := windows.CreateMutex(nil, false, mutexNamePtr)
	defer windows.CloseHandle(hMutex)

	// 2. 等待获取互斥锁
	_, err = windows.WaitForSingleObject(hMutex, windows.INFINITE)
	if err != nil {
		return fmt.Errorf("WaitForSingleObject failed: %v", err)
	}
	defer windows.ReleaseMutex(hMutex)

	// 3. 创建或打开共享内存
	memSize := getSharedMemorySize()
	namePtr, err := windows.UTF16PtrFromString(MEM_NAME)
	if err != nil {
		return fmt.Errorf("UTF16PtrFromString failed: %v", err)
	}

	hMemory, err := windows.CreateFileMapping(
		windows.InvalidHandle,
		nil,
		windows.PAGE_READWRITE,
		0,       // 高32位大小
		memSize, // 低32位大小
		namePtr)
	if err != nil {
		// 如果已存在则打开
		hMemory, err = openFileMapping(
			windows.FILE_MAP_WRITE,
			false,
			namePtr)
		if err != nil {
			return fmt.Errorf("Create/OpenFileMapping failed: %v", err)
		}
	}
	defer windows.CloseHandle(hMemory)

	// 4. 映射到当前进程地址空间
	pMemory, err := windows.MapViewOfFile(
		hMemory,
		windows.FILE_MAP_WRITE,
		0,
		0,
		0)
	if err != nil {
		return fmt.Errorf("MapViewOfFile failed: %v", err)
	}
	defer windows.UnmapViewOfFile(pMemory)

	// 5. 写入数据
	sharedData := (*SharedMemoryData)(unsafe.Pointer(pMemory))
	sharedData.URLReady = 1
	sharedData.HTMLReady = 0
	sharedData.CookiesReady = 0
	sharedData.ImagePathReady = 1
	sharedData.PID = uint32(windows.GetCurrentProcessId())

	//URL
	urlUTF16, err := windows.UTF16FromString(url)
	if err != nil {
		return fmt.Errorf("UTF16FromString failed: %v", err)
	}

	copySize := len(urlUTF16)
	if copySize > len(sharedData.URL)-1 {
		copySize = len(sharedData.URL) - 1
	}
	copy(sharedData.URL[:], urlUTF16[:copySize])
	sharedData.URL[copySize] = 0 // null终止

	//imagePath
	imagePathUTF16, err := windows.UTF16FromString(imagePath)
	if err != nil {
		return fmt.Errorf("UTF16FromString failed: %v", err)
	}

	copySize = len(imagePathUTF16)
	if copySize > len(sharedData.ImagePath)-1 {
		copySize = len(sharedData.ImagePath) - 1
	}
	copy(sharedData.ImagePath[:], imagePathUTF16[:copySize])
	sharedData.ImagePath[copySize] = 0 // null终止

	return nil
}

func ReadHTMLFromSharedMemory() (string, error) {
	// 创建或打开互斥锁
	mutexNamePtr, err := windows.UTF16PtrFromString(MUTEX_NAME)
	if err != nil {
		return "", fmt.Errorf("UTF16PtrFromString failed: %v", err)
	}
	hMutex, err := windows.CreateMutex(nil, false, mutexNamePtr)
	defer windows.CloseHandle(hMutex)

	// 等待获取互斥锁所有权
	waitResult, err := windows.WaitForSingleObject(hMutex, windows.INFINITE)
	if err != nil {
		return "", fmt.Errorf("WaitForSingleObject failed: %v", err)
	}
	if waitResult != windows.WAIT_OBJECT_0 {
		return "", fmt.Errorf("unexpected wait result: %d", waitResult)
	}
	defer windows.ReleaseMutex(hMutex)

	const maxWaitTime = 30 * time.Second
	const checkInterval = 100 * time.Millisecond

	namePtr, err := windows.UTF16PtrFromString(MEM_NAME)
	if err != nil {
		return "", err
	}

	hMemory, err := openFileMapping(
		windows.FILE_MAP_READ,
		false,
		namePtr)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(hMemory)

	pMemory, err := windows.MapViewOfFile(
		hMemory,
		windows.FILE_MAP_READ,
		0,
		0,
		0)
	if err != nil {
		return "", err
	}
	defer windows.UnmapViewOfFile(pMemory)

	sharedData := (*SharedMemoryData)(unsafe.Pointer(pMemory))
	startTime := time.Now()

	for sharedData.HTMLReady == 0 {
		if time.Since(startTime) > maxWaitTime {
			return "", fmt.Errorf("timeout waiting for HTML")
		}
		// 在等待期间暂时释放互斥锁
		windows.ReleaseMutex(hMutex)
		time.Sleep(checkInterval)
		// 重新获取互斥锁
		waitResult, err := windows.WaitForSingleObject(hMutex, windows.INFINITE)
		if err != nil {
			return "", fmt.Errorf("WaitForSingleObject failed: %v", err)
		}
		if waitResult != windows.WAIT_OBJECT_0 {
			return "", fmt.Errorf("unexpected wait result: %d", waitResult)
		}
	}

	html := windows.UTF16ToString(sharedData.HTML[:])
	return html, nil
}

func ReadCookiesFromSharedMemory() (string, error) {
	// 创建或打开互斥锁
	mutexNamePtr, err := windows.UTF16PtrFromString(MUTEX_NAME)
	if err != nil {
		return "", fmt.Errorf("UTF16PtrFromString failed: %v", err)
	}
	hMutex, err := windows.CreateMutex(nil, false, mutexNamePtr)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(hMutex)

	// 等待获取互斥锁所有权
	waitResult, err := windows.WaitForSingleObject(hMutex, windows.INFINITE)
	if err != nil {
		return "", fmt.Errorf("WaitForSingleObject failed: %v", err)
	}
	if waitResult != windows.WAIT_OBJECT_0 {
		return "", fmt.Errorf("unexpected wait result: %d", waitResult)
	}
	defer windows.ReleaseMutex(hMutex)

	const maxWaitTime = 30 * time.Second
	const checkInterval = 100 * time.Millisecond

	namePtr, err := windows.UTF16PtrFromString(MEM_NAME)
	if err != nil {
		return "", err
	}

	hMemory, err := openFileMapping(
		windows.FILE_MAP_READ,
		false,
		namePtr)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(hMemory)

	pMemory, err := windows.MapViewOfFile(
		hMemory,
		windows.FILE_MAP_READ,
		0,
		0,
		0)
	if err != nil {
		return "", err
	}
	defer windows.UnmapViewOfFile(pMemory)

	sharedData := (*SharedMemoryData)(unsafe.Pointer(pMemory))
	startTime := time.Now()

	for sharedData.CookiesReady == 0 {
		if time.Since(startTime) > maxWaitTime {
			return "", fmt.Errorf("timeout waiting for Cookies")
		}
		// 在等待期间暂时释放互斥锁
		windows.ReleaseMutex(hMutex)
		time.Sleep(checkInterval)
		// 重新获取互斥锁
		waitResult, err := windows.WaitForSingleObject(hMutex, windows.INFINITE)
		if err != nil {
			return "", fmt.Errorf("WaitForSingleObject failed: %v", err)
		}
		if waitResult != windows.WAIT_OBJECT_0 {
			return "", fmt.Errorf("unexpected wait result: %d", waitResult)
		}
	}

	cookies := windows.UTF16ToString(sharedData.Cookies[:])
	return cookies, nil
}

func ReadImageReadyFromSharedMemory() (bool, error) {
	// 创建或打开互斥锁
	mutexNamePtr, err := windows.UTF16PtrFromString(MUTEX_NAME)
	if err != nil {
		return false, fmt.Errorf("UTF16PtrFromString failed: %v", err)
	}
	hMutex, err := windows.CreateMutex(nil, false, mutexNamePtr)
	defer windows.CloseHandle(hMutex)

	// 等待获取互斥锁所有权
	waitResult, err := windows.WaitForSingleObject(hMutex, windows.INFINITE)
	if err != nil {
		return false, fmt.Errorf("WaitForSingleObject failed: %v", err)
	}
	if waitResult != windows.WAIT_OBJECT_0 {
		return false, fmt.Errorf("unexpected wait result: %d", waitResult)
	}
	defer windows.ReleaseMutex(hMutex)

	const maxWaitTime = 30 * time.Second
	const checkInterval = 100 * time.Millisecond

	namePtr, err := windows.UTF16PtrFromString(MEM_NAME)
	if err != nil {
		return false, err
	}

	hMemory, err := openFileMapping(
		windows.FILE_MAP_READ,
		false,
		namePtr)
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(hMemory)

	pMemory, err := windows.MapViewOfFile(
		hMemory,
		windows.FILE_MAP_READ,
		0,
		0,
		0)
	if err != nil {
		return false, err
	}
	defer windows.UnmapViewOfFile(pMemory)

	sharedData := (*SharedMemoryData)(unsafe.Pointer(pMemory))
	startTime := time.Now()

	for sharedData.ImagePathReady == 1 {
		if time.Since(startTime) > maxWaitTime {
			return false, fmt.Errorf("timeout waiting for ImagePath")
		}
		// 在等待期间暂时释放互斥锁
		windows.ReleaseMutex(hMutex)
		time.Sleep(checkInterval)
		// 重新获取互斥锁
		waitResult, err := windows.WaitForSingleObject(hMutex, windows.INFINITE)
		if err != nil {
			return false, fmt.Errorf("WaitForSingleObject failed: %v", err)
		}
		if waitResult != windows.WAIT_OBJECT_0 {
			return false, fmt.Errorf("unexpected wait result: %d", waitResult)
		}
	}

	return sharedData.ImagePathReady == 0, nil
}
