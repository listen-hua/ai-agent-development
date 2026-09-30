# 微光 AI 生图模块代码边界与独立拆分交接清单

> 用途：把当前单体中的“AI 生图”独立成单独服务或单独项目时，供开发、测试和运维逐项核对。  
> 基线：当前工作区代码（包括尚未提交的多画布、Pixian 抠图、GPT Image 2 分辨率路由等最新实现）。  
> 标记含义：`[MOVE]` 整体迁入生图项目；`[ADAPT]` 是共享文件，只抽取其中生图部分；`[KEEP]` 留在微光主站，通过链接或接口对接；`[COMPAT]` 兼容代码，确认无旧客户端后删除。

## 1. 模块边界结论

AI 生图当前不是一个完全独立的进程，而是微光模块化单体中的可选模块：

```text
员工/IAM/飞书身份
        │ 内部用户 UUID + 最终权限
        ▼
Vue 生图页面 ── /api/v1/image-agent/* ── Go ImageAgent
        │                                  ├─ PostgreSQL 生图表
        │                                  ├─ MinIO 图片对象
        │                                  ├─ ClamAV 文件扫描
        │                                  ├─ XGAPI / Comfly
        │                                  └─ Pixian.ai
        │
        └─ Worker 每 5 秒领取生图/抠图任务并回写画布
```

独立时建议边界如下：

- 新生图服务拥有：中转站、模型、项目、快捷提示词、画布、节点、图片资产、生图任务、Pixian 配置与任务。
- 微光主站继续拥有：IAM/飞书登录、内部用户、组织架构、统一权限管理、导航入口。
- 两边通过可信身份令牌传递 `user_id`、`feishu_open_id`、部门/职位与最终权限；不能信任浏览器自己提交的用户信息。
- 生图服务继续检查 `agent_use`（员工使用）和 `image_manage`（后台管理），项目 ACL 继续在生图服务端执行。
- 图片二进制由生图服务自己的 MinIO Bucket 管理，不建议继续经过微光主站数据库或文件服务。

## 2. 后端代码清单

### 2.1 可整体迁移的生图代码 `[MOVE]`

| 文件 | 职责 | 独立后处理 |
|---|---|---|
| `backend/internal/domain/image_agent.go` | 生图全部领域实体、JSON DTO、状态字段 | 迁入新服务的 `domain` 包 |
| `backend/internal/httpapi/image_agent.go` | 员工端、管理端 HTTP Handler；上传、画布、任务、资源读取 | 接入新服务认证中间件 |
| `backend/internal/httpapi/image_background_removal.go` | Pixian 配置、测试、统计和抠图任务 Handler | 与 Pixian 服务一起迁移 |
| `backend/internal/service/image_agent.go` | 核心业务：ACL、配置、画布、资源、任务、预览图、安全校验、审计 | 保留为生图领域服务 |
| `backend/internal/service/image_agent_worker.go` | 生图及抠图 Worker、重试、结果保存、画布节点更新、清理 | 拆成新服务的 worker 进程 |
| `backend/internal/service/image_background_removal.go` | Pixian 配置、连接测试、任务创建和校验 | 整体迁移 |
| `backend/internal/integration/imageproxy/client.go` | XGAPI/Comfly/OpenAI 兼容调用；Chat、Images、通用 edits、GPT Image 2 像素映射和安全下载 | 整体迁移；名称可改为 `relay` |
| `backend/internal/integration/pixian/client.go` | Pixian Basic Auth、multipart 上传、余额和响应头解析 | 整体迁移 |
| `backend/internal/store/image_agent_postgres.go` | 生图 PostgreSQL 实现、事务、乐观锁、任务领取 | 整体迁移 |
| `backend/internal/store/image_background_removal_postgres.go` | Pixian PostgreSQL 实现 | 整体迁移 |
| `backend/internal/store/image_agent_memory.go` | 生图内存仓库，主要用于测试 | 迁入测试/开发实现 |
| `backend/internal/store/image_background_removal_memory.go` | Pixian 内存仓库 | 迁入测试/开发实现 |

对应测试也一并迁移：

- `backend/internal/integration/imageproxy/client_test.go`
- `backend/internal/integration/pixian/client_test.go`
- `backend/internal/service/image_agent_test.go`
- `backend/internal/service/image_agent_resolution_test.go`
- `backend/internal/store/image_agent_memory_test.go`

### 2.2 共享文件中的生图接缝 `[ADAPT]`

