package util

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Dasongzi1366/AutoGo/https"
)

// HttpClient 网络请求客户端
type HttpClient struct{}

// NewHttpClient 创建一个新的HTTP客户端实例
func NewHttpClient() *HttpClient {
	return &HttpClient{}
}

// Get 发送GET请求
// url: 请求的URL
// timeout: 请求的超时时间（毫秒），如果为0则不设置超时
// 返回值: 状态码和响应数据
func (h *HttpClient) Get(url string, timeout int) (int, []byte) {
	code, data := https.Get(url, timeout)
	return code, data
}

// GetWithRetry 带重试机制的GET请求
// url: 请求的URL
// timeout: 请求的超时时间（毫秒）
// maxRetries: 最大重试次数
// retryDelay: 重试间隔时间（毫秒）
// 返回值: 状态码、响应数据和是否成功
func (h *HttpClient) GetWithRetry(url string, timeout int, maxRetries int, retryDelay int) (int, []byte, bool) {
	for i := 0; i <= maxRetries; i++ {
		code, data := https.Get(url, timeout)

		// 成功的HTTP状态码范围：200-299
		if code >= 200 && code < 300 {
			return code, data, true
		}

		// 如果不是最后一次重试，则等待后重试
		if i < maxRetries {
			fmt.Printf("GET请求失败，状态码: %d，%d毫秒后重试 (%d/%d)\n", code, retryDelay, i+1, maxRetries)
			time.Sleep(time.Duration(retryDelay) * time.Millisecond)
		}
	}

	return 0, nil, false
}

// PostMultipart 发送带有文件的POST请求
// url: 请求的URL
// fileName: 文件名
// fileData: 文件数据
// 返回值: 状态码和响应数据
func (h *HttpClient) PostMultipart(url string, fileName string, fileData []byte) (int, []byte) {
	code, data := https.PostMultipart(url, fileName, fileData, 0)
	return code, data
}

// PostJSON 发送JSON格式的POST请求
// url: 请求的URL
// jsonData: 要发送的数据（将被序列化为JSON）
// headers: 自定义请求头
// timeout: 请求的超时时间（毫秒）
// 返回值: 状态码和响应数据
func (h *HttpClient) PostJSON(url string, jsonData interface{}, headers map[string]string, timeout int) (int, []byte, error) {
	// 将数据序列化为JSON
	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		return 0, nil, fmt.Errorf("JSON序列化失败: %v", err)
	}

	// 创建一个临时JSON文件进行上传（使用PostMultipart实现JSON POST）
	// 注意：这里利用了现有的PostMultipart方法，实际应用中可能需要真正的JSON POST实现
	code, data := https.PostMultipart(url, "data.json", jsonBytes, 0)
	return code, data, nil
}

// PostJSONWithRetry 带重试机制的JSON POST请求
func (h *HttpClient) PostJSONWithRetry(url string, jsonData interface{}, headers map[string]string, timeout int, maxRetries int, retryDelay int) (int, []byte, bool, error) {
	for i := 0; i <= maxRetries; i++ {
		code, data, err := h.PostJSON(url, jsonData, headers, timeout)
		if err != nil {
			return 0, nil, false, err
		}

		// 成功的HTTP状态码范围：200-299
		if code >= 200 && code < 300 {
			return code, data, true, nil
		}

		// 如果不是最后一次重试，则等待后重试
		if i < maxRetries {
			fmt.Printf("JSON POST请求失败，状态码: %d，%d毫秒后重试 (%d/%d)\n", code, retryDelay, i+1, maxRetries)
			time.Sleep(time.Duration(retryDelay) * time.Millisecond)
		}
	}

	return 0, nil, false, nil
}

// PostMultipartWithRetry 带重试机制的PostMultipart请求
// url: 请求的URL
// fileName: 文件名
// fileData: 文件数据
// maxRetries: 最大重试次数
// retryDelay: 重试间隔时间（毫秒）
// 返回值: 状态码、响应数据和是否成功
func (h *HttpClient) PostMultipartWithRetry(url string, fileName string, fileData []byte, maxRetries int, retryDelay int) (int, []byte, bool) {
	for i := 0; i <= maxRetries; i++ {
		code, data := https.PostMultipart(url, fileName, fileData, 0)

		// 成功的HTTP状态码范围：200-299
		if code >= 200 && code < 300 {
			return code, data, true
		}

		// 如果不是最后一次重试，则等待后重试
		if i < maxRetries {
			fmt.Printf("POST请求失败，状态码: %d，%d毫秒后重试 (%d/%d)\n", code, retryDelay, i+1, maxRetries)
			time.Sleep(time.Duration(retryDelay) * time.Millisecond)
		}
	}

	return 0, nil, false
}

// IsSuccessCode 判断HTTP状态码是否为成功状态
func (h *HttpClient) IsSuccessCode(code int) bool {
	return code >= 200 && code < 300
}

// GetAsString 发送GET请求并返回字符串形式的响应
func (h *HttpClient) GetAsString(url string, timeout int) (int, string) {
	code, data := https.Get(url, timeout)
	return code, string(data)
}

// PostMultipartAsString 发送POST请求并返回字符串形式的响应
func (h *HttpClient) PostMultipartAsString(url string, fileName string, fileData []byte) (int, string) {
	code, data := https.PostMultipart(url, fileName, fileData, 0)
	return code, string(data)
}

// 全局HTTP客户端实例
var HttpRequest = NewHttpClient()
