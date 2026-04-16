# Weekly Usage Log CSV Export Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
>
> 本计划已确认采用方案 B：新增后端 CSV 导出接口，并将 `request_path` 提升为独立字段。
> 本计划遵循当前仓库既有文件布局，不使用 `task-flow` 或 `ads-python-workflow` 的文件组织方式。

**Goal:** 为普通用户的使用日志页面增加“按周导出 CSV”能力，并且历史日志与新增日志都必须支持按 `request_path` 精确筛选与导出。

**Architecture:** 基于现有 `/api/log/self` 查询链路新增 `/api/log/self/export`，后端按当前筛选条件流式返回 CSV。日志模型新增一等字段 `request_path`，新日志写入时同步落列；历史日志通过应用层批处理回填 `other.request_path -> request_path`，避免依赖数据库 JSON 方言，从而保证 SQLite / MySQL / PostgreSQL 三库都能做精确筛选与导出。

**Tech Stack:** Go + Gin + GORM, React 18 + Semi UI + Vite, Bun, i18next, SQLite/MySQL/PostgreSQL.

---

## 1. 已确认范围

- 目标用户：普通用户，非管理员。
- 入口页面：`/console/log`。
- 首版交付：
  - 使用日志页新增 `request_path` 筛选；
  - 使用日志页新增“导出 CSV”按钮；
  - 导出当前筛选条件对应的“当前登录用户日志”；
  - 历史日志与新增日志都支持按 `request_path` 精确筛选；
  - 历史日志与新增日志都支持按 `request_path` 导出；
  - 日期快捷筛选继续复用现有 `DatePicker` 预设；
  - 保留已有“本周”，补“上周”。
- 首版不做：
  - 不修改权限模型；
  - 不让非管理员导出其他用户日志；
  - 不做异步导出任务；
  - 不新增独立导出页面。

## 2. 当前现状与关键判断

### 2.1 现有基础

- 前端日志页结构已经完整分层：
  - 页面入口：`web/src/pages/Log/index.jsx`
  - 页面装配：`web/src/components/table/usage-logs/index.jsx`
  - 筛选组件：`web/src/components/table/usage-logs/UsageLogsFilters.jsx`
  - 顶部动作区：`web/src/components/table/usage-logs/UsageLogsActions.jsx`
  - 数据逻辑：`web/src/hooks/usage-logs/useUsageLogsData.jsx`
- 后端用户日志接口已经存在：
  - `GET /api/log/self`
  - `GET /api/log/self/stat`
- 前端日期预设已经有“本周”，定义在 `web/src/constants/console.constants.js`。
- 仓库已经在多个写日志链路中采集 `request_path` 到 `other`：
  - `service/log_info_generate.go`
  - `service/task_billing.go`
  - `controller/relay.go`

### 2.2 当前缺口

- 没有用户级 CSV 导出接口；
- 没有 `request_path` 筛选输入；
- `request_path` 只存在于 `other` JSON 文本中，不是独立列，不能作为可靠的跨库筛选条件；
- 历史日志没有完成结构化回填，因此不能满足“历史日志也必须精确筛选”的要求。

### 2.3 结论

- 方案 B 不只是“新增导出接口”，还必须包含“历史数据结构化回填”；
- 历史回填不是附加项，而是主线交付的一部分；
- 功能完成标准不是“新日志可用”，而是“回填完成并验证后，历史日志与新增日志统一可用”。

## 3. 已确认设计

### 3.1 数据模型设计

- 在 `model.Log` 中新增：

```go
RequestPath string `json:"request_path" gorm:"index;default:''"`
```

- 保持 `other["request_path"]` 不移除：
  - 旧展开视图仍可读取；
  - 新逻辑将其视为历史兼容来源，而不是主查询字段。
- 写入策略：
  - `RecordConsumeLog`：优先 `params.RequestPath`，回退 `params.Other["request_path"]`
  - `RecordErrorLog`：优先 `c.Request.URL.Path`，回退 `other["request_path"]`
  - `RecordTaskBillingLog`：新增 `RequestPath` 参数，优先显式传入，否则从 `other["request_path"]` 回退

### 3.2 查询与过滤设计

- 新增统一过滤器结构，建议放在 `model/log.go`：

```go
type LogFilter struct {
	UserID         *int
	LogType        int
	StartTimestamp int64
	EndTimestamp   int64
	ModelName      string
	Username       string
	TokenName      string
	Group          string
	RequestID      string
	RequestPath    string
	ChannelID      int
}
```

- 新增统一查询构造函数，例如：

```go
func applyLogFilters(tx *gorm.DB, filters LogFilter) (*gorm.DB, error)
```

- 复用范围：
  - `GetUserLogs`
  - `GetAllLogs`
  - CSV 导出查询
- `request_path` 首版只做精确匹配，统一使用 `=`。

### 3.3 导出接口设计

- 新增用户接口：

```text
GET /api/log/self/export
```

- 鉴权：
  - `middleware.UserAuth()`
- 参数复用当前日志页查询参数：
  - `type`
  - `token_name`
  - `model_name`
  - `start_timestamp`
  - `end_timestamp`
  - `group`
  - `request_id`
  - `request_path`
- 返回头：

```text
Content-Type: text/csv; charset=utf-8
Content-Disposition: attachment; filename="usage-logs-YYYY-MM-DD.csv"
```

- CSV 列顺序固定为：
  - `used_at`
  - `username`
  - `token_name`
  - `model_name`
  - `request_path`
  - `quota`
  - `prompt_tokens`
  - `completion_tokens`
  - `ip`
  - `request_id`
  - `group`
  - `log_type`

### 3.4 历史日志回填设计

- 新增专用回填模块，建议文件：
  - `model/log_request_path_backfill.go`
- 回填原则：
  - 只使用普通列查询与应用层 JSON 解析；
  - 不使用 MySQL JSON_EXTRACT / PostgreSQL JSONB / SQLite JSON1 作为主方案。
- 回填流程建议：
  1. 查找 `request_path = ''` 且 `other != ''` 的日志，按 `id` 升序分批读取；
  2. 使用 `common.UnmarshalJsonStr` 或 `common.StrToMap` 解析 `other`；
  3. 若包含非空 `request_path`，则更新对应日志行的 `request_path` 列；
  4. 记录批次统计：扫描数、更新数、缺失数、失败数；
  5. 循环直到没有待处理行。
- 回填必须具备：
  - **幂等性**：重复执行不会产生错误结果；
  - **分批处理**：避免大表全量加载；
  - **可恢复性**：进程中断后可继续执行；
  - **可观测性**：能看到回填进度和最终覆盖率。
- 状态持久化建议：
  - 使用 `Option` 表记录回填状态，例如：
    - `LogRequestPathBackfillStatus`
    - `LogRequestPathBackfillCursor`
    - `LogRequestPathBackfillScanned`
    - `LogRequestPathBackfillUpdated`
    - `LogRequestPathBackfillMissing`
    - `LogRequestPathBackfillFinishedAt`
- 交付要求：
  - 回填执行完成后，必须输出覆盖率审计结果；
  - 若仍有无法回填的历史日志，必须给出明确数量和原因；
  - 在覆盖率未达标前，不视为功能完成。

### 3.5 上线与交付标准

- 新日志写入 `request_path` 列；
- 历史日志完成回填；
- `request_path` 精确筛选对历史日志和新日志都可用；
- CSV 导出对历史日志和新日志都可用；
- 回填覆盖率有明确验证记录；
- 前端展示的筛选与导出功能只在回填验证通过后上线。

## 4. 实施计划

### Task 1: 固化验收标准与覆盖率要求

**Files:**
- Modify: `task_plan.md`
- Review: `model/log.go`
- Review: `model/option.go`
- Review: `controller/log.go`

**Step 1: 固化首版边界**

- 仍然只支持“普通用户导出自己的日志”；
- `request_path` 为精确匹配。

**Step 2: 固化功能完成标准**

- 新日志可用不算完成；
- 历史日志回填完成并验证通过才算完成。

**Step 3: 固化覆盖率要求**

- 需要有回填统计与审计输出；
- 需要有“剩余缺失数据”的可见结果。

### Task 2: 先写失败测试，覆盖筛选、导出和回填

**Files:**
- Create: `controller/log_export_test.go`
- Create: `model/log_filter_test.go`
- Create: `model/log_request_path_backfill_test.go`
- Review: `controller/token_test.go`
- Review: `model/task_cas_test.go`

**Step 1: 写权限失败测试**

- 未登录访问 `/api/log/self/export` 应失败；
- 登录用户只能导出自己的日志。

**Step 2: 写筛选失败测试**

- `token_name` / `model_name` / `group` / `request_id` / `request_path` / 时间范围都应生效；
- `request_path` 必须按精确匹配处理。

**Step 3: 写回填失败测试**

- 构造历史日志：`request_path` 为空但 `other` 内含 `request_path`；
- 运行回填后，列值应被补齐；
- 重复运行回填结果应保持不变；
- 对 `other` 中没有路径的历史日志，需计入缺失统计。

**Step 4: 写 CSV 输出失败测试**

- 表头顺序固定；
- `request_path` 应落在独立列；
- 回填后的历史日志也能被导出到正确列。

**Step 5: 跑最小测试集**

Run:

```powershell
go test ./controller ./model -run "Log|Export|Filter|Backfill"
```

Expected:

```text
FAIL
```

### Task 3: 扩展日志模型并打通新日志写入

**Files:**
- Modify: `model/log.go`
- Review: `model/main.go`

**Step 1: 给 `Log` 增加 `RequestPath` 字段**

- 依赖现有 `AutoMigrate(&Log{})` 完成列新增；
- 确认主库和独立日志库都会执行迁移。

**Step 2: 扩展日志写入参数**

- `RecordConsumeLogParams` 增加 `RequestPath string`
- `RecordTaskBillingLogParams` 增加 `RequestPath string`

**Step 3: 将已有采集同步到新列**

- `RecordConsumeLog`：优先 `params.RequestPath`，回退 `params.Other["request_path"]`
- `RecordErrorLog`：优先 `c.Request.URL.Path`，回退 `other["request_path"]`
- `RecordTaskBillingLog`：优先 `params.RequestPath`，回退 `params.Other["request_path"]`

**Step 4: 运行模型测试**

Run:

```powershell
go test ./model -run "Log|Filter|Backfill"
```

Expected:

```text
部分测试仍 FAIL，因为回填和导出接口尚未实现
```

### Task 4: 实现统一过滤器

**Files:**
- Modify: `model/log.go`
- Review: `model/token.go`

**Step 1: 新增 `LogFilter`**

- 把查询条件统一收口。

**Step 2: 新增 `applyLogFilters(...)`**

- `GetUserLogs` / `GetAllLogs` / 导出查询共用同一套条件。

**Step 3: 使用普通列过滤 `request_path`**

- 统一走 `logs.request_path = ?`
- 不依赖 JSON 字段查询。

**Step 4: 运行过滤器测试**

Run:

```powershell
go test ./model -run "Log|Filter"
```

Expected:

```text
PASS
```

### Task 5: 实现历史日志回填与覆盖率审计

**Files:**
- Create: `model/log_request_path_backfill.go`
- Modify: `model/log.go`
- Modify: `model/option.go` 或复用现有 `UpdateOption`
- Test: `model/log_request_path_backfill_test.go`

**Step 1: 实现批处理回填函数**

- 推荐函数：

```go
func BackfillLogRequestPath(batchSize int) (BackfillResult, error)
```

**Step 2: 分批扫描待回填日志**

- 条件：`request_path = ''`
- 排序：`id asc`
- 每批处理固定数量。

**Step 3: 解析 `other` 并回填**

- 使用 `common.UnmarshalJsonStr` 或 `common.StrToMap`
- 只提取 `request_path`
- 对可提取项更新列值。

**Step 4: 记录状态和统计**

- 使用 `Option` 表记录状态、游标和统计；
- 支持中断恢复；
- 支持最终审计。

**Step 5: 定义缺失数据处理规则**

- 若历史日志 `other` 中不存在 `request_path`：
  - 计入 `missing`；
  - 审计报告中必须体现；
  - 该结果会影响是否允许上线。

**Step 6: 跑回填测试**

Run:

```powershell
go test ./model -run "Backfill"
```

Expected:

```text
PASS
```

### Task 6: 新增 CSV 导出接口

**Files:**
- Modify: `controller/log.go`
- Modify: `router/api-router.go`
- Optionally Create: `service/log_export.go`
- Test: `controller/log_export_test.go`

**Step 1: 新增 handler**

- 推荐命名：`ExportUserLogsCSV`

**Step 2: 解析并复用统一筛选参数**

- 与 `GetUserLogs` 使用同一套参数语义；
- 导出不接受分页参数。

**Step 3: 流式写出 CSV**

- 使用 Go 标准库 `encoding/csv`
- 按行写入响应；
- 避免一次性拼接大字符串。

**Step 4: 校验历史日志导出**

- 使用已经回填的历史日志数据验证；
- 确认 `request_path` 能正确出现在 CSV 中。

**Step 5: 跑控制器测试**

Run:

```powershell
go test ./controller -run "Log|Export"
```

Expected:

```text
PASS
```

### Task 7: 接前端筛选与导出按钮

**Files:**
- Modify: `web/src/hooks/usage-logs/useUsageLogsData.jsx`
- Modify: `web/src/components/table/usage-logs/UsageLogsFilters.jsx`
- Modify: `web/src/components/table/usage-logs/UsageLogsActions.jsx`
- Modify: `web/src/constants/console.constants.js`
- Review: `web/src/helpers/utils.jsx`

**Step 1: 抽取统一查询参数构造函数**

- 列表、统计、导出共用同一套 query 参数。

**Step 2: 增加 `request_path` 筛选**

- 作为普通文本输入框加入日志页筛选表单。

**Step 3: 增加导出按钮**

- 放在 `UsageLogsActions.jsx`
- 点击后发起下载；
- 导出中禁用重复点击。

**Step 4: 周范围体验补齐**

- 保留“本周”；
- 新增“上周”。

**Step 5: 前端验证**

Run:

```powershell
cd web
bun run build
```

Expected:

```text
vite build succeeds
```

### Task 8: i18n、联调、回填执行与上线验证

