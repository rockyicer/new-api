# 腾讯云部署 new-api 并提供 OpenAI 兼容接入的详细操作指南

## 1. 先说结论

你想做的事情，技术上应当拆成两部分：

1. 把 `new-api` 部署到腾讯云服务器，提供一个稳定的 OpenAI 兼容入口。
2. 用合规的上游 API 凭证做多 Key / 多渠道调度，再把 `base_url + api_key` 发给你的注册用户，让他们在 VSCode 里直接使用。

但有一件事不要做：

- 不要购买很多 `ChatGPT` 网页账号，然后把这些账号当作“号池”转卖或共享给别人使用。

原因有三点：

1. `ChatGPT` 订阅和 `OpenAI API` 是分开计费、分开管理的，不是一个东西。
2. 官方明确要求 API 访问应使用 API keys，并建议用 project 级别的限额与权限管理。
3. 如果在不受支持的国家/地区访问或向外提供 API 访问，账号可能被限制或停用。

所以，这份文档采用的是可长期运营的正规方案：

- 上游使用 `OpenAI API key`、`Azure OpenAI` 或其他你有权接入/分发的模型服务。
- `new-api` 负责统一鉴权、用户额度、日志、模型映射、OpenAI 兼容接口。
- 你的用户只拿到：
  - `Base URL`: `https://api.your-domain.com/v1`
  - `API Key`: 他们自己的 `new-api` 访问令牌

## 2. 合规边界与适用前提

这部分很重要，先看完再部署。

### 2.1 不建议的做法

下面这些做法都不建议：

- 购买多个 ChatGPT Plus / Pro 网页账号，拿网页登录态去轮转代理。
- 用个人聊天账号给第三方用户“转售 GPT”。
- 在不受支持的国家/地区直接对外提供 OpenAI API 访问。

### 2.2 这份文档默认假设

这份文档默认你满足下面条件：

1. 你使用的是合规的上游 API 凭证。
2. 你有权把该能力嵌入自己的系统或面向自己的用户群体提供服务。
3. 你愿意自己承担服务协议、隐私、计费、日志与合规责任。

### 2.3 中国境内公众服务的合规提醒

如果你的服务是“向中国境内公众开放注册和使用”的生成式 AI 服务，请不要只看技术文档就直接上线。

需要额外确认：

- 是否触发《生成式人工智能服务管理暂行办法》中的“向境内公众提供生成式人工智能服务”场景。
- 是否需要备案、安全评估、内容治理、用户协议、隐私政策、投诉举报入口等。
- 你使用的上游模型服务，是否允许这种运营方式。

如果你只是给自己团队或少量内测用户用，风险和义务与“面向公众运营”不同，但也仍然需要遵守上游服务条款。

## 3. 三种部署/运营方案

先给你三个方案，推荐你直接走方案 B。

### 方案 A：最省事的测试版

架构：

- 1 台腾讯云 Lighthouse
- Docker Compose
- `new-api + PostgreSQL + Redis`
- 临时用公网 IP 访问

优点：

- 最快能跑起来
- 适合自己先验证后台功能

缺点：

- 没有域名和 HTTPS
- 不适合给真实用户长期使用
- VSCode 等客户端配置体验较差

适合：

- 你先自己试通全流程

### 方案 B：新手推荐的可运营版

架构：

- 1 台腾讯云 Lighthouse
- Ubuntu 22.04 LTS
- Docker Compose 跑 `new-api + PostgreSQL + Redis`
- Nginx 反代
- 域名 + HTTPS
- `new-api` 内部做用户、额度、令牌、渠道调度

优点：

- 成本和复杂度都可控
- 适合 0 到 1
- 用户可以稳定用 `https://your-domain/v1 + api_key`

缺点：

- 还是单机
- PostgreSQL、Redis、应用都在同一台机器上

适合：

- 新手第一次正式部署
- 10 到 200 个用户的早期阶段

### 方案 C：正式运营扩展版

架构：

- 多台 CVM / Lighthouse
- 托管 PostgreSQL
- 托管 Redis
- CLB / WAF / CDN
- 对象存储备份

优点：

- 更稳
- 更适合持续运营

缺点：

- 成本更高
- 运维复杂度显著上升

适合：

- 已经有稳定用户量
- 已经验证商业模式

### 推荐

建议直接用 **方案 B**。

下面的步骤全部按方案 B 写。

