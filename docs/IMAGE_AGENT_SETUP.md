# AI 生图配置与使用

## 首次配置

数据库迁移 `011_image_agent.sql` 会创建两个默认中转站：

- XGAPI：`https://api.xgapi.top/v1`
- Comfly AI：`https://ai.comfly.org/v1`

`https://ai.comfly.org/topup` 是充值页面，不是 API Base URL。

1. 使用超级管理员进入“用户与权限”，为负责人员授予 `image_admin`（生图管理员）角色。
2. 进入“生图管理 → 中转站”，分别编辑两个中转站并填写 API Key。
3. 点击“测试连接”。成功后点击“同步模型”。
4. 进入“模型”，只启用确认支持生图的模型，并配置：
   - `Chat Completions`：适合多模态生图、参考图和图片反推。
   - `Images Generations`：调用 `/images/generations`；首期不开放参考图编辑。
   - 支持的 1K/2K/4K 档位、最大生成数量、参考图和图片反推能力。
5. 在“项目”中配置公司项目及飞书部门/人员可见范围。
6. 可在“功能按键”中配置全局或项目级快捷提示词。项目级相同 `action_key` 会覆盖全局配置。

员工只有在中转站已配置密钥、模型已启用且自己拥有项目权限时，才能在“AI 画图”页面选择对应配置。

## 卡通化快捷提示词

模板可以包含一次数值字段：

```json
{
  "cartoonization_strength": 0,
  "prompt": "把参考图转换为卡通插画"
}
```

后台只接受 `0–1` 范围内的数值。员工点击此类功能按键后不会立即生成，而会看到五档滑块：

- `0`：Very weak stylization.
- `0.25`：Light stylization.
- `0.5`：Medium stylization.
- `0.75`：Strong stylization.
- `1`：Maximum stylization.

不含该字段的功能按键会覆盖文本描述并立即发起生图。

## 数据与安全边界

- API Key 使用 `AGENT_SECRET_ENCRYPTION_KEY` 派生的 AES-GCM 密钥加密，管理页面只返回尾号。
- 原图、参考图和生成结果存入 MinIO，浏览器只能通过带登录鉴权的资源接口读取。
- 单张参考图不超过 10 MB，最多 5 张，单次任务参考图合计不超过 30 MB。
- 远程图片结果只允许从中转站配置的域名下载；服务端会拒绝私网地址、跨域重定向、非图片内容和超大文件，并执行 ClamAV 扫描。
- 系统不会自动切换中转站，避免改变费用归属和数据发送边界。
- 每名员工在每个项目下拥有独立画布；画布位置和视口使用版本号保存，发生多标签页冲突时加载服务器最新版本。

## 协议响应兼容

系统支持解析：

- Images API 的 `data[].b64_json` 和 `data[].url`
- Chat API 的 `choices[].message.images[].image_url.url`
- Data URL、Base64 和经过域名校验的 HTTPS 图片 URL

图片反推固定调用 Chat Completions，只接收文本结果。生图数量会拆成独立子请求，同一任务最多并发两个，网络错误、HTTP 429 和 5xx 最多重试三次。

## 当前边界

- “工作流”首期只有入口占位，不创建节点或执行工作流。
- 每个中转站只有一把当前有效密钥，不做密钥池或自动故障切换。
- 画布是员工私有画布，不提供多人共同编辑。
- 没有配置真实中转站 API Key 时，可以完成页面、权限、项目、画布、上传和资源鉴权测试，但不能完成真实生图。