**Files:**
- Modify: `web/src/i18n/locales/zh-CN.json`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/vi.json`
- Modify: `web/src/i18n/locales/zh-TW.json`

**Step 1: 补文案**

- `导出 CSV`
- `导出中`
- `导出成功`
- `请求路径`
- `上周`

**Step 2: 执行历史回填**

- 在目标环境运行回填；
- 记录最终统计结果。

**Step 3: 验证覆盖率与缺失数**

- 确认历史日志回填后的 `request_path` 覆盖率；
- 若仍有 `missing`，必须评估是否阻塞上线。

**Step 4: 联调**

- 使用历史日志做 `request_path` 精确筛选；
- 使用历史日志做 CSV 导出；
- 使用新日志做同样验证；
- 打开 CSV 检查字段、编码和时间格式。

**Step 5: 全量验证**

Run:

```powershell
go test ./...
cd web
bun run build
```

Expected:

```text
backend tests pass
frontend build succeeds
```

## 5. 关键风险

- 最大风险不是 CSV，而是历史日志里是否真的都保留了 `other.request_path`。
- 如果最早的一批历史日志在采集阶段就没有 `request_path`，那是历史数据缺失问题，不是迁移问题；计划已要求必须输出缺失统计并据此决定是否阻塞上线。
- 回填若做成全表一次性处理，容易拖垮大表；因此必须分批、可恢复、可审计。
- `RecordTaskBillingLog` 当前没有显式 `RequestPath` 参数，不补齐会让异步任务日志继续产生空洞。
- CSV 中文编码与 Excel 兼容性要实测。

## 6. 执行顺序建议

1. 先写测试，把“历史日志也必须支持”写进失败用例。
2. 再补 `Log.RequestPath` 与新日志写入。
3. 然后实现统一过滤器。
4. 再实现历史日志回填与覆盖率审计。
5. 然后实现 CSV 导出接口。
6. 最后接前端筛选与导出按钮。
7. 回填执行并验证通过后再上线前端功能。

## 7. 当前状态

- 方案已确认。
- 历史日志支持已确认为硬要求。
- 当前计划中已无待确认阻塞项，可以进入实现阶段。

---

## 8. 新增需求：支持管理员或普通用户按 API 令牌配置每日或每月额度并立即拦截（已确认采用方案 B）

### 8.1 现状检查结论

- 项目**已经存在**订阅额度重置机制：
  - 数据模型：`model/subscription.go`
  - 周期枚举：`daily / weekly / monthly / custom / never`
  - 重置执行：`model.ResetDueSubscriptions(...)`
  - 后台任务：`service/subscription_reset_task.go`
  - 启动入口：`main.go` 中的 `service.StartSubscriptionQuotaResetTask()`
- 项目**已经存在**普通用户与管理员共用的 token 管理入口：
  - 前端页面：`/console/token`
  - 前端创建/编辑组件：`web/src/components/table/tokens/modals/EditTokenModal.jsx`
  - 后端接口：`/api/token/*`
  - 权限中间件：`middleware.UserAuth()`
- 项目**还不存在**按 API 令牌维度的周期额度限制：
  - `model.Token` 目前只有 `remain_quota / used_quota / unlimited_quota`
  - `model.ValidateUserToken(...)` 只校验状态、过期、总额度
  - `service.PreConsumeTokenQuota(...)` / `service.BillingSession` / `service/task_billing.go` 只处理总额度扣减与返还，不处理日/月额度窗口
- 结论：
  - 可以复用“订阅周期重置”的设计经验；
  - 这个需求天然适合落在现有 token 管理页，而不是新建用户设置页；
  - 但 token 周期限额不能直接照搬订阅任务，否则会把“立即拦截”错误地依赖在定时任务上。

### 8.2 需求范围与首版边界

- 配置主体：`API Token`，并且**同一账户下的不同 token 可以独立配置不同的周期额度**
- 适用角色：**令牌拥有者**，包括普通用户和管理员；两者都走同一套 token 管理页与 `/api/token/*` 接口
- 配置入口：**只放在 token 管理页的创建/编辑侧栏 `EditTokenModal` 中**，不新增用户设置页，不新增独立配置页面
- 新增配置项：
  - `quota_period`
    - `none`
    - `daily`
    - `monthly`
  - `quota_limit`
- 交互要求：
  - 普通用户或管理员在 token 管理页点击“添加令牌”进入创建态后，就能直接设置“日额度 / 月额度”
  - 普通用户或管理员在 token 管理页打开已有 token 的编辑态后，也能直接设置“日额度 / 月额度”
  - 首版验收以“新建 token 可配置并保存成功，编辑已有 token 也可配置并保存成功”为准
- 限制语义：
  - 与当前 `remain_quota` 总额度并行存在；
  - 任一条件不满足都要拦截请求；
  - `UnlimitedQuota=true` 不应绕过周期额度限制
- 恢复语义：
  - `daily` 到次日窗口自动恢复；
  - `monthly` 到下月窗口自动恢复；
  - 恢复必须由请求链路懒触发完成，不能依赖后台任务恰好准点跑到
- 额度单位：
  - 首版仍沿用仓库现有“quota 单位”作为后端存储与结算单位；
  - 管理端文案明确它表示“美元等值额度”；
  - 不单独引入新的 money ledger 或独立汇率表。

### 8.3 已确认方案 B

- 采用 `token` 持久化周期窗口状态：
  - `quota_period`
  - `quota_limit`
  - 当前窗口已用额度
  - 上次重置时间
  - 下次重置时间
- 采用请求链路懒重置：
  - 请求到来时判断是否跨日 / 跨月
  - 若跨窗口则先滚动窗口，再做额度判定
- 采用同步拦截：
  - 在进入上游前完成预扣费检查
  - 超限立即拒绝，不依赖后台任务
- 采用同步结算与回滚：
  - 差额补扣
  - 差额返还
  - 异步任务退款 / 重算
  - 都要同步维护当前窗口已用额度
- 明确不采用的执行路线：
  - 不采用“只靠定时任务重置”的方案
  - 不采用“每次实时聚合日志表求和”的方案

### 8.4 已确认设计

#### 8.4.1 数据模型

- 在 `model.Token` 中新增字段：

```go
QuotaPeriod        string `json:"quota_period" gorm:"type:varchar(16);default:'none'"`
QuotaLimit         int    `json:"quota_limit" gorm:"default:0"`
QuotaUsedInPeriod  int    `json:"quota_used_in_period" gorm:"default:0"`
QuotaLastResetTime int64  `json:"quota_last_reset_time" gorm:"type:bigint;default:0"`
QuotaNextResetTime int64  `json:"quota_next_reset_time" gorm:"type:bigint;default:0;index"`
```

- 设计约束：
  - `quota_period=none` 或 `quota_limit<=0` 视为未启用周期额度
  - `QuotaUsedInPeriod` 仅记录当前窗口已占用额度
  - 不复用 `TokenStatusExhausted` 表示周期超限；否则新周期恢复会被状态机卡死

#### 8.4.2 模型辅助函数

- 建议新增文件：`model/token_period_quota.go`
- 建议新增辅助函数：
  - `NormalizeTokenQuotaPeriod(period string) string`
  - `CalcTokenQuotaNextResetTime(base time.Time, period string) int64`
  - `RefreshTokenQuotaWindow(token *Token, now time.Time) (changed bool)`
  - `CanConsumeTokenPeriodQuota(token *Token, quota int) error`
  - `ApplyTokenPeriodQuotaDelta(...)`
- 关键策略：
  - 每次校验或扣费前，先判断是否跨窗口；
  - 若已跨窗口，则先把 `QuotaUsedInPeriod` 清零并滚动 `QuotaLastResetTime / QuotaNextResetTime`；
  - 这个“懒重置”必须在同步请求链路中完成。

#### 8.4.3 一致性策略

- 周期限额启用后，不能继续完全依赖现有异步更新路径：
  - `BatchUpdateTypeTokenQuota` 会延迟 DB 写入，不满足“立即拦截”
  - Redis 目前只对 `RemainQuota` 做增减，不会同步 `QuotaUsedInPeriod`
- 首版建议：
  - **对启用了周期额度的 token，跳过 BatchUpdate 聚合路径，改为同步 DB 更新**
  - **对启用了周期额度的 token，命中 Redis 缓存后仍需回源 DB 获取最新窗口状态**
  - 成功写入后刷新整条 token 缓存，而不是只增减 `RemainQuota`
- 并发要求：
  - 周期额度的“检查 + 扣减/返还”必须放在同一条模型写路径内；
  - 不能把“先读后判定”和“后写入”散落在多个 service 层函数中。

#### 8.4.4 拦截与扣费链路

- 需要覆盖的主链路：
  - `middleware/auth.go` / `model.ValidateUserToken(...)`
  - `service.PreConsumeTokenQuota(...)`
  - `service.BillingSession.preConsume(...)`
  - `service.BillingSession.Settle(...)`
  - `service.PostConsumeQuota(...)`
  - `service/task_billing.go`
- 推荐行为：
  - `ValidateUserToken(...)`
    - 新增“当前窗口已耗尽”的快速拒绝；
    - 但最终是否允许本次请求继续，还要以预扣费阶段的精确额度判断为准。
  - `PreConsumeTokenQuota(...)`
    - 成为“进入上游前”的硬拦截点；
    - 若本次预扣费会让 `QuotaUsedInPeriod + preConsumedQuota > QuotaLimit`，直接拒绝。
  - `BillingSession.shouldTrust(...)`
    - 对启用了周期额度的 token 禁用信任额度旁路；
    - 否则会出现请求直接放行、结算时才发现超限的问题。
  - `BillingSession.Settle(...)` / `PostConsumeQuota(...)`
    - 差额补扣时同步增加 `QuotaUsedInPeriod`
    - 差额返还时同步减少 `QuotaUsedInPeriod`
  - `service/task_billing.go`
    - 异步任务预扣、退款、差额重算都必须同步调整 `QuotaUsedInPeriod`

#### 8.4.5 超限错误语义

- 返回要求：
  - 不继续转发到上游
  - 返回明确错误提示
  - 错误信息需要包含“当前 token 已达到每日/月度额度上限”以及“下次恢复时间”
- 建议补充后端 i18n 文案：
  - 中文
  - 英文

#### 8.4.6 管理端交互

- 首版前端配置入口：
  - `web/src/components/table/tokens/modals/EditTokenModal.jsx`
  - `web/src/components/table/tokens/index.jsx`
- 首版前端数据流：
  - `web/src/hooks/tokens/useTokensData.jsx`
- 首版前端只读展示：
  - `web/src/components/table/tokens/TokensColumnDefs.jsx`
- 首版交互：
  - `EditTokenModal` 同时覆盖“添加令牌”的创建态和“编辑令牌”的编辑态
  - **token 创建/编辑侧栏**新增：
    - 周期额度模式：未启用 / 每日 / 每月
    - 周期额度上限
  - token 列表增加只读展示：
    - 当前周期已用
    - 下次重置时间
- 不做：
  - 不新增单独 token 周期额度页面
  - 不在首版新增用户页的同类配置入口
  - 不支持在用户管理页跨账号配置其他用户的 token
  - 不把配置入口放到其他页面；配置动作以 token 管理页中的 `EditTokenModal` 为唯一入口

### 8.5 实施计划

### Task 9: 固化验收标准与建模边界

**Files:**
- Modify: `task_plan.md`
- Review: `model/token.go`
- Review: `service/quota.go`
- Review: `service/billing_session.go`
- Review: `service/task_billing.go`

**Step 1: 固化首版范围**

- 只支持 token 维度
- 只支持 `none / daily / monthly`
- 适用于普通用户与管理员管理自己的 token
- 只在 token 管理页的 `EditTokenModal` 提供配置入口，覆盖创建态与编辑态

**Step 2: 固化正确性要求**

- 拦截必须发生在进入上游前
- 新周期恢复不能依赖定时任务
- 退款与差额结算必须回滚周期已用额度

**Step 3: 固化非目标**

- 不引入新的订阅计划字段
- 不改用户总钱包/订阅结算口径
- 不做用户页批量配置入口
- 不做管理员在用户管理页跨账号配置他人 token 的入口

### Task 10: 先写失败测试，覆盖重置、拦截、结算与回滚

**Files:**
- Create: `model/token_period_quota_test.go`
- Modify: `controller/token_test.go`
- Modify: `service/task_billing_test.go`
- Create or Modify: `service/billing_session_test.go`

**Step 1: 写模型测试**

- `daily` 窗口滚动
- `monthly` 窗口滚动
- `none` 不启用
- 超限判定

**Step 2: 写 token CRUD 测试**

- 创建 token 时可保存 `quota_period / quota_limit`
- 更新 token 时可切换模式与重置窗口
- 同一套 token CRUD 对普通用户与管理员自有 token 都成立
- 非法值校验应失败

**Step 3: 写结算链路测试**

- 预扣费达到上限时立即拦截
- 退款后 `QuotaUsedInPeriod` 回退
- 差额补扣与差额返还都正确更新周期已用额度
- 启用周期额度后，trust bypass 不得跳过预扣

**Step 4: 写异步任务测试**

- `RefundTaskQuota(...)` 回滚周期已用额度
- `RecalculateTaskQuota(...)` 调整周期已用额度

**Step 5: 跑最小测试集确认失败**

Run:

```powershell
go test ./model ./controller ./service -run "Token.*Quota|BillingSession|TaskQuota"
```

### Task 11: 扩展 Token 模型、迁移与 CRUD

**Files:**
- Modify: `model/token.go`
- Modify: `model/main.go`
- Modify: `controller/token.go`
- Modify: `constant/cache_key.go`
- Modify: `model/token_cache.go`

**Step 1: 扩展模型字段**

- 为 `Token` 增加周期额度字段
- 为 `Update()` 的字段白名单补齐新字段

**Step 2: 迁移兼容三库**

- 确认新增列在 SQLite / MySQL / PostgreSQL 下可自动迁移
- 必要时在 `model/main.go` 增加显式补列逻辑

**Step 3: 扩展 CRUD 校验**

- `quota_period` 只能取允许值
- `quota_limit` 不能为负
- 关闭周期额度时应清理窗口状态

### Task 12: 实现 token 周期窗口辅助逻辑

**Files:**
- Create: `model/token_period_quota.go`
- Modify: `model/token.go`

**Step 1: 实现周期标准化与下次重置时间计算**

- 日级窗口按本地自然日
- 月级窗口按自然月

**Step 2: 实现懒重置**

- 请求到来时发现已跨窗口则重置 `QuotaUsedInPeriod`
- 同步更新 `QuotaLastResetTime / QuotaNextResetTime`

**Step 3: 实现统一的周期额度 delta 调整入口**

- 正向扣减
- 返还回滚
- 不允许 `QuotaUsedInPeriod` 变成负数

### Task 13: 接入同步拦截与结算链路

**Files:**
- Modify: `middleware/auth.go`
- Modify: `service/quota.go`
- Modify: `service/billing.go`
- Modify: `service/billing_session.go`
- Modify: `service/task_billing.go`

**Step 1: 接入快速拒绝**

- 对已经达到当前窗口上限的 token 返回明确错误

**Step 2: 接入预扣费硬拦截**

- 在 `PreConsumeTokenQuota(...)` 中引入周期额度检查
- 启用周期额度时关闭 trust bypass

**Step 3: 接入差额结算与退款**

- `Settle(...)` 与 `PostConsumeQuota(...)` 正确维护 `QuotaUsedInPeriod`
- `RefundTaskQuota(...)` / `RecalculateTaskQuota(...)` 正确维护 `QuotaUsedInPeriod`

**Step 4: 处理缓存与批量更新**

- 启用周期额度的 token 走同步写路径
- 变更后刷新完整 token 缓存

### Task 14: token 管理页创建/编辑表单与展示

**Files:**
- Modify: `web/src/components/table/tokens/modals/EditTokenModal.jsx`
- Modify: `web/src/hooks/tokens/useTokensData.jsx`
- Modify: `web/src/components/table/tokens/TokensColumnDefs.jsx`
- Modify: `web/src/components/table/tokens/TokensTable.jsx`

**Step 1: 表单新增字段**

- 在 `EditTokenModal` 的创建态新增周期额度模式
- 在 `EditTokenModal` 的创建态新增周期额度上限
- 在 `EditTokenModal` 的编辑态新增周期额度模式
- 在 `EditTokenModal` 的编辑态新增周期额度上限

**Step 2: 列表展示状态**

- 当前周期已用额度
- 下次重置时间

**Step 3: 提交与回填**

- 创建/更新 token 时传递新字段
- 编辑已有 token 时正确回显
- 创建新 token 时正确带入默认值并提交
- 验证在 token 管理页“添加令牌”和“编辑令牌”两条入口都可以完成 daily / monthly 配置

### Task 15: i18n、错误提示与全量验证

**Files:**
- Modify: `i18n/en.json`
- Modify: `i18n/zh.json`
- Modify: `web/src/i18n/locales/zh-CN.json`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/vi.json`
- Modify: `web/src/i18n/locales/zh-TW.json`

**Step 1: 补错误文案**

- 当前 token 已达到每日额度上限
- 当前 token 已达到月额度上限
- 下次恢复时间

**Step 2: 跑自动化验证**

Run:

```powershell
go test ./...
cd web
bun run build
```

**Step 3: 跑手工验收**

- 普通用户新建一个 `daily` token，验证当天达到上限后立即拦截
- 调整时间或测试窗口，验证次日恢复
- 管理员账号新建或编辑一个 `monthly` token，验证月级恢复
- 验证退款与异步任务差额结算不会把周期已用额度算错

### 8.6 关键风险

- 最大技术风险不是字段新增，而是**现有 trust bypass / BatchUpdate / Redis remain_quota 增量缓存**与“立即拦截”目标存在天然冲突。
- 如果不对启用周期额度的 token 走同步写路径，计划会在并发和缓存延迟下失真。
- 现有预扣费是基于 `QuotaToPreConsume` 的估算值；若某些链路实际结算值高于预扣值，需要额外审计是否会出现单次请求轻微超出窗口上限的情况。

### 8.7 已确认决策

- 已确认采用方案 B：在 token 上持久化周期窗口状态，按请求链路懒重置并同步拦截
- 已确认适用对象为令牌拥有者，普通用户和管理员都可以为自己的 token 配置周期额度
- 已确认配置入口只放在 token 管理页的 `EditTokenModal`，并覆盖“添加令牌”和“编辑令牌”两种状态
- 已确认首版只做 token 维度，不做用户维度
- 已确认首版继续沿用现有 quota 单位，前端以“美元等值额度”解释

---

## 9. 数据看板第 1 / 第 2 个 Tab 14 天按天展示（追加计划）

### 9.1 方案比较

- 方案 A：继续走当前共享查询链路，把 Dashboard 全部图表默认时间窗统一切到 14 天，并在前端按天重聚合。
  - 优点：实现路径最短。
  - 缺点：会误伤第 3、第 4 个 tab 以及搜索弹窗后的共享图表逻辑，范围过大。
- 方案 B：只为第 1、第 2 个 tab 新增专用“最近 14 天”数据加载和按天聚合逻辑，第 3、第 4 个 tab 继续沿用当前链路。
  - 优点：最贴合你的新范围，改动集中，能把“前后几个小时”的问题限定在这两个 tab 内解决。
  - 缺点：Dashboard 会多一条图表专用数据路径。
- 方案 C：新增后端“最近 14 天按天聚合”的日报接口，前端第 1、第 2 个 tab 直接消费日报数据。
  - 优点：接口语义清晰，后续可复用。
  - 缺点：首版改动面偏大；当前 `/api/data/self` 与 `/api/data` 已能提供足够原始数据，没有必要先扩后端接口。

### 9.2 推荐结论

- 推荐采用 **方案 B**。
- 本次范围调整为同时修改 Dashboard 的：
  - 第 1 个 tab：`消耗分布 / 模型消耗分布`
  - 第 2 个 tab：`消耗趋势 / 模型消耗趋势`
- 两个 tab 都固定展示“最近 14 天，按天统计”，不再显示小时级刻度。
- 第 1 个 tab 保持“按模型分组的日消耗分布”。
- 第 2 个 tab 调整为“按模型分组的日消耗趋势”。
- 第 3、第 4 个 tab：
  - `调用次数分布`
  - `调用次数排行`
  暂不改行为。

# 执行总览（Execution Overview）

- **目标**：将 Dashboard 的第 1、第 2 个 tab 一并改为最近 14 天按天展示，解决当前只显示前后几个小时的问题。
- **范围**：修改前端 Dashboard 的图表数据链路、图表装配逻辑与必要文案，复用现有 `/api/data/self`、`/api/data` 接口，不修改 `quota_data` 的小时级持久化粒度。
- **核心策略**：
  - 为第 1、第 2 个 tab 增加专用 14 天数据加载逻辑；
  - 使用现有小时级 `quota_data` 结果，在前端二次按天聚合；
  - 固定生成 14 个自然日时间点，缺失日期补零；
  - 将第 2 个 tab 从当前“标题写消耗趋势、实现却按 count 画线”的状态收敛为真正的日消耗趋势；
  - 第 3、第 4 个 tab 和搜索弹窗的共享逻辑保持不变。
- **预期结果**：
  - 第 1 个 tab 的 X 轴改为最近 14 天日期，按天展示模型消耗分布；
  - 第 2 个 tab 的 X 轴同样改为最近 14 天日期，按天展示模型消耗趋势；
  - 管理员和普通用户都能在各自 Dashboard 上看到连续 14 天的按天数据。

### Task 16: 固化需求边界与验收口径

**Files:**
- Modify: `task_plan.md`
- Review: `web/src/components/dashboard/ChartsPanel.jsx`
- Review: `web/src/hooks/dashboard/useDashboardData.js`
- Review: `web/src/hooks/dashboard/useDashboardCharts.jsx`
- Review: `web/src/helpers/dashboard.jsx`

**Step 1: 固化范围**

- 同时修改第 1 个和第 2 个图表页签。
- 两个 tab 都固定展示最近 14 天，不再显示小时级刻度。
- 单位保持为现有 quota/美元等值额度，不改统计口径。

**Step 2: 固化不改项**

- 不修改 `quota_data` 表的写入粒度，仍保持小时级缓存与落库。
- 不新增后端日报接口。
- 不调整 `调用次数分布`、`调用次数排行` 两个 tab 的行为。
- 不改变搜索弹窗现有交互作为首版前置条件。

**Step 3: 固化验收标准**

- 第 1、第 2 个 tab 的 X 轴都必须固定展示 14 个自然日。
- 每个自然日都必须出现，即使当天无数据也要显示 0。
- 第 1 个 tab 保持“按模型分组的堆叠分布”。
- 第 2 个 tab 必须改为真正的“按日消耗趋势”，不再沿用当前按 count 组装却命名为“消耗趋势”的实现。

### Task 17: 为第 1 / 第 2 个 Tab 增加专用最近 14 天数据加载链路

**Files:**
- Modify: `web/src/hooks/dashboard/useDashboardData.js`
- Review: `controller/usedata.go`
- Review: `router/api-router.go`

**Step 1: 新增图表专用时间窗**

- 在 Dashboard 数据层新增“最近 14 天”查询窗口生成逻辑：
  - `end_timestamp` 取当前时刻向后补 1 小时的现有风格，避免当天尾部数据遗漏；
  - `start_timestamp` 取最近 13 天的自然日起点，保证覆盖 14 个自然日。

**Step 2: 新增专用加载函数**

- 在 `useDashboardData.js` 中新增仅供第 1、第 2 个 tab 使用的数据请求函数，例如：
  - `loadConsumptionChartsQuotaData`
- 普通用户继续走：
  - `/api/data/self`
- 管理员继续走：
  - `/api/data/?username=...`

**Step 3: 保持共享链路不受影响**

- `loadQuotaData` 继续服务当前搜索弹窗和第 3、第 4 个 tab。
- Dashboard 初始化和顶部刷新按钮同时刷新：
  - 共享图表数据
  - 第 1、第 2 个 tab 的 14 天专用数据

### Task 18: 新增“最近 14 天按天聚合”前端辅助函数

**Files:**
- Modify: `web/src/helpers/dashboard.jsx`

**Step 1: 新增固定日期点生成函数**

- 生成最近 14 个自然日日期标签；
- 日期格式同年优先 `MM-DD`，跨年自动带年份；
- 输出顺序固定为从旧到新。

**Step 2: 新增按天聚合函数**

- 基于现有 `quota_data` 小时级数据，在前端将同一天、同模型的数据累加到单个 bucket。
- 聚合维度：
  - `day`
  - `model_name`
- 统计字段：
  - `quota`

**Step 3: 新增补零逻辑**

- 对 14 天内没有数据的日期补零。
- 对某一天没有某个模型的数据，也要补出对应 `quota=0` 的图表点，避免第 1、第 2 个 tab 在时间轴上断层。

### Task 19: 重构第 1 / 第 2 个 Tab 的图表组装逻辑

**Files:**
- Modify: `web/src/hooks/dashboard/useDashboardCharts.jsx`

**Step 1: 将前两个图表与后两个图表的数据路径拆开**

- 当前 `updateChartData` 会一次性驱动全部图表，并共用 `generateChartTimePoints`。
- 需要把第 1、第 2 个 tab 的图表装配逻辑拆成独立路径，避免继续走当前“最多 7 个点、按小时/默认粒度”的通路。

**Step 2: 生成第 1 个 tab 的 14 天分布数据**

- 对“模型消耗分布”：
  - X 轴使用固定 14 天日期；
  - Y 轴使用每天各模型的 quota 总量；
  - `seriesField` 继续使用 `Model`；
  - tooltip 展示每日每模型额度值与总计。

**Step 3: 生成第 2 个 tab 的 14 天趋势数据**

- 对“模型消耗趋势”：
  - X 轴使用固定 14 天日期；
  - Y 轴改为每天各模型的 quota 总量；
  - 折线图维持按模型分组；
  - tooltip 展示每日每模型额度值与总计。

**Step 4: 保持标题与总计语义一致**

- 第 1 个 tab 标题仍为“模型消耗分布”。
- 第 2 个 tab 标题仍为“模型消耗趋势”。
- 两者的 `subtext` 都应与 quota 口径一致。

### Task 20: 视图文案与空态优化

**Files:**
- Modify: `web/src/components/dashboard/ChartsPanel.jsx`
- Modify: `web/src/i18n/locales/zh-CN.json`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/vi.json`
- Modify: `web/src/i18n/locales/zh-TW.json`

**Step 1: 补充必要文案**

- 如需新增以下文案：
  - `最近14天`
  - `按天统计`
  - `最近14天模型消耗分布`
  - `最近14天模型消耗趋势`
- 只在确实需要前端显示时新增，避免无意义 key 膨胀。

**Step 2: 校正无数据场景**

- 即使 14 天内无任何记录，第 1、第 2 个 tab 也应显示 14 天时间轴和零值图表，而不是退回“前后几个小时 + 无数据”。
- 如图表库在全 0 数据下表现异常，再补统一空态兜底。

### Task 21: 验证与回归

**Files:**
- Review: `web/src/hooks/dashboard/useDashboardData.js`
- Review: `web/src/hooks/dashboard/useDashboardCharts.jsx`
- Review: `web/src/helpers/dashboard.jsx`
- Review: `web/src/components/dashboard/ChartsPanel.jsx`

**Step 1: 自动化验证**

Run:

```powershell
cd web
bun run build
```

**Step 2: 手工验收**

- 普通用户打开 `/console/dashboard`：
  - 第 1 个 tab 显示最近 14 天日期；
  - 第 2 个 tab 同样显示最近 14 天日期；
  - 每天无数据时仍保留日期点；
  - tooltip 中每日总消耗与模型拆分正确。
- 管理员打开 Dashboard：
  - 在默认态和指定 `username` 检索后，第 1、第 2 个 tab 都保持 14 天按天展示。
- 顶部刷新按钮点击后：
  - 前两个 tab 和其他卡片都正常刷新；
  - 不出现 tab 切换错乱或旧数据残留。

**Step 3: 回归检查**

- `调用次数分布`、`调用次数排行` 不因本次改动被迫切换为 14 天固定模式。
- 搜索弹窗仍可正常打开、关闭和触发共享数据刷新。
- Dashboard 首屏性能无明显退化。

### 9.3 关键风险

- 当前 `quota_data` 是按小时落库，首版如果只做前端二次聚合，必须确保“按天累加”不会因时区或字符串格式化导致日期错位。
- `generateChartTimePoints` 当前有 `MAX_TREND_POINTS = 7` 的隐式假设；如果不拆分前两个 tab 的数据路径，很容易继续被 7 个点限制。
- 第 2 个 tab 当前标题为“模型消耗趋势”，但实现实际按 `count` 组装，这次改动需要显式纠正统计口径，否则会出现“名字改了、数据没改”的假修复。
- Dashboard 当前一个 `updateChartData` 同时驱动多个图表；若拆分不彻底，容易把第 3、第 4 个 tab 也误改成日粒度。

### 9.4 已确认决策

- 已确认首版同时修改第 1 和第 2 个 tab。
- 已确认两个 tab 的目标展示均为“最近 14 天、按天统计”。
- 已确认第 1 个 tab 展示按模型分组的日消耗分布。
- 已确认第 2 个 tab 展示按模型分组的日消耗趋势，并纠正当前按 count 组装的语义偏差。
- 已确认优先复用现有 `/api/data/self`、`/api/data` 接口，不新增后端日报接口。
- 已确认优先通过前端专用数据加载与日聚合实现，不修改 `quota_data` 的落库粒度。

## 10. 基于 API Key 的学生用量查询入口实施计划

> I'm using the writing-plans skill to create the implementation plan.
>
> 已确认采用方案 2：学生在网页输入完整 `sk-...`，后端验证后换取“只读 portal session”，后续所有查询都绑定到该 `token_id`，不再让前端持续携带真实 API key。

**Goal:** 在不新增学生用户账号的前提下，为每个 token 提供一个“学生查询入口”，让学生可以在网页输入自己的 `sk-...` 后，查看该 token 的消耗额度、RPM/TPM、使用日志和 CSV 导出。

**Architecture:** 前端新增 `token-portal` 独立入口和登录页，学生提交 `sk-...` 到后端 `POST /api/token-portal/login`。后端复用现有 token 只读校验能力校验 key，写入一组与普通用户 session 并存的 portal session key，后续 `GET /api/token-portal/log`、`/stat`、`/export` 全部强制使用 `token_id` 精确过滤。前端复用现有使用日志页组件，新增 `tokenPortal` 模式切换数据源和筛选字段，而不重写整套表格。

**Tech Stack:** Go + Gin + GORM, session cookie, React 18 + Vite + Semi UI, axios, Bun。

---

### 10.1 已确认范围

- 学生输入的是完整 API key，也就是实际调用使用的 `sk-...`。
- 学生只能查看当前输入 token 对应的数据，不能切换到其他 token 或其他用户。
- 学生需要看到：
  - 顶部消耗额度
  - RPM / TPM
  - 日志表格
  - 时间筛选
  - 请求路径 / 模型 / Request ID 筛选
  - 导出 CSV
- 学生不需要看到：
  - token 管理
  - 额度修改
  - 账号登录
  - 管理员日志
  - 以用户维度切换查询
- 首版允许“已过期 / 已禁用 / 已耗尽”的 token 仍可查询历史记录，只要 token 记录仍存在且所属用户未被封禁。
- token 被硬删除后，portal session 应失效。

### 10.2 已确认设计决策

- **不直接复用** `GET /api/log/token` 作为前端主数据源，因为它当前只提供最近一小批日志，没有 stat、export、分页和 portal 会话能力。
- **不使用** `token_name` 作为 portal 查询的主过滤条件，必须使用 `token_id` 精确过滤，避免同账号内 token 同名时串数据。
- **不将真实 `sk-...` 落地到 localStorage / sessionStorage / URL query**。前端只在登录提交那一次携带完整 key。
- portal session 和普通用户 session **必须共存但互不覆盖**，不能复用 `username` / `role` / `id` 这组普通登录 key。
- 前端 portal 请求 **不复用** 当前全局 `API` 实例，因为全局 401 处理会跳转 `/login?expired=true`，对 portal 模式是错误的。
- 新增一个 `TokenPortalAPI` 或等价的 portal 专用 helper，401 / 403 时跳转 `/token-portal/login?expired=true`。
- 学生页复用现有 `usage-logs` 组件，采用 `mode='tokenPortal'`，小步扩展，不重写大量 UI。

### Task 22: 后端 portal 会话与路由合同

**Files:**
- Create: `controller/token_portal.go`
- Create: `middleware/token_portal.go`
- Create: `controller/token_portal_test.go`
- Modify: `router/api-router.go`
- Review: `middleware/auth.go`
- Review: `model/token.go`

**Step 1: 先写失败的后端集成测试**

- 覆盖以下场景：
  - `POST /api/token-portal/login` 使用合法 `sk-...` 成功
  - 非法 key 登录失败
  - `GET /api/token-portal/me` 在未登录时返回 401/403
  - `POST /api/token-portal/logout` 只清理 portal 会话
  - 普通用户 session 与 portal session 同时存在时，互不影响

**Step 2: 定义 portal session key**

- 在 `middleware/token_portal.go` 固定 portal 会话 key，例如：
  - `portal_token_id`
  - `portal_user_id`
  - `portal_token_name`
  - `portal_masked_key`
- 禁止复用 `username` / `role` / `id` 这组普通登录字段。

**Step 3: 实现 portal 登录接口**

- `POST /api/token-portal/login`
- 请求体建议：

```json
{
  "api_key": "sk-..."
}
```

- 验证语义对齐 `TokenAuthReadOnly()`：
  - 允许已过期 / 已禁用 / 已耗尽 token 查询历史记录
  - 仍然检查 token 是否存在
  - 仍然检查所属用户是否已被封禁
- 响应只返回非敏感信息，例如：
  - `token_name`
  - `masked_key`
  - `token_id`
- 禁止在响应中回传完整 key。

**Step 4: 实现 portal 会话读取中间件**

- 新增 `TokenPortalAuth()`，每次请求都基于 `portal_token_id` 重新读 DB 或 cache 验证 token 仍存在。
- 中间件应在 context 写入：
  - `token_id`
  - `token_name`
  - `id`（token 所属 user id，仅供内部查询组合使用，不视为登录用户）
  - `portal_mode=true`

**Step 5: 实现 portal 会话查询 / 退出接口**

- `GET /api/token-portal/me`
- `POST /api/token-portal/logout`
- logout 只清理 portal session key，不影响普通账号登录状态。

**Step 6: 跑后端相关测试**

Run:

```powershell
go test ./controller -run TokenPortal -v
```

### Task 23: 将日志查询升级为 `token_id` 精确过滤

**Files:**
- Modify: `model/log.go`
- Create: `model/log_token_filter_test.go`
- Review: `controller/log.go`

**Step 1: 先写 `token_id` 隔离测试**

- 构造同一个用户的两个 token，名称故意可相同或相近。
- 记录日志后，断言按 `token_id=A` 查询时，不会拿到 `token_id=B` 的数据。

**Step 2: 扩展 `LogFilter`**

- 在 `model.LogFilter` 新增：

```go
TokenID *int
```

- 保持现有 `TokenName` 字段，供普通用户页面继续使用。

**Step 3: 更新 `applyLogFilters()`**

- portal 模式下必须优先按 `logs.token_id = ?` 过滤。
- 如果 `TokenID != nil`，则不依赖 `TokenName` 做主隔离条件。

**Step 4: 新增 portal 专用查询封装**

- 在 `model/log.go` 内新增或封装：
  - `GetTokenLogsByFilter(filters LogFilter, startIdx int, num int)`
  - `GetTokenLogsForExport(filters LogFilter)`
- 内部都基于 `applyLogFilters()`，不重复拼 SQL。

**Step 5: 跑 model 测试**

Run:

```powershell
go test ./model -run "TokenFilter|LogFilter" -v
```

### Task 24: portal 统计 / 导出接口

**Files:**
- Modify: `controller/token_portal.go`
- Modify: `controller/log.go`
- Modify: `model/log.go`
- Modify: `router/api-router.go`
- Create: `controller/token_portal_export_test.go`

**Step 1: 先写 stat 和 export 的失败测试**

- `GET /api/token-portal/log/stat`
- `GET /api/token-portal/log`
- `GET /api/token-portal/log/export`
- 断言查询结果只包含当前 portal session 绑定的 `token_id` 数据。

**Step 2: 新增统计层 helper**

- 不要继续使用仅按 `username + token_name` 组合的 `SumUsedQuota(...)` 作为 portal 主入口。
- 新增 `SumUsedQuotaByFilter(filters LogFilter)` 或等价 portal 专用封装，确保 `token_id` 是主过滤条件。

**Step 3: 实现 portal 日志列表 / 统计 / 导出**

- `GET /api/token-portal/log`
- `GET /api/token-portal/log/stat`
- `GET /api/token-portal/log/export`
- 只接受以下筛选参数：
  - `type`
  - `start_timestamp`
  - `end_timestamp`
  - `model_name`
  - `request_id`
  - `request_path`
- 忽略 `username` / `channel` / `token_name` / `group` 这些不应由学生控制的字段。

**Step 4: 复用 CSV 写出逻辑**

- 继续复用 `writeLogsCSV(...)`
- 但导出数据源改为 portal 专用 filter 路径。

**Step 5: 验证 CSV 列不泄露其他 token**

- 导出中可保留 `token_name` 列，作为当前 token 的名称展示。
- 禁止出现其他 token 的日志行。

**Step 6: 跑 controller 测试**

Run:

```powershell
go test ./controller -run "TokenPortal|Export" -v
```

### Task 25: 前端 portal 专用 API helper 与 401 跳转

**Files:**
- Create: `web/src/helpers/tokenPortalApi.js`
- Modify: `web/src/helpers/index.js`
- Review: `web/src/helpers/api.js`
- Review: `web/src/helpers/utils.jsx`

**Step 1: 新增 portal 专用 axios 实例**

- 不要复用当前全局 `API`，因为它默认携带 `New-API-User`，且 401 会强制跳回 `/login`。
- 新增 `TokenPortalAPI`，特性：
  - 不带 `New-API-User`
  - 默认同源 cookie
  - 401 / 403 时跳转 `/token-portal/login?expired=true`

**Step 2: 封装 portal helper 函数**

- `loginWithTokenPortal(apiKey)`
- `logoutTokenPortal()`
- `getTokenPortalSession()`
- `getTokenPortalLogs(params)`
- `getTokenPortalStat(params)`
- `exportTokenPortalLogs(params)`

**Step 3: 审查前端敏感信息落地**

- 禁止在 localStorage / sessionStorage 保存完整 `sk-...`
- 如需本地展示，只存 `masked_key` 和 `token_name`

**Step 4: 验证 401 跳转差异**

- 普通用户路由继续跳 `/login`
- portal 路由只跳 `/token-portal/login`

**Step 5: 前端构建验证**

Run:

```powershell
cd web
bun run build
```

### Task 26: 新增“API 用量查询”入口与登录页

**Files:**
- Create: `web/src/pages/TokenPortalLogin/index.jsx`
- Create: `web/src/components/auth/TokenPortalLoginForm.jsx`
- Modify: `web/src/App.jsx`
- Modify: `web/src/components/auth/LoginForm.jsx`
- Modify: `web/src/components/layout/headerbar/UserArea.jsx`

**Step 1: 新增前端路由**

- `/token-portal/login`
- `/token-portal/log`

**Step 2: 新增顶部入口**

- 在 `UserArea.jsx` 的未登录区域新增“API 用量查询”按钮或链接。
- 保持现有 `登录` / `注册` 按钮不被覆盖。

**Step 3: 新增登录页**

- 页面只有一个输入框：`API 密钥`
- 一个主按钮：`查询`
- 不显示用户名 / 密码 / OAuth / 注册相关逻辑

**Step 4: 在普通登录页增加导航**

- 在 `LoginForm.jsx` 内新增一个明显的次入口，跳到 `/token-portal/login`
- 不要把两种登录表单混在一个提交处理函数里

**Step 5: 登录成功后跳转**

- `POST /api/token-portal/login` 成功后跳转 `/token-portal/log`
- 登录失败时只显示 portal 错误提示，不改写普通用户 localStorage

**Step 6: 构建验证**

Run:

```powershell
cd web
bun run build
```

### Task 27: 复用现有使用日志页，新增 `tokenPortal` 模式

**Files:**
- Create: `web/src/pages/TokenPortalLog/index.jsx`
- Modify: `web/src/components/table/usage-logs/index.jsx`
- Modify: `web/src/components/table/usage-logs/UsageLogsFilters.jsx`
- Modify: `web/src/components/table/usage-logs/UsageLogsActions.jsx`
- Modify: `web/src/hooks/usage-logs/useUsageLogsData.jsx`
- Review: `web/src/pages/Log/index.jsx`

**Step 1: 为 `usage-logs` 添加 `mode` 入口**

- 默认保持 `mode='user'`
- portal 页传入 `mode='tokenPortal'`

**Step 2: 在 hook 里切换数据源**

- 普通用户继续使用：
  - `/api/log/self`
  - `/api/log/self/stat`
  - `/api/log/self/export`
- portal 模式使用：
  - `/api/token-portal/log`
  - `/api/token-portal/log/stat`
  - `/api/token-portal/log/export`

**Step 3: 缩减 portal 模式的筛选项**

- 保留：
  - `dateRange`
  - `model_name`
  - `request_id`
  - `request_path`
  - `logType`
- 隐藏：
  - `username`
  - `channel`
  - `token_name`
  - `group`

**Step 4: 复用统计卡片和导出按钮**

- 保留现有顶部 `消耗额度 / RPM / TPM`
- 保留 `导出 CSV`
- 在 portal 模式下新增一个 `退出查询` 按钮，调用 `/api/token-portal/logout`

**Step 5: 在页面加载时读取 portal session**

- 进入 `/token-portal/log` 时先读 `/api/token-portal/me`
- 未登录或会话失效时直接跳 `/token-portal/login?expired=true`
- 如需在页面上展示当前 token 信息，只展示 `token_name` 或 `masked_key`

**Step 6: 前端构建验证**

Run:

```powershell
cd web
bun run build
```

### Task 28: portal 模式下的过期 / 退出 / 共存回归

**Files:**
- Modify: `web/src/components/table/usage-logs/UsageLogsActions.jsx`
- Modify: `web/src/components/auth/TokenPortalLoginForm.jsx`
- Modify: `controller/token_portal.go`
- Modify: `middleware/token_portal.go`
- Review: `web/src/components/auth/LoginForm.jsx`

**Step 1: 验证 portal 过期后的前端去向**

- 会话失效后应跳回 `/token-portal/login?expired=true`
- 禁止误跳到 `/login?expired=true`

**Step 2: 验证 portal logout**

- portal logout 之后，portal 页立即不可用
- 普通用户若同时已登录，不应被波及

**Step 3: 验证普通登录回归**

- 普通 `/login`
- 普通 `/console/log`
- 普通 `/api/log/self`
- 都不应被 portal 模式改坏

### Task 29: 文案与多语言

**Files:**
- Modify: `web/src/i18n/locales/zh-CN.json`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/vi.json`
- Modify: `web/src/i18n/locales/zh-TW.json`

**Step 1: 新增 portal 入口文案**

- `API 用量查询`
- `输入 API 密钥`
- `查询`
- `退出查询`
- `API 密钥无效`
- `查询会话已过期`

**Step 2: 审查是否复用现有 key**

- 只在确实没有可复用 key 时才新增
- 避免 portal 功能引入大量重复文案

**Step 3: 构建验证**

Run:

```powershell
cd web
bun run build
```

### Task 30: 联调验收与回归

**Files:**
- Review: `controller/token_portal.go`
- Review: `middleware/token_portal.go`
- Review: `model/log.go`
- Review: `web/src/components/auth/TokenPortalLoginForm.jsx`
- Review: `web/src/hooks/usage-logs/useUsageLogsData.jsx`
- Review: `web/src/components/table/usage-logs/UsageLogsFilters.jsx`
- Review: `web/src/components/table/usage-logs/UsageLogsActions.jsx`

**Step 1: 后端自动化验证**

Run:

```powershell
go test ./controller ./model -run "TokenPortal|Export|LogFilter" -v
```

**Step 2: 前端构建验证**

Run:

```powershell
cd web
bun run build
```

**Step 3: 手工验收**

- 在未登录状态下，顶部能看到“API 用量查询”入口
- 点击入口后进入只有一个 API key 输入框的页面
- 输入有效 `sk-...` 后进入 `/token-portal/log`
- 顶部能看到消耗额度 / RPM / TPM
- 日志列表只包含当前 token 的记录
- `request_path` / `model_name` / `request_id` / 时间筛选均可用
- `导出 CSV` 只包含当前 token 的日志
- `退出查询` 后页面回到 `/token-portal/login`

**Step 4: 回归检查**

- 普通用户 `/login` 流程不变
- 普通用户 `/console/log` 流程不变
- 现有 `/api/usage/token` 和 `/api/log/token` 接口不被破坏
- 同一个浏览器内，portal session 和管理员 session 可并存

### 10.3 关键风险

- 当前前端全局 `showError()` 对 401 的默认去向是 `/login`，如果没有 portal 专用 API 实例，portal 页异常时会跳错页。
- 当前日志筛选以 `token_name` 为现有用户场景的约定，如果 portal 没有强制切换到 `token_id`，在“同名 token”时会出现串数据风险。
- portal session 若直接复用普通用户 session 字段，容易把“API key 查询”和“账号登录”冲在一起。
- 如果在前端 localStorage/sessionStorage 本地保存完整 `sk-...`，就相当于把真实调用密钥暴露到浏览器持久化存储，这与方案 2 的安全目标相冲突。

### 10.4 已确认决策

- 已确认学生输入完整 `sk-...`，但前端只负责提交一次，后续改用 portal session。
- 已确认 portal 查询主键使用 `token_id`，不使用 `token_name` 作为主隔离条件。
- 已确认前端复用现有 `usage-logs` 组件，采用 `tokenPortal` 模式小步扩展。
- 已确认顶部和登录页都要提供“API 用量查询”入口。
- 已确认 portal 401 / 403 跳回 `/token-portal/login`，不跳回普通 `/login`。

---

## 11. JustAPI 品牌替换计划（待 Review）

### 11.1 需求摘要

- 目标：把仓库内“面向用户/运营可见”的 `New API / NewAPI / new-api` 品牌展示替换为 `JustAPI`。
- 指定图标：使用仓库内现有的 `icon/justapi_icon.png` 作为新的默认品牌图标。
- 本轮交付：先给出计划到 `task_plan.md` 供 review，不直接执行代码替换。

### 11.2 方案对比

**方案 A：仅做品牌展示层替换（推荐）**

- 修改系统默认站点名、默认 logo、浏览器标题、页脚/About/Home 文案、i18n 中明确指向本产品的文案，以及 README/项目文档中的产品名称。
- 保留 Go module path、import path、HTTP header 名、外部 `newapi` URL path、缓存命名空间等技术标识不动。
- 优点：风险最低，能最快覆盖你截图里的站点名和头部图标。
- 风险：仓库里仍会保留一部分技术层 `new-api` 字符串。

**方案 B：品牌展示层 + 本地运维元数据一起替换**

- 在方案 A 基础上，再评估 Docker service/container name、Pyroscope app name、磁盘缓存目录、update checker UA、日志前缀等本地运维字符串。
- 优点：仓库中残留的 `new-api` 会更少。
- 风险：可能影响现有监控、脚本、部署目录和缓存复用。

**方案 C：字面意义的“全仓全替换”**

- 连 `go.mod` module path、所有 Go imports、`New-API-User` header、`NewAPIError` 类型名、外部 `/api/newapi/*` 拉取地址、GitHub 仓库链接、docker image/name 等一并重命名。
- 不推荐直接做。
- 原因：这已经不是品牌替换，而是 fork/重命名工程，极易引入编译错误、兼容性中断和外部依赖失效。

**推荐结论**

- 第一阶段按方案 A 落地，先把你截图里的品牌名、默认图标、浏览器标题和主要页面文案替换成 `JustAPI`。
- 方案 B 作为可选第二阶段，单独评审后再做。
- 方案 C 只有在明确接受“这是仓库/模块/兼容协议层级的重命名工程”时才应启动。

### 11.3 已定位的关键落点

- 默认品牌源头：
  - `common/constants.go`
  - `model/option.go`
  - `controller/misc.go`
- 前端默认回退值：
  - `web/src/helpers/utils.jsx`
  - `web/src/helpers/data.js`
  - `web/index.html`
- 头部/登录/页脚展示：
  - `web/src/components/layout/headerbar/HeaderLogo.jsx`
  - `web/src/components/layout/PageLayout.jsx`
  - `web/src/components/layout/Footer.jsx`
  - `web/src/components/auth/LoginForm.jsx`
  - `web/src/components/auth/RegisterForm.jsx`
  - `web/src/components/auth/PasswordResetForm.jsx`
  - `web/src/components/auth/PasswordResetConfirm.jsx`
  - `web/src/components/auth/TokenPortalLoginForm.jsx`
- 图标资源现状：
  - `icon/justapi_icon.png` 已存在，但它位于仓库根目录 `icon/`，浏览器不会直接把这个目录当成前端静态资源目录。
  - 第一阶段需要把该图标复制/落地到 `web/public/`，或把默认 logo 路径改成一个浏览器可访问的静态路径。
- 高风险技术标识：
  - `github.com/QuantumNous/new-api` imports / module path
  - `New-API-User` header
  - `NewAPIError` 类型名
  - `/api/newapi/*` / `docs.newapi.pro` / `newapi.pro`
  - `new-api-worker`
  - `new-api:...` Redis namespace
  - `new-api-body-cache` / `PYROSCOPE_APP_NAME=new-api`

### Task 31: 固化第一阶段范围与残留白名单

**Files:**
- Modify: `task_plan.md`
- Review: `common/constants.go`
- Review: `web/src/helpers/utils.jsx`
- Review: `web/index.html`
- Review: `web/src/components/layout/Footer.jsx`

**Step 1: 统一本次改动口径**

- 第一阶段只替换“品牌展示层”的 `New API / NewAPI / new-api`。
- 不碰 module path、imports、header、外部 API path、缓存 key、协议名和兼容名。

**Step 2: 建立残留白名单**

- 把允许保留的 `new-api` 分类为：
  - 兼容协议/接口路径
  - 第三方仓库/文档链接
  - 监控/缓存/部署元数据
  - 内部类型名/技术实现名

**Step 3: 给 review 明确输出**

- review 时只需要确认：
  - 是否接受方案 A 作为第一阶段
  - 是否要把方案 B 拆成第二阶段
  - 是否真的要推进高风险的方案 C

**Verification:**

```powershell
rg -n --hidden -S "new API|New API|new-api|NewAPI|newapi|New-API"
```

**Deliverables:**

- 第一阶段范围定义
- 残留 `new-api` 白名单原则

### Task 32: 打通默认品牌名与默认图标链路

**Files:**
- Modify: `common/constants.go`
- Modify: `web/src/helpers/utils.jsx`
- Modify: `web/index.html`
- Modify: `web/src/components/layout/PageLayout.jsx`
- Create or Replace: `web/public/justapi_icon.png`

**Step 1: 处理图标落点**

- 推荐把 `icon/justapi_icon.png` 复制到 `web/public/justapi_icon.png`。
- 不建议让前端直接引用仓库根目录的 `icon/`，因为该目录默认不会被 Vite 作为静态资源公开。

**Step 2: 统一默认品牌值**

- 把后端默认 `SystemName` 改成 `JustAPI`。
- 把默认 `Logo` 或前端 fallback logo 路径指向新的静态资源。

**Step 3: 统一浏览器级展示**

- 更新 `web/index.html` 的默认 `<title>` 和 favicon。
- 确保 `PageLayout` 从状态接口拿到 logo/system name 后，浏览器标题和标签页图标也同步更新。

**Verification:**

```powershell
cd web
bun run build
```

**Deliverables:**

- 默认品牌名从 `New API` 切换到 `JustAPI`
- 默认图标链路指向 `justapi_icon.png`

### Task 33: 替换前端主要页面中的品牌展示文案

**Files:**
- Modify: `web/src/pages/About/index.jsx`
- Modify: `web/src/pages/Home/index.jsx`
- Modify: `web/src/components/layout/Footer.jsx`
- Review: `web/src/components/layout/headerbar/HeaderLogo.jsx`
- Review: `web/src/components/auth/LoginForm.jsx`
- Review: `web/src/components/auth/RegisterForm.jsx`
- Review: `web/src/components/auth/PasswordResetForm.jsx`
- Review: `web/src/components/auth/PasswordResetConfirm.jsx`
- Review: `web/src/components/auth/TokenPortalLoginForm.jsx`

**Step 1: 替换硬编码品牌文案**

- 把明确指向本站产品的 `New API / NewAPI` 文案替换为 `JustAPI`。
- 保留 `system_name` 的动态展示机制，不要把动态品牌能力改没。

**Step 2: 处理外部链接显示名**

- 对 `QuantumNous/new-api`、`new-api-horizon` 等外部仓库链接，不要机械替换 URL。
- 需要区分“链接文案要不要改”和“链接地址能不能改”这两件事。

**Step 3: 对齐截图中的头部区域**

- 头部品牌应通过统一的 `system_name + logo` 链路展示。
- 登录、注册、找回密码、portal 登录页也要共享同一品牌源。

**Verification:**

```powershell
rg -n "New API|NewAPI" web/src/pages/About web/src/pages/Home web/src/components/layout
```

**Deliverables:**

- 头部/页脚/About/Home 主品牌文案替换完成
- 登录相关页面默认图标与品牌名对齐

### Task 34: 清理 i18n 与文档中的产品自指文案

**Files:**
- Modify: `web/src/i18n/locales/zh-CN.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/vi.json`
- Review: `README.md`
- Review: `docs/**/*.md`

**Step 1: 只改“明确是在说本产品”的文案**

- 例如站点介绍、仓库说明、页脚署名、产品说明等。

**Step 2: 标记歧义项**

- 以下类型不要直接批量替换，必须人工判断：
  - “如果上游是 New API”
  - `new-api-worker`
  - `docs.newapi.pro`
  - `https://newapi.pro`
  - `new-api` 作为兼容格式/生态名称出现的地方

