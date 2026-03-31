# new-api 二开仓库的持续发布与服务器更新建议

## 1. 这份建议基于什么

这份文档是基于以下三类信息重新整理的：

1. `docs/installation/部署海外API服务器.pdf` 里的既有讨论，重点是“西欧客户、后续要扩容、仓库会持续更新”。
2. 当前仓库已经存在的发布资产：
   - [Dockerfile](../../Dockerfile)
   - [docker-compose.yml](../../docker-compose.yml)
   - [new-api.service](../../new-api.service)
   - [.github/workflows/docker-image-alpha.yml](../../.github/workflows/docker-image-alpha.yml)
   - [.github/workflows/docker-image-arm64.yml](../../.github/workflows/docker-image-arm64.yml)
   - [.github/workflows/release.yml](../../.github/workflows/release.yml)
3. 你当前的真实诉求：
   - 你会持续在本机二次开发；
   - 这个 repo 更新会比较频繁；
   - 你希望推送后服务器尽快看到变化；
   - 后续希望能平滑扩容，而不是每次都手工上线。

结论先说在前面：

- 不建议把线上服务器当开发机。
- 不建议每次上线都 `ssh` 到服务器手工改代码再重启。
- 不建议生产长期追 `upstream latest`。
- 最合适的思路是：`你自己的 fork + 你自己的镜像仓库 + 版本化发布 + 自动部署 + 明确回滚点`。

## 2. 重新给出的总体建议

如果把“更新频繁”和“后续扩容”这两个因素放到第一优先级，我建议分成两段来做，而不是一步到位上最重的架构。

### 方案结论

我推荐你采用下面这条路线：

1. 第一阶段：
   用 `Docker Compose + 自建镜像 + GitHub Actions 自动部署到生产服务器`。
2. 第二阶段：
   当客户数、并发和可用性要求明显上升后，再把应用层迁到 `K3s`，但数据库、Redis、镜像、分支和发布规范保持不变。

这比“现在立刻上完整 Kubernetes”更适合你当前仓库状态，也比“单机手工 `git pull` + `docker compose up -d`”稳定得多。

### 为什么我不建议你当前直接上最重的 K3s/GitOps

`new-api` 这个仓库现在最成熟、最直接的交付资产其实还是：

- 单个 [Dockerfile](../../Dockerfile)
- 单个 [docker-compose.yml](../../docker-compose.yml)
- 已有的镜像构建 workflow

也就是说，仓库天然更偏向“容器镜像发布”，而不是“现成 Helm Chart + 成熟 GitOps 模板 + 现成 K8s manifests”的状态。

所以最现实的做法不是跳过这一层，而是先把这层做规范：

- 你自己的私有 fork
- 你自己的 Registry
- 你自己的镜像 tag
- 你自己的生产发布 workflow
- 你自己的回滚机制

等这套链路跑顺后，再把部署目标从 `docker compose` 换成 `K3s`，迁移成本会低很多。

## 3. 我给你的正式推荐方案

### 3.1 推荐级别

### 推荐方案 A：现在就做，最务实

`GitHub 私有仓库 + GHCR + 单生产服务器 Docker Compose + GitHub Actions 自动发布`

这是我认为你当前最优的起步方案。

它解决的是这几个核心问题：

- 你本机改完代码以后，只需要 `git push`；
- 每次发布都能生成一个明确版本；
- 服务器不是手工改代码，而是拉取你自己的新镜像；
- 出问题可以按 tag 回滚；
- 后面从单机升级到 K3s 时，镜像和发布规范不用推倒重来。

### 推荐方案 B：中期升级路线

`GitHub 私有仓库 + GHCR + Hetzner/Scaleway 上的 K3s + 外部 PostgreSQL/Redis + GitOps`

这个方案不是现在第一天就必须做，但应该作为你 3 到 6 个月内的演进目标保留。

### 不推荐方案

以下方式不建议作为长期生产发布方案：

1. 直接追公共镜像 `calciumion/new-api:latest`
2. 线上服务器 `git pull` 你的开发分支后原地编译
3. 多台服务器分别手工更新
4. 生产环境长期使用 SQLite

原因都一样：版本不可控、回滚困难、多人或多节点环境容易漂移，也不利于扩容。

## 4. 推荐的发布链路

### 4.1 你应该维护哪些仓库与环境

建议至少拆成下面这几层：

