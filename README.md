# AI Gateway

一个基于 Go 实现的轻量级 AI 代理网关，用于将不同前缀的请求转发到指定上游服务，并记录每次调用的请求、响应、状态码和耗时，提供简单的管理页面查看调用日志。

## 功能特性

- 按路由前缀转发请求到不同上游服务
- 自动保留前缀后的路径和查询参数
- 将调用日志写入 MySQL
- 自动初始化 `call_logs` 表
- 提供管理页面查看调用记录
- 支持按路由筛选、分页和自动刷新
- 支持 Docker 部署

## 项目结构

```text
AIGateway/
├─ main.go                     # 程序入口
├─ config.yaml                 # 运行配置文件（本地文件，已被 .gitignore 忽略）
├─ docker-compose.yml          # 容器启动配置
├─ Dockerfile                  # 镜像构建文件
├─ aigateway.conf              # Nginx 反向代理示例
├─ deploy.sh                   # Linux 部署脚本
└─ internal/
   ├─ config/                  # 配置加载与校验
   ├─ middleware/              # 请求日志中间件
   ├─ proxy/                   # 反向代理逻辑
   ├─ store/                   # MySQL 访问与表初始化
   └─ web/                     # 管理页面与日志 API
```

## 运行要求

- Go 1.24.4 或更高版本
- MySQL 5.7+/8.0+
- Docker / Docker Compose（可选）

## 配置说明

程序默认读取根目录下的 `config.yaml`，也可以通过参数指定：

```bash
./aigateway -config config.yaml
```

推荐配置模板如下：

```yaml
server:
  port: 8080

mysql:
  host: 127.0.0.1
  port: 3306
  user: your_user
  password: your_password
  database: aigateway

routes:
  - prefix: /deepseek/anthropic
    baseUrl: https://api.deepseek.com/anthropic
  - prefix: /openai
    baseUrl: https://api.openai.com
```

字段说明：

- `server.port`：网关监听端口
- `mysql.*`：MySQL 连接信息
- `routes[].prefix`：对外暴露的访问前缀
- `routes[].baseUrl`：对应上游服务地址

例如当配置为：

- `prefix: /deepseek/anthropic`
- `baseUrl: https://api.deepseek.com/anthropic`

则请求：

```text
POST /deepseek/anthropic/v1/messages
```

会被转发到：

```text
https://api.deepseek.com/anthropic/v1/messages
```

## 本地启动

1. 准备 MySQL 数据库
2. 在项目根目录创建 `config.yaml`
3. 安装依赖并启动

```bash
go mod tidy
go run . -config config.yaml
```

如果需要编译后运行：

```bash
go build -o aigateway .
./aigateway -config config.yaml
```

程序启动后会：

- 连接 MySQL
- 自动创建 `call_logs` 表
- 注册配置中的所有代理路由
- 启动管理页面和日志接口

## 管理页面与接口

- 管理页面：`http://localhost:8080/admin`
- 日志接口：`http://localhost:8080/api/logs`

日志接口支持参数：

- `route`：按路由前缀筛选
- `page`：页码，默认从 1 开始
- `pageSize`：每页数量，默认 20，最大 100

示例：

```text
GET /api/logs?route=/deepseek/anthropic&page=1&pageSize=20
```

## 数据表说明

程序会自动创建如下日志表：

- `route`：命中的路由前缀
- `method`：请求方法
- `request_url`：请求 URL
- `request_body`：请求体
- `response_status`：响应状态码
- `response_body`：响应体
- `duration_ms`：请求耗时
- `created_at`：记录创建时间

为避免数据过大，请求体和响应体会被截断后再写入数据库。

## Docker 部署

项目已包含 `Dockerfile` 和 `docker-compose.yml`。

### 方式一：直接使用 Docker Compose

先准备 `config.yaml`，然后执行：

```bash
docker compose up -d --build
```

默认映射：

- 容器内端口：`8080`
- 宿主机端口：`${SERVER_PORT:-8080}`

停止服务：

```bash
docker compose down
```

### 方式二：使用 deploy.sh

适用于 Linux 环境：

```bash
chmod +x deploy.sh
./deploy.sh
```

说明：

- 脚本会尝试构建 Linux 二进制文件
- 脚本会执行 `docker compose up -d --build`
- 脚本会检查 `/admin` 页面是否可访问

## Nginx 反向代理示例

仓库中的 `aigateway.conf` 提供了一个示例配置，可将外部访问路径 `/aigateway/` 转发到本地 `8080` 端口。

例如：

- 外部访问：`http://your-host/aigateway/admin`
- 实际转发：`http://127.0.0.1:8080/admin`