**Step 3: 文档链接分离处理**

- 如果 JustAPI 的官网/文档/仓库地址尚未准备好，第一阶段只改显示文案，不虚构新的 URL。

**Verification:**

```powershell
rg -n --glob "web/src/i18n/locales/*.json" --glob "docs/**/*.md" --glob "README*" -S "New API|NewAPI|new-api|newapi"
```

**Deliverables:**

- 已确认可替换的文案清单
- 需要保留或二次确认的歧义清单

### Task 35: 评审第二阶段可选的运维/元数据替换

**Files:**
- Review: `.env.example`
- Review: `docker-compose.yml`
- Review: `Dockerfile`
- Review: `common/init.go`
- Review: `common/pyro.go`
- Review: `common/disk_cache.go`
- Review: `model/subscription.go`
- Review: `web/src/helpers/api.js`
- Review: `web/src/components/settings/OtherSetting.jsx`
- Review: `controller/topup_stripe.go`

**Step 1: 识别这类字符串是否会影响兼容性**

- 例如：
  - `new-api` 目录名
  - `new-api` container/service name
  - `new-api-update-checker`
  - `new-api:subscription_plan:v1`
  - `new-api-body-cache`

**Step 2: 给每类项做三分法决策**

- 立即替换
- 延后到第二阶段
- 永久保留为兼容标识