| 文件 | 当前生图接缝 | 独立后修改 |
|---|---|---|
| `backend/internal/store/repository.go` | `ImageRepository` 全接口位于此文件第 122 行附近；审计使用主 `Repository.AppendAudit` | 把 `ImageRepository` 移到生图服务；将审计抽象为最小 `AuditWriter` |
| `backend/internal/store/memory.go` | 组合生图内存数据结构 | 主站删除对应字段，新服务保留 |
| `backend/internal/store/memory_test.go` | 共享仓库测试含生图数据 | 拆分测试 |
| `backend/internal/httpapi/server.go` | 注入 `*service.ImageAgent`；注册 `/api/v1/image-agent/*` 和 `/api/v1/admin/image-agent/*`；检查 `image_manage` | 主站移除这些路由，网关把路径转发给新服务 |
| `backend/cmd/server/main.go` | 构造 `ImageRepository`、`ImageAgent`、默认配置并注入 HTTP Server | 新服务创建独立 `cmd/api`；主站移除构造 |
| `backend/cmd/worker/main.go` | 构造 `ImageDispatcher`，与提醒/会议/按摩共用 5 秒 Tick | 新服务创建独立 `cmd/worker`，只处理生图与抠图 |
| `backend/internal/service/agent_registry.go` | 默认注册 `agent_key=image_generator` | 主站可保留入口元数据，也可改成外部应用注册信息 |
| `backend/internal/config/config.go` | 读取 MinIO、ClamAV、加密密钥和 `GEMINI_IMAGE_MODEL` | 新生图服务使用自己的配置结构 |
| `backend/internal/config/config_test.go` | 校验上述环境变量 | 随配置拆分 |
| `backend/internal/blob/minio.go` | 共享 `blob.Store`，保存图片和制度文件 | 复制接口与实现到新服务，使用独立 Bucket/前缀 |
| `backend/internal/security/clamav.go` | 共享上传扫描接口 `security.Scanner` | 复制接口与实现，仍可访问同一 ClamAV 服务 |
| `backend/internal/domain` 中的 `User`、`ACL`、`AuditEvent` | 生图服务直接依赖主站用户、资源 ACL 和审计类型 | 在新服务定义稳定的身份 Claims、项目 ACL 与审计 DTO |

`ImageAgent` 当前构造函数明确显示了全部运行依赖：

```go
NewImageAgent(
    repo store.ImageRepository,
    audit store.Repository,
    blobs blob.Store,
    scanner security.Scanner,
    encryptionSecret string,
    registries ...*AgentRegistry,
)
```

拆分后应缩小为 `ImageRepository + AuditWriter + ObjectStore + Scanner + SecretCipher`，不要把微光主站的整个 `Repository` 带进新项目。

### 2.3 后端路由清单

员工接口，要求已认证且最终拥有 `agent_use`：

```text
GET    /api/v1/image-agent/options
GET    /api/v1/image-agent/prompt-actions
GET    /api/v1/image-agent/prompt-actions/{id}/preview
GET    /api/v1/image-agent/canvases
POST   /api/v1/image-agent/canvases
GET    /api/v1/image-agent/canvases/trash
GET    /api/v1/image-agent/canvases/{canvasID}
PATCH  /api/v1/image-agent/canvases/{canvasID}
DELETE /api/v1/image-agent/canvases/{canvasID}
POST   /api/v1/image-agent/canvases/{canvasID}/restore
DELETE /api/v1/image-agent/canvases/{canvasID}/nodes/{nodeID}
POST   /api/v1/image-agent/canvases/{canvasID}/nodes/batch-delete
POST   /api/v1/image-agent/canvases/{canvasID}/imports
POST   /api/v1/image-agent/canvases/{canvasID}/background-removal-jobs
GET    /api/v1/image-agent/background-removal-jobs/{jobID}
POST   /api/v1/image-agent/assets
GET    /api/v1/image-agent/assets/{id}/content
POST   /api/v1/image-agent/jobs
GET    /api/v1/image-agent/jobs
GET    /api/v1/image-agent/jobs/{id}
```

管理接口，要求最终拥有 `image_manage`：