1. `upstream`
   - 指向原始 `new-api` 仓库。
   - 用于你定期同步社区更新。
2. `origin`
   - 你的私有 fork 或你自己的公司仓库。
   - 所有客户可见版本都从这里发。
3. `staging`
   - 预发布环境。
   - 先看功能、配置、数据库迁移、渠道联通是否正常。
4. `production`
   - 面向客户的正式环境。

如果预算有限，第一阶段至少也要做到：

- 1 个正式仓库
- 1 台 staging 服务器
- 1 台 production 服务器

如果现在实在不想上两台服务器，至少也要保留两个 Compose 配置或两套 `.env`，不要把“测试发布”和“客户正式流量”混在一起。

### 4.2 推荐的 Git 分支与版本策略

我建议不要用“所有改动都直接推 main 然后上线”的方式。

建议采用下面这套最小规则：

1. `main`
   - 日常开发集成分支。
2. `release/prod`
   - 生产发布分支。
3. `vX.Y.Z` tag
   - 真正上线给客户的版本。

推荐流程：

1. 你在功能分支开发；
2. 合并到 `main`；
3. `main` 自动部署到 staging；
4. staging 验证通过后，在 `release/prod` 打 tag；
5. tag 触发生产镜像构建与生产部署。

这样做的好处是：

- staging 能提前看到变化；
- production 只吃稳定 tag；
- 任意一个客户问题都能追溯到具体版本。

### 4.3 推荐的镜像策略

当前仓库已经有镜像构建 workflow，但默认目标偏向公共镜像。你的 fork 里建议改成：

1. Registry 使用 `GHCR`
2. 镜像命名使用你自己的组织名
   - 例如：`ghcr.io/<your-org>/justapi`
3. 同时保留两类 tag：
   - 不可变 tag：`git-sha` 或 `vX.Y.Z`
   - 环境 tag：`staging`、`prod`

推荐规则：

- staging 部署使用 `staging-<short_sha>`
- production 部署使用 `vX.Y.Z`
- 不要让 production 永远盯着 `latest`

这样回滚时只需要把服务器上的镜像 tag 改回上一版，再执行一次部署。

## 5. 生产服务器应该怎么部署

### 5.1 第一阶段的推荐架构

如果你现在主要目标是“频繁更新、客户能看到、运维先不要过重”，我建议先上这个结构：

1. 1 台应用服务器
   - Docker Compose 跑你的 `justapi` 应用
   - 不直接跑源码
2. 1 个 PostgreSQL
   - 最好独立实例
   - 最差也要单独 volume 并做备份
3. 1 个 Redis
   - 独立服务
4. 1 个反向代理
   - Nginx 或 Caddy
5. 域名 + HTTPS

如果预算允许，我更建议：

1. staging 一台
2. production 一台
3. PostgreSQL 独立
4. Redis 独立

### 为什么第一阶段仍然推荐 Docker Compose

因为你当前仓库对 Compose 的适配度最高：

- [docker-compose.yml](../../docker-compose.yml) 已经给出服务编排基础；
- [Dockerfile](../../Dockerfile) 已经能完成前后端一体构建；
- [docker-compose.yml](../../docker-compose.yml) 里已经有健康检查；
- 文档里已经明确多机部署时要共享 `SESSION_SECRET`、`CRYPTO_SECRET`、DB、Redis。

对你而言，短期最应该补的不是“更复杂的编排系统”，而是“把更新动作规范化、自动化、可回滚化”。

### 5.2 服务器供应商建议

如果面向西欧客户，我建议这样选：

### 成本优先

`Hetzner`

适合你想先把成本压低，同时愿意自己管更多运维的情况。

### 平衡优先

`Scaleway`

如果你更在意后面平滑过渡到托管数据库、托管 Redis、托管 K8s，Scaleway 会比 Hetzner 更省心。

### 企业叙事优先

`OVHcloud`

如果你后面要面对欧洲企业客户采购、合规沟通或者希望更强调欧洲本地云，OVHcloud 更容易讲故事。

### 我对你当前阶段的推荐

如果你现在主要想先把“频繁发布 + 低成本”跑通，我仍然偏向：

`Hetzner 德国区 + Docker Compose 生产发布`

原因不是它最完美，而是：

- 成本更友好；
- 对西欧访问还可以；
- 更适合你先把发布链路跑顺。