**Step 3: 若决定替换，先补迁移说明**

- 明确缓存失效、监控名称变更、部署脚本调整和回滚方式。

**Verification:**

```powershell
rg -n --hidden -S "new-api|NewAPI|New-API" .env.example docker-compose.yml Dockerfile common model web/src/helpers/api.js web/src/components/settings/OtherSetting.jsx controller/topup_stripe.go
```

**Deliverables:**

- 第二阶段候选项清单
- 每项的风险判断与迁移说明需求

### Task 36: 联调验证与验收口径

**Files:**
- Review: `common/constants.go`
- Review: `web/index.html`
- Review: `web/src/helpers/utils.jsx`
- Review: `web/src/components/layout/PageLayout.jsx`
- Review: `web/src/components/layout/Footer.jsx`
- Review: `web/src/pages/About/index.jsx`
- Review: `web/src/pages/Home/index.jsx`

**Step 1: 后端编译/测试验证**

```powershell
go test ./...
```

**Step 2: 前端构建验证**

```powershell
cd web
bun run build
```

**Step 3: 手工验收**

- 打开首页，确认头部品牌名显示为 `JustAPI`
- 头部 logo、登录页 logo、页脚 logo、浏览器标签页图标一致
- `About`、`Home`、页脚中的本站品牌文案完成替换
- 不应误改外部链接地址、兼容协议名或内部技术标识