```text
GET/POST       /api/v1/admin/image-agent/relays
PUT            /api/v1/admin/image-agent/relays/{id}
POST           /api/v1/admin/image-agent/relays/{id}/test
POST           /api/v1/admin/image-agent/relays/{id}/models/sync
GET            /api/v1/admin/image-agent/models
PUT            /api/v1/admin/image-agent/models/{id}
GET/POST       /api/v1/admin/image-agent/projects
PUT/DELETE     /api/v1/admin/image-agent/projects/{id}
GET/POST       /api/v1/admin/image-agent/prompt-actions
PUT/DELETE     /api/v1/admin/image-agent/prompt-actions/{id}
PUT/DELETE     /api/v1/admin/image-agent/prompt-actions/{id}/preview
GET/PUT        /api/v1/admin/image-agent/background-removal/config
POST           /api/v1/admin/image-agent/background-removal/test
POST           /api/v1/admin/image-agent/background-removal/account/refresh
GET            /api/v1/admin/image-agent/background-removal/statistics
GET            /api/v1/admin/image-agent/background-removal/jobs
```

兼容接口 `[COMPAT]`，新项目不要继续使用：

```text
GET    /api/v1/image-agent/projects/{id}/canvas
PATCH  /api/v1/image-agent/projects/{id}/canvas
DELETE /api/v1/image-agent/projects/{id}/canvas/nodes/{nodeID}
POST   /api/v1/image-agent/projects/{id}/canvas/imports
```

它们属于旧的“一个项目一张画布”模型。当前前端已经使用 `/canvases/{canvasID}`。

## 3. 前端代码清单

### 3.1 可整体迁移 `[MOVE]`

页面和路由目标：

- `frontend/src/views/ImageCanvasListView.vue`：画布列表入口。
- `frontend/src/views/ImageAgentView.vue`：无限画布工作区。
- `frontend/src/views/admin/ImageAgentAdminView.vue`：生图管理总页。

数据、服务和状态编排：

- `frontend/src/types/image-agent.ts`：全部生图前端类型。
- `frontend/src/services/image-agent.ts`：员工/管理 API、资源 URL、响应兼容归一化。
- `frontend/src/composables/image-agent/useImageWorkspace.ts`：画布加载、项目/模型选择、生图、反推、参考图、保存队列、轮询、导入、删除、抠图。
- `frontend/src/composables/image-agent/useImageCanvasList.ts`：多画布、回收站。
- `frontend/src/composables/image-agent/useImageAgentAdmin.ts`：中转站、模型、项目、快捷按键后台状态。
- `frontend/src/composables/image-agent/useBackgroundRemovalAdmin.ts`：Pixian 后台状态。

员工组件目录 `frontend/src/components/image-agent/`：

| 组件 | 职责 |
|---|---|
| `InfiniteImageCanvas.vue` | Vue Flow 事件协调、框选、多选、平移、粘贴/拖入、智能吸附、批量删除/抠图 |
| `ImageCanvasNode.vue` | 图片节点、生成状态、删除、参考图手柄、分辨率与来源悬浮信息 |
| `ImageOperationPanel.vue` | 文生图表单、项目/中转站/模型、比例、质量、数量、快捷按键 |
| `ReferenceImageSlots.vue` | 最多 5 张参考图和画布拖入 |
| `ImagePromptActionButton.vue` | 快捷提示词与预览图悬浮层 |
| `ImageGenerationSourceBadge.vue` | 中转站、显示模型、实际模型 ID、Pixian 来源标识 |
| `CanvasAlignmentGuides.vue` | 紫色对齐线和 24px 间距提示 |
| `CartoonStrengthControl.vue` | 卡通化强度五档控制 |
| `ImageCanvasListCard.vue` | 画布卡片及首图预览 |
| `ImageCanvasCreateDialog.vue` | 新建画布 |
| `ImageCanvasTrashDrawer.vue` | 30 天回收站和恢复 |

后台组件目录 `frontend/src/components/image-agent-admin/`：

- `ImageRelayAdminPanel.vue`
- `ImageModelAdminPanel.vue`
- `ImageProjectAdminPanel.vue`
- `ImagePromptActionAdminPanel.vue`
- `PromptActionPreviewPicker.vue`
- `ImageBackgroundRemovalAdminPanel.vue`

纯工具函数：

- `frontend/src/utils/imageCanvasLayout.ts`：自动排版、碰撞避让、吸附候选、参考线。
- `frontend/src/utils/imageCanvasPersistence.ts`：坐标容差、布局指纹、服务端/本地节点合并。
- `frontend/src/utils/imageCanvasImport.ts`：剪贴板、外部文件拖入校验。
- `frontend/src/utils/imagePrompt.ts`：快捷提示词、卡通化字段、15,000 字限制相关逻辑。
- `frontend/src/utils/imagePromptPreview.ts`：预览图 URL 和状态。
- `frontend/src/utils/imageModelSearch.ts`：模型模糊搜索。

