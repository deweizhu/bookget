#include "HttpClient.h"


std::string HttpClient::get(const std::string& url,
                              const std::vector<std::pair<std::string, std::string>>& headers) {
    try {
        // Parse URL (same as in download method)
        size_t protocol_end = url.find("://");
        if (protocol_end == std::string::npos) {
            throw std::runtime_error("Invalid URL format");
        }

        std::string protocol = url.substr(0, protocol_end);
        bool is_https = (protocol == "https");

        size_t host_start = protocol_end + 3;
        size_t path_start = url.find('/', host_start);
        std::string host = url.substr(host_start, path_start - host_start);
        std::string path = path_start == std::string::npos ? "/" : url.substr(path_start);

        // Parse host and port
        size_t port_pos = host.find(':');
        std::string server = port_pos == std::string::npos ? host : host.substr(0, port_pos);
        std::string port = port_pos == std::string::npos ? 
                          (is_https ? "443" : "80") : 
                          host.substr(port_pos + 1);

        // Resolve hostname
        tcp::resolver::results_type endpoints = resolver_.resolve(server, port);
            
        if (is_https) {
            // HTTPS connection
            asio::connect(ssl_socket_.lowest_layer(), endpoints);
            ssl_socket_.handshake(ssl::stream_base::client);
                
            // Disable certificate verification
            SSL_set_verify(ssl_socket_.native_handle(), SSL_VERIFY_NONE, nullptr);
                
            // Send HTTP request
            std::string request = build_request(host, path, headers, "GET");
            asio::write(ssl_socket_, asio::buffer(request));

            // Handle response and return string
            return handle_string_response(ssl_socket_);
        } else {
            // HTTP connection
            asio::connect(socket_, endpoints);
                
            // Send HTTP request
            std::string request = build_request(host, path, headers, "GET");
            asio::write(socket_, asio::buffer(request));

            // Handle response and return string
            return handle_string_response(socket_);
        }
    } catch (std::exception& e) {
        std::cerr << "Exception in GET request: " << e.what() << std::endl;
        return "";
    }
}

std::string HttpClient::post(const std::string& url,
                               const std::string& body,
                               const std::vector<std::pair<std::string, std::string>>& headers) {
    try {
        // Parse URL (same as in download method)
        size_t protocol_end = url.find("://");
        if (protocol_end == std::string::npos) {
            throw std::runtime_error("Invalid URL format");
        }

        std::string protocol = url.substr(0, protocol_end);
        bool is_https = (protocol == "https");

        size_t host_start = protocol_end + 3;
        size_t path_start = url.find('/', host_start);
        std::string host = url.substr(host_start, path_start - host_start);
        std::string path = path_start == std::string::npos ? "/" : url.substr(path_start);

        // Parse host and port
        size_t port_pos = host.find(':');
        std::string server = port_pos == std::string::npos ? host : host.substr(0, port_pos);
        std::string port = port_pos == std::string::npos ? 
                          (is_https ? "443" : "80") : 
                          host.substr(port_pos + 1);

        // Resolve hostname
        tcp::resolver::results_type endpoints = resolver_.resolve(server, port);
            
        if (is_https) {
            // HTTPS connection
            asio::connect(ssl_socket_.lowest_layer(), endpoints);
            ssl_socket_.handshake(ssl::stream_base::client);
                
            // Disable certificate verification
            SSL_set_verify(ssl_socket_.native_handle(), SSL_VERIFY_NONE, nullptr);
                
            // Send HTTP request
            std::string request = build_post_request(host, path, headers, body);
            asio::write(ssl_socket_, asio::buffer(request));

            // Handle response and return string
            return handle_string_response(ssl_socket_);
        } else {
            // HTTP connection
            asio::connect(socket_, endpoints);
                
            // Send HTTP request
            std::string request = build_post_request(host, path, headers, body);
            asio::write(socket_, asio::buffer(request));

            // Handle response and return string
            return handle_string_response(socket_);
        }
    } catch (std::exception& e) {
        std::cerr << "Exception in POST request: " << e.what() << std::endl;
        return "";
    }
}

std::string HttpClient::build_request(const std::string& host, 
                            const std::string& path,
                            const std::vector<std::pair<std::string, std::string>>& headers,
                            const std::string& method,
                            const std::string& body) {
    std::string request = 
        "GET " + path + " HTTP/1.1\r\n"
        "Host: " + host + "\r\n";
        
    // 添加自定义头
    for (const auto& header : headers) {
        request += header.first + ": " + header.second + "\r\n";
    }
        
    request += "Connection: close\r\n\r\n";
    return request;
}

std::string HttpClient::build_post_request(const std::string& host,
                                             const std::string& path,
                                             const std::vector<std::pair<std::string, std::string>>& headers,
                                             const std::string& body) {
    std::string request = 
        "POST " + path + " HTTP/1.1\r\n"
        "Host: " + host + "\r\n"
        "Content-Length: " + std::to_string(body.length()) + "\r\n";
        
    // Add custom headers
    for (const auto& header : headers) {
        request += header.first + ": " + header.second + "\r\n";
    }
        
    request += "Connection: close\r\n\r\n";
    request += body;
    return request;
}