**Step 4: 残留检索**

```powershell
rg -n --hidden -S "New API|NewAPI|new-api|newapi|New-API"
```

- 预期：剩余命中只来自白名单中的技术/兼容/外部引用。

**Deliverables:**

- 构建通过记录
- 手工验收清单
- 残留命中与白名单对照结果

### 11.4 待你 review 的关键决策

- 是否确认按“方案 A 先落地，方案 B 另开评审”的节奏执行。
- `docs.newapi.pro`、GitHub 仓库链接、`new-api-worker` 这类生态/兼容标识是否先保留。
- 对外显示文案和对外链接地址是否允许分阶段处理：
  - 第一阶段只改显示名为 `JustAPI`
  - 第二阶段在新域名/新仓库准备好后再改 URL

### 11.5 执行决议（2026-03-31）

- 已确认立即执行：**方案 A：仅做品牌展示层替换**。
- 已确认暂不执行：方案 B、方案 C；两者保留在本计划中，后续按需要单独启动。
- 执行要求：
  - 严格按方案 A 范围落地；
  - 自行迭代，不中途暂停等待 review；
  - 每个子任务完成后单独提交一次 commit。

### 11.6 执行结果（2026-03-31）

- 方案 A 已按计划执行完成；方案 B、方案 C 保留，未进入实现。
- 已完成并提交的子任务：
  - `662b8e48` `docs: confirm JustAPI phase A execution`
  - `7608998d` `feat: switch default branding to JustAPI`
  - `20792794` `feat: rebrand primary UI text to JustAPI`
  - `34c8f7b0` `feat: rebrand docs and i18n copy to JustAPI`
- 本轮验收结果：
  - `go test ./common -run TestDefaultBranding -count=1` 通过。
  - `go test ./...` 未全绿；当前失败项为 `model/token_period_quota_test.go:121` 的 `TestValidateUserTokenResetsExpiredMonthlyPeriodQuotaWindow`，与本轮品牌替换无直接关联。
  - `bun run build` 在 `web/` 目录通过。
  - 首页/控制台默认品牌名、浏览器标题、默认图标、About/Footer、主要展示文案、i18n key/value 与 README 可见品牌文案均已切换为 `JustAPI`。
- 本轮刻意保留的残留项：
  - 仓库 URL、镜像名、Docker container/service name、`docs.newapi.pro`、`new-api-worker`、`new-api-update-checker`、`/var/cache/new-api`、`/llm-metadata/api/newapi/*`
  - 兼容性提示文案中把 `New API` 作为“上游转发项目名”出现的语句
- 后续若启动方案 B / C，再单独评审这些运维/兼容性标识的替换策略。

## 12. 侧边栏默认隐藏切换为管理员纯控制（2026-04-16）

### 12.1 已确认范围

- 目标行为：管理员在全局侧边栏配置里打开 `chat`、`console.midjourney`、`console.task` 后，刷新页面即可显示对应 Tab，不再被旧的“默认隐藏”逻辑拦住。
- 不再保留以下硬编码默认隐藏：
  - 后端新用户默认 `chat.enabled=false`
  - 后端新用户默认 `console.midjourney=false`
  - 后端新用户默认 `console.task=false`
  - 前端无用户配置时的 `hiddenDefaults`
- 保留现有两层配置体系：
  - 管理员全局 `SidebarModulesAdmin` 作为可见性上限
  - 用户个人 `sidebar_modules` 作为个人偏好存储
- 为了消除历史脏数据，需要补一次性清理迁移，把旧版本硬编码写入的 `false` 恢复为可显示状态；迁移仅执行一次，避免持续覆盖用户后续新选择。

### 12.2 方案比较

- 方案 A：只删除前后端默认隐藏代码，不处理历史用户数据。
  - 优点：改动最小。
  - 缺点：历史用户数据库里已经写入的 `false` 仍会继续阻塞显示，无法满足“管理员开了就能显示”。
- 方案 B：删除默认隐藏代码，并增加一次性历史清理迁移。
  - 优点：能同时修复新用户默认值和历史用户遗留配置，满足本轮目标。
  - 缺点：需要维护一次性迁移测试和状态位。
- 方案 C：改成管理员配置直接覆盖用户层，对这几个模块忽略用户个人配置。
  - 优点：管理员开关一开必定显示。
  - 缺点：会改变“个人边栏设置”语义，侵入性更强，也不必要。

### 12.3 执行决议

- 本轮采用方案 B。
- 设计假设：
  - 旧版本写入的 `chat.enabled=false`、`console.midjourney=false`、`console.task=false` 属于需要清理的历史默认值；
  - 迁移完成后，用户如果再次手动关闭这些项，后续不会被重复打开。

### Step-01: 固化新行为并先写失败测试

**Status:** Done

**AC:**

- 新的默认侧边栏配置不再强制隐藏 `chat`、`midjourney`、`task`。
- 存量用户若因旧逻辑被写入上述隐藏值，一次性迁移会将其恢复为可显示。
- 一次性迁移执行完成后，不会在后续重复覆盖用户新修改。

**Verification:**

```powershell
go test ./model -run 'TestGenerateDefaultSidebarConfigForRole_UsesVisibleDefaults|TestBackfillSidebarLegacyVisibleDefaults_UpdatesExistingUsersOnce' -count=1
```

**Deliverables:**

- 更新后的模型级失败测试
- 与本轮目标一致的默认值断言
- 一次性历史清理迁移断言

**Iteration Log:**

- Attempt-1 (2026-04-16): 先把“默认值改为可见”和“历史遗留 false 一次性清理”的行为写成失败测试，再进入实现。
- Result (2026-04-16): `go test ./model -run 'TestGenerateDefaultSidebarConfigForRole_UsesVisibleDefaults|TestBackfillSidebarLegacyVisibleDefaults_UpdatesExistingUsersOnce' -count=1` 先在旧实现上按预期 red，失败点为缺少 `BackfillSidebarLegacyVisibleDefaults` 与对应状态常量。

### Step-02: 调整后端默认值与历史清理迁移

**Status:** Done

**AC:**

- 新用户初始化侧边栏配置时，不再写入 `chat=false`、`midjourney=false`、`task=false`。
- `migrateDB()` 不再执行旧的隐藏默认值 backfill。
- 增加一个新的、只跑一次的历史清理迁移，保证历史用户不再被旧数据挡住。

**Verification:**

```powershell
go test ./model -run 'TestGenerateDefaultSidebarConfigForRole_UsesVisibleDefaults|TestBackfillSidebarLegacyVisibleDefaults_UpdatesExistingUsersOnce' -count=1
```

**Deliverables:**

- 更新后的后端默认配置生成逻辑
- 新的一次性历史清理迁移
- 清理后的迁移状态记录

**Iteration Log:**

- Attempt-1 (2026-04-16): 将后端侧边栏默认值改回可见，移除旧的 3 个隐藏 backfill 入口，并新增一次性 `BackfillSidebarLegacyVisibleDefaults` 清理历史关闭值。
- Result (2026-04-16): 定向模型测试转绿，旧遗留 `chat.enabled=false`、`console.midjourney=false`、`console.task=false` 会被一次性恢复为可显示。

### Step-03: 调整前端默认回退，去掉硬编码隐藏

**Status:** Done

**AC:**

- `useSidebar` 在用户无个人配置时，不再通过 `hiddenDefaults` 把这几个模块默认设为隐藏。
- 页面刷新后，管理员已开启的模块对无配置/已清理用户可直接显示。

**Verification:**

```powershell
cd web
bun run build
```

**Deliverables:**

- 更新后的前端默认回退逻辑
- 与管理员配置一致的默认可见性

**Iteration Log:**

- Attempt-1 (2026-04-16): 删除 `useSidebar` 中的 `hiddenDefaults`，让无用户配置时的默认可见性直接跟随管理员允许项。
- Result (2026-04-16): 前端构建通过；无个人配置时不再被前端默认回退强制隐藏 `chat`、`midjourney`、`task`。

### Step-04: 定向验证与回归

**Status:** Done

**AC:**

- 定向后端测试通过。
- 前端构建通过。
- 不引入新的已知启动期侧边栏迁移错误。

**Verification:**

```powershell
go test ./model -run 'TestGenerateDefaultSidebarConfigForRole_UsesVisibleDefaults|TestBackfillSidebarLegacyVisibleDefaults_UpdatesExistingUsersOnce' -count=1
go test ./controller -run TestCalculateUserPermissions_AllowsRootSidebarSettings -count=1
cd web
bun run build
```

**Deliverables:**

- 可复制的验证命令
- 本轮实现结果与残留风险说明

**Iteration Log:**

- Attempt-1 (2026-04-16): 先跑 `go test ./controller -run TestCalculateUserPermissions_AllowsRootSidebarSettings -count=1` 与 `bun run build`；其中前端构建第一次因 124 秒超时中断，非编译错误。
- Attempt-2 (2026-04-16): 将 `bun run build` 超时延长后重跑，并在 `gofmt` 后复跑定向 Go 测试。
- Result (2026-04-16):
  - `go test ./model -run 'TestGenerateDefaultSidebarConfigForRole_UsesVisibleDefaults|TestBackfillSidebarLegacyVisibleDefaults_UpdatesExistingUsersOnce' -count=1` 通过。
  - `go test ./controller -run TestCalculateUserPermissions_AllowsRootSidebarSettings -count=1` 通过。
  - `bun run build` 在 `web/` 目录通过，Vite 输出 built in 2m 41s。

## 13. 红框侧边栏项改为管理员唯一控制源（2026-04-16）

### 13.1 需求理解摘要

- 个人页“边栏设置”不再允许用户修改红框对应的侧边栏分组。
- 红框范围按本轮用户最新截图与说明，落为 `chat`、`console`、`personal` 三个分组。
- 对这三个分组，管理员页“侧边栏管理（全局控制）”成为唯一控制源：
  - 管理员打开则最终侧边栏显示；
  - 管理员关闭则最终侧边栏隐藏；
  - 用户历史 `sidebar_modules` 中的对应值不再影响最终显示。

### 13.2 待定参数清单

- 无。本轮按用户明确确认的红框范围直接实现。

### 13.3 执行决议

- 本轮采用“管理员唯一控制源 + 个人页移除可编辑项 + 保存时剥离旧用户值”的完整方案。
- 不采用“只隐藏个人页 UI、但继续保留旧个人配置生效”的半方案，避免行为与界面不一致。
- `admin` 分组不在本轮红框范围内，维持原有个人页可编辑语义。

### Step-05: 先写前端失败测试锁定管理员唯一控制语义

**Status:** Done

**AC:**

- 纯函数测试能够覆盖三件事：
  - `chat`、`console`、`personal` 三个分组在最终显示逻辑中忽略用户关闭值；
  - 用户保存配置时会剥离这三个分组；
  - 个人页边栏设置 UI 会过滤掉这三个分组。

**Verification:**

```powershell
cd web
node --test src/helpers/sidebar-config.test.js
```

**Deliverables:**

- 新增的前端纯函数测试文件
- 明确表达管理员唯一控制语义的失败断言

**Iteration Log:**

- Attempt-1 (2026-04-16): 新增 `web/src/helpers/sidebar-config.test.js`，先用当前旧语义的 helper 触发 red。
- Result (2026-04-16): `node --test src/helpers/sidebar-config.test.js` 按预期失败，失败点包括最终显示仍受用户值影响、保存未剥离红框分组、个人页仍显示红框分组。

### Step-06: 实现管理员唯一控制逻辑与个人页过滤

**Status:** Done

**AC:**

- `useSidebar` 最终显示逻辑对 `chat`、`console`、`personal` 三个分组不再读取用户个人配置。
- 个人页“边栏设置”不再显示上述三个分组。
- 用户保存/加载/重置个人边栏配置时，不再保留上述三个分组的个人值。
- 当个人页已经没有任何可编辑分组时，“边栏设置”标签页不再显示。

**Verification:**

```powershell
cd web
node --test src/helpers/sidebar-config.test.js
```

**Deliverables:**

- 新增的 `web/src/helpers/sidebar-config.js`
- 更新后的 `web/src/hooks/common/useSidebar.js`
- 更新后的 `web/src/components/settings/personal/cards/NotificationSettings.jsx`

**Iteration Log:**

- Attempt-1 (2026-04-16): 抽出 `buildFinalSidebarConfig`、`filterUserEditableSidebarSections`、`stripAdminOnlySectionsFromUserSidebarConfig` 三个纯函数，并接入 hook 与个人设置页。
- Result (2026-04-16): 纯函数测试转绿；红框分组现在同时满足“个人页不可改”“保存时会剥离”“最终显示忽略用户值”三条要求。

### Step-07: 前端验证与回归

**Status:** Done

**AC:**

- 新增纯函数测试通过。
- 前端生产构建通过。
- 不引入新的前端编译错误。

**Verification:**

```powershell
cd web
node --test src/helpers/sidebar-config.test.js
bun run build
```

**Deliverables:**

- 可复现的验证命令
- 本轮管理员唯一控制改动的验证结果

**Iteration Log:**

- Attempt-1 (2026-04-16): 先重跑 `node --test src/helpers/sidebar-config.test.js`，再跑 `bun run build` 做编译回归。
- Result (2026-04-16):
  - `node --test src/helpers/sidebar-config.test.js` 通过，3 个测试全部转绿。
  - `bun run build` 在 `web/` 目录通过，Vite 输出 built in 1m 26s。

## 14. 保留 JustAPI 定制前提下分批吸收 Upstream 新能力（2026-04-16）

### 14.1 需求理解摘要

- 目标不是直接把 `upstream/main` 整体合并进当前分支，而是“尽量保住现在的 JustAPI 定制，同时吃到 upstream 的新能力”。
- 本轮需要先把 `QuantumNous/new-api:main` 相对当前分支 `rockyicer` 的 `166` 个上游提交整理成一份可执行的分主题清单。
- 该清单必须按 `建议带 / 可选 / 暂缓` 三列分类，作为后续分批 cherry-pick / backport 的依据。
- 计划必须写入同一份 `task_plan.md`，并沿用现有 Step 编号继续追加。

### 14.2 待定参数清单

- 默认保留 `JustAPI` branding、关于页、联系页、部署文档、token portal、token period quota、sidebar 控制逻辑，不主动让上游覆盖。
- 默认不在第一批引入 dashboard 大改、group ratio / pricing settings 大改、footer / layout / i18n 表现层重构。
- 默认将 merge commits 仅作为追溯线索，不作为首选 cherry-pick 目标；优先挑选其下的叶子功能提交。

### 14.3 执行决议

- 采用“按主题分批回引”的策略，不做一次性 `merge upstream/main`。
- 先锁定分叉事实与冲突画像，再生成三列分层清单，最后给出后续批次化执行路线。
- 当前基线结论：
  - `rockyicer` 相对 `upstream/main` 为 `behind 166 / ahead 53`。
  - 分叉点为 `d096a2e5`。
  - 自分叉点以来，两边真正改到同一文件的重叠文件数为 `34`。
  - 上游改动密度最高的区域是 `web/src` 与 `relay/channel`；你的自定义改动密度最高的区域是 `web/src`、`router/api-router.go`、`model/*`、`controller/*`。
- 因此，适合先带入“低冲突高价值”的 provider / relay / security / payment 主题，不适合一上来碰 dashboard、ratio setting、layout 和 branding。

### 14.4 Upstream 166 提交分主题清单（三列）