上述目录中的 `*.test.ts` 均属于生图模块测试，应随模块迁移。

### 3.2 主站接缝 `[ADAPT]/[KEEP]`

| 文件 | 生图相关内容 | 独立后建议 |
|---|---|---|
| `frontend/src/router/index.ts` | `/image-agent`、`/image-agent/canvases/:canvasId`、`/admin/image-agent` 三条路由 | `[KEEP]` 改成外部地址或微前端挂载入口；也可由网关保持原 URL |
| `frontend/src/components/layout/AppShell.vue` | “AI 画图”“生图管理”菜单、标题和激活判断 | `[KEEP]` 保留导航，目标指向独立应用 |
| `frontend/src/services/api.ts` | Axios、运行时 API 地址、Cookie、401 Session 恢复 | `[ADAPT]` 新生图前端复用协议，但不要直接复制主站 Auth Store 耦合 |
| `frontend/src/stores/auth.ts` | `agent_use`、`image_manage` 权限结果 | `[KEEP]` 主站负责登录；独立应用消费可信权限接口或短期令牌 |
| `frontend/src/main.ts` | Element Plus、全局样式和启动流程 | `[ADAPT]` 新生图前端建立自己的入口 |
| `frontend/package.json` | Vue Flow、Vue、Element Plus、Axios 等依赖 | `[ADAPT]` 拆分出最小依赖 |

生图前端直接依赖的关键包：

```text
vue 3.5
vue-router
pinia
axios
element-plus
@vue-flow/core 1.48.2
@vue-flow/background
@vue-flow/controls
@vue-flow/minimap
```

## 4. 数据库与迁移清单

字段级结构、实体关系、状态说明和交接查询示例见 [`IMAGE_AGENT_DATABASE_SCHEMA.md`](./IMAGE_AGENT_DATABASE_SCHEMA.md)。

### 4.1 生图拥有的表

基础迁移 `backend/migrations/011_image_agent.sql` 创建：

```text
image_relays             中转站及加密 API Key、Base URL、允许下载域名
image_models             远端模型、协议、能力和开放状态
image_projects           公司项目、启用状态和资源 ACL
image_prompt_actions     快捷提示词、项目作用范围和预览图元数据
image_canvases           用户私人画布、视口、版本和软删除
image_assets             MinIO 对象元数据、来源、尺寸、所属用户/项目
image_jobs               生图/反推异步任务、租约、重试和幂等键
image_canvas_nodes       画布节点、坐标、状态、资产和来源快照
image_job_outputs        每个输出的状态、资产和分辨率审计
```

Pixian 迁移 `025_pixian_background_removal.sql` 创建：

```text
pixian_background_removal_config  加密凭证、启停、测试模式、超时、并发、余额
background_removal_jobs           抠图批次任务
background_removal_items          每张源图与输出图的状态和扣费信息
```

### 4.2 必须按顺序带走的迁移

| 迁移 | 内容 |
|---|---|
| `011_image_agent.sql` | 生图基础表、默认中转站和默认项目 |
| `012_comfly_image_output_host.sql` | Comfly 输出域名兼容 |
| `015_comfly_apiproxy_output_host.sql` | Comfly 历史代理域名 |
| `017_image_prompt_action_previews.sql` | 快捷按键预览图 |
| `020_image_canvas_multi.sql` | 多画布、回收站、解除项目一对一 |
| `021_xgapi_image_output_host.sql` | XGAPI 输出域名 |
| `022_image_prompt_action_multi_projects.sql` | 快捷按键多项目作用范围 |
| `023_image_original_ratio.sql` | `original` 原图比例 |
| `024_xgapi_apiproxy_output_host.sql` | XGAPI 代理域名补充 |
| `025_pixian_background_removal.sql` | Pixian 配置、任务、资产血缘 |
| `026_comfly_aiproxy_output_host.sql` | Comfly 新代理域名补充 |
| `027_gpt_image_2_resolution.sql` | GPT Image 2 协议、请求/实际分辨率字段 |
| `028_image_node_generation_source.sql` | 节点生成中转站与模型快照 |
| `029_xgapi_base_url.sql` | XGAPI 新 Base URL |
| `030_image_model_relay_routing.sql` | 实际请求模型 ID、远端端点能力 |
| `031_gpt_image_2_pixel_sizes.sql` | Comfly/XGAPI GPT Image 2 专用协议修正 |