## 6. 你应该如何把更新推送到服务器

### 6.1 不推荐的更新方式

不要采用下面这种生产习惯：

```bash
ssh server
cd /srv/justapi
git pull
docker compose up -d --build
```

这个方式的问题是：

1. 服务器变成了构建机；
2. 线上构建耗时长且不稳定；
3. 很难知道当前线上到底是哪一个 commit；
4. 回滚时很麻烦；
5. 多台服务器时几乎一定出配置漂移。

### 6.2 推荐的更新方式

推荐发布链路如下：

```text
本机开发
-> push 到 GitHub 私有仓库
-> GitHub Actions 跑测试与构建
-> 构建镜像并推送到 GHCR
-> GitHub Actions 通过 SSH 在服务器执行部署脚本
-> 服务器拉取新镜像并重启容器
-> 健康检查通过后对客户生效
```

服务器上实际执行的动作应该是“拉镜像”，不是“拉源码”。

### 6.2.1 把这条链路逐步展开

上面那 6 个箭头，如果真正落地到生产，可以理解为一条固定的发布流水线。

建议你按下面的方式理解每一步：

1. 开发者在本机完成改动并提交
2. 代码进入 GitHub 私有仓库
3. GitHub Actions 先验证代码能不能发布
4. 验证通过后构建镜像并推送到 GHCR
5. GitHub Actions 连接生产服务器执行部署脚本
6. 服务器只拉取新镜像并重启容器
7. 健康检查通过后，本次发布完成
8. 如果任一步失败，流水线停止，不继续往下

这条链路最重要的原则是：

- 本机负责开发和提交
- GitHub Actions 负责测试、构建、推送和触发部署
- 服务器只负责运行“已经构建好的镜像”

### 6.2.2 第 1 步：push 到 GitHub 私有仓库

这一阶段发生在你的本地电脑。

#### 你在本地做什么

1. 修改代码
2. 本地验证
3. 提交 commit
4. push 到 GitHub 私有仓库

建议至少保持下面这个最小规范：

1. 功能开发走分支
   - 例如：`feature/sidebar-cleanup`
2. 日常集成进 `main`
3. 生产发布不直接靠普通 push，而是靠 tag
   - 例如：`v1.2.3`

#### 这一步的输入和输出

- 输入：
  - 你本地的源码改动
- 输出：
  - GitHub 私有仓库中的新 commit 或新 tag

#### 这一步失败时怎么办

如果 push 都还没成功，就不要进入后面的发布链路。先在本地解决：

- merge 冲突
- 测试失败
- 构建失败
- 版本号错误

### 6.2.3 第 2 步：GitHub Actions 跑测试与构建

这一阶段发生在 GitHub Actions Runner 上。

它的目标不是“发布”，而是先回答一个问题：

`这次提交有没有资格进入生产发布流程。`

#### 建议 CI 至少做哪些事情

1. `checkout` 代码
2. 安装 Bun
3. 安装 Go
4. 跑前端构建
5. 跑后端测试或至少跑后端编译
6. 可选：做一次镜像 build 验证，但这一步可以不 push

#### 最小建议

对于你当前仓库，CI 最低限度建议包含：

```text
go test ./...
bun run build
docker build .
```

如果 `go test ./...` 成本太高，过渡阶段也可以先用：

```text
go build ./...
bun run build
docker build .
```

#### 这一步的输入和输出

- 输入：
  - GitHub 仓库中的 commit
- 输出：
  - 一次通过的 CI 记录
  - 或一次失败的 CI 记录

#### 这一步失败时怎么办

只要 CI 失败，就不要继续推镜像，更不要部署到服务器。

这一步失败通常意味着：

1. 代码编译不过
2. 测试没过
3. Dockerfile 已经不能正确打包

### 6.2.4 第 3 步：构建镜像并推送到 GHCR

这一阶段仍然发生在 GitHub Actions Runner 上。

#### 为什么这里推荐 GHCR

因为你的代码已经在 GitHub 上，GHCR 和 Actions 的配合最短：

1. 权限模型简单
2. 不必额外引入第三方镜像仓库
3. tag、commit、release 都容易绑定

#### 推荐的镜像命名方式

建议至少保留两类 tag：

1. 不可变版本 tag
   - 例如：`ghcr.io/<your-org>/justapi:v1.2.3`