| 建议带 | 可选 | 暂缓 |
| --- | --- | --- |
| Relay / provider 兼容与新能力：`ff29900f` llama.cpp cache-hit token、`263b9bc6` Claude inline file、`3ab65a82` Azure `/v1/responses/compact`、`c734db34` minimax image、`18373c6e` `e5b5331d` Wan 2.7、`b7c0f754` `22692b3f` `dafc7618` Seedance 2.0 / Duration / fail reason、`79527c0a` `f449e06b` `9816ad87` HEIC/HEIF、`23fde25b` Gemini streaming、`3cad6b9d` `82c2008d` `bb5b9eac` `8b221615` Claude 兼容修复、`d22f889e` xAI/Grok、`160cb285` zhipu_4v、`274307b0` `53cf37a4` ali usage string、`3bda738e` compact model pricing、`a19a63b9` vllm-omini 字段补齐 | Channel affinity / retry 规则：`b09337e6` preferred channel honor skip-retry、`5fe8e98e` codex/claude affinity 默认 skip retry、`45f65c29` regex ignored upstream models、`6154b8e3` `22b6b167` 暴露 skip-retry UI、`70560d53` `116e0b8f` `1ad25576` include_model_name | Dashboard / analytics / chart 大改：`606a4eee` admin user analytics、`77897a81` chart axes / sorting、`7cfaf6c3` dimension / ranking、`b2dd4acc` 消耗分布图滚动闪烁修复 |
| Auth / security / ops：`d955a0c0` OAuth 回当前页、`f40eb4e5` oauth bind callback、`e099117c` account binding POST、`2819e3a1` login error handling、`59c582d1` token auth 防信息泄漏、`20399d3c` SSRF 防护、`a5e20269` Docker / release CI hardening、`cf1b4853` 错误日志 env、`a18ea3cc` AUTH LOGIN 发件支持 | 日志与运维体验：`e904579a` `dcd09116` `13122aa0` `49db5147` 服务端日志文件管理、`c9611c49` usage logs stream status tooltip、`b4df9955` error logs 中的 isStream 修复 | Group ratio / pricing / settings UI：`78e4cb3c` group ratio rules 重做、`dc83c4af` RatioSetting 切到 `ModelPricingCombined`、`ed7f8399` model price error UX 大改 |
| Payment / topup / subscription：`8aaec8b1` TopUp.PaymentMethod、`b2a40d33` Stripe async webhook、`040e8c1d` amount-first quota adjust、`d15e14b1` `bf130c5c` quota adjustment logs 带管理员用户名、`2bedd31b` 订阅卡片显示 next quota reset | Playground / resilience / small UX：`559c98f2` Web ErrorBoundary、`4cd0e365` `427fb7ea` playground max_tokens、`a706f002` EditChannelModal advanced settings localStorage、`aafbd788` API info copy button、`8bb9a42f` `670abee2` channel clipboard magic string | Layout / footer / i18n 表现层：`7399e472` slide-in animations、`310d618a` footer layout、`5402bf41` 暴露 i18n instance 到 window、`1baf4a63` localization files 更新 |
| 小而值钱的后端修复：`1911520e` oauth bearer token type normalize、`ded4a124` OpenAI detail 空字段修复、`deff59a5` scanner buffer 提高与 gpt-5.4-nano prefix、`e520977e` forced beta query、`9f61407b` `d4a470a6` OpenRouter billing 语义修复、`926e1781` Claude cache usage 保留、`ab99c308` image count double-counting 修复 | Docs / deps / maintenance：`cf86fe5f` `e80d867f` BT 文档、`b81d3427` `3d0ac2d0` axios、`40dc43f4` x/image、`ded3bb9c` bedrockruntime、electron 相关 dependabot 提交 | 直接改 branding / 文案 / README 表现层的上游提交默认暂缓，避免冲掉 `JustAPI` 定制；若后续只取技术改动，可单独人工摘取，不整批带入 |

**Notes:**

- 上表已经覆盖 166 个上游提交的主要功能主题；其中 `42` 个 merge commits 默认只作追溯来源，不作为首选回引对象。
- 若某主题存在 merge commit 与叶子提交同时出现，后续执行时优先 cherry-pick 叶子功能提交，避免把无关上下文一起卷入。

### Step-08: 固化 upstream 基线与冲突画像

**Status:** Done

**AC:**

- 确认当前分支相对 `upstream/main` 的真实差异不是 GitHub 页面误报。
- 给出明确的 `behind / ahead` 数字、分叉点 SHA 和重叠文件数。
- 列出后续集成最需要注意的高冲突目录。

**Verification:**

```powershell
git fetch upstream main
git rev-list --count rockyicer..upstream/main
git rev-list --count upstream/main..rockyicer
git merge-base rockyicer upstream/main
$mb = git merge-base rockyicer upstream/main
$up = git diff --name-only $mb upstream/main | Sort-Object -Unique
$mine = git diff --name-only $mb rockyicer | Sort-Object -Unique
$mineSet = @{}; foreach ($f in $mine) { $mineSet[$f] = $true }
$overlap = $up | Where-Object { $mineSet.ContainsKey($_) }
$overlap.Count
```

**Deliverables:**

- 分叉事实确认
- 高冲突目录清单
- 后续分批回引的风险基线

**Iteration Log:**

- Attempt-1 (2026-04-16): 先抓取 `upstream/main`，再分别计算 `rockyicer..upstream/main` 与 `upstream/main..rockyicer`，避免被 GitHub 网页的单向提示误导。
- Result (2026-04-16): 确认当前状态为 `behind 166 / ahead 53`，分叉点为 `d096a2e5`，真正同文件重叠数为 `34`，高冲突区集中在 `web/src`、`controller/*`、`model/*`、`service/*`。

### Step-09: 生成 166 提交分主题清单并完成三列分层

**Status:** Done

**AC:**

- 将 166 个上游提交按主题归类，而不是按时间平铺。
- 输出 `建议带 / 可选 / 暂缓` 三列清单。
- 明确说明哪些 merge commits 只用于追溯，不推荐直接 cherry-pick。

**Verification:**

