#!/bin/sh

ver=$(date "+%y.%m%d")
#ver="24.0116"
commit="${ver}"
sed -i '/const version = */c const version = "'"$commit"'"' config/init.go

buildWindows() {
    ver=$1
    targetDir="target/bookget-${ver}.windows-amd64/"
    mkdir -p $targetDir
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o "${targetDir}/bookget.exe" .
    cp config.ini "${targetDir}/config.ini"
    cp -R target/bookget-gui/* $targetDir
    cp target/dezoomify-rs/x86_64-windows/dezoomify-rs.exe "${targetDir}/dezoomify-rs.exe"
    cd target/ || return
    tar cjf bookget-${ver}.windows-amd64.tar.bz2 "bookget-${ver}.windows-amd64"
    cd ../
    rm -fr target/bookget-${ver}.windows-amd64/
}

buildLinux() {
    ver=$1
    targetDir="target/bookget-${ver}.linux-amd64/"
    mkdir -p $targetDir
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "${targetDir}/bookget" .
    cp config.ini "${targetDir}/config.ini"
    cp target/dezoomify-rs/x86_64-linux/dezoomify-rs "${targetDir}/dezoomify-rs"
    cd target/ || return
    tar cjf bookget-${ver}.linux-amd64.tar.bz2 "bookget-${ver}.linux-amd64"
    cd ../
    rm -fr target/bookget-${ver}.linux-amd64/
}

buildDarwin() {
    targetDir="target/bookget-${ver}.macOS/"
    mkdir -p $targetDir
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o "${targetDir}/bookget" .
    cp config.ini "${targetDir}/config.ini"
    cp target/dezoomify-rs/x86_64-apple/dezoomify-rs "${targetDir}/dezoomify-rs"
    cd target/ || return
    tar cjf bookget-${ver}.macOS.tar.bz2 "bookget-${ver}.macOS"
    cd ../
    rm -fr target/bookget-${ver}.macOS/
}

buildDarwinArm64() {
    targetDir="target/bookget-${ver}.macOS-arm64/"
    mkdir -p $targetDir
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o "${targetDir}/bookget" .

    cp config.ini "${targetDir}/config.ini"
    cp target/dezoomify-rs/aarch64-apple/dezoomify-rs "${targetDir}/dezoomify-rs"
    cd target/ || return
    tar cjf bookget-${ver}.macOS-arm64.tar.bz2 "bookget-${ver}.macOS-arm64"
    cd ../
    rm -fr target/bookget-${ver}.macOS-arm64/
}

buildWindows $ver
buildLinux $ver
buildDarwin $ver
buildDarwinArm64 $ver
