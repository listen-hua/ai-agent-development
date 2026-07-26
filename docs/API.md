# API 摘要

所有 `/api/v1` 员工及管理接口使用 `ai_agent_session` HttpOnly Cookie。生产写操作应由同源 Nginx 提供服务。

## 员工端

- `POST /api/v1/auth/feishu/exchange`：用飞书 code 换取本系统会话。
- `GET /api/v1/me`：当前用户与角色。
- `GET|POST /api/v1/conversations`：会话列表与创建。
- `GET|DELETE /api/v1/conversations/{id}`：读取消息或删除自己的会话。
- `POST /api/v1/conversations/{id}/messages`：提交问题，返回 `run_id`。
- `POST /api/v1/conversations/{id}/context/reset`：清除当前会话的滚动摘要和未完成任务状态。
- `GET /api/v1/runs/{run_id}/events`：SSE，事件依次为 `status`、`delta`、`citation`、`done` 或 `error`；上下文分析阶段的 `status.metadata.stage` 为 `contextualizing`。
- `GET /api/v1/reminders`、`GET /api/v1/reminders/{id}`：查看自己的提醒和详情。
- `POST /api/v1/reminder-actions`：创建待确认的提醒设置。
- `POST /api/v1/reminders/{id}/actions`：创建修改、暂停、恢复或删除操作。
- `POST /api/v1/reminder-actions/{id}/confirm|cancel`：确认或取消操作；所有提醒变更必须经过这一步。
- `GET /api/v1/reminders/{id}/deliveries`：查看最近的发送结果。
- `POST /api/v1/meeting-booking-actions/{id}/confirm`：确认预约、取消或改期；预约/改期可传 `option_id`。
- `POST /api/v1/meeting-booking-actions/{id}/cancel`：放弃待确认操作，不改变飞书日程。
- `GET /api/v1/meeting-bookings`：查看自己通过行政 AI 创建的会议室预约。
- `GET /api/v1/image-agent/options`：返回当前员工可用的中转站、模型、项目和功能按键。
- `GET|PATCH /api/v1/image-agent/projects/{id}/canvas`：读取或按版本号保存当前员工在项目下的私人画布。
- `POST /api/v1/image-agent/assets`、`GET /api/v1/image-agent/assets/{id}/content`：上传参考图及鉴权读取图片。
- `POST /api/v1/image-agent/jobs`：创建文生图或图片反推任务。
- `GET /api/v1/image-agent/jobs`、`GET /api/v1/image-agent/jobs/{id}`：读取任务列表和异步执行状态。

## 管理端

- `/api/v1/admin/knowledge/sources`：上传/飞书文件夹资料源。
- `/api/v1/admin/knowledge/documents`：制度列表、上传与发布。
- `PUT /api/v1/admin/knowledge/documents/{id}/acl`：更新文档可见范围。
- `GET /api/v1/admin/directory/options`：知识管理员可用的部门、职务和人员选项；部门直接读取飞书通讯录组织架构，并返回真实名称、父部门和完整层级路径。
- `GET /api/v1/admin/users`、`PUT /api/v1/admin/users/{id}/roles`：超级管理员查看用户并分配后台角色。
- `POST /api/v1/admin/users/sync`：同步应用通讯录可见范围内的完整部门员工，并更新组织属性。
- `/api/v1/admin/agent/configs`：版本化模型与 RAG 配置。
- `/api/v1/admin/notifications`：草稿、AI 润色、审核、发送与取消。
- `/api/v1/admin/audit`、`/api/v1/admin/metrics`：审计与质量指标。
- `/api/v1/admin/work-calendar`：维护公司节假日和调休覆盖；修改后会立即重算工作日提醒。
- `GET /api/v1/admin/meeting-rooms`：会议室、预定限制、共享日历、同步状态和最多 100 条待人工处理的异常预约。
- `POST /api/v1/admin/meeting-rooms/sync`：立即从飞书同步会议室及预定限制。
- `POST /api/v1/admin/meeting-rooms/calendar`：幂等初始化应用专用共享日历。
- `/api/v1/admin/image-agent/relays`：中转站 CRUD、连接测试和模型同步。
- `/api/v1/admin/image-agent/models`：配置模型协议、开放状态和生图能力。
- `/api/v1/admin/image-agent/projects`：配置项目状态及飞书组织 ACL。
- `/api/v1/admin/image-agent/prompt-actions`：管理全局和项目级功能按键。

管理接口由后端再次检查 `knowledge_admin`、`notification_admin`、`image_admin`、`auditor` 或 `super_admin`，不依赖前端菜单隐藏。

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
- 会议室状态：`meeting_room.meeting_room.status_changed_v1`

处理器先按 `event_id` 幂等，再将 AI 问答异步执行并立即确认事件。旧的 `/api/v1/integrations/feishu/events` 和 `/api/v1/integrations/feishu/cards` 已移除。

机器人提醒链路同样使用长连接。用户私聊机器人说“今天下午 3 点提醒我写周报”后，Worker 异步解析并发送确认卡片；只有用户点击确认才创建提醒。到点后应用机器人按 `open_id` 向用户私聊发送卡片。