`016_unified_permissions.sql` 是主站统一权限迁移 `[KEEP]`，不要直接复制整份到生图数据库。新服务只需接受并检查 `agent_use` 与 `image_manage` 两个最终权限键。

### 4.3 数据迁移顺序

如果要带现有生产数据：

1. 先在新库完整执行上述结构迁移。
2. 按外键顺序导出：`image_relays → image_models → image_projects → image_prompt_actions → image_canvases → image_assets → image_jobs → image_canvas_nodes → image_job_outputs → Pixian tables`。
3. 用户 ID 必须保持与微光主站内部 UUID 的映射，或建立 `external_user_id` 映射表。
4. 同步复制 MinIO 对象，并保持 `object_key` 不变。
5. 最后校验表记录数、画布节点资产引用、任务输出资产引用和对象存在性。
6. API Key/Pixian 密文只有在新服务继续使用同一个 `AGENT_SECRET_ENCRYPTION_KEY` 时才能解密。

## 5. 外部服务和安全边界

### 5.1 中转站

- XGAPI 当前 Base URL：`https://api.xgapiproxy.win/v1`（由迁移修正）。
- Comfly 当前 Base URL：配置表为准。
- 模型协议：`chat_completions`、`images_generations`、`gpt_image_2`；XGAPI Gemini 参考图还有通用 edits 路由。
- `request_model_id` 是实际发给中转站的 ID；`model_id/display_name` 是员工选择和展示值。
- GPT Image 2 使用 `/images/generations` 或 `/images/edits`，分辨率通过真实 `size=宽x高` 传递。
- 远程图片 URL 必须通过 `allowed_output_hosts` 校验，禁止为了省事关闭 SSRF 白名单。

### 5.2 Pixian

- 固定服务：`https://api.pixian.ai/api/v2/remove-background`。
- HTTP Basic Auth，凭证只保存在服务端加密字段。
- 输出必须是可解码透明 PNG，通过 MIME、像素、大小和 ClamAV 校验后才写入 MinIO。

### 5.3 文件与对象存储

- 上传支持 JPG/PNG/WEBP；业务层执行大小、真实 MIME、图片解码和项目 ACL 校验。
- 图片资源读取必须走 `GET /assets/{id}/content` 并校验用户所有权/项目权限，不能公开 MinIO 地址。
- 预览图、参考图、上传图、生成图和抠图结果都在对象存储，数据库只保存元数据和 `object_key`。
- 独立部署建议使用单独 Bucket，例如 `shimmer-image-agent`，避免清理策略误伤制度文件。

## 6. 权限、身份和审计对接契约

### 6.1 推荐身份令牌 Claims

微光主站完成 IAM/飞书认证后，签发 5 分钟短期服务令牌给生图模块：

```json
{
  "iss": "shimmer-core",
  "aud": "shimmer-image-agent",
  "sub": "内部用户UUID",
  "name": "员工姓名",
  "feishu_open_id": "脱敏示例",
  "department_ids": ["部门ID"],
  "position_ids": ["职位ID"],
  "permission_keys": ["agent_use", "image_manage"],
  "exp": 0,
  "jti": "一次令牌ID"
}
```

要求：

- 新生图服务校验签名、`iss`、`aud`、过期时间和时钟偏差。
- 普通接口要求 `agent_use`，管理接口要求 `image_manage`。
- 项目 ACL 仍使用可信的部门、职位、人员信息做资源级校验。
- 浏览器传来的 `user_id`、部门和权限字段一律不可信。
- 本地开发身份只能在显式开发模式启用。

### 6.2 审计

当前生图操作通过主仓库的 `AppendAudit` 写入统一审计表。独立后有两种选择：

1. 推荐：生图服务本地保存不可覆盖审计，再异步投递摘要到微光审计中心。
2. 过渡：实现 `AuditWriter` HTTP 客户端，调用微光内部审计接口；失败时本地 Outbox 重试，不能因为审计中心短暂不可用丢事件。

## 7. Worker 与一致性要点