## 4. 目标架构图

```text
User / VSCode Plugin
        |
        | HTTPS
        v
api.your-domain.com
        |
        v
Nginx (80/443)
        |
        v
new-api (Docker, localhost:3000)
        |
        +--> PostgreSQL
        |
        +--> Redis
        |
        +--> Upstream providers
              - OpenAI API projects / keys
              - Azure OpenAI
              - Other compliant upstreams
```

## 5. 第 0 步：先决定你要不要直接用 OpenAI

这一点必须先想清楚。

### 可以直接用 OpenAI 的前提

只有在下面条件成立时，才建议你把 OpenAI 作为正式上游：

- 你的账户、主体和使用地区都在 OpenAI 支持范围内。
- 你用的是 `API key`，不是 ChatGPT 网页账号。
- 你的业务模式符合上游服务条款。

### 如果不满足

如果你不确定是否满足，建议先这样做：

1. 先把 new-api 部署好。
2. 先接入你当前合规可用的上游，例如 Azure OpenAI、DeepSeek、Gemini、腾讯混元等。
3. 对外仍然提供 OpenAI 兼容接口。

这意味着：

- 你的用户在 VSCode 里仍然是填 `Base URL + API Key`。
- 只是上游不一定是 OpenAI 原厂。

## 6. 第 1 步：购买腾讯云服务器

### 6.1 机器类型

对新手来说，优先选：

- `腾讯云 Lighthouse（轻量应用服务器）`

原因：

- 控制台更简单
- 套餐更直观
- 比 CVM 更适合第一次上云

### 6.2 推荐规格

这是经验值，不是官方硬性要求：

- 最低可用：`2C4G`
- 更稳妥：`4C8G`
- 系统盘：`60GB SSD` 或更高
- 操作系统：`Ubuntu 22.04 LTS`

### 6.3 地域选择建议

如果你计划接 OpenAI 这类国际上游，通常更建议优先考虑：

- `中国香港`
- `新加坡`
- `东京`

不要默认选中国大陆地域，原因不是 new-api 本身，而是你的上游可达性、条款适配和网络路径都可能受影响。

### 6.4 防火墙端口

在腾讯云控制台放通这些端口：

- `22`：SSH
- `80`：HTTP
- `443`：HTTPS

不建议把 `3000` 直接暴露给公网。

## 7. 第 2 步：准备域名与 HTTPS

### 7.1 准备一个子域名

建议单独使用一个 API 子域名，例如：

- `api.your-domain.com`

### 7.2 DNS 解析

在 DNS 控制台新增一条解析：

- Type: `A`
- Host: `api`
- Value: 你的 Lighthouse 公网 IP

### 7.3 HTTPS

推荐两种方式：

1. 用腾讯云 SSL 证书服务申请/部署证书
2. 用 `certbot` 给 Nginx 配 Let’s Encrypt

如果你已经在腾讯云体系里管理域名和证书，优先走腾讯云 SSL 会更省心。

## 8. 第 3 步：登录服务器并安装基础环境

先用 SSH 登录服务器。

```bash
ssh ubuntu@YOUR_SERVER_IP
```

更新系统并安装基础软件：

```bash
sudo apt update
sudo apt upgrade -y
sudo apt install -y git curl nginx docker.io docker-compose-plugin
sudo systemctl enable --now nginx
sudo systemctl enable --now docker
sudo usermod -aG docker $USER
```

执行完 `usermod` 后，退出 SSH 再重新登录一次，确保当前用户能直接使用 Docker。

验证：

```bash
docker version
docker compose version
nginx -v
```

## 9. 第 4 步：拉取项目并准备生产配置

```bash
cd ~
git clone https://github.com/QuantumNous/new-api.git
cd new-api
mkdir -p data logs deploy
```

生成几个随机密钥，先记下来：

```bash
openssl rand -hex 32
openssl rand -hex 32
openssl rand -hex 24
```

其中你至少需要：

- 一个给 `SESSION_SECRET`
- 一个给 `CRYPTO_SECRET`
- 一个给 PostgreSQL 密码

## 10. 第 5 步：创建生产用 Docker Compose 文件

不要直接拿仓库里的默认密码上线。

在 `~/new-api/deploy/docker-compose.prod.yml` 写入下面内容：

