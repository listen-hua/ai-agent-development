# 飞书会议室预约配置

行政 AI 使用应用身份和专用共享日历代员工预约，不保存用户 OAuth Token。只有启用、当前未禁用，并且本次时长不触发审批的会议室会进入候选。

## 1. 开通权限

在飞书开放平台进入企业自建应用，申请并由企业管理员审批以下能力：

- `vc:room:readonly`：查询会议室列表、状态和预定限制。
- `calendar:room:readonly`：查询会议室忙闲。
- `calendar:calendar:create`、`calendar:calendar:read`：创建和读取应用共享日历。
- `calendar:calendar.event:create`、`calendar:calendar.event:read`、`calendar:calendar.event:update`、`calendar:calendar.event:delete`：创建、读取、更新和删除日程及参与人。
- `calendar:calendar.free_busy:read`：批量查询参会人忙闲。
- 获取通讯录基本信息、用户组织架构信息：把参会人姓名安全解析为当前应用的 `open_id`。
- `im:message:send_as_bot`：发送确认、预约结果和会后归还提醒。

应用必须开启机器人能力；通讯录权限范围和应用可用范围必须覆盖所有可能参与预约的员工。权限名称在不同控制台版本中可能展示为中文，以 API 调试台对上述接口给出的权限为准。

## 2. 配置长连接事件

在“事件与回调 → 事件配置”选择“使用长连接接收事件”，添加：

- `meeting_room.meeting_room.status_changed_v1`
- 已有的 `im.message.receive_v1`

在“回调配置”继续使用长连接并保留 `card.action.trigger`，用于确认卡片。发布一个包含新权限和事件的应用版本。

## 3. 数据库升级

Compose 的 `migrate` 服务会在 API 和 Worker 启动前依次执行迁移。升级后可单独确认最新会议草稿迁移：

```powershell
docker compose run --rm migrate
```

`018_meeting_booking_drafts.sql` 用于保存 15 分钟有效的多轮预约/取消草稿；已有数据卷和全新数据卷都会由 `migrate` 服务安全执行。

## 4. 首次初始化

1. 重建并启动 API、Worker 和 Web。
2. 用 `notification_admin` 或 `super_admin` 打开“会议室管理”。
3. 点击“初始化共享日历”。这个操作只会创建一次“行政 AI 会议室预约”共享日历；之后数据库保存 `calendar_id`。
4. 点击“立即同步”，确认页面能看到会议室容量、可预定时段、审批限制和禁用状态。
5. 如企业已经为应用创建了共享日历，可在 `.env` 填写 `FEISHU_MEETING_CALENDAR_ID`，无需再点击初始化。

Worker 每 15 分钟增量刷新会议室快照；收到会议室状态事件时会立即刷新。忙闲数据不会缓存，生成候选和最终确认时都会实时向飞书查询。

## 5. 验收

准备一间空闲室、一间占用室和一间需要审批的会议室，依次验证：

- “明天下午 3 点预约 4 人会议室”会先要求填写会议主题，并明确选择参会人或“仅自己参会”。
- 添加参会人后，任何一个人的忙碌时段都不会被推荐。
- 点击确认后，参会人日历出现日程，会议室参与人的 RSVP 为 `accept`。
- 在“我的会议”取消预约，或对行政助手说“取消明天下午三点的周报评审会议”，都会先生成确认动作；确认后飞书日程被删除并通知参会人。
- 取消和改期只能操作当前用户通过行政 AI 创建的预约；日程已被人工删除时，重复取消按幂等成功处理。
- 会议结束时，组织者收到关闭设备、带走物品并恢复会议室的私聊提醒。
