# 获取日期格式化的版本号
$ver = $( Get-Date -Format "yy.MMdd" )

function setVersion
{
    $filePath = "config/init.go"
    $tempFilePath = "$filePath.tmp"

    # 读取文件内容
    $fileContent = Get-Content -Path $filePath -Raw

    # 使用正则表达式替换版本号
    $pattern = 'const Version = "[^"]*"'
    $replacement = "const Version = `"$ver`""
    $updatedContent = [regex]::Replace($fileContent, $pattern, $replacement)

    # 将更新后的内容写入临时文件
    $updatedContent | Set-Content -Path $tempFilePath

    # 覆盖原文件
    Move-Item -Path $tempFilePath -Destination $filePath -Force

    Write-Output "版本号已替换为: $ver"
}

setVersion $ver

# 配置 GOPROXY 环境变量
$env:GOPROXY = "https://goproxy.cn,direct"
go mod tidy
go mod download

function BuildAndPackageWin
{
    $ENV:CGO_ENABLED = 0
    $ENV:GOOS = "windows"
    $ENV:GOARCH = "amd64"

    $target = "target/bookget-$ver.$ENV:GOOS-$ENV:GOARCH"
    mkdir -Force $target
    go build -o "$target/bookget.exe" .
}

function BuildAndPackageLinux
{
    $ENV:CGO_ENABLED = 0
    $ENV:GOOS = "linux"
    $ENV:GOARCH = "amd64"

    $target = "target/bookget-$ver.$ENV:GOOS-$ENV:GOARCH"
    mkdir -Force $target
    go build -o "$target/bookget" .
}

function BuildAndPackageDarwin
{
    $ENV:CGO_ENABLED = 0
    $ENV:GOOS = "darwin"
    $ENV:GOARCH = "arm64"

    $target = "target/bookget-$ver.$ENV:GOOS-$ENV:GOARCH"
    mkdir -Force $target
    go build -o "$target/bookget" .
}

# 调用函数
BuildAndPackageWin
BuildAndPackageLinux
BuildAndPackageDarwin