```yaml
version: "3.8"

services:
  new-api:
    image: calciumion/new-api:latest
    container_name: new-api
    restart: always
    command: --log-dir /app/logs
    depends_on:
      - postgres
      - redis
    ports:
      - "127.0.0.1:3000:3000"
    volumes:
      - ../data:/data
      - ../logs:/app/logs
    environment:
      - TZ=Asia/Shanghai
      - SQL_DSN=postgresql://newapi:CHANGE_DB_PASSWORD@postgres:5432/newapi?sslmode=disable
      - REDIS_CONN_STRING=redis://redis:6379/0
      - SESSION_SECRET=CHANGE_SESSION_SECRET
      - CRYPTO_SECRET=CHANGE_CRYPTO_SECRET
      - ERROR_LOG_ENABLED=true
      - BATCH_UPDATE_ENABLED=true

  postgres:
    image: postgres:15
    container_name: new-api-postgres
    restart: always
    environment:
      POSTGRES_USER: newapi
      POSTGRES_PASSWORD: CHANGE_DB_PASSWORD
      POSTGRES_DB: newapi
    volumes:
      - pg_data:/var/lib/postgresql/data

  redis:
    image: redis:7
    container_name: new-api-redis
    restart: always

volumes:
  pg_data:
```

把下面这些值全部替换掉：

- `CHANGE_DB_PASSWORD`
- `CHANGE_SESSION_SECRET`
- `CHANGE_CRYPTO_SECRET`

启动：

```bash
cd ~/new-api/deploy
docker compose -f docker-compose.prod.yml up -d
```

查看状态：

```bash
docker compose -f docker-compose.prod.yml ps
docker logs -f new-api
```

如果成功，先用本机访问：

```bash
curl http://127.0.0.1:3000/api/status
```

## 11. 第 6 步：配置 Nginx 反向代理

创建 Nginx 站点配置：

```bash
sudo nano /etc/nginx/sites-available/new-api
```

写入：

```nginx
map $http_upgrade $connection_upgrade {
    default upgrade;
    '' close;
}

server {
    listen 80;
    server_name api.your-domain.com;

    client_max_body_size 50m;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
    }
}
```

启用站点：

```bash
sudo ln -s /etc/nginx/sites-available/new-api /etc/nginx/sites-enabled/new-api
sudo nginx -t
sudo systemctl reload nginx
```

这时你应该可以访问：

- `http://api.your-domain.com`

## 12. 第 7 步：配置 HTTPS

### 方式 1：腾讯云 SSL 证书

如果你用腾讯云 SSL 证书：

1. 在腾讯云 SSL 控制台申请证书
2. 完成域名验证
3. 下载 Nginx 证书文件，或使用腾讯云支持的自动部署
4. 把证书路径填到 Nginx 配置里

Nginx HTTPS 示例：

```nginx
server {
    listen 443 ssl http2;
    server_name api.your-domain.com;

    ssl_certificate /etc/nginx/ssl/fullchain.pem;
    ssl_certificate_key /etc/nginx/ssl/privkey.pem;

    client_max_body_size 50m;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
    }
}

server {
    listen 80;
    server_name api.your-domain.com;
    return 301 https://$host$request_uri;
}
```

然后重载：

```bash
sudo nginx -t
sudo systemctl reload nginx
```

### 方式 2：certbot

如果你更熟悉标准 Linux 运维，也可以用 `certbot --nginx` 自动签发和续期。

## 13. 第 8 步：首次打开 new-api 并初始化系统

打开：

- `https://api.your-domain.com`

第一次会进入系统初始化向导。

### 8.1 初始化时怎么选

建议这样选：

- 使用模式：`对外运营模式`
- 管理员账号：自己设置一个用户名和强密码

不要选：

- `自用模式`，因为你要给注册用户使用
- `演示站点模式`，因为你不是做演示页

### 8.2 初始化后先做这几件事

1. 登录管理员账号
2. 修改站点名称、Logo、用户协议、隐私政策
3. 开启 2FA
4. 检查系统设置里的注册开关
5. 设置新用户初始额度

## 14. 第 9 步：打开注册与用户额度

你要给注册用户发 API，因此必须把“注册 -> 登录 -> 生成个人令牌”这条链打通。

### 9.1 开启注册

后台找到：

- `系统设置`
- `配置登录注册`

至少开启：

- `允许新用户注册`
- `允许通过密码进行注册`

可选开启：

- `通过密码注册时需要进行邮箱验证`

