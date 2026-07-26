# 知行 · 公司内部行政 AI Agent

一个面向公司内部员工的制度问答与飞书通知系统。员工可在飞书 H5 或机器人中提问；管理人员可以管理制度版本、模型配置、通知审批和审计指标。系统默认执行严格 RAG：没有可靠制度依据就拒绝回答。

## 已实现能力

- Vue 3 + TypeScript 响应式员工端和同应用管理后台，包含 `/chat`、知识库、Agent 配置、通知、质量与审计页面。
- Go 模块化后端，提供飞书免登、HttpOnly 会话、RBAC、会话和 SSE 流式事件。
- 用户与权限后台：超级管理员显式授予知识管理员、通知管理员和审计员；管理员身份不会根据飞书职务自动推导。
- 飞书通讯录组织属性同步：部门、职务、职级、序列和人员类型可用于文档 ACL；同组条件为 AND，多组规则为 OR。
- 阿里云百炼 OpenAI 兼容适配：`qwen-plus`、`text-embedding-v4`、`qwen3-rerank`；未配置密钥时自动使用可重复的本地演示模型。
- 制度文件查毒、Tika 解析、分块、Embedding、PostgreSQL 全文 + pgvector 混合检索、Rerank、ACL 过滤和引用验证。
- 制度草稿—发布生命周期，发布前不会参与员工问答。
- AI 通知润色、人工审核、幂等飞书卡片发送；不能跳过审核。
- 行政 AI 智能预约普通会议室：同时校验会议室和参会人忙闲，确认后创建飞书日程，支持取消、改期和会后归还提醒。
- AI 生图：支持两个 OpenAI 兼容中转站、模型能力配置、项目 ACL、快捷提示词、图片反推和持久化无限画布。
- 飞书消息事件验签令牌校验、加密事件解密、事件幂等和机器人异步回复。
- PostgreSQL、Redis、MinIO、Tika、ClamAV、API、Worker 和 Nginx 的 Docker Compose 基础设施。
- 本地开发数据仓库和演示账号；配置 `DATABASE_URL` 后自动使用 PostgreSQL 持久化仓库。

## 本地启动

环境要求：Go 1.25、Node.js 22、Docker 29+。

```powershell
Copy-Item .env.example .env
docker compose up -d postgres redis minio tika clamav

Set-Location backend
go run ./cmd/server
```

另开终端：

```powershell
Set-Location frontend
npm.cmd install
npm.cmd run dev
```

打开 `http://localhost:5173`，选择“管理员”或“普通员工”演示登录。演示数据包含一份已发布的休假和报销制度。若不启动 Docker，也可以清空 `DATABASE_URL`、`MINIO_ENDPOINT`、`CLAMAV_ADDR` 和 `TIKA_URL`，系统会使用内存仓库与纯文本解析。

完整容器启动：

```powershell
docker compose up --build
```

启动后打开 `http://127.0.0.1:8088`。完整容器模式仅由 Nginx 暴露 Web 与 `/api`，API 的 `8080` 端口只在 Compose 内部网络开放，避免与本机服务冲突。Worker 使用独立的 `/app/worker` 入口，不会启动 HTTP 服务。

## 生产配置

1. 复制 `.env.example` 为 `.env`，替换所有示例密码，设置至少 32 字节随机 `SESSION_SECRET`。
2. 配置阿里云百炼业务空间专属 `DASHSCOPE_BASE_URL` 与 `DASHSCOPE_API_KEY`。密钥仅存在部署 Secret 中，后台页面不会返回明文。
3. 创建飞书企业自建应用，同时启用“网页应用”和“机器人”。配置 H5 地址与 OAuth 回调；事件和新版卡片回调均选择“使用长连接接收”，不需要配置公网回调地址。
4. 设置 `APP_ENV=production` 和 `DEV_AUTH_ENABLED=false`。生产模式下 PostgreSQL 或 MinIO连接失败会阻止 API 启动。
5. 数据服务只加入内部 Docker 网络；公网网关只暴露 Nginx 的 H5/API。

### 飞书最小能力清单

- 网页免登：获取用户身份基本信息；需要持续用户授权时再申请 `offline_access`。
- 通讯录同步：至少开启“获取通讯录基本信息”。知识库部门可见范围还需要 `contact:department.base:readonly`（部门名称）和 `contact:department.organize:readonly`（部门层级），并将应用通讯录可见范围覆盖计划使用本系统的完整组织架构；按 ACL 所用字段再开启“获取用户组织架构信息”和“获取用户雇佣信息”。使用职级/序列时还需对应的只读权限。
- 机器人问答：接收单聊消息、接收群内 `@` 消息、以应用身份发送消息。
- 通知：`im:message:send_as_bot`，应用可用范围必须覆盖目标用户。
- 会议室预约：`vc:room:readonly`、`calendar:room:readonly`、`calendar:calendar:create/read`、`calendar:calendar.event:create/read/update/delete`、`calendar:calendar.free_busy:read`；应用必须开启机器人能力。
- 云空间同步：读取文件夹清单、文件元数据、云文档正文/导出和文件下载权限；目标文件夹需要显式共享给应用身份。

具体 scope 名称会随飞书控制台版本变化，应在 API 调试台按实际接口生成并由企业管理员审核，禁止为了省事申请整租户全量权限。

