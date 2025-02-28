# DomainSpriteDDNSClient

基于 [DomainSprite项目](https://github.com/zouXH-god/DomainSprite) 的轻量级动态DNS更新客户端，专为自动化域名记录管理设计。支持主流DNS服务商API，实现IP变更的实时同步。

## 🌟 功能特性

- 智能配置优先级：命令行参数 > 环境变量 > 配置文件
- 双阶段操作流程：安全初始化 + 增量更新
- 自动持久化Token：加密存储认证信息
- 跨平台支持：Windows/Linux/macOS全兼容
- 企业级错误处理：自动重试机制（最大3次）
- 详细日志追踪：支持不同日志级别（DEBUG/INFO/ERROR）

## 📦 快速使用

### [下载二进制包](https://github.com/zouXH-god/DomainSpriteDDNSClient/releases)

## ⚙️ 配置指南

### 配置方式（按优先级排序）

1. **命令行参数**
```bash
./DomainSpriteClient \
  -baseUrl="https://api.your-dns-provider.com" \
  -accessSalt="your_secret_salt" 
```

2. **环境变量**（推荐生产环境使用）
```bash
export BASE_URL="https://api.your-dns-provider.com"
export ACCESS_SALT="your_secret_salt"
./DomainSpriteClient
```

3. **.env文件**
```ini
# .env 示例
BASE_URL = "https://api.your-dns-provider.com"
ACCESS_SALT = "your_secret_salt"
```

```bash
./DomainSpriteClient
```

### 配置参数说明

| 参数            | 必填 | 默认值     | 描述                     |
|-----------------|------|------------|------------------------|
| baseUrl         | ✅   | -          | DomainSprite服务端 API端点  |
| accessSalt      | ✅   | -          | 服务端认证盐                 |