2. 环境 tag
   - 例如：`ghcr.io/<your-org>/justapi:prod`

更稳妥的方式是再加一层 commit tag：

1. `ghcr.io/<your-org>/justapi:sha-abc1234`
2. `ghcr.io/<your-org>/justapi:v1.2.3`
3. `ghcr.io/<your-org>/justapi:prod`

#### 建议真正上线时优先用哪个 tag

生产服务器最好优先拉：

- `vX.Y.Z`
  或
- `sha-<short_commit>`

而不是长期拉 `latest`。

#### 这一步的输入和输出

- 输入：
  - 已通过 CI 的源码
- 输出：
  - GHCR 里的一份可拉取镜像
  - 最好还能拿到镜像 digest

#### 这一步失败时怎么办

如果镜像 push 失败，就不要继续 SSH 部署。

常见失败原因是：

1. GHCR 登录失败
2. 包权限不对
3. 镜像 tag 命名冲突
4. Dockerfile 中途失败

### 6.2.5 第 4 步：GitHub Actions 通过 SSH 在服务器执行部署脚本

这一阶段开始从 GitHub 进入你的生产服务器。

#### 这里不建议怎么做

不建议在 GitHub Actions 里直接写很长一串远程命令，例如：

```bash
ssh server "cd /srv/justapi && docker login ... && docker compose pull && docker compose up -d"
```

短期可以跑，但后面会越来越难维护。

#### 更推荐的方式

把服务器端动作收敛到一个固定脚本里，例如：

```text
/srv/justapi/deploy.sh
```

GitHub Actions 只负责调用它，例如：

```bash
ssh deploy@your-server "/srv/justapi/deploy.sh v1.2.3"
```

#### 为什么要这样做

这样做有三个直接好处：

1. GitHub Actions 配置更短
2. 服务器逻辑更集中，排查更容易
3. 回滚时也能复用同一个脚本

#### 这一步的输入和输出

- 输入：
  - GitHub Actions 传入的版本号
  - 例如：`v1.2.3`
- 输出：
  - 生产服务器开始执行部署动作

#### 这一步需要哪些 GitHub Secrets

至少要有：

1. `SSH_HOST`
2. `SSH_PORT`
3. `SSH_USER`
4. `SSH_PRIVATE_KEY`

如果服务器还需要自己登录 GHCR，通常还需要：

1. `GHCR_USERNAME`
2. `GHCR_TOKEN`

### 6.2.6 第 5 步：服务器拉取新镜像并重启容器

这一阶段发生在你的生产服务器上。

服务器上建议固定一个部署目录：

```text
/srv/justapi/
  docker-compose.yml
  .env
  deploy.sh
```

#### `deploy.sh` 应该做什么

它应该尽量只做下面这些确定动作：

1. 接收一个版本参数
   - 例如：`v1.2.3`
2. 登录 GHCR
3. 把镜像 tag 写入环境变量或 `.env`
4. 执行 `docker compose pull`
5. 执行 `docker compose up -d`
6. 触发健康检查

#### 一个推荐的执行顺序

```text
接收版本参数
-> 校验参数不为空
-> 登录 GHCR
-> 更新 IMAGE_TAG
-> docker compose pull
-> docker compose up -d --remove-orphans
-> 等待容器稳定
-> 检查 /api/status
```

#### 这里为什么不用 `--build`

因为生产服务器不应该再构建镜像。

生产服务器的职责应该只有：

1. 拉取镜像
2. 启动镜像
3. 验证镜像是否健康

#### 这一步的输入和输出

- 输入：
  - 版本号
  - 服务器上的 `.env`
  - 服务器上的 `docker-compose.yml`
- 输出：
  - 新版本容器被拉起

### 6.2.7 第 6 步：健康检查通过后对客户生效

这一阶段是发布是否真正完成的判定点。

#### 健康检查至少做什么

最少建议检查：

1. 容器是否存活
2. 应用端口是否监听
3. `GET /api/status` 是否返回成功

如果你后面有域名和反向代理，再加一层外部检查：

1. `https://your-domain/api/status`

如果你想更稳，还可以加一层简单业务冒烟检查：

1. 登录页是否可访问
2. 控制台首页是否能打开
3. 一条测试 token 是否能成功请求一个便宜模型

#### 为什么这里说“客户生效”

因为到了这一步：