### 飞书长连接配置

1. 在“事件与回调 → 事件配置”中选择“使用长连接接收事件”，添加 `im.message.receive_v1`；如需自动刷新组织信息，再添加 `contact.user.created_v3`、`contact.user.updated_v3` 和 `contact.user.deleted_v3`；启用会议室预约时再添加 `meeting_room.meeting_room.status_changed_v1`。
2. 如使用新版卡片交互，在“回调配置”中选择“使用长连接接收回调”，添加 `card.action.trigger`。旧版 `card.action.trigger_v1` 不支持长连接，本项目不再提供旧版 HTTP 回调端点。
3. 在 `.env` 中配置 `FEISHU_APP_ID`、`FEISHU_APP_SECRET`。`FEISHU_APP_LINK` 留空时会根据 App ID 自动生成指向 `/chat` 的工作台链接，也可填写自定义 AppLink。长连接不使用 `FEISHU_VERIFICATION_TOKEN` 或 `FEISHU_ENCRYPT_KEY`。
4. 发布应用版本并确保应用可用范围覆盖试点员工。API 容器启动后出现 `feishu long connection ready` 表示建连成功；SDK 会自动重连。

消息事件在长连接处理器中完成事件 ID 幂等后立即转入异步 RAG，避免阻塞飞书要求的 3 秒响应窗口。网页应用优先通过 `tt.requestAuthCode` 获取免登 code，再调用 `POST /api/v1/auth/feishu/exchange`；仅在客户端不提供该方法时回退到新版 `requestAccess`。

### 首位超级管理员

在 `.env` 的 `BOOTSTRAP_SUPER_ADMIN_OPEN_IDS` 中填写首位管理员的飞书 `open_id`。API 启动时会为数据库中已有的对应用户补授 `super_admin`；如果用户尚未首次登录，则在首次登录时授予。随后可在“用户与权限”页面分配其他后台角色。完成初始化后建议清空该变量并重建 API 容器，数据库中已授予的角色不会被登录同步覆盖。

已有数据库升级时先执行：

```powershell
docker compose cp backend/migrations/002_identity_acl.sql postgres:/tmp/002_identity_acl.sql
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U ai_agent -d ai_agent -f /tmp/002_identity_acl.sql
```

不要通过 PowerShell 文本管道把含中文的 SQL 传给 `psql`；部分 Windows 环境会把 UTF-8 中文转换成字面量 `?`。先复制文件再由容器内的 `psql -f` 读取可保留原始字节。

会议室预约的飞书后台配置、首次同步和已有数据库升级步骤见 [docs/FEISHU_MEETING_SETUP.md](docs/FEISHU_MEETING_SETUP.md)。

AI 生图中转站、模型、项目和快捷提示词的配置步骤见 [docs/IMAGE_AGENT_SETUP.md](docs/IMAGE_AGENT_SETUP.md)。

### 多轮上下文

- `DASHSCOPE_CONTEXT_MODEL` 配置低成本上下文模型，默认 `qwen-flash`；后台的 Agent 配置也可按版本修改、发布和回滚。
- H5 以 `conversation_id` 隔离上下文；新建会话不会继承旧会话，`POST /api/v1/conversations/{id}/context/reset` 可在原会话中建立新的历史边界。
- 飞书机器人按“用户 + chat_id + agent_key”绑定会话，30 分钟无交互自动新建上下文；发送“新会话”“清除上下文”“重新开始”或“算了”可立即重置。
- 上下文模型只做意图识别与独立问题改写。制度证据每轮重新执行 ACL 检索，提醒和会议室操作仍需确认后才会执行。

已有数据库升级到上下文版本时执行：

```powershell
docker compose cp backend/migrations/009_conversation_context.sql postgres:/tmp/009_conversation_context.sql
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U ai_agent -d ai_agent -f /tmp/009_conversation_context.sql
```

## 工程结构

```text
backend/
  cmd/server            HTTP API 与飞书长连接
  cmd/worker            同步、通知、留存任务进程
  internal/domain       领域类型与 ACL
  internal/service      RAG、知识库、通知用例
  internal/model        阿里云与本地模型适配
  internal/store        内存与 PostgreSQL 仓库
  internal/integration  飞书客户端
  migrations            pgvector 数据库结构
frontend/src/
  views                 路由级组合页面
  components            聊天、知识、配置、通知功能组件
  composables           异步用例状态
  services              HTTP DTO 与 SSE
  stores                跨页面身份状态
ops/                    备份与恢复脚本
```

## 验证

```powershell
Set-Location backend
go test ./...
go build ./cmd/server
go build ./cmd/worker

Set-Location ../frontend
npm.cmd run test:unit -- --run
npm.cmd run build
```

数据库迁移会创建 pgvector HNSW 索引、全文索引和业务表。制度 ACL 在检索结果进入 Rerank 和生成模型前执行；无权限内容不会进入模型上下文或引用。

## 当前边界

- Worker 已提供 15 分钟调度进程和同步任务接口；飞书云文档的实际文件夹授权必须在目标企业中配置后才能完成端到端联调。
- OCR 通过 `OCR_URL` 预留为独立内网服务；Tika 无法提取到文本时会明确失败，不会把空文档发布。
- 生产上线前仍需完成企业安全审批、真实制度评测集、飞书限流压测和备份恢复演练。
