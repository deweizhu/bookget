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

/*

asio::io_context io_context;
ssl::context ssl_ctx(ssl::context::tls_client);
ssl_ctx.set_verify_mode(SSL_VERIFY_NONE);
HttpClient httpClient(io_context, ssl_ctx);

// Simple GET request
std::string response = httpClient.get("https://example.com/api/data");

// GET with custom headers
std::vector<std::pair<std::string, std::string>> headers = {
    {"Authorization", "Bearer token123"},
    {"Accept", "application/json"}
};
std::string response = httpClient.get("https://api.example.com/data", headers);

// POST request with JSON body
std::string json_body = "{\"name\":\"John\", \"age\":30}";
headers.push_back({"Content-Type", "application/json"});
std::string response = httpClient.post("https://api.example.com/users", json_body, headers);
*/


using asio::ip::tcp;
namespace ssl = asio::ssl;

const int MAX_HEADERS = 4096;

class HttpClient {
public:
    HttpClient(asio::io_context& io_context, ssl::context& ssl_ctx)
        : resolver_(io_context), 
          socket_(io_context),
          ssl_socket_(io_context, ssl_ctx) {};

    std::string get(const std::string& url,
                   const std::vector<std::pair<std::string, std::string>>& headers = {});
    
    std::string post(const std::string& url,
                    const std::string& body,
                    const std::vector<std::pair<std::string, std::string>>& headers = {});

    bool download(const std::string& url, 
                 const std::string& output_filename,
                 const std::vector<std::pair<std::string, std::string>>& headers = {});
    void reset();

private:

     std::string build_request(const std::string& host, 
                             const std::string& path,
                             const std::vector<std::pair<std::string, std::string>>& headers,
                             const std::string& method = "GET",
                             const std::string& body = "");
    
     std::string build_post_request(const std::string& host,
                                  const std::string& path,
                                  const std::vector<std::pair<std::string, std::string>>& headers,
                                  const std::string& body);

    template <typename SocketType>
    bool handle_response(SocketType& socket, const std::string& output_filename);

    template <typename SocketType>
    std::string handle_string_response(SocketType& socket);

    tcp::resolver resolver_;
    tcp::socket socket_;
    ssl::stream<tcp::socket> ssl_socket_;

    std::vector<std::pair<std::string, std::string>> m_headers = {};
};


