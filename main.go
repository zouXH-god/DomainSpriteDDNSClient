package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultDataPath = "data.json"
	maxResponseSize = 1 << 20
)

type RecordInfo struct {
	ID            string    `json:"id"`
	DomainID      string    `json:"domainId"`
	DomainName    string    `json:"domainName"`
	Line          string    `json:"line"`
	RecordName    string    `json:"recordName"`
	RecordType    string    `json:"recordType"`
	RecordContent string    `json:"recordContent"`
	Status        string    `json:"status"`
	Locked        bool      `json:"locked"`
	Proxied       bool      `json:"proxied"`
	TTL           int64     `json:"ttl"`
	Weight        int32     `json:"weight"`
	Settings      string    `json:"settings"`
	Meta          string    `json:"meta"`
	Comment       string    `json:"comment"`
	Tags          []string  `json:"tags"`
	CreateTime    time.Time `json:"createTime"`
	UpdateTime    time.Time `json:"updateTime"`
	DNSFrom       string    `json:"dnsFrom"`
}

type SavedData struct {
	Token      string     `json:"token"`
	RecordInfo RecordInfo `json:"recordInfo"`
}

type APIResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type legacySavedData struct {
	Data SavedData `json:"data"`
}

type apiError struct {
	Status  int
	Code    int
	Message string
}

func (e *apiError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("服务端返回 HTTP %d", e.Status)
	}
	return fmt.Sprintf("服务端返回 HTTP %d：%s", e.Status, e.Message)
}

func endpoint(baseURL, path string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + path
}

func request(ctx context.Context, client *http.Client, method, url, accessSalt string, body any, out any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("编码请求: %w", err)
		}
	}
	for attempt := 1; attempt <= 3; attempt++ {
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, reader)
		if err != nil {
			return fmt.Errorf("创建请求: %w", err)
		}
		if accessSalt != "" {
			req.Header.Set("AccessSalt", accessSalt)
		}
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "DomainSpriteDDNSClient/2")

		resp, doErr := client.Do(req)
		if doErr != nil {
			if attempt < 3 && ctx.Err() == nil {
				time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
				continue
			}
			return fmt.Errorf("请求服务端: %w", doErr)
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("读取响应: %w", readErr)
		}
		if len(raw) > maxResponseSize {
			return errors.New("服务端响应超过 1 MiB 限制")
		}
		var envelope APIResponse
		decodeErr := json.Unmarshal(raw, &envelope)
		if (resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable) && attempt < 3 {
			time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			message := http.StatusText(resp.StatusCode)
			if decodeErr == nil && envelope.Message != "" {
				message = envelope.Message
			}
			return &apiError{Status: resp.StatusCode, Code: envelope.Code, Message: message}
		}
		if decodeErr != nil {
			return fmt.Errorf("解析服务端响应: %w", decodeErr)
		}
		if envelope.Code != 0 && envelope.Code != http.StatusOK {
			return &apiError{Status: resp.StatusCode, Code: envelope.Code, Message: envelope.Message}
		}
		if out != nil {
			if err := json.Unmarshal(envelope.Data, out); err != nil {
				return fmt.Errorf("解析响应数据: %w", err)
			}
		}
		return nil
	}
	return errors.New("请求重试次数已耗尽")
}

func makeIP2A(ctx context.Context, client *http.Client, baseURL, accessSalt, dataPath string) error {
	var saved SavedData
	if err := request(ctx, client, http.MethodPost, endpoint(baseURL, "/fast/ip2a"), accessSalt, nil, &saved); err != nil {
		return err
	}
	if saved.Token == "" || saved.RecordInfo.DomainName == "" {
		return errors.New("服务端未返回完整 Token 或域名信息")
	}
	if err := saveData(dataPath, saved); err != nil {
		return err
	}
	fmt.Printf("域名信息：【%s.%s】\n", saved.RecordInfo.RecordName, saved.RecordInfo.DomainName)
	return nil
}

func sendRecordUpdate(ctx context.Context, client *http.Client, baseURL, dataPath string) error {
	saved, err := loadData(dataPath)
	if err != nil {
		return err
	}
	if saved.Token == "" {
		return errors.New("本地状态中缺少更新 Token，请删除状态文件后重新初始化")
	}
	fmt.Printf("域名信息：【%s.%s】\n", saved.RecordInfo.RecordName, saved.RecordInfo.DomainName)
	var updated SavedData
	if err := request(ctx, client, http.MethodPut, endpoint(baseURL, "/fast/record"), "", map[string]string{"token": saved.Token}, &updated); err != nil {
		return err
	}
	if updated.Token != "" {
		if err := saveData(dataPath, updated); err != nil {
			return err
		}
	}
	fmt.Println("记录更新成功")
	return nil
}

func loadData(path string) (SavedData, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return SavedData{}, fmt.Errorf("读取状态文件: %w", err)
	}
	var saved SavedData
	if err := json.Unmarshal(raw, &saved); err == nil && saved.Token != "" {
		return saved, nil
	}
	var legacy legacySavedData
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return SavedData{}, fmt.Errorf("解析状态文件: %w", err)
	}
	if legacy.Data.Token == "" {
		return SavedData{}, errors.New("状态文件中没有 Token")
	}
	return legacy.Data, nil
}

func saveData(path string, saved SavedData) error {
	raw, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return fmt.Errorf("编码状态文件: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建状态目录: %w", err)
	}
	file, err := os.CreateTemp(dir, ".domainsprite-ddns-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时状态文件: %w", err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("写入状态文件: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("替换状态文件: %w", err)
	}
	return nil
}

func main() {
	_ = godotenv.Load()
	baseURLFlag := flag.String("baseUrl", "", "DomainSprite API 基础地址")
	accessSaltFlag := flag.String("accessSalt", "", "快速 DDNS AccessSalt（仅初始化需要）")
	dataPathFlag := flag.String("dataPath", "", "本地状态文件路径")
	timeoutFlag := flag.Duration("timeout", 15*time.Second, "单次运行总超时")
	flag.Parse()

	baseURL := firstNonEmpty(*baseURLFlag, os.Getenv("BASE_URL"))
	accessSalt := firstNonEmpty(*accessSaltFlag, os.Getenv("ACCESS_SALT"))
	dataPath := firstNonEmpty(*dataPathFlag, os.Getenv("DATA_PATH"), defaultDataPath)
	if baseURL == "" {
		fmt.Fprintln(os.Stderr, "错误：缺少 baseUrl（或 BASE_URL）")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()
	client := &http.Client{Timeout: *timeoutFlag}

	if _, err := os.Stat(dataPath); errors.Is(err, os.ErrNotExist) {
		if accessSalt == "" {
			fmt.Fprintln(os.Stderr, "错误：首次初始化需要 accessSalt（或 ACCESS_SALT）")
			os.Exit(1)
		}
		fmt.Println("初始化快速 DDNS 记录...")
		if err := makeIP2A(ctx, client, baseURL, accessSalt, dataPath); err != nil {
			fmt.Fprintf(os.Stderr, "初始化失败：%v\n", err)
			os.Exit(1)
		}
		fmt.Println("初始化完成，Token 已安全保存；后续更新无需 AccessSalt")
		return
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "检查状态文件失败：%v\n", err)
		os.Exit(1)
	}

	if err := sendRecordUpdate(ctx, client, baseURL, dataPath); err != nil {
		fmt.Fprintf(os.Stderr, "更新失败：%v\n", err)
		os.Exit(1)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