如果你不想开放公网注册，也可以：

- 关闭公开注册
- 由管理员在 `用户管理` 中手工创建用户

### 9.2 设置新用户初始额度

后台找到：

- `运营设置`
- `额度设置`
- `新用户初始额度`

建议你一开始就设成明确数值，不要留空。

比如：

- 内测：`100000`
- 小范围试用：`500000`

具体数值取决于你的计费规则和模型倍率。

## 15. 第 10 步：理解“上游账号池”在 new-api 里应该怎么做

这里是最关键的部分。

你真正要管理的，不是“很多 ChatGPT 网页账号”，而是：

- 很多上游 `API keys`
- 或很多上游 `project keys`
- 或很多 `Azure OpenAI deployment credentials`

在 new-api 里，对应的是：

- `渠道（Channel）`
- `多密钥渠道（Multi-key Channel）`
- `优先级（Priority）`
- `权重（Weight）`

### 10.1 你真正想要的“平衡”其实有两类

你说“不要某些号用得多、某些号用得少”，这里面有两个目标：

1. **流量分配尽量平均**
2. **额度用完时能及时发现并摘掉坏 Key**

这两个目标，在 new-api 里不是完全同一个配置。

### 10.2 方案 1：每个 Key 一个独立渠道

做法：

- 一个 OpenAI API key 建一个渠道
- 所有渠道使用同一组模型
- 同一个分组
- 同一个优先级
- 同一个权重

优点：

- 支持余额查询与余额更新
- 某个 Key 用完了，更容易禁用
- 运营上更透明

缺点：

- 渠道选择是“同优先级下按权重随机”，不是严格轮询
- 长时间看会比较均衡，短时间内不一定完全平均

适合：

- 新手第一阶段
- 你更看重“可观察性”和“可维护性”

### 10.3 方案 2：一个多密钥渠道，内部设为 polling

做法：

- 把多个 Key 按换行写进一个渠道
- 创建渠道时选择多密钥模式
- 多密钥策略选 `polling`

优点：

- 比较接近“轮询均匀分配”
- 非常适合多个完全同质的 Key

缺点：

- **多密钥渠道不支持余额查询**
- 当某个 Key 额度不足时，运营排查没有“每 Key 单独渠道”直观

适合：

- 你非常在意“尽量平均轮询”
- 这些 Key 完全同类同模型
- 你接受手工管理 Key 状态

### 10.4 方案 3：推荐的折中方案

建议你这样做：

1. 先用 **方案 1：每 Key 一个渠道**
2. 如果后续发现分配仍不够均匀，再把一部分完全同质的 Key 改成 **多密钥 polling**

对新手来说，这个折中方案最稳。

## 16. 第 11 步：推荐的渠道配置方式

### 11.1 新手推荐做法：每个 Key 一个渠道

假设你有 5 个 OpenAI API keys。

你就在后台 `渠道管理` 里创建 5 个渠道：

- Name: `openai-01`
- Name: `openai-02`
- Name: `openai-03`
- Name: `openai-04`
- Name: `openai-05`

每个渠道都设置成：

- Type: `OpenAI`
- Group: `default`
- Models: 例如 `gpt-4o,gpt-4.1-mini,text-embedding-3-large`
- Priority: `10`
- Weight: `100`
- Base URL: 默认或官方 API 地址

这样同优先级下，new-api 会按权重做选择。

### 11.2 如果某些 Key 配额更大

如果某个 Key 的预算明显更高，可以给它更高权重：

- 普通 Key：`Weight=100`
- 大预算 Key：`Weight=200`

这意味着它会拿到大约双倍流量。

### 11.3 如果你更想接近轮询平均

如果你的 5 个 Key 完全相同，并且你希望更平均：

- 在新增渠道时，选择多密钥模式
- `Mode`: `multi_to_single`
- `Multi Key Mode`: `polling`
- 把 5 个 key 按换行填到同一个渠道里

示意：

```text
sk-key-1
sk-key-2
sk-key-3
sk-key-4
sk-key-5
```

## 17. 第 12 步：渠道配置的实操建议

### 12.1 只给用户暴露你准备好的模型

不要一开始就把所有模型都开放。

建议只放你真的要卖/要用的几类：

- 文本主力模型
- 便宜模型
- embedding 模型

例如：

- `gpt-4o`
- `gpt-4.1-mini`
- `text-embedding-3-large`