- `ImageDispatcher.Tick` 当前由统一 Worker 每 5 秒调用。
- PostgreSQL 使用任务状态、`next_attempt_at`、`locked_until` 和 Claim 方法领取任务；多 Worker 时依靠数据库租约避免重复执行。
- 生图数量 1/2/4/8 会拆成输出处理；结果支持部分成功。
- 网络、429、5xx按策略重试；模型无通道/不存在等业务错误不重复扣费重试。
- 画布使用 `version` 乐观锁；前端旧保存响应不能覆盖本地新坐标。
- 生图占位节点、输出节点、抠图占位节点都与任务绑定。删除节点不等于删除任务或图片资产。
- 软删除画布保留 30 天；运行中任务可完成，恢复后可见。
- 独立服务必须同时部署 API 和 Worker，只有 API 会导致任务一直处于排队状态。

## 8. 配置与部署清单

必须提供：

```text
DATABASE_URL                         生图独立 PostgreSQL
REDIS_ADDR/REDIS_PASSWORD            若保留缓存/限流；核心任务仍在 PostgreSQL
MINIO_ENDPOINT
MINIO_ACCESS_KEY
MINIO_SECRET_KEY
MINIO_BUCKET                         建议独立 Bucket
MINIO_SECURE
CLAMAV_ADDR
AGENT_SECRET_ENCRYPTION_KEY          解密中转站和 Pixian 密钥；必须永久保存
GEMINI_IMAGE_MODEL                   默认模型元数据兼容项
SESSION/JWT 验签配置                 与微光身份契约一致
```

运行单元：

```text
image-api       Go HTTP API
image-worker    与 API 同镜像，运行 Worker 入口
image-web       Vue 静态站点（若不作为微前端打包）
postgres        生图业务真源
minio           图片对象
clamav          上传和输出扫描
```

Redis不是生图任务的业务真源；任务状态、租约和幂等性在 PostgreSQL。Tika/pgvector属于制度知识库，独立生图服务不需要。

## 9. 建议的独立项目目录

```text
shimmer-image-agent/
├─ backend/
│  ├─ cmd/api/
│  ├─ cmd/worker/
│  ├─ internal/domain/
│  ├─ internal/httpapi/
│  ├─ internal/service/
│  ├─ internal/store/
│  ├─ internal/integration/imageproxy/
│  ├─ internal/integration/pixian/
│  ├─ internal/blob/
│  ├─ internal/security/
│  └─ migrations/
├─ frontend/
│  └─ src/{views,components,composables,services,types,utils}/
├─ deploy/
│  ├─ docker-compose.yml
│  └─ nginx.conf
└─ docs/
   └─ API_AND_AUTH_CONTRACT.md
```

## 10. 实际拆分步骤

1. 冻结本清单所列接口和领域 JSON，先不改用户可见行为。
2. 从共享代码中抽出 `ImageRepository`、`AuditWriter`、`ObjectStore`、`Scanner` 和身份 Claims。
3. 建立独立数据库并执行 011、012、015、017、020–031 中与生图有关的迁移。
4. 迁移 Go 生图包，创建独立 API/Worker 入口，跑全量 Go 测试。
5. 迁移 Vue 页面、组件、composable、service、type、utils 和测试。
6. 在公司网关保持 `/api/v1/image-agent/*` 与 `/api/v1/admin/image-agent/*` 路径不变，先代理到新服务，降低前端切换风险。
7. 接入微光签发的短期身份令牌，做 IAM 登录、飞书免登、普通员工、管理员和项目 ACL 验收。
8. 复制 MinIO 对象和数据库数据，校验对象引用；短暂停写后做最终增量同步。
9. 切换 Worker，确保同一时刻只有旧或新的一套 Worker 能领取生图任务。
10. 观察无重复任务、无 401、无图片 404、无画布版本循环后，再从微光单体删除旧实现。

## 11. 对接验收清单

- [ ] IAM 与飞书入口映射到同一内部用户 UUID。
- [ ] 普通员工只有 `agent_use`，不能访问管理接口。
- [ ] `image_manage` 管理员可维护中转站、模型、项目、快捷按键和 Pixian。
- [ ] 项目部门/人员/职位 ACL 在服务端生效。
- [ ] 多画布、新建、回收站、恢复、首图预览正常。
- [ ] 框选、多选拖动、吸附、批量删除和粘贴/外部拖入正常。
- [ ] 参考图和“原图”比例正常，最多 5 张。
- [ ] XGAPI、Comfly 的 Gemini/GPT Image 2 路由和尺寸正确。
- [ ] 返回低分辨率时保留图片并展示实际像素，不重复扣费重试。
- [ ] 图片悬停显示生成中转站、显示模型和实际请求模型。
- [ ] Pixian 抠图、透明 PNG、来源继承、余额和扣费统计正常。
- [ ] API 重启不丢任务；Worker 重启可继续处理。
- [ ] 多 Worker 不重复领取任务；幂等键不重复创建任务。
- [ ] MinIO 对象不可匿名访问，跨用户/跨项目资源读取返回 403/404。
- [ ] 加密密钥、API Key、Pixian Secret 不出现在响应、日志和前端构建产物。
- [ ] 旧项目画布兼容接口下线前已确认没有客户端调用。