1. 新容器已经起来
2. 新前端静态资源已经由新镜像提供
3. 后端逻辑也已经切换到新版本

也就是说，客户此时再访问，就会看到新功能。

### 6.2.8 失败停在哪一步，回滚怎么做

这条流水线不是“每次都一路冲到底”，而应该是“哪一步失败就停在哪一步”。

建议规则如下：

1. CI 失败
   - 停在 GitHub Actions
   - 不推镜像
   - 不部署服务器
2. 镜像 push 失败
   - 停在 GHCR 之前
   - 不 SSH 到服务器
3. SSH 部署失败
   - 停在服务器部署阶段
   - 保持当前线上版本不动
4. 健康检查失败
   - 立即回滚到上一个 tag

#### 最简单可用的回滚方案

只要你始终使用明确版本 tag，例如：

- `v1.2.2`
- `v1.2.3`

那么回滚动作其实很简单：

1. 重新执行一次部署脚本
2. 把参数从 `v1.2.3` 改回 `v1.2.2`

也就是：

```bash
/srv/justapi/deploy.sh v1.2.2
```

所以真正重要的不是“如何回滚”，而是：

- 你必须保留历史镜像 tag
- 你必须不要只用 `latest`

### 6.2.9 你最少要准备哪些东西

如果你想把这条链路真正跑起来，最少要准备这些：

#### GitHub 侧

1. 私有仓库
2. GitHub Actions workflow
3. GHCR 包权限
4. 仓库 Secrets

#### 服务器侧

1. 一台生产服务器
2. Docker 或 Podman 兼容运行时
3. `docker-compose.yml`
4. `.env`
5. `deploy.sh`
6. Nginx 或 Caddy

#### 配置侧

1. `SESSION_SECRET`
2. `CRYPTO_SECRET`
3. `SQL_DSN`
4. `REDIS_CONN_STRING`
5. 反代域名和 HTTPS

### 6.2.10 你应该如何理解整条链路

把这条链路翻成一句话就是：

`你只负责 push 代码；GitHub 负责验证、打包、发镜像、远程触发部署；服务器只负责拉镜像和运行；健康检查通过后客户看到新版本。`

如果你后面继续推进，我建议下一步就按这条链路补 3 个真正能落地的文件：

1. `docker-compose.prod.yml`
2. `deploy.sh`
3. `.github/workflows/cd-prod.yml`

### 6.3 生产部署脚本应该做什么

建议在服务器上准备一个稳定的部署目录，例如：

```text
/srv/justapi/
  docker-compose.yml
  .env
  deploy.sh
```

其中：

1. `docker-compose.yml`
   - 只引用你自己的镜像，例如 `ghcr.io/<your-org>/justapi:v1.2.3`
2. `.env`
   - 存放生产环境密钥
3. `deploy.sh`
   - 只做固定动作：
     - `docker login ghcr.io`
     - `docker compose pull`
     - `docker compose up -d`
     - 检查 `/api/status`

这一步的核心思想是：

- 服务器只消费“已经构建好的产物”；
- 构建和测试都发生在 CI；
- 服务器只负责运行。

### 6.4 GitHub Actions 应该怎么拆

我建议至少拆成两个 workflow：

### Workflow 1：CI

触发时机：

- push 到 `main`
- pull request 到 `main`

职责：

1. 跑 Go 测试
2. 跑前端构建
3. 构建镜像但不推 production tag

### Workflow 2：CD

触发时机：

- push `vX.Y.Z` tag
- 或手工 `workflow_dispatch`

职责：

1. 登录 GHCR
2. 构建并推送版本镜像
3. 通过 SSH 调用 production 服务器的 `deploy.sh`
4. 发布后检查健康状态

### 仓库里已有内容怎么复用

当前仓库已有：

- [docker-image-alpha.yml](../../.github/workflows/docker-image-alpha.yml)
- [docker-image-arm64.yml](../../.github/workflows/docker-image-arm64.yml)
- [release.yml](../../.github/workflows/release.yml)

建议你在自己的 fork 里：

1. 保留镜像构建思路；
2. 把目标镜像仓库从公共仓库改成你自己的 GHCR；
3. 新增一个生产部署 workflow；
4. 把生产发布绑定到 tag，而不是普通 push。

## 7. 客户如何看到新变化

客户是否能“看到新变化”，本质上取决于三件事：