### 12.2 用“标签”管理同类渠道

如果你有很多渠道，建议给同类渠道打相同标签。

例如：

- Tag: `openai-prod`

这样后面批量改优先级/权重会更方便。

### 12.3 先不要混太多不同上游

对新手来说，第一阶段不要在同一个用户组里同时混：

- OpenAI
- Azure OpenAI
- Gemini
- 其他若干家

原因：

- 延迟特征不同
- 兼容细节不同
- 排障更难

最好先把一条链跑稳，再扩上游。

## 18. 第 13 步：如何给注册用户发 API

你有两种方式。

### 方式 A：让每个用户自己生成个人访问令牌

这是最推荐的。

流程：

1. 用户注册并登录
2. 打开个人设置
3. 进入 `安全设置`
4. 找到 `系统访问令牌`
5. 点击 `生成令牌`

然后用户拿到：

- Base URL: `https://api.your-domain.com/v1`
- API Key: 个人系统访问令牌

优点：

- 最清晰
- 用户和额度天然绑定
- 你可以按用户看用量

### 方式 B：管理员在“令牌管理”里创建专用 Token

适合：

- 某些用户不需要网页登录
- 你想直接发一个服务令牌给他们

但对新手来说，先用方式 A 即可。

## 19. 第 14 步：让用户在 VSCode 里使用

只要插件支持 OpenAI-compatible API，就能接。

### 14.1 统一发给用户的接入信息

你应该发给用户：

- `Base URL`: `https://api.your-domain.com/v1`
- `API Key`: 用户自己的 new-api token
- `Model`: 例如 `gpt-4o`

### 14.2 以 Cline 为例

官方文档明确支持自定义 OpenAI-compatible base URL。

用户配置时填：

- Provider: `OpenAI Compatible`
- Base URL: `https://api.your-domain.com/v1`
- API Key: 你发给他的 token
- Model: `gpt-4o` 或你开放的模型名

如果用户使用 Cline CLI，示例命令类似：

```bash
cline auth -p openai -k YOUR_NEW_API_TOKEN -b https://api.your-domain.com/v1
```

### 14.3 以 Continue 为例

Continue 文档也支持自定义 `apiBase`。

示例：

```yaml
name: My Config
version: 0.0.1
schema: v1

models:
  - name: My NewAPI
    provider: openai
    model: gpt-4o
    apiKey: YOUR_NEW_API_TOKEN
    apiBase: https://api.your-domain.com/v1
```

### 14.4 常见报错排查

如果用户在 VSCode 里报错，优先检查：

1. Base URL 有没有写成 `https://api.your-domain.com/v1`
2. Token 是否复制完整
3. 模型名是否与你在 new-api 中开放的一致
4. 用户额度是否足够
5. 渠道是否可用

## 20. 第 15 步：如何做额度和轮转平衡

### 15.1 如果你更看重“均匀轮转”

选：

- 多密钥渠道
- `Multi Key Mode = polling`

适合：

- Key 同质
- 你追求尽量平均轮询

### 15.2 如果你更看重“余额可见、坏 Key 可摘除”

选：

- 每个 Key 一个渠道
- 相同优先级
- 相同权重

适合：

- 新手
- 需要运维可视化

### 15.3 我的实际建议

对你这个阶段，建议按下面顺序来：

1. 先做 `每个 Key 一个渠道`
2. 所有渠道：
   - 相同 `Priority`
   - 相同 `Weight`
   - 相同 `Group`
   - 相同模型列表
3. 观察一周使用情况
4. 如果你仍然强烈希望更均匀，再改成 `multi-key polling`

### 15.4 不要追求“绝对平均”

你真正要优化的是：

- 用户体验稳定
- 坏 Key 能及时摘除
- 整体成本可控

不是把每个 Key 精确打到一模一样的调用次数。

只要：

- 同权重渠道长期分布接近
- 不会集中打爆某一个 Key
- 某个 Key 异常时能被摘掉

就已经足够运营。

## 21. 第 16 步：上线前检查清单

上线前至少确认下面这些项：

- 域名已经解析到服务器
- HTTPS 已正常工作
- `http://` 已跳转到 `https://`
- `new-api` 后台可以登录
- 已经完成初始化
- 注册功能已按你的策略开启或关闭
- 新用户初始额度已设置
- 至少 1 个可用渠道已配置
- 至少 1 个测试用户可生成个人访问令牌
- VSCode 里已用测试账号跑通一次对话

