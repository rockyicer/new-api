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