## 12. 完整文件清单（交接时逐项勾选）

以下是当前代码基线中应纳入生图拆分评审的完整清单。测试文件和实现文件必须一起迁移，不能只复制页面。

### 后端实现与测试

```text
[ADAPT] backend/cmd/server/main.go
[ADAPT] backend/cmd/worker/main.go
[ADAPT] backend/internal/blob/minio.go
[ADAPT] backend/internal/config/config.go
[ADAPT] backend/internal/config/config_test.go
[MOVE]  backend/internal/domain/image_agent.go
[MOVE]  backend/internal/httpapi/image_agent.go
[MOVE]  backend/internal/httpapi/image_background_removal.go
[ADAPT] backend/internal/httpapi/server.go
[MOVE]  backend/internal/integration/imageproxy/client.go
[MOVE]  backend/internal/integration/imageproxy/client_test.go
[MOVE]  backend/internal/integration/pixian/client.go
[MOVE]  backend/internal/integration/pixian/client_test.go
[ADAPT] backend/internal/security/clamav.go
[ADAPT] backend/internal/service/agent_registry.go
[MOVE]  backend/internal/service/image_agent.go
[MOVE]  backend/internal/service/image_agent_test.go
[MOVE]  backend/internal/service/image_agent_resolution_test.go
[MOVE]  backend/internal/service/image_agent_worker.go
[MOVE]  backend/internal/service/image_background_removal.go
[MOVE]  backend/internal/store/image_agent_postgres.go
[MOVE]  backend/internal/store/image_agent_memory.go
[MOVE]  backend/internal/store/image_agent_memory_test.go
[MOVE]  backend/internal/store/image_background_removal_postgres.go
[MOVE]  backend/internal/store/image_background_removal_memory.go
[ADAPT] backend/internal/store/repository.go
[ADAPT] backend/internal/store/memory.go
[ADAPT] backend/internal/store/memory_test.go
```

### 数据库

```text
[MOVE]  backend/migrations/011_image_agent.sql
[MOVE]  backend/migrations/012_comfly_image_output_host.sql
[MOVE]  backend/migrations/015_comfly_apiproxy_output_host.sql
[KEEP]  backend/migrations/016_unified_permissions.sql
[MOVE]  backend/migrations/017_image_prompt_action_previews.sql
[MOVE]  backend/migrations/020_image_canvas_multi.sql
[MOVE]  backend/migrations/021_xgapi_image_output_host.sql
[MOVE]  backend/migrations/022_image_prompt_action_multi_projects.sql
[MOVE]  backend/migrations/023_image_original_ratio.sql
[MOVE]  backend/migrations/024_xgapi_apiproxy_output_host.sql
[MOVE]  backend/migrations/025_pixian_background_removal.sql
[MOVE]  backend/migrations/026_comfly_aiproxy_output_host.sql
[MOVE]  backend/migrations/027_gpt_image_2_resolution.sql
[MOVE]  backend/migrations/028_image_node_generation_source.sql
[MOVE]  backend/migrations/029_xgapi_base_url.sql
[MOVE]  backend/migrations/030_image_model_relay_routing.sql
[MOVE]  backend/migrations/031_gpt_image_2_pixel_sizes.sql
```

### 前端页面、组件与业务代码