## 22. 第 17 步：日常运维怎么做

### 17.1 看容器状态

```bash
cd ~/new-api/deploy
docker compose -f docker-compose.prod.yml ps
docker logs --tail 200 new-api
```

### 17.2 更新 new-api

```bash
cd ~/new-api
git pull
cd deploy
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

### 17.3 备份 PostgreSQL

```bash
docker exec -t new-api-postgres pg_dump -U newapi newapi > ~/newapi-backup.sql
```

### 17.4 备份应用数据和日志

```bash
cd ~/new-api
tar czf ~/newapi-data-$(date +%F).tar.gz data logs
```

### 17.5 发现某个渠道异常怎么办

操作顺序建议：

1. 到 `渠道管理` 看该渠道最近报错
2. 手工测试该渠道
3. 如果确认 Key 坏了，先禁用该渠道
4. 替换新 Key
5. 再启用

## 23. 第 18 步：我建议你现在就这样开始

如果你现在就要开工，最简执行顺序如下：

1. 购买 `1 台 Lighthouse`
2. 选 `Ubuntu 22.04 + 2C4G 或 4C8G`
3. 放通 `22/80/443`
4. 绑定 `api.your-domain.com`
5. 按本文部署 `new-api + PostgreSQL + Redis + Nginx`
6. 初始化系统时选 `对外运营模式`
7. 打开 `允许新用户注册`
8. 设置 `新用户初始额度`
9. 用 **每个 Key 一个渠道** 的方式接入上游
10. 自己注册一个普通用户
11. 生成个人访问令牌
12. 在 VSCode 里用 `https://api.your-domain.com/v1 + token` 跑通一次

## 24. 常见问题

### Q1：我能不能买很多 ChatGPT Plus 账号来做号池？

不建议。

应该用：

- OpenAI API keys
- Azure OpenAI
- 或其他你有权接入/分发的上游 API

### Q2：我必须一开始就做多机高可用吗？

不用。

对新手，先把单机可运营版跑稳更重要。

### Q3：用户一定要登录网站吗？

不一定。

但如果你要做“注册用户 + 额度 + 个人 API key”，让用户先登录再自己生成令牌，是最省事、最清晰的做法。

### Q4：我给用户的 URL 到底写哪个？

通常写：

- `https://api.your-domain.com/v1`

### Q5：我需要直接暴露 3000 端口吗？

不需要。

建议只暴露：

- `80`
- `443`

由 Nginx 反代到 `127.0.0.1:3000`。

## 25. 外部参考资料

以下资料用于确认产品与合规边界，实际部署命令仍以本文为主：

- 腾讯云 Lighthouse 产品页：https://cloud.tencent.com/product/lighthouse
- 腾讯云轻量应用服务器防火墙模板规则：https://cloud.tencent.com/document/product/1207/96199
- 腾讯云 SSL 证书自动部署概览：https://cloud.tencent.com/document/product/400/83538
- OpenAI API 支持国家/地区：https://help.openai.com/ja-jp/articles/5347006-openai-api-supported-countries-and-territories
- OpenAI API 与 ChatGPT 分开计费说明：https://help.openai.com/es-es/articles/8156019-how-can-i-move-my-chatgpt-subscription-to-the-api
- OpenAI API 生产最佳实践（API keys / project limits）：https://platform.openai.com/docs/guides/production-best-practices/model-overview
- Cline OpenAI-compatible 自定义 Base URL：https://docs.cline.bot/cline-cli/installation
- Continue 自定义 `apiBase` 示例：https://docs.continue.dev/customize/model-providers/more/zai
- 中国网信网《生成式人工智能服务管理暂行办法》：https://www.cac.gov.cn/2023-07/13/c_1690898327029107.htm

## 26. 仓库内与本文直接相关的实现依据

本文中的轮转与路由建议，直接对应仓库里的这些能力：

- 渠道加权随机：`model/ability.go`
- 缓存模式下的按优先级 + 权重选路：`model/channel_cache.go`
- 多密钥模式 `random / polling`：`model/channel.go` 与 `constant/multi_key_mode.go`
- 多密钥管理接口：`controller/channel.go`
- 多密钥渠道不支持余额查询：`controller/channel-billing.go`
- OpenAI 兼容 `/v1/*` 路由：`router/relay-router.go`

