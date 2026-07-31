# 公司 IAM 接入与统一权限说明

微光同时支持飞书免登和普通浏览器 IAM 登录。两种入口绑定到同一个内部用户 UUID，并使用同一份最终权限：

```text
最终权限 =（IAM 授予 ∪ 本地允许）− 本地拒绝
```

- IAM 用于公司级角色、部门和员工授权。
- 微光后台用于项目内临时补充或明确收回权限。
- 本地拒绝优先级最高；删除本地覆盖后恢复继承 IAM。
- 管理权限不会自动绕过制度文档或生图项目 ACL。
- 旧角色仅保留一个版本用于迁移和审计，运行时接口不再按角色授权。

## 1. IAM 后台创建应用

在 `https://iam.shimmergames.com` 创建“微光 AI Agent”应用：

- `page_url`：`https://<公司域名>/?app_id=<IAM_APP_ID>`
- `api_url`：`https://<公司域名>`
- 页面和 API 使用同一个 HTTPS 域名。
- 将域名加入 IAM Cookie 和微前端可信域名。

创建八个权限键：

| 权限键 | 用途 |
| --- | --- |
| `agent_use` | 聊天、生图、提醒、会议室和个人历史 |
| `knowledge_manage` | 制度知识库 |
| `agent_manage` | Agent、Prompt 和模型配置 |
| `image_manage` | 生图中转站、模型、项目和快捷提示词 |
| `notification_manage` | 通知中心 |
| `calendar_manage` | 工作日历和会议室管理 |
| `audit_view` | 质量指标和审计日志 |
| `user_manage` | 用户和本地权限覆盖 |

所有使用者最终必须拥有 `agent_use`。

## 2. IAM 服务端权限查询接口

飞书入口没有 IAM 用户 Cookie，因此 IAM 服务还需提供应用级查询接口：

```http
POST /api/v2/app-user-effective-permissions
X-IAM-App-ID: <app_id>
X-IAM-App-Secret: <app_secret>
Content-Type: application/json
```

按 IAM 用户查询：

```json
{"iam_user_id": 123}
```

或按飞书用户查询：

```json
{"feishu_user_id": "飞书 user_id"}
```

成功响应：

```json
{
  "code": 0,
  "data": {
    "iam_user_id": 123,
    "permission_keys": ["agent_use", "knowledge_manage"],
    "policy_version": 7,
    "updated_at": "2026-07-31T10:00:00+08:00"
  }
}
```

微光只接受固定的八个权限键，未知键会被忽略。`IAM_APP_SECRET` 只在服务端请求头中使用，不会返回前端或写入日志。

## 3. 服务端配置

在 `.env` 中配置：

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

FEISHU_APP_ID=<飞书应用ID>
FEISHU_APP_SECRET=<飞书应用密钥>
```

IAM 首次登录使用 IAM 身份对应的飞书 `user_id` 查询飞书通讯录，再绑定现有 `open_id` 用户。飞书应用的通讯录可见范围必须覆盖全部 IAM 使用者。系统不会按姓名猜测或合并账号。

首位管理员可临时配置：

```dotenv
BOOTSTRAP_SUPER_ADMIN_OPEN_IDS=ou_xxx
```

该用户首次登录或 API 启动同步时，会在本地权限策略中允许全部八项权限。完成管理员初始化后建议移除该环境变量。

## 4. 数据库升级和部署

执行：

```powershell
docker compose up --build -d
docker compose logs migrate
docker compose logs api
```

`migrate` 服务会执行 `016_unified_permissions.sql`，创建：

- `local_permission_policies`：本地允许、拒绝、乐观版本和变更原因。
- `iam_permission_snapshots`：IAM 最近成功权限、策略版本、同步时间和错误。

迁移会把旧管理角色转换为本地允许权限。`employee` 不会自动变成本地 `agent_use`，从而保留 IAM 撤销普通员工访问的能力。

## 5. 权限行为

- IAM 登录使用当前 IAM Token 确认身份，再读取 IAM 权限。
- 飞书登录和飞书机器人使用可信的 `feishu_user_id` 调用应用级权限查询。
- 每个请求都从 PostgreSQL 读取本地允许和拒绝，因此本地修改无需重新登录即可生效。
- IAM 成功结果缓存 30 秒；本地策略不使用该缓存。
- IAM 故障时，30 秒内的成功快照可继续提供全部权限。
- IAM 故障超过 30 秒后，IAM 来源的管理权限失败关闭；五分钟内只保留快照中的 `agent_use`。
- 本地允许在 IAM 故障时仍有效，本地拒绝始终优先。
- 最后一名有效 `user_manage` 管理员不能移除自己的权限。

旧接口：

```text
PUT /api/v1/admin/users/{id}/roles
```

固定返回 `410 Gone`。新接口为：

```text
GET  /api/v1/admin/users/{id}/permissions
PUT  /api/v1/admin/users/{id}/permissions
POST /api/v1/admin/users/{id}/permissions/refresh
```

## 6. 验收

1. 同一员工分别从 IAM 和飞书登录，确认内部用户 ID、菜单和 API 权限一致。
2. 在 IAM 授予 `notification_manage`，本地保持继承，确认最终允许。
3. 本地拒绝 `notification_manage`，确认无需重新登录便立即拒绝。
4. 删除本地拒绝，确认最多 30 秒后恢复 IAM 权限。
5. IAM 未授权 `image_manage`，本地允许后确认可以进入生图管理。
6. 停止 IAM 服务，确认本地允许继续有效，IAM 管理权限超过 30 秒后关闭。
7. 检查权限变更、同步失败、故障降级和拒绝均写入审计。
8. 检查浏览器构建产物、接口响应、日志和 Redis 中不存在 IAM App Secret 或明文 Token。

出现身份绑定冲突时，不要按姓名合并。应核对 IAM 用户 ID、IAM 中绑定的飞书 `user_id`、飞书应用可见范围以及数据库中的 `feishu_open_id` 映射。