```text
[MOVE]  frontend/src/views/ImageCanvasListView.vue
[MOVE]  frontend/src/views/ImageAgentView.vue
[MOVE]  frontend/src/views/admin/ImageAgentAdminView.vue
[MOVE]  frontend/src/types/image-agent.ts
[MOVE]  frontend/src/services/image-agent.ts
[MOVE]  frontend/src/services/image-agent.test.ts
[MOVE]  frontend/src/composables/image-agent/useImageWorkspace.ts
[MOVE]  frontend/src/composables/image-agent/useImageWorkspace.test.ts
[MOVE]  frontend/src/composables/image-agent/useImageCanvasList.ts
[MOVE]  frontend/src/composables/image-agent/useImageAgentAdmin.ts
[MOVE]  frontend/src/composables/image-agent/useBackgroundRemovalAdmin.ts
[MOVE]  frontend/src/composables/image-agent/useBackgroundRemovalAdmin.test.ts
[MOVE]  frontend/src/components/image-agent/InfiniteImageCanvas.vue
[MOVE]  frontend/src/components/image-agent/InfiniteImageCanvas.test.ts
[MOVE]  frontend/src/components/image-agent/ImageCanvasNode.vue
[MOVE]  frontend/src/components/image-agent/ImageCanvasDrag.test.ts
[MOVE]  frontend/src/components/image-agent/ImageOperationPanel.vue
[MOVE]  frontend/src/components/image-agent/ImageOperationPanel.test.ts
[MOVE]  frontend/src/components/image-agent/ReferenceImageSlots.vue
[MOVE]  frontend/src/components/image-agent/ImagePromptActionButton.vue
[MOVE]  frontend/src/components/image-agent/ImagePromptActionButton.test.ts
[MOVE]  frontend/src/components/image-agent/ImageGenerationSourceBadge.vue
[MOVE]  frontend/src/components/image-agent/CanvasAlignmentGuides.vue
[MOVE]  frontend/src/components/image-agent/CartoonStrengthControl.vue
[MOVE]  frontend/src/components/image-agent/ImageCanvasListCard.vue
[MOVE]  frontend/src/components/image-agent/ImageCanvasListCard.test.ts
[MOVE]  frontend/src/components/image-agent/ImageCanvasCreateDialog.vue
[MOVE]  frontend/src/components/image-agent/ImageCanvasTrashDrawer.vue
[MOVE]  frontend/src/components/image-agent-admin/ImageRelayAdminPanel.vue
[MOVE]  frontend/src/components/image-agent-admin/ImageModelAdminPanel.vue
[MOVE]  frontend/src/components/image-agent-admin/ImageProjectAdminPanel.vue
[MOVE]  frontend/src/components/image-agent-admin/ImageProjectAdminPanel.test.ts
[MOVE]  frontend/src/components/image-agent-admin/ImagePromptActionAdminPanel.vue
[MOVE]  frontend/src/components/image-agent-admin/ImagePromptActionAdminPanel.test.ts
[MOVE]  frontend/src/components/image-agent-admin/PromptActionPreviewPicker.vue
[MOVE]  frontend/src/components/image-agent-admin/ImageBackgroundRemovalAdminPanel.vue
[MOVE]  frontend/src/utils/imageCanvasLayout.ts
[MOVE]  frontend/src/utils/imageCanvasLayout.test.ts
[MOVE]  frontend/src/utils/imageCanvasPersistence.ts
[MOVE]  frontend/src/utils/imageCanvasPersistence.test.ts
[MOVE]  frontend/src/utils/imageCanvasImport.ts
[MOVE]  frontend/src/utils/imageCanvasImport.test.ts
[MOVE]  frontend/src/utils/imagePrompt.ts
[MOVE]  frontend/src/utils/imagePrompt.test.ts
[MOVE]  frontend/src/utils/imagePromptPreview.ts
[MOVE]  frontend/src/utils/imagePromptPreview.test.ts
[MOVE]  frontend/src/utils/imageModelSearch.ts
[MOVE]  frontend/src/utils/imageModelSearch.test.ts
[ADAPT] frontend/src/router/index.ts
[ADAPT] frontend/src/components/layout/AppShell.vue
[ADAPT] frontend/src/services/api.ts
[ADAPT] frontend/src/stores/auth.ts
[ADAPT] frontend/src/main.ts
[ADAPT] frontend/package.json
```

### 部署和文档

```text
[ADAPT] .env.example
[ADAPT] docker-compose.yml
[MOVE]  docs/IMAGE_AGENT_SETUP.md
[KEEP]  docs/IMAGE_AGENT_EXTRACTION_HANDOFF.md
```

## 13. 搜索定位命令

后续代码发生变化时，可用以下命令重新发现生图边界：

```powershell
rg --files frontend/src backend | rg -i "image|canvas|pixian|background_removal"
rg -n "image-agent|image_generator|image_manage" frontend/src backend
rg -n "image_(relays|models|projects|prompt_actions|canvases|assets|jobs|canvas_nodes|job_outputs)" backend
rg -n "XGAPI|Comfly|GPTImage|Pixian|GenerateUniversalEdit" backend
```

这份清单是拆分边界文档，不代表已经把模块物理迁出；正式拆分时应按第 10 节执行并完成第 11 节验收。
