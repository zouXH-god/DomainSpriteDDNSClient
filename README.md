# DomainSpriteDDNSClient

DomainSprite 的轻量动态 DNS 客户端。首次运行创建或获取快速 DDNS 记录并保存更新 Token，后续运行根据当前公网来源 IP 更新该记录。

客户端使用新版安全接口：

- `POST /fast/ip2a`：使用 `AccessSalt` 请求头创建或获取记录。
- `PUT /fast/record`：通过 JSON body 传递 Token 更新记录，Token 不进入 URL。

## 使用方法

### 命令行参数

```bash
./DomainSpriteDDNSClient \
  -baseUrl="https://domainsprite.example.com" \
  -accessSalt="your-fast-ddns-access-salt"
```

首次运行成功后，Token 会保存到 `data.json`；后续更新只需：

```bash
./DomainSpriteDDNSClient -baseUrl="https://domainsprite.example.com"
```

可用参数：

| 参数 | 环境变量 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-baseUrl` | `BASE_URL` | 无 | DomainSprite 服务地址，必填 |
| `-accessSalt` | `ACCESS_SALT` | 无 | 首次初始化所需的快速 DDNS AccessSalt |
| `-dataPath` | `DATA_PATH` | `data.json` | Token 状态文件位置 |
| `-timeout` | — | `15s` | 单次运行总超时 |

参数优先级为命令行、环境变量、默认值。程序也会读取当前目录的 `.env` 文件。

`.env` 示例：

```dotenv
BASE_URL=https://domainsprite.example.com
ACCESS_SALT=replace-with-your-access-salt
DATA_PATH=/var/lib/domainsprite-ddns/data.json
```

## 定时运行

客户端每次执行只检查和更新一次，适合通过 cron、systemd timer 或 Windows 任务计划定期调用。

Linux cron 示例（每 5 分钟）：

```cron
*/5 * * * * /usr/local/bin/DomainSpriteDDNSClient -baseUrl=https://domainsprite.example.com -dataPath=/var/lib/domainsprite-ddns/data.json
```

## 安全说明

- Token 只通过 HTTPS JSON 请求体发送，不放入查询参数。
- 状态文件使用同目录临时文件、`fsync` 和原子替换写入。
- Unix 状态文件权限为 `0600`。仍应确保运行账户和状态目录不被其他用户读取。
- AccessSalt 只在首次初始化时需要，不会写入状态文件。
- 生产环境必须使用可信 HTTPS 服务地址。
- 日志不会输出 AccessSalt 或 Token。

## 兼容性

新版客户端可读取旧版本保存的 `{ "data": { ... } }` 状态文件，并在下一次服务端返回完整数据时写成精简格式。服务端需要提供 DomainSprite 的新版 `POST /fast/ip2a` 和 `PUT /fast/record` 接口。

## 开发验证

```bash
gofmt -w .
go test ./...
go vet ./...
go build ./...
```
