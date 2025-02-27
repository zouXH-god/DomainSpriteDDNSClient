package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type RecordInfo struct {
	Id            string    `json:"id"`              // 记录ID
	DomainId      string    `json:"domainId" `       // 域名ID
	DomainName    string    `json:"domainName"  `    // 域名
	Line          string    `json:"line"  `          // 解析线路
	RecordName    string    `json:"recordName"  `    // 记录名称
	RecordType    string    `json:"recordType"  `    // 记录类型
	RecordContent string    `json:"recordContent"  ` // 记录值
	Status        string    `json:"status"  `        // 记录状态
	Locked        bool      `json:"locked"`          // 是否锁定
	Proxied       bool      `json:"proxied"`         // 是否启用代理
	Ttl           int64     `json:"ttl" `            // TTL
	Weight        int32     `json:"weight" `         // 权重
	Settings      string    `json:"settings"`        // 设置
	Meta          string    `json:"meta"`            // 元数据
	Comment       string    `json:"comment"`         // 备注
	Tags          []string  `json:"tags"  `          // 标签
	CreateTime    time.Time `json:"createTime"`      // 创建时间
	UpdateTime    time.Time `json:"updateTime"`      // 更新时间
	DnsFrom       string    `json:"dnsFrom"`         // 域名解析来源
}

type IpToAResponse struct {
	Data struct {
		Token      string     `json:"token"`
		RecordInfo RecordInfo `json:"recordInfo"`
	} `json:"data"`
}

const dataPath = "data.json"

func makeIp2A(baseUrl, AccessSalt string) error {
	urlPath := "/fast/ip2a"
	url := baseUrl + urlPath
	println(url)
	println(AccessSalt)

	// 创建HTTP请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Add("AccessSalt", AccessSalt)

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("非200状态码: %d", resp.StatusCode)
	}

	// 解析响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	var response IpToAResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("JSON解析失败: %v", err)
	}

	// 保存响应数据
	data, _ := json.MarshalIndent(response, "", "  ")
	if err := os.WriteFile(dataPath, data, 0644); err != nil {
		return fmt.Errorf("保存文件失败: %v", err)
	}

	return nil
}

func sendRecordUpdate(baseUrl string) error {
	// 读取保存的数据
	data, err := os.ReadFile(dataPath)
	if err != nil {
		return fmt.Errorf("读取数据文件失败: %v", err)
	}

	var savedData IpToAResponse
	if err := json.Unmarshal(data, &savedData); err != nil {
		return fmt.Errorf("解析保存数据失败: %v", err)
	}

	// 构造请求URL
	urlPath := "/fast/updateRecord?token=" + savedData.Data.Token
	url := baseUrl + urlPath

	// 发送请求
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("更新请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("更新返回非200状态码: %d", resp.StatusCode)
	}

	fmt.Println("记录更新成功")
	return nil
}

func main() {
	// 加载环境变量
	_ = godotenv.Load()

	// 解析命令行参数
	var (
		baseUrlPtr    = flag.String("baseUrl", "", "API基础地址")
		accessSaltPtr = flag.String("accessSalt", "", "访问盐值")
	)
	flag.Parse()

	// 获取配置（环境变量优先，命令行参数覆盖）
	baseUrl := firstNonEmpty(
		*baseUrlPtr,
		os.Getenv("BASE_URL"),
	)
	AccessSalt := firstNonEmpty(
		*accessSaltPtr,
		os.Getenv("ACCESS_SALT"),
	)

	// 校验必要参数
	if baseUrl == "" {
		fmt.Println("错误: 缺少必要参数(baseUrl)")
		os.Exit(1)
	}

	// 检查数据文件是否存在
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		fmt.Println("初始化记录...")
		if err := makeIp2A(baseUrl, AccessSalt); err != nil {
			fmt.Printf("初始化失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("初始化完成，已保存记录信息")
	} else {
		fmt.Println("发送记录更新...")
		if err := sendRecordUpdate(baseUrl); err != nil {
			fmt.Printf("更新失败: %v\n", err)
			os.Exit(1)
		}
	}
}

// 辅助函数：获取第一个非空值
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