1. 新镜像是否已经部署到 production；
2. 数据库迁移是否已经完成；
3. 前端静态资源缓存是否正确刷新。

### 7.1 前端变化

这个仓库前端使用 Vite 构建，默认会生成带 hash 的静态资源文件名。只要完成新镜像部署，浏览器通常会自动请求新的静态资源，不需要你手工改文件名。

如果你后面在前面加了 CDN，再额外考虑缓存刷新策略即可。

### 7.2 后端变化

后端变化在新容器起来后就会生效，但要特别注意：

- 新版本是否引入新的环境变量；
- 新版本是否引入新的数据迁移；
- 新版本是否需要新的上游渠道配置。

所以发布前建议在 staging 先验证：

1. 登录正常
2. 用户额度正常
3. 令牌调用正常
4. 关键模型路由正常
5. 你新增的功能页面可正常访问

## 8. 扩容应该怎么留后路

### 8.1 第一阶段先把什么设计对

即使你现在只上一台生产机，也建议从第一天就遵守这些规则：

1. 数据不要用 SQLite
2. `SESSION_SECRET` 固定且安全保存
3. `CRYPTO_SECRET` 固定且安全保存
4. DB 和 Redis 尽量独立于应用
5. 生产配置不要写死在镜像里
6. 发布永远通过镜像 tag，不通过手工源码构建

这些规则一旦做对，后面从单机升级到多机时会轻松很多。

### 8.2 什么时候升级到 K3s

当你出现下面任意两条时，就应该认真考虑升级到 `K3s`：

1. 需要两个以上应用节点
2. 需要无感滚动更新
3. 单机更新窗口已经影响客户
4. 需要更明确的资源配额和副本管理
5. 需要自动恢复与更好的可观测性

### 升级时什么不需要变

如果你按这份文档做，未来升级到 K3s 时，下面这些原则都不需要变：

1. 代码仍然在你自己的 fork
2. 镜像仍然发到你自己的 GHCR
3. 版本仍然用 tag
4. staging / production 分离仍然保留
5. DB 与 Redis 仍然作为共享依赖

变化的只是部署目标从：

`docker compose pull && docker compose up -d`

变成：

`helm upgrade` 或 GitOps 同步。

## 9. 风险与强制提醒

### 9.1 不要追 upstream latest 直接上线

你这个仓库后续会有自己的品牌、自己的功能、自己的配置差异。直接跟随公共镜像或公共 release，会导致：

- 你的改动被覆盖；
- 回滚点不清楚；
- 客户问题难以追责；
- 配置兼容风险上升。

### 9.2 AGPL 需要你自己确认

当前项目许可证是 AGPLv3。你要面向客户提供服务，这件事不能只从技术角度看，必须自己确认许可证义务和公司法务边界。

### 9.3 生产一定要做备份

至少要备份：

1. PostgreSQL
2. Redis 中需要保留的数据
3. 生产 `.env`
4. 反向代理配置

否则你上线再频繁，出一次事故就会把前面的发布效率全部抵消。

## 10. 我给你的最终落地建议

如果你让我现在直接拍板，我会建议你这样做：

### 第一步：立刻执行

1. 把仓库固定为你的私有 fork
2. 把镜像发布目标改成你自己的 GHCR
3. 建一台 staging 服务器
4. 建一台 production 服务器
5. 生产用 `Docker Compose + PostgreSQL + Redis + HTTPS`
6. 新增一个 GitHub Actions 生产部署 workflow

### 第二步：形成发布规范

1. 平时开发合并到 `main`
2. `main` 自动部署 staging
3. 需要给客户可见时打 `vX.Y.Z`
4. tag 自动触发 production 部署
5. 保留上一版镜像 tag 用于回滚

### 第三步：客户增长后升级

1. 应用层迁到 `K3s`
2. 数据库与 Redis 继续外置
3. 发布链路保持不变
4. 再引入 GitOps

## 11. 一句话结论

对“这个 repo 会经常更新，而且客户要尽快看到新变化”这个目标，当前最优解不是“服务器上直接 `git pull`”，也不是“立刻上完整重型 K8s”，而是：

`你自己的 fork + 你自己的 GHCR 镜像 + tag 发布 + GitHub Actions 自动部署到 Docker Compose 生产机`

这条路线最符合当前仓库现状，也最容易在后续平滑升级到 `K3s` 扩容。
