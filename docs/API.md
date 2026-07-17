# API 摘要

所有 `/api/v1` 员工及管理接口使用 `ai_agent_session` HttpOnly Cookie。生产写操作应由同源 Nginx 提供服务。

## 员工端

- `POST /api/v1/auth/feishu/exchange`：用飞书 code 换取本系统会话。
- `GET /api/v1/me`：当前用户与角色。
- `GET|POST /api/v1/conversations`：会话列表与创建。
- `GET|DELETE /api/v1/conversations/{id}`：读取消息或删除自己的会话。
- `POST /api/v1/conversations/{id}/messages`：提交问题，返回 `run_id`。
- `GET /api/v1/runs/{run_id}/events`：SSE，事件依次为 `status`、`delta`、`citation`、`done` 或 `error`。
- `GET /api/v1/reminders`、`GET /api/v1/reminders/{id}`：查看自己的提醒和详情。
- `POST /api/v1/reminder-actions`：创建待确认的提醒设置。
- `POST /api/v1/reminders/{id}/actions`：创建修改、暂停、恢复或删除操作。
- `POST /api/v1/reminder-actions/{id}/confirm|cancel`：确认或取消操作；所有提醒变更必须经过这一步。
- `GET /api/v1/reminders/{id}/deliveries`：查看最近的发送结果。

## 管理端

- `/api/v1/admin/knowledge/sources`：上传/飞书文件夹资料源。
- `/api/v1/admin/knowledge/documents`：制度列表、上传与发布。
- `PUT /api/v1/admin/knowledge/documents/{id}/acl`：更新文档可见范围。
- `GET /api/v1/admin/directory/options`：知识管理员可用的部门、职务和人员选项。
- `GET /api/v1/admin/users`、`PUT /api/v1/admin/users/{id}/roles`：超级管理员查看用户并分配后台角色。
- `POST /api/v1/admin/users/sync`：同步所有已登录用户的飞书组织属性。
- `/api/v1/admin/agent/configs`：版本化模型与 RAG 配置。
- `/api/v1/admin/notifications`：草稿、AI 润色、审核、发送与取消。
- `/api/v1/admin/audit`、`/api/v1/admin/metrics`：审计与质量指标。
- `/api/v1/admin/work-calendar`：维护公司节假日和调休覆盖；修改后会立即重算工作日提醒。

管理接口由后端再次检查 `knowledge_admin`、`notification_admin`、`auditor` 或 `super_admin`，不依赖前端菜单隐藏。

受限文档 ACL 使用规则组；单组字段是 AND，多组是 OR：

```json
{
  "scope": "restricted",
  "rules": [
    { "department_ids": ["od_finance"], "job_titles": ["会计"] },
    { "user_ids": ["内部用户 UUID"] }
  ]
}
```

## 飞书事件与回调

飞书消息事件和新版卡片交互不暴露 HTTP API，统一由 API 进程中的官方 Go SDK WebSocket 长连接接收：

- 事件：`im.message.receive_v1`
- 通讯录事件：`contact.user.created_v3`、`contact.user.updated_v3`、`contact.user.deleted_v3`
- 回调：`card.action.trigger`

处理器先按 `event_id` 幂等，再将 AI 问答异步执行并立即确认事件。旧的 `/api/v1/integrations/feishu/events` 和 `/api/v1/integrations/feishu/cards` 已移除。

机器人提醒链路同样使用长连接。用户私聊机器人说“今天下午 3 点提醒我写周报”后，Worker 异步解析并发送确认卡片；只有用户点击确认才创建提醒。到点后应用机器人按 `open_id` 向用户私聊发送卡片。
