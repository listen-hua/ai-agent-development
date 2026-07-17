# 飞书个人提醒配置

个人提醒复用当前企业自建应用的机器人和 WebSocket 长连接，不需要新增 HTTP 回调地址。

## 飞书开放平台

1. 在应用能力中启用“机器人”。
2. 在事件与回调中选择“使用长连接接收事件”。
3. 订阅 `im.message.receive_v1`，并启用回调 `card.action.trigger`。
4. 为应用开通发送消息、接收用户发给机器人的消息所需权限，并让应用对试用员工可见。
5. 发布新版本；未发布的权限和事件配置不会在正式企业中生效。

项目已有的 `FEISHU_APP_ID`、`FEISHU_APP_SECRET` 和 `FEISHU_APP_LINK` 会被直接复用。`FEISHU_APP_LINK` 建议指向网页应用内“我的提醒”页面，方便用户从卡片返回管理。

## 服务配置

```env
DASHSCOPE_REMINDER_MODEL=qwen-flash
REMINDER_TIMEZONE=Asia/Shanghai
REMINDER_POLL_SECONDS=5
REMINDER_GRACE_MINUTES=30
REMINDER_MAX_ACTIVE=100
```

Worker 必须常驻运行。系统会把待发送记录持久化到 PostgreSQL，并以稳定幂等键调用飞书；短暂网络错误按退避策略重试，重启不会丢任务。超过计划时间 30 分钟的任务标记为错过，不再补发。

“工作日”默认按周一至周五判断。通知管理员或超级管理员可在“工作日历”中录入节假日和调休，也可导入 UTF-8 CSV：

```csv
date,is_workday,note
2026-10-01,false,国庆节
2026-10-10,true,调休上班
```

## 验收建议

- 分别从 H5 和机器人创建一次性、每天、工作日和每周提醒，确认都先出现二次确认。
- 重复点击确认只能生成一个提醒。
- 验证暂停、恢复、修改和删除均不能操作其他用户的提醒。
- 临时将提醒设置到未来几分钟，检查机器人私聊、发送记录和重启后的恢复情况。
- 把当天改为公司休息日，确认工作日提醒自动顺延。
