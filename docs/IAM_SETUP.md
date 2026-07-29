# 公司 IAM 接入说明

微光同时支持两种身份入口：

- 飞书客户端内继续使用飞书免登和本地角色。
- 普通浏览器使用公司 IAM 微前端、IAM Cookie 和 IAM 模块权限。

同一员工首次通过 IAM 登录时，服务端会使用 IAM 返回的飞书 `user_id` 查询飞书通讯录，并绑定到已有 `feishu_open_id` 用户。系统不会按姓名猜测或合并账号。

## 1. IAM 后台创建应用

在 `https://iam.shimmergames.com` 创建“微光 AI Agent”应用：

- `app_id`：建议使用 `shimmer-ai-agent`，最终以 IAM 中实际创建的值为准。
- `page_url`：`https://<公司域名>/?app_id=<IAM_APP_ID>`
- `api_url`：`https://<公司域名>`
- 页面和 API 必须使用同一 HTTPS 域名。
- 将该域名加入 IAM Cookie 和微前端可信域名。

创建以下权限：

| 权限键 | 用途 |
| --- | --- |
| `agent_use` | 员工聊天、生图、提醒、会议室和个人历史 |
| `knowledge_manage` | 制度知识库 |
| `agent_manage` | Agent、Prompt 和模型配置 |
| `image_manage` | 生图中转站、模型、项目和快捷提示词 |
| `notification_manage` | 通知中心 |
| `calendar_manage` | 工作日历和会议室管理 |
| `audit_view` | 质量指标和审计日志 |
| `user_manage` | 用户与本地角色管理 |

所有使用者至少需要 `agent_use`。其他权限按 IAM 角色或部门授予。IAM 管理员不会自动成为微光的 `super_admin`。

## 2. 服务端配置

在 `.env` 中填写：

```dotenv
APP_ENV=production
DEV_AUTH_ENABLED=false
PUBLIC_URL=https://<公司域名>

IAM_ENABLED=true
IAM_BASE_URL=https://iam.shimmergames.com
IAM_APP_ID=<IAM应用ID>
IAM_APP_SECRET=<IAM应用密钥>
IAM_AUTH_CACHE_SECONDS=30
IAM_HTTP_TIMEOUT_SECONDS=5
```

`IAM_APP_SECRET`只能放在服务器 Secret 或 `.env` 中，不能配置为 Vite 环境变量，也不能进入浏览器构建产物。

IAM 首次绑定依赖飞书通讯录接口，因此还必须配置：

```dotenv
FEISHU_APP_ID=<飞书应用ID>
FEISHU_APP_SECRET=<飞书应用密钥>
```

飞书应用通讯录可见范围需要覆盖全部 IAM 使用者，并具备读取用户基本信息和组织信息的权限。

## 3. 部署

执行：

```powershell
docker compose up --build -d
```

`migrate` 服务会依次执行 `backend/migrations` 中的幂等迁移，包括 IAM 身份字段和唯一索引。可以通过以下命令检查：

```powershell
docker compose ps
docker compose logs migrate
docker compose logs api
```

浏览器应从 IAM 应用入口或以下地址进入：

```text
https://<公司域名>/?app_id=<IAM_APP_ID>
```

应用会保存 `app_id`，之后直接访问公司域名也能初始化 IAM。首次访问时页面上方加载 IAM 公共头部，微光在 IAM 初始化完成前只显示加载状态。

## 4. 鉴权行为

- 浏览器请求始终携带 Cookie，业务后端从 `iam_user_token`读取 IAM Token。
- 服务端通过 `/api/v2/user-action-authenticate`校验当前接口对应的权限。
- 成功鉴权按 Token 哈希和权限键缓存30秒；拒绝、Token 失效和 IAM 异常不会缓存。
- IAM 返回 `510000`时原样返回，IAM SDK 会弹出登录失效提示并跳转。
- IAM 返回 `510001`时页面显示无权限，服务端不会使用本地管理员角色兜底。
- 飞书入口不加载 IAM SDK，继续使用本地角色。
- 知识库和生图项目 ACL 始终使用绑定后的同一个本地用户和飞书组织信息。

## 5. 验收

1. 让一个已经通过飞书使用过微光的员工从 IAM 入口登录。
2. 在“用户与权限”或数据库中确认其内部用户 `id`未变化，并已填写 `iam_user_id`和 `feishu_user_id`。
3. 确认原聊天、提醒和生图画布仍然可见。
4. 在 IAM 中分别授予和撤销一个管理权限，确认菜单和接口最多30秒内变化。
5. 删除或使 `iam_user_token`失效，确认出现 IAM 登录过期处理。
6. 检查浏览器网络响应、前端静态文件、API 日志和 Redis 键，确认不存在 IAM App Secret 或明文用户 Token。

如果提示“账号绑定冲突”，不要直接修改用户名或按姓名合并。应核对 IAM 用户绑定的飞书 `user_id`、飞书应用可见范围和数据库中的 `feishu_open_id`映射。