```powershell
git log --reverse --date=short --pretty=format:"%h`t%ad`t%s" rockyicer..upstream/main
$subjects = git log --pretty=format:'%s' rockyicer..upstream/main
$groups = $subjects | ForEach-Object {
  if ($_ -match '^(feat|fix|refactor|chore|docs|style|security|test)(\(|:|\b)') { $matches[1].ToLower() }
  elseif ($_ -match '^Merge pull request') { 'merge' }
  elseif ($_ -match '^Update\b') { 'update' }
  else { 'other' }
} | Group-Object | Sort-Object Count -Descending
$groups | ForEach-Object { "{0}`t{1}" -f $_.Count, $_.Name }
```

**Deliverables:**

- 三列分层清单
- 提交类型统计
- 主题化摘要与代表性 commit hash

**Iteration Log:**

- Attempt-1 (2026-04-16): 先把 166 个提交全部拉平成 subject 列表，再以功能主题重组，最后按收益/冲突比落到三列。
- Result (2026-04-16): 完成 `建议带 / 可选 / 暂缓` 清单；统计结果为 `42 feat / 51 fix / 12 refactor / 42 merge`，确认 merge commits 仅作追溯用途。

### Step-10: 批次 A 计划（建议带）

**Status:** Done

**AC:**

- 新建一条集成分支，例如 `sync-upstream-2026-04-16`。
- 第一批仅处理 `建议带` 列中的主题：
  - provider / relay 兼容与新能力；
  - auth / security / ops；
  - payment / topup / subscription；
  - 小而值钱的后端修复。
- 第一批不得引入 branding、dashboard、layout、ratio setting 表现层大改。

**Verification:**

```powershell
git switch -c sync-upstream-2026-04-16 rockyicer
git cherry-pick <batch-a-commit-list>
go test ./...
cd web
bun run build
```

**Deliverables:**

- 批次 A 的明确 cherry-pick 清单
- 一条独立的集成分支
- 冲突处理记录与验证结果

**Result:**

- 已创建集成分支 `sync-upstream-2026-04-16`，并在该分支完成 Batch A 集成。
- 已带入的 upstream 提交分组：
  - provider / relay: `ff29900f`、`263b9bc6`、`3ab65a82`、`23fde25b`、`3cad6b9d`、`82c2008d`、`160cb285`、`274307b0`
  - OAuth / auth / security / ops: `f40eb4e5`、`1911520e`、`d955a0c0`、`e099117c`、`2819e3a1`、`59c582d1`、`20399d3c`、`cf1b4853`
  - payment / topup / subscription: `8aaec8b1`、`b2a40d33`、`2bedd31b`、`040e8c1d`、`d15e14b1`
  - compatibility / format support: `79527c0a`、`f449e06b`、`a18ea3cc`
- 额外做了 1 处最小 manual backport：在 `dto.Usage` 中补回 `usage_semantic / usage_source` 字段，以承接上游 Claude usage 修复链路，而不引入无关旧 UI 改动。
- 过程中 2 次遇到“提交本身依赖未满足”的情况：
  - `82c2008d` 首次验证失败，补齐 `dto.Usage` 字段后恢复通过。
  - `d15e14b1` 首次引入时因缺少 `040e8c1d` 的 add_quota 基础而失败，先回退，待 `040e8c1d` 成功合入后重新 cherry-pick 并通过验证。
- 未引入 branding、dashboard、layout、ratio setting 大改，保持 JustAPI 定制不被上游表现层重写。

**Iteration Log:**

- Attempt-1 (Planned): 先按 provider / security / payment 三个子批次拆开执行，每个子批次单独验证，避免一次性引入过大上下文。
- Attempt-2 (2026-04-16): 进入执行阶段，先切集成分支，再把 Batch A 精简成低冲突 cherry-pick 清单；每个子批次完成后都要做冲突检查、定向测试与前端构建验证。
- Result-1 (2026-04-16): provider / relay 子批次完成，解决了 `relay/channel/claude/relay_claude_test.go`、`relay/channel/claude/relay-claude.go`、`service/convert.go` 的连续冲突，并通过 `go test ./relay/channel/claude ./dto ./service` 与 `go test ./... -run '^$'`。
- Result-2 (2026-04-16): payment / topup / subscription 子批次完成，解决了 `controller/topup_stripe.go` 的两轮冲突，并通过 `go test ./controller ./model ./relay/helper ./relay/channel/gemini ./oauth ./service`、`go test ./... -run '^$'`。
- Result-3 (2026-04-16): OAuth / auth / security / ops 子批次完成，`web/src/helpers/api.js`、`controller/user.go`、`router/api-router.go`、`web/src/i18n/locales/*.json` 均已校验通过，并通过 `bun run build`。
- Result-4 (2026-04-16): `040e8c1d` 高重叠批次实际只在 `EditTokenModal.jsx` 与中英文 locale 出现冲突；完成手工合并后，重新带回 `d15e14b1`，并通过 `go test ./service ./dto ./relay/channel/claude`、`go test ./... -run '^$'` 与 `bun run build`。

### Step-11: 批次 B 计划（可选）

**Status:** Done

**AC:**

- 仅在批次 A 稳定后再处理 `可选` 列。
- 重点围绕：
  - channel affinity / retry 规则；
  - 服务端日志管理与 usage log UX；
  - playground / ErrorBoundary / 小型前端体验增强。
- 对 `controller/user.go`、`model/token.go`、`service/quota.go`、`service/task_billing.go` 这类重叠点逐文件评估，不允许盲目整批并入。

**Verification:**

```powershell
git cherry-pick <batch-b-commit-list>
go test ./...
cd web
bun run build
```

**Deliverables:**

- 批次 B 的候选提交列表
- 冲突热点文件逐项处理说明
- 功能回归验证记录

**Iteration Log:**

- Attempt-1 (Planned): 先做 channel affinity，再做 log management / usage log UX，最后做 playground / ErrorBoundary，小步前进。
- Attempt-2 (2026-04-16): 进入执行阶段；先从 `rockyicer` 切出 Batch B 集成分支，按 `channel affinity / log management / usage log / playground / frontend resilience` 五个子批次回引 upstream，并在每个子批次后执行冲突检查与验证。
- Result-1 (2026-04-16): `channel affinity / retry` 子批次完成，成功带入 `skip-retry` 默认模板、规则列表展示、`include_model_name` 后端与 UI 开关，以及 preferred channel disabled 时 honor skip-retry 的修复；两次冲突都集中在 `web/src/pages/Setting/Operation/SettingsChannelAffinity.jsx`，已保留本地 `buildChannelAffinityRulePayload` 结构并手工合并通过；验证通过 `go test ./service/... ./setting/operation_setting/...` 与 `cd web && bun run build`。
- Result-2 (2026-04-16): `log management / usage log UX` 子批次完成，成功带入服务端日志文件列表与清理 API、gin logger race 修复、局部删除失败提示、按钮对齐和 usage log `stream status` tooltip；冲突仅落在 `zh-CN/zh-TW` locale，按“保留当前分支既有 key，再补日志管理 key”策略手工合并；验证通过 `go test ./common ./controller ./logger ./router -run '^$'` 与 `cd web && bun run build`。
- Result-3 (2026-04-16): `playground / ErrorBoundary` 子批次完成，成功带入 `max_tokens` 输入处理增强与页面级 `ErrorBoundary`；`ErrorBoundary` 冲突仅在 locale 文件尾部，已只补两条错误页文案，不混入尚未计划回引的其他 key；验证通过 `cd web && bun run build`。
- Result-4 (2026-04-16): `EditChannelModal / token clipboard / regex ignored models` 子批次完成，成功带入 `regex:` 前缀忽略模型、复制连接信息、剪贴板自动填入及剪贴板错误保护；其中 `a706f002` 的“高级设置展开状态持久化”依赖 upstream 旧版折叠/侧滑结构，与你当前本地分段式 `EditChannelModal` 设计不兼容，直接合入会撕裂 JSX，因此最终保留前 3 个提交功能，不保留该持久化提交；验证通过 `go test ./controller -run ChannelUpstream -count=1` 与 `cd web && bun run build`。

### Step-12: 批次 C 计划（暂缓）

**Status:** Done

**AC:**

- 对 `暂缓` 列中的 dashboard、ratio settings、layout、branding 表现层改动，不在前两批中混入。
- 在是否需要长期接近 upstream UI/交互风格的问题上，先做一次产品层决策，再决定是否引入。
- 若决定引入，需单独建立专题集成分支，而不是混入批次 A/B。

**Verification:**

```powershell
git log --oneline rockyicer..upstream/main -- web/src/components/dashboard web/src/components/layout web/src/pages/Setting
```

**Deliverables:**

- 暂缓区专题清单
- 是否需要后续专题集成的决策门
- 避免冲掉 `JustAPI` 定制的边界说明

**Iteration Log:**

- Attempt-1 (Planned): 在批次 A/B 完成并稳定后，再决定是否开启 dashboard / settings / layout 的专题集成。
- Result-1 (2026-04-16): 已完成专题评估并明确“不在本轮引入 Batch C”。原因是 dashboard / ratio settings / layout / branding 仍然与 `JustAPI` 定制区高度重叠，且这类提交大多是表现层重写而非底层能力补丁；在 Batch A/B 已吸收主要功能增强的前提下，继续合入只会显著提升冲突成本并增加回归风险。
- Result-2 (2026-04-16): 本轮最终决策为“Step-12 完成为评估与冻结，而不是继续回引代码”。后续如要靠近 upstream UI，需要单独新开专题分支，围绕 `web/src/components/dashboard`、`web/src/components/layout`、`web/src/pages/Setting`、`web/src/i18n/locales/*` 做产品层再决策，而不是混入当前功能同步批次。

## 15. 靠近 Upstream UI 的专题集成计划（2026-04-16）

### 15.1 需求理解摘要

- 本专题承接 `Step-12` 的冻结结论，但目标已经从“是否继续引入”转为“如何在不冲掉 JustAPI branding 的前提下，继续靠近 upstream UI”。
- 已确认的产品方向不是整站回退到 upstream，而是：
  - `layout / dashboard` 尽量靠近 upstream；
  - `branding` 继续保留 `JustAPI`；
  - `ratio settings` 作为高价值但高冲突区，允许在专题分支中单独消化。
- 本专题明确排除“把 upstream 品牌外观整块覆盖本地站点”的路径；允许吸收 upstream 的容器结构、图表逻辑、设置页组织方式、通用 CSS 细节，但不允许把 `JustAPI` 的品牌入口、Logo、站点名、页脚文案、About/Contact/Docs 页面替换掉。

### 15.2 已确认边界

- `branding` 必须保留的范围已经确认如下：
  - 顶部导航的品牌名和入口结构；
  - 页脚文案与链接；
  - `About / Contact / Docs` 页面和跳转；
  - Logo、站点名。
- 因此，本专题中允许靠近 upstream 的区域是：
  - dashboard 信息组织、图表逻辑、辅助面板交互；
  - ratio settings 的页签结构、组合定价组件、可折叠规则区；
  - layout 的容器语义、局部响应式与 CSS polish；
  - footer 的语义结构和样式 class，但不包含品牌内容替换。
- 本专题默认不碰：
  - `HeaderBar` 中现有的站点品牌与主导航 IA；
  - `PageLayout` 中 `syncDocumentBranding(getSystemName(), getLogo())` 这条品牌同步链；
  - `Footer.jsx` 里的 JustAPI 专属文案、链接目标和分组名称；
  - `About / Contact / Docs` 对应页面与路由入口。

### 15.3 方案对比与选型

- 方案 A（已选，推荐）：“品牌壳 + upstream 内容核”
  - 保留当前 `HeaderBar / Footer / About / Contact / Docs` 作为品牌壳；
  - 只把 `dashboard`、`RatioSetting`、部分 `PageLayout` 的结构和交互向 upstream 对齐；
  - 对 `layout` 只吸收通用能力和样式 polish，不吸收 upstream 的品牌内容。
- 方案 B：整块替换 upstream layout/footer/dashboard，再把 JustAPI 品牌补回去。
  - 优点：视觉更接近 upstream 原貌；
  - 缺点：最容易把 JustAPI 导航、页脚和品牌内容先冲掉，再进入漫长回填，冲突风险最高。
- 方案 C：只引 dashboard 和 ratio settings，不动 layout。
  - 优点：最安全；
  - 缺点：整体 UI 收敛度不足，达不到“layout / dashboard 靠 upstream”的目标。
- 结论：
  - 采用方案 A；
  - 允许“内容核”更新，但品牌壳必须作为硬边界保留；
  - 因此所有 upstream UI 提交都要先按“整组引入 / 局部移植 / 明确排除”分层。

### 15.4 专题分支与批次设计

- 计划单开专题分支：`sync-upstream-ui-2026-04-16`
- 该分支内部继续拆成 3 个执行子批次，避免一次性 merge 整片前端改写：

1. `UI-1 Dashboard`
   - 目标：使 dashboard 靠近 upstream，但不碰站点品牌外壳。
   - 候选 upstream 提交：
     - `606a4eee feat(dashboard): add admin user analytics and fix chart labels`
     - `77897a81 feat(dashboard): enhance chart axes and update sorting logic`
     - `aafbd788 feat(dashboard): add copy button next to API link in API info panel`
     - `b2dd4acc fix(dashboard): 修复消耗分布图表悬浮时滚动条闪烁`
   - 涉及文件：
     - `controller/usedata.go`
     - `model/usedata.go`
     - `router/api-router.go`
     - `web/src/components/dashboard/ChartsPanel.jsx`
     - `web/src/components/dashboard/index.jsx`
     - `web/src/components/dashboard/ApiInfoPanel.jsx`
     - `web/src/helpers/dashboard.jsx`
     - `web/src/hooks/dashboard/useDashboardCharts.jsx`
     - `web/src/hooks/dashboard/useDashboardData.js`
     - `web/src/index.css`
     - `web/src/i18n/locales/*`
   - 引入策略：
     - `606a4eee` 与 `77897a81` 作为同一组处理，优先整组进入，因为图表行为和后端数据支撑耦合；
     - `aafbd788` 可独立进入；
     - `b2dd4acc` 仅在复现当前分布图 flicker 时进入，否则作为可选补丁保留。

2. `UI-2 Ratio Settings`
   - 目标：让倍率/定价设置页接近 upstream 新结构，但避免把 locale 大改和 token 表格无关差异整包卷入。
   - 候选 upstream 提交：
     - `dc83c4af refactor(settings): update RatioSetting component to use ModelPricingCombined and adjust tab structure`
     - `78e4cb3c feat(web): redesign group ratio rules with collapsible grouped layout`
     - `c2006093 fix(GroupTable): prevent Input cursor jumping to end on keystroke`
   - 经过文件级检查后的结论：
     - 早先提到的 `741aaf44` 实际只改 `SettingsChannelAffinity.jsx`，不属于本专题的 ratio settings 目标，本次从候选中移除。
   - 涉及文件：
     - `web/src/components/settings/RatioSetting.jsx`
     - `web/src/pages/Setting/Ratio/GroupRatioSettings.jsx`
     - `web/src/pages/Setting/Ratio/ModelPricingCombined.jsx`
     - `web/src/pages/Setting/Ratio/components/AutoGroupList.jsx`
     - `web/src/pages/Setting/Ratio/components/GroupGroupRatioRules.jsx`
     - `web/src/pages/Setting/Ratio/components/GroupSpecialUsableRules.jsx`
     - `web/src/pages/Setting/Ratio/components/GroupTable.jsx`
     - `web/src/components/table/tokens/TokensColumnDefs.jsx`
     - `web/src/components/table/tokens/TokensTable.jsx`
     - `web/src/components/table/tokens/modals/EditTokenModal.jsx`
     - `web/src/hooks/tokens/useTokensData.jsx`
     - `web/src/i18n/locales/*`
   - 引入策略：
     - `dc83c4af` 不建议直接整条 cherry-pick，因为它携带了大规模 locale 重排和 token table 联动改动，优先采用“手工移植结构 + 选择性摘取逻辑”的方式；
     - `78e4cb3c` 只建议局部摘取 `GroupGroupRatioRules.jsx` 与 `GroupSpecialUsableRules.jsx` 的折叠式布局思路，不建议整条直接进入；
     - `c2006093` 在 `GroupTable.jsx` 已落地后可作为独立修复整条引入。

3. `UI-3 Layout Polish`
   - 目标：只吸收 upstream 的 layout mechanics 和局部 polish，不替换 JustAPI 品牌内容。
   - 候选 upstream 提交：
     - `310d618a style: enhance footer layout and add custom class for styling`
     - `b2dd4acc fix(dashboard): 修复消耗分布图表悬浮时滚动条闪烁`（若 UI-1 未进入，可在此作为全局 CSS patch）
   - 明确排除：
     - `7399e472 feat: add slide-in animations and update translations for new UI elements`
   - 排除原因：
     - `7399e472` 实际重写了 `EditChannelModal.jsx` 并带入整批新 UI 文案，与本地 JustAPI 已经稳定的渠道编辑体验不相容；
     - 它不是纯 layout polish，而是泛化 UI 重写，超出本专题“保留品牌壳”的边界。
   - 涉及文件：
     - `web/src/components/layout/PageLayout.jsx`
     - `web/src/components/layout/Footer.jsx`
     - `web/src/index.css`
   - 引入策略：
     - `310d618a` 只允许部分 hunk 进入：保留 class / semantic / responsive polish，禁止覆盖页脚文案和链接；
     - `PageLayout.jsx` 若需要靠近 upstream，只能吸收容器结构、边栏折叠和滚动区组织，不得改变品牌同步逻辑与主导航信息架构。

### 15.5 验证、回滚与冲突处理策略

- 批次级验证门槛：
  - 每个子批次都必须独立完成冲突检查、功能自检和构建验证，不能等到 3 个子批次全部结束后再统一兜底。
  - 推荐验证顺序：
    1. `git diff --name-only --diff-filter=U` 确认无未解决冲突；
    2. 运行与该批次直接相关的 Go 定向测试；
    3. `cd web && bun run build` 做前端编译兜底；
    4. 必要时在浏览器中做人工 smoke test，重点核对 dashboard、ratio page、footer 与 branding 保留情况。
- 子批次推荐验证命令：
  - `UI-1 Dashboard`

```powershell
go test ./controller ./model ./router -run '^$' -count=1
cd web
bun run build
```

  - `UI-2 Ratio Settings`

```powershell
go test ./controller ./service ./setting/... -run '^$' -count=1
cd web
bun run build
```

  - `UI-3 Layout Polish`

```powershell
cd web
bun run build
```

- 回滚点设计：
  - 主题分支创建后，在每个子批次完成并验证通过后打一个清晰提交点，例如：
    - `ui-sync: dashboard upstream alignment`
    - `ui-sync: ratio settings upstream alignment`
    - `ui-sync: layout polish without branding overwrite`
  - 若某个子批次验证失败且修复成本继续膨胀，则只回退该子批次提交，不影响前序已验证通过的子批次。
- 冲突处理准则：
  - 任何改动只要触达以下文件，就默认进入“高冲突审查”：
    - `web/src/components/layout/PageLayout.jsx`
    - `web/src/components/layout/Footer.jsx`
    - `web/src/components/dashboard/*`
    - `web/src/components/settings/RatioSetting.jsx`
    - `web/src/pages/Setting/Ratio/*`
    - `web/src/i18n/locales/*`
  - `locales/*` 只补本专题真正需要的新 key，不整包替换；
  - 只要某个 upstream 提交同时修改品牌文案和通用布局，就必须拆 hunk，不能整条 cherry-pick；
  - 只要某个 upstream 提交同时修改 dashboard/ratio 目标文件和无关的渠道管理、品牌页、通用动画，就默认优先手工移植，不整条引入。

### 15.6 暴露的未知项（待用户回答）

- `UI-1 Dashboard` 的信息架构是否要完全跟 upstream 对齐：
  - 重点是 dashboard 卡片顺序、图表排列顺序、管理员专属分析模块的展示位置；
  - 若不完全对齐，需要明确哪些顺序保留当前 JustAPI 布局。
- `UI-1 Dashboard` 中 API 信息面板的 copy button 是否直接保留 upstream 行为：
  - 该改动风险很低，但会轻微改变当前面板操作节奏；
  - 默认建议保留。
- `UI-2 Ratio Settings` 是否允许“一次性切到 `ModelPricingCombined`”：
  - 若允许，冲突大但最终结构更接近 upstream；
  - 若不允许，则需要保留当前页面骨架，只吸收可折叠规则区和局部交互，集成工作量会更高。
- `UI-2 Ratio Settings` 的 locale 处理策略：
  - 是接受这一页相关的 locale 成片更新，还是坚持只补最小 key；
  - 默认建议只补最小 key，降低对你已有 JustAPI 文案的冲击。
- `UI-3 Layout Polish` 的吸收上限：
  - 只吸收 `Footer.jsx` 的 semantic/layout polish，还是连 footer 响应式对齐细节也一起带入；
  - 默认建议吸收结构和响应式，不吸收任何文案与链接替换。
- 主题分支的合流方式：
  - 是每个子批次验证通过后就回合到 `rockyicer`；
  - 还是 3 个子批次全部完成后再一次性回合；
  - 默认建议每个子批次单独验收并回合，便于风险切分。

### 15.7 用户已确认的执行决策（2026-04-16）

- `UI-1 Dashboard`
  - dashboard 卡片顺序、图表顺序、管理员分析模块位置：完全跟 upstream。
  - API 信息面板的 copy button：保留 upstream 行为。
- `UI-2 Ratio Settings`
  - 允许一次性切到 `ModelPricingCombined`。
  - 接受该页相关的 locale 成片更新。
- `UI-3 Layout Polish`
  - `Footer.jsx` 允许吸收更多 upstream 样式细节，但不得覆盖 JustAPI 的文案、链接与品牌内容。
- 主题分支合流方式
  - 每个子批次验证通过后就立即回合到 `rockyicer`。

### Step-13: 冻结 UI 专题方向与品牌边界

**Status:** Done

**AC:**

- 明确本专题不是整站回退 upstream branding，而是 `layout / dashboard` 靠 upstream、`branding` 保留 JustAPI。
- 明确 branding 保留范围至少包括：
  - 顶部导航的品牌名和入口结构；
  - 页脚文案与链接；
  - `About / Contact / Docs` 页面和跳转；
  - Logo、站点名。
- 将上述边界写回 `task_plan.md`，作为后续集成的硬约束。

**Verification:**

```powershell
rg -n "brand|branding|PageLayout|Footer|About|Contact|Docs" web/src/components/layout web/src/pages web/src/helpers
```

**Deliverables:**

- UI 专题的方向冻结记录
- branding 保留边界清单
- 不得覆盖的本地品牌元素列表

**Iteration Log:**

- Attempt-1 (2026-04-16): 基于 `Step-12` 的冻结结论继续追问产品边界，确认“layout / dashboard 靠 upstream，branding 保留 JustAPI”。
- Result (2026-04-16): 已完成方向冻结；品牌保留范围已明确到顶部导航、页脚、About/Contact/Docs、Logo、站点名。

### Step-14: 方案对比、推荐与选型落盘

**Status:** Done

**AC:**

- 至少形成 2-3 个方案并给出 trade-off。
- 明确推荐方案以及不推荐其他方案的原因。
- 将最终选型写回 `task_plan.md`。

**Verification:**

```powershell
git log --oneline rockyicer..upstream/main -- web/src/components/dashboard web/src/components/layout web/src/pages/Setting/Ratio web/src/components/settings/RatioSetting.jsx
```

**Deliverables:**

- 方案 A / B / C 的对比
- 推荐方案及理由
- 已确认采用的专题策略

**Iteration Log:**

- Attempt-1 (2026-04-16): 对比“品牌壳 + upstream 内容核”“先全量替换再补品牌”“只引 dashboard 不动 layout”三种路径。
- Result (2026-04-16): 已选择方案 A，因为它最符合“靠近 upstream UI，同时不冲掉 JustAPI branding”的目标。

### Step-15: 专题分支拆分、候选 commit 与引入方式设计

**Status:** Done

**AC:**

- 给出单独专题分支名称。
- 将专题拆成 `UI-1 Dashboard / UI-2 Ratio Settings / UI-3 Layout Polish` 三个子批次。
- 对每个子批次明确：
  - 候选 upstream commit；
  - 主要修改文件；
  - 整组引入 / 局部移植 / 明确排除 的处理方式。

**Verification:**

```powershell
git show --stat --oneline 606a4eee 77897a81 aafbd788
git show --stat --oneline dc83c4af 78e4cb3c c2006093
git show --stat --oneline 310d618a 7399e472 b2dd4acc
```

**Deliverables:**

- 专题分支名
- 三个子批次的 commit 清单
- 每个子批次的文件范围和引入方式

**Iteration Log:**

- Attempt-1 (2026-04-16): 先按 dashboard / ratio / layout 三块拆批次，再从 `git show --stat` 回看文件级影响面。
- Result (2026-04-16): 已完成分批设计；确认 `606a4eee + 77897a81` 适合作为 dashboard 核心组处理，`dc83c4af` 与 `78e4cb3c` 适合手工移植，`7399e472` 明确排除。

### Step-16: 验证标准、回滚点与冲突处理准则固化

**Status:** Done

**AC:**

- 为每个子批次给出最小验证命令集。
- 明确每个子批次完成后的回滚点策略。
- 明确 locale、layout、branding 重叠文件的冲突处理规则。

**Verification:**

```powershell
git diff --name-only --diff-filter=U
cd web
bun run build
```

**Deliverables:**

- 分批验证清单
- 回滚点设计
- 高冲突文件处置规则

**Iteration Log:**

- Attempt-1 (2026-04-16): 将“每批次都要检查是否和当前代码冲突、是否功能正确”的要求固化为统一验证和回滚规则。
- Result (2026-04-16): 已形成批次级验证、按提交点回滚和高冲突文件优先人工合流的统一准则。
- Result-2 (2026-04-16): 基于用户对 Step-17 的确认，Ratio Settings 允许页级成片 locale 更新；冲突处理以“限定在该专题页面范围内的整页 JSON 合并”为准，不再强行退回到最小 key 策略。

### Step-17: 待用户回答的未知项

**Status:** Done

**AC:**

- 把当前无法从本地代码独立推断、但会显著影响 UI 专题实现方式的未知项一次性列出。
- 未知项必须足够具体，用户可以直接逐条回答。
- 只有在这些未知项被确认后，后续执行步骤才允许从 `Not Started` 进入 `In Progress`。

**Verification:**

```powershell
Write-Output "等待用户回答 Step-17 中的未知项后继续执行"
```

**Deliverables:**

- 一份可逐条答复的未知项清单
- 每条未知项对应的默认建议
- 后续执行步骤的解锁条件

**Iteration Log:**

- Attempt-1 (2026-04-16): 将 dashboard 信息架构、ratio 切换深度、locale 策略、layout 吸收上限、分支合流方式整理为待答列表。
- Result-1 (2026-04-16): 当前 Blocked 于用户产品决策；默认建议已写入 `15.6 暴露的未知项（待用户回答）`。
- Result-2 (2026-04-16): 用户已完成全部答复，解锁条件已满足；最终决策已写入 `15.7 用户已确认的执行决策（2026-04-16）`。

### Step-18: 执行 UI-1 Dashboard 专题集成

**Status:** Done

**AC:**

- 在 `sync-upstream-ui-2026-04-16` 上完成 dashboard 子批次集成。
- 不破坏现有 JustAPI branding。
- dashboard 相关后端与前端修改在功能上保持自洽并通过构建验证。

**Verification:**

```powershell
git switch -c sync-upstream-ui-2026-04-16 rockyicer
git cherry-pick 606a4eee 77897a81
git cherry-pick aafbd788
cd web
bun run build
```

**Deliverables:**

- dashboard 专题集成提交
- 冲突处理记录
- dashboard smoke test 结论

**Iteration Log:**

- Attempt-1 (Planned): 先处理 `606a4eee + 77897a81`，确认后端 usedata 接口与图表逻辑一致，再决定是否补 `aafbd788` 与 `b2dd4acc`。
- Attempt-2 (2026-04-16): 用户已确认 dashboard 顺序完全跟 upstream，并保留 copy button；按该决策进入实现阶段，优先整组集成 `606a4eee + 77897a81`，再补 `aafbd788`，并视 flicker 复现情况决定是否引入 `b2dd4acc`。
- Attempt-3 (2026-04-16): 在 `sync-upstream-ui-2026-04-16` 上依次引入 `606a4eee`、`77897a81`、`aafbd788`、`b2dd4acc`；发生冲突的文件集中在 `web/src/components/dashboard/index.jsx`、`web/src/components/dashboard/ChartsPanel.jsx`、`web/src/hooks/dashboard/useDashboardCharts.jsx`、`web/src/hooks/dashboard/useDashboardData.js`。
- Result-1 (2026-04-16): 冲突已人工收口：保留本地日消耗图数据链路，吸收 upstream 的管理员分析模块、图表顺序、卡片顺序和 API 信息复制按钮，并把本轮触及代码内的注释统一为英文。
- Result-2 (2026-04-16): 验证通过 `go test ./controller ./model ./router -run '^$' -count=1` 和 `cd web && bun run build`；专题集成提交为 `b4d93c74`、`722621f3`、`1d0f0702`、`30c2ce5c`、`02b53fbb`，并已通过 `aa7ea9f6 merge: ui-1 dashboard upstream sync` 回合到 `rockyicer`。

### Step-19: 执行 UI-2 Ratio Settings 专题集成

**Status:** Done

**AC:**

- 在不冲掉现有 JustAPI 文案与无关 token 管理行为的前提下，引入 upstream 的 ratio settings 结构增强。
- `GroupTable.jsx` 的输入体验正确，页面可以完成构建。
- locale 改动范围受控，符合 Step-17 的最终决定。

**Verification:**

```powershell
cd web
bun run build
```

**Deliverables:**

- ratio settings 专题集成提交
- 手工移植与局部摘取说明
- locale 差异控制记录

**Iteration Log:**

- Attempt-1 (Planned): 先手工移植 `dc83c4af` 的页签结构与 `ModelPricingCombined` 入口，再局部摘取 `78e4cb3c` 的折叠规则区，最后补 `c2006093`。
- Attempt-2 (2026-04-16): 先直接尝试 `78e4cb3c`，发现当前树尚未具备其依赖的 ratio 组件基线，且与 locale 文件发生大面积冲突，因此中止该次 cherry-pick，改为先切到结构升级提交。
- Attempt-3 (2026-04-16): 改按 `dc83c4af` -> `c2006093` -> `78e4cb3c` 顺序执行；`dc83c4af` 和 `78e4cb3c` 仅在 6 个 locale 文件上冲突，使用 stage-2/stage-3 JSON 并集脚本合并，限定在 Ratio Settings 相关页面范围内。
- Result-1 (2026-04-16): 已一次性切换到 `ModelPricingCombined` 结构，并补齐折叠规则布局与 `GroupTable` 输入体验修复；最终专题提交为 `03d443cc`、`acaec8c8`、`5ce8359c`。
- Result-2 (2026-04-16): 验证通过 `go test ./controller ./service ./setting/... -run '^$' -count=1` 和 `cd web && bun run build`；本批次已通过 `b6b1411c merge: ui-2 ratio settings upstream sync` 回合到 `rockyicer`。

### Step-20: 执行 UI-3 Layout Polish 专题集成

**Status:** Done

**AC:**

- 在保留 `JustAPI` 品牌壳的前提下，引入 layout / footer 的 upstream polish。
- `Footer.jsx` 的文案、链接、分组名称不被 upstream 覆盖。
- `PageLayout.jsx` 不破坏品牌同步链、主导航入口和现有边栏行为。

**Verification:**

```powershell
cd web
bun run build
```

**Deliverables:**

- layout polish 专题集成提交
- `Footer.jsx` 局部移植说明
- branding 保留自检记录

**Iteration Log:**

- Attempt-1 (Planned): 只移植 `310d618a` 中与语义结构、class 名和响应式有关的 hunk；若 dashboard flicker 仍存在，再补 `b2dd4acc`。
- Attempt-2 (2026-04-16): 直接 cherry-pick `310d618a`；`web/src/index.css` 自动合并，`web/src/components/layout/Footer.jsx` 出现单点冲突。
- Result-1 (2026-04-16): 已保留 upstream 页脚容器结构、响应式布局和样式类细节，同时把品牌归属固定为 `JustAPI`，未让 upstream 的品牌文案覆盖本地页脚链接、站点名和 branding 语义。
- Result-2 (2026-04-16): `cd web && bun run build` 已通过；浏览器快照验收确认首页页脚仍显示 `JustAPI` 品牌与既有链接结构。专题提交为 `62cfaa84`，并已通过 `b18b6f62 merge: ui-3 layout polish upstream sync` 回合到 `rockyicer`。

### Step-21: UI 专题总验证、回合与验收

**Status:** Done

**AC:**

- `UI-1 / UI-2 / UI-3` 都完成且通过验证。
- 确认 dashboard、ratio settings、layout 都靠近 upstream，但 branding 仍保持 JustAPI。
- 形成是否回合到 `rockyicer` 的最终决策与提交策略。

**Verification:**

```powershell
go test ./controller ./model ./router -run '^$' -count=1
go test ./service ./setting/... -run '^$' -count=1
cd web
bun run build
```

**Deliverables:**

- UI 专题最终验收记录
- 是否回合到 `rockyicer` 的结论
- 后续 push / PR 建议

**Iteration Log:**

- Attempt-1 (Planned): 待 `Step-17` 解锁并完成 `Step-18` ~ `Step-20` 后执行总验证，再决定分批回合还是一次性回合。
- Attempt-2 (2026-04-16): 按用户要求执行“每个子批次通过后就回 `rockyicer`”；因此 `UI-1`、`UI-2`、`UI-3` 已分别通过 `aa7ea9f6`、`b6b1411c`、`b18b6f62` 合入主工作分支。
- Result-1 (2026-04-16): 总验证已在 `rockyicer` 上通过：`go test ./controller ./model ./router -run '^$' -count=1`、`go test ./service ./setting/... -run '^$' -count=1`、`cd web && bun run build`。
- Result-2 (2026-04-16): 目标达成：Dashboard 卡片/图表/管理员分析位置靠 upstream，API 信息复制按钮保留 upstream 行为；Ratio Settings 已切换到 `ModelPricingCombined` 并接受页面级 locale 更新；Footer/Layout 吸收了更多 upstream polish，同时顶部导航品牌名和入口结构、页脚文案与链接、About/Contact/Docs 页面与跳转、Logo 和站点名仍保持 JustAPI。
- Result-3 (2026-04-16): 本轮没有遗留“实在无法解决”的冲突；唯一需要调整执行顺序的点是 Ratio Settings 批次从原计划的“先摘 `78e4cb3c`”改为“先 `dc83c4af` 再 `c2006093` 再 `78e4cb3c`”，原因已记录在 Step-19。

## 16. 令牌设置追加：IP 黑名单（2026-04-16）

### 16.1 需求摘要

- 在令牌设置的“访问限制”区域新增 `IP 黑名单` 配置项。
- 凡是命中黑名单的客户端 IP，均不允许继续使用该令牌调用大模型接口。
- 保留现有 `IP 白名单（支持 CIDR 表达式）` 能力，并与新黑名单共同工作。

### 16.2 默认实现决策（如无进一步指示，按此执行）

- 新字段命名默认采用 `deny_ips`，与现有 `allow_ips` 对称。
- 输入格式与白名单完全一致：支持单 IP、CIDR 表达式、一行一个。
- 黑名单优先级高于白名单：
  - 若命中黑名单，直接拒绝；
  - 若未命中黑名单且配置了白名单，则仍需继续满足白名单。
- 首版只限制“使用令牌访问模型/relay 接口”的请求，不扩展到令牌管理页、用量查询页或普通控制台页面。
- 客户端 IP 继续复用 `c.ClientIP()` 与现有网关/反代链路，不单独引入新的来源头解析逻辑。
- 令牌列表页沿用现有 `IP限制` 列，但展示逻辑升级为可区分“白名单 / 黑名单 / 同时配置”的摘要，避免配置了黑名单却在列表中完全不可见。

### 16.3 待定参数（当前不阻塞计划落盘）

- 若你后续希望“黑名单也要拦截令牌用量查询接口”，可在实现阶段把范围从 relay-only 扩到 token usage/read-only session；当前计划默认不做。
- 若你希望“黑白名单冲突时以白名单优先”，需要改写 `16.2` 的默认优先级；当前计划默认“黑名单优先拒绝”。

### Step-22: 固化 IP 黑名单的数据契约与访问语义

**Status:** Done

**AC:**

- 明确新字段名、JSON 字段名、默认值和与 `allow_ips` 的组合行为。
- 明确拦截范围：仅限令牌访问模型接口，不影响令牌管理与普通前端页面。
- 明确优先级：黑名单先判定，白名单后判定。
- 明确错误返回语义，避免继续堆叠新的硬编码中文错误。

**Verification:**

```powershell
rg -n "AllowIps|allow_ips|TokenAuth|EditTokenModal|TokensColumnDefs|IsIpInCIDRList" model controller middleware web/src -S
```

**Deliverables:**

- 字段与优先级约定
- 访问范围边界
- 前后端落点清单

**Iteration Log:**

- Attempt-1 (2026-04-16): 已完成现状梳理；确认现有令牌白名单落点在 `model/token.go`、`controller/token.go`、`middleware/auth.go`、`web/src/components/table/tokens/modals/EditTokenModal.jsx`、`web/src/components/table/tokens/TokensColumnDefs.jsx`。
- Result-1 (2026-04-16): 数据契约已冻结：新字段采用 `deny_ips`，JSON 字段名同名，默认空字符串；输入格式与 `allow_ips` 保持一致，支持单 IP 与 CIDR，一行一个。
- Result-2 (2026-04-16): 访问语义已冻结：首版仅拦截令牌访问模型/relay 接口；判定顺序为“先黑名单、后白名单”；命中黑名单直接拒绝，未命中黑名单但配置了白名单时仍需继续满足白名单。
- Result-3 (2026-04-16): 错误返回策略已冻结：为“客户端 IP 无法解析 / 命中令牌 IP 黑名单 / 不在令牌白名单中”补充 i18n key，而不是继续沿用硬编码中文错误消息。

### Step-23: 后端模型、持久化与鉴权链路追加 `deny_ips`

**Status:** Done

**AC:**

- 在 `model.Token` 中新增 `DenyIps *string \`json:"deny_ips" gorm:"default:''"\``。
- 为黑名单提供与白名单对称的解析能力；优先复用现有 `GetIpLimits()` / `common.IsIpInCIDRList()` 的思路，避免重复造一套 CIDR 解析逻辑。
- `controller/token.go` 的新增/更新链路能正确接收、持久化并回传 `deny_ips`。
- `model.Token.Update()` 的字段白名单包含 `deny_ips`，并保持 token cache 失效/回填逻辑完整。
- `middleware/auth.go` 在令牌鉴权阶段先判断黑名单，再判断白名单；命中黑名单时返回 403。
- 为“客户端 IP 无法解析 / 命中令牌 IP 黑名单 / 不在令牌白名单中”补充明确的 i18n 消息 key，避免新增硬编码文案。
- 确认 SQLite / MySQL / PostgreSQL 下新增列行为安全；若 `AutoMigrate` 在测试/历史库场景下不足，再补充兼容性回填或测试 helper。

**Verification:**

```powershell
go test ./controller ./model ./middleware -run "Test.*Token.*IP|Test.*TokenAuth.*" -count=1
```

**Deliverables:**

- 后端字段与鉴权改造方案
- 错误消息与返回码约定
- 数据库兼容策略

**Iteration Log:**

- Attempt-1 (2026-04-16): 现状已确认：`allow_ips` 现由 `c.ShouldBindJSON(&token)` 直绑到 `model.Token`，实际拦截位于 `middleware/auth.go:351-365`；新增黑名单可沿用同一链路扩展。
- Attempt-2 (2026-04-16): 先按 TDD 补 `controller/token_test.go` 与新的 `middleware/auth_token_ip_test.go`，让 `deny_ips` 持久化和黑名单优先语义先变红，再进入实现。
- Result-1 (2026-04-16): 后端数据链路已补齐：`model.Token` 新增 `DenyIps *string`，`controller/token.go` 的 Add/Update 链路可接收、持久化并回传 `deny_ips`，`model.Token.Update()` 也已将 `deny_ips` 纳入字段白名单。
- Result-2 (2026-04-16): 鉴权链路已改为“先黑名单、后白名单”，复用现有 `common.IsIpInCIDRList()` 解析，不额外引入新的 IP 来源头；命中黑名单和白名单不匹配都返回 403，其中黑名单/白名单拒绝都附带 `access_denied` 错误码。
- Result-3 (2026-04-16): 后端 i18n 已补齐 `token.client_ip_invalid`、`token.ip_blacklisted`、`token.ip_not_allowed` 三个 key，并同步到 `en / zh-CN / zh-TW` locale；旧的硬编码中文错误已移除。
- Result-4 (2026-04-16): SQLite 历史测试库通过 `ensureTokenIPRuleColumns()` 兼容旧表结构；正式库继续依赖 `AutoMigrate(&Token{})` 自动补列，不需要额外一次性迁移。

### Step-24: 前端令牌设置与列表展示补齐黑名单入口

**Status:** Done

**AC:**

- `EditTokenModal.jsx` 的初始化表单值新增 `deny_ips: ''`。
- 在“访问限制”卡片中新增 `IP黑名单（支持CIDR表达式）` 文本域，位置与 `allow_ips` 同区块，文案含义清晰且不与白名单混淆。
- 新增占位文案与说明文案，例如“禁止的 IP，一行一个，不填写则不限制”“命中黑名单的 IP 将被拒绝使用该令牌调用模型接口”。
- `TokensColumnDefs.jsx` 的 `IP限制` 列能展示黑名单配置摘要，避免用户只能在编辑抽屉里看到黑名单。
- 补齐相关前端 i18n key，至少覆盖 `zh-CN / zh-TW / en / fr / ja / ru / vi`。

**Verification:**

```powershell
cd web
bun run build
```

**Deliverables:**

- 令牌编辑弹窗黑名单入口
- 列表摘要展示方案
- 前端多语言 key 清单

**Iteration Log:**

- Attempt-1 (2026-04-16): 已确认当前白名单 UI 落点在 `EditTokenModal.jsx:713-725`，现有列表展示落点在 `TokensColumnDefs.jsx:616-619`；黑名单可在同一“访问限制”卡片中对称追加。

### Step-25: 为黑名单补定向测试与回归用例

**Status:** Not Started

**AC:**

- 新增 controller 测试，覆盖 `deny_ips` 在 Add/Update 过程中的持久化。
- 新增 middleware 或等效集成测试，覆盖：
  - 命中黑名单时拒绝；
  - 未命中黑名单且满足白名单时放行；
  - 同时命中黑白名单时按“黑名单优先”拒绝；
  - 黑名单为空时不改变现有白名单行为。
- 保持现有令牌相关测试不回归。

**Verification:**

```powershell
go test ./controller -run "Test(Add|Update)Token.*DenyIps|TestUpdateTokenMasksKeyInResponse" -count=1
go test ./middleware -run "Test.*TokenAuth.*IP.*" -count=1
```

**Deliverables:**

- 黑名单持久化测试
- 黑白名单优先级测试
- 令牌鉴权回归结果

**Iteration Log:**

- Attempt-1 (2026-04-16): 已确认现有 `controller/token_test.go` 具备 Add/Update token 的测试基线，可直接追加 `deny_ips` 相关断言；中间件侧需要补新的 token auth IP 场景用例。
- Attempt-2 (2026-04-16): 新增失败测试后，控制器测试先按预期 red，失败点为 `deny_ips` 落库为空；中间件测试第一次误用了带 `-` 的 token key，触发了现有 Authorization 解析逻辑的截断，已调整测试数据后重新获得有效 red。
- Result-1 (2026-04-16): 新增控制器测试 `TestAddTokenPersistsDenyIps`、`TestUpdateTokenPersistsDenyIps`，覆盖新增/更新时的 `deny_ips` 持久化。
- Result-2 (2026-04-16): 新增中间件测试 `TestTokenAuthRejectsDeniedIP`、`TestTokenAuthAllowsWhitelistedIPWhenNotDenied`、`TestTokenAuthPrefersDenyListOverAllowList`、`TestTokenAuthKeepsAllowListBehaviorWithoutDenyList`，覆盖黑名单命中、黑名单优先、白名单放行和“无黑名单时保持现有白名单语义”。
- Result-3 (2026-04-16): 定向回归已通过：
  - `go test ./controller -run 'Test(GetAllTokensMasksKeyInResponse|SearchTokensMasksKeyInResponse|GetTokenMasksKeyInResponse|UpdateTokenMasksKeyInResponse|GetTokenKeyRequiresOwnershipAndReturnsFullKey|AddTokenPersistsPeriodQuotaSettings|AddTokenPersistsDenyIps|UpdateTokenResetsPeriodQuotaWindowWhenModeChanges|UpdateTokenPersistsDenyIps|AddTokenRejectsInvalidPeriodQuotaValues)$' -count=1`
  - `go test ./middleware -run 'TestTokenAuth(RejectsDeniedIP|AllowsWhitelistedIPWhenNotDenied|PrefersDenyListOverAllowList|KeepsAllowListBehaviorWithoutDenyList)$' -count=1`

### Step-26: 功能收口、页面验收与计划回写

**Status:** Not Started

**AC:**

- 在令牌设置弹窗中可以看到并保存 `IP 黑名单`。
- 使用命中黑名单的 IP 调用模型接口时被拒绝，且返回信息清晰。
- 未命中黑名单的正常 IP 不会被误伤。
- `task_plan.md` 记录最终实现结果、冲突处理和验证结论。

**Verification:**

```powershell
go test ./controller ./model ./middleware -run "Test.*Token.*IP|Test.*TokenAuth.*" -count=1
cd web
bun run build
```

**Deliverables:**

- 功能验收记录
- 回归验证记录
- 最终收口说明

**Iteration Log:**

- Attempt-1 (2026-04-16): 当前仅追加计划，尚未进入实现阶段；执行时将优先按 Step-23 -> Step-24 -> Step-25 -> Step-26 推进。
