#pragma once
#include "framework.h"
#include <iostream>
#include <fstream>
#include <string>
#include <vector>

#pragma comment(lib, "libssl.lib")
#pragma comment(lib, "libcrypto.lib")
#pragma comment(lib, "crypt32.lib")
#pragma comment(lib, "ws2_32.lib")


#include <asio.hpp>
#include <asio/ssl.hpp>

using asio::ip::tcp;
namespace ssl = asio::ssl;

const int MAX_HEADERS = 4096;

class HTTPDownloader {
public:
    HTTPDownloader(asio::io_context& io_context, ssl::context& ssl_ctx)
        : resolver_(io_context), 
          socket_(io_context),
          ssl_socket_(io_context, ssl_ctx) {};

    bool download(const std::string& url, 
                 const std::string& output_filename,
                 const std::vector<std::pair<std::string, std::string>>& headers = {});
    void reset();

private:
    std::string build_request(const std::string& host, 
                             const std::string& path,
                             const std::vector<std::pair<std::string, std::string>>& headers);

    template <typename SocketType>
    bool handle_response(SocketType& socket, const std::string& output_filename);

    tcp::resolver resolver_;
    tcp::socket socket_;
    ssl::stream<tcp::socket> ssl_socket_;

    std::vector<std::pair<std::string, std::string>> m_headers = {};
};