template <typename SocketType>
std::string HttpClient::handle_string_response(SocketType& socket) {
    asio::streambuf response;
    
    // 1. 读取HTTP头直到空行
    asio::read_until(socket, response, "\r\n\r\n");

    // 2. 解析状态行
    std::istream response_stream(&response);
    std::string http_version;
    unsigned int status_code;
    std::string status_message;
    
    response_stream >> http_version >> status_code;
    std::getline(response_stream, status_message);

    // 3. 验证HTTP响应格式
    if (!response_stream || http_version.substr(0, 5) != "HTTP/") {
        throw std::runtime_error("Invalid HTTP response");
    }

    // 4. 检查状态码
    if (status_code != 200) {
        throw std::runtime_error("HTTP request failed with status: " + 
                              std::to_string(status_code) + " " + status_message);
    }

    // 5. 解析Content-Length（如果存在）
    size_t content_length = 0;
    std::string header;
    while (std::getline(response_stream, header) && header != "\r") {
        if (header.find("Content-Length:") != std::string::npos) {
            content_length = std::stoul(header.substr(16));
        }
    }

    // 6. 预分配内存（如果知道长度）
    std::string result;
    if (content_length > 0) {
        result.reserve(content_length);
    }

    // 7. 读取头部已缓冲的body数据
    if (response.size() > 0) {
        result.append(
            asio::buffers_begin(response.data()),
            asio::buffers_end(response.data())
        );
        response.consume(response.size());
    }

    // 8. 高效读取剩余body数据
    asio::error_code ec;
    while (asio::read(socket, response, asio::transfer_at_least(1024), ec)) {
        result.append(
            asio::buffers_begin(response.data()),
            asio::buffers_end(response.data())
        );
        response.consume(response.size());
    }

    // 9. 检查错误（排除正常EOF）
    if (ec != asio::error::eof) {
        throw asio::system_error(ec);
    }

    return result;
}


bool HttpClient::download(const std::string& url, 
                const std::string& output_filename,
                const std::vector<std::pair<std::string, std::string>>& headers) {
    try {
        // 解析URL
        size_t protocol_end = url.find("://");
        if (protocol_end == std::string::npos) {
            std::cerr << "Invalid URL format" << std::endl;
            return false;
        }

        std::string protocol = url.substr(0, protocol_end);
        bool is_https = (protocol == "https");

        size_t host_start = protocol_end + 3;
        size_t path_start = url.find('/', host_start);
        std::string host = url.substr(host_start, path_start - host_start);
        std::string path = path_start == std::string::npos ? "/" : url.substr(path_start);

        // 解析主机名和端口
        size_t port_pos = host.find(':');
        std::string server = port_pos == std::string::npos ? host : host.substr(0, port_pos);
        std::string port = port_pos == std::string::npos ? 
                            (is_https ? "443" : "80") : 
                            host.substr(port_pos + 1);

        // 解析主机名
        tcp::resolver::results_type endpoints = resolver_.resolve(server, port);
            
        if (is_https) {
            // HTTPS 连接
            asio::connect(ssl_socket_.lowest_layer(), endpoints);
            ssl_socket_.handshake(ssl::stream_base::client);
                
            // 设置不验证证书
            SSL_set_verify(ssl_socket_.native_handle(), SSL_VERIFY_NONE, nullptr);
                
            // 发送HTTP请求
            std::string request = build_request(host, path, headers);
            asio::write(ssl_socket_, asio::buffer(request));

            // 处理响应
            return handle_response(ssl_socket_, output_filename);
        } else {
            // HTTP 连接
            asio::connect(socket_, endpoints);
                
            // 发送HTTP请求
            std::string request = build_request(host, path, headers);
            asio::write(socket_, asio::buffer(request));

            // 处理响应
            return handle_response(socket_, output_filename);
        }
    } catch (std::exception& e) {
        std::cerr << "Exception: " << e.what() << std::endl;
        return false;
    }
}


template <typename SocketType>
bool HttpClient::handle_response(SocketType& socket, const std::string& output_filename) {
    asio::streambuf response;
    std::ofstream output_file(output_filename, std::ios::binary);
    if (!output_file) {
        std::cerr << "Failed to open output file" << std::endl;
        return false;
    }

    // 读取HTTP头
    asio::read_until(socket, response, "\r\n\r\n");

    // 检查响应状态
    std::istream response_stream(&response);
    std::string http_version;
    unsigned int status_code;
    std::string status_message;
    response_stream >> http_version >> status_code;
    std::getline(response_stream, status_message);

    if (!response_stream || http_version.substr(0, 5) != "HTTP/") {
        std::cerr << "Invalid HTTP response" << std::endl;
        return false;
    }

    if (status_code != 200) {
        std::cerr << "HTTP request failed with status code: " << status_code << std::endl;
        return false;
    }

    // 跳过剩余的HTTP头
    std::string header;
    while (std::getline(response_stream, header) && header != "\r") {}

    // 保存响应体
    if (response.size() > 0) {
        output_file << &response;
    }

    // 读取剩余的数据
    asio::error_code error;
    while (asio::read(socket, response, asio::transfer_at_least(1), error)) {
        output_file << &response;
    }

    if (error != asio::error::eof) {
        throw asio::system_error(error);
    }

    return true;
}

void HttpClient::reset() {
    // 定期完全释放内存
    if (m_headers.capacity() > MAX_HEADERS * 2) {
        m_headers.shrink_to_fit();
    } else {
        m_headers.clear();
    }
}
