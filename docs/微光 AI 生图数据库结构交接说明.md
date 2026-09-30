# 微光 AI 生图数据库结构交接说明

> 数据库：PostgreSQL 17。  
> 图片文件：MinIO。PostgreSQL 只保存业务数据、图片元数据和 MinIO `object_key`，不直接保存图片二进制。  
> 本文以当前运行中的 `ai_agent` 数据库真实结构为准，适用于开发交接、故障排查和后续独立拆分。

## 1. 总体关系

```text
users
 ├─< image_canvases ─< image_canvas_nodes >─ image_assets
 │        │                    │                  │
 │        │                    ├─ image_jobs ─< image_job_outputs
 │        │                    │       │               │
 │        │                    │       ├─ image_relays │
 │        │                    │       ├─ image_models │
 │        │                    │       └─ image_projects
 │        │                    │                       │
 │        └─< background_removal_jobs ─< background_removal_items
 │                                             │
 │                                             ├─ source_asset_id
 │                                             └─ result_asset_id
 │
 ├─< image_assets
 ├─< image_jobs
 └─< background_removal_jobs

image_relays ─< image_models
image_projects ─< image_assets / image_jobs / background_removal_items
image_projects ← image_prompt_actions.project_id 或 project_ids[]
pixian_background_removal_config（全局单例配置）
```

符号说明：`A ─< B` 表示一条 A 记录可以对应多条 B 记录。

## 2. 表分类

| 分类 | 数据表 | 用途 |
|---|---|---|
| 配置 | `image_relays` | XGAPI、Comfly等中转站配置和加密API Key |
| 配置 | `image_models` | 中转站模型、协议和能力 |
| 配置 | `image_projects` | 公司项目、启停和项目ACL |
| 配置 | `image_prompt_actions` | 功能按键、提示词模板和预览图 |
| 画布 | `image_canvases` | 用户私人无限画布、视口、版本和回收站状态 |
| 画布 | `image_canvas_nodes` | 画布中的每张图片或生成占位节点 |
| 资源 | `image_assets` | MinIO图片对象的元数据 |
| 任务 | `image_jobs` | 生图和图片反推任务主表 |
| 任务 | `image_job_outputs` | 生图任务的每张输出明细 |
| 抠图配置 | `pixian_background_removal_config` | Pixian全局配置、加密凭证和余额快照 |
| 抠图任务 | `background_removal_jobs` | 一次批量抠图任务 |
| 抠图任务 | `background_removal_items` | 批量任务中每张图片的处理明细 |

## 3. 配置类数据表

### 3.1 `image_relays`：生图中转站

一条记录代表一个中转站，例如 XGAPI 或 Comfly。

| 字段 | 类型 | 含义 |
|---|---|---|
| `id` | `uuid` | 主键 |
| `relay_key` | `text` | 中转站唯一业务标识，唯一约束 |
| `name` | `text` | 后台和员工端展示名称 |
| `base_url` | `text` | OpenAI兼容接口根地址 |
| `enabled` | `boolean` | 是否允许新任务使用 |
| `timeout_seconds` | `integer` | 请求超时，限制为10–600秒 |
| `allowed_output_hosts` | `text[]` | 允许下载生成图片的域名白名单，用于防止SSRF |
| `encrypted_api_key` | `text` | 加密后的API Key，禁止返回前端或直接交接明文 |
| `api_key_hint` | `text` | 后台显示的脱敏尾号 |
| `created_by` / `updated_by` | `uuid` | 创建和修改管理员，关联`users.id` |
| `created_at` / `updated_at` | `timestamptz` | 创建和更新时间 |

关系：

- `image_models.relay_id → image_relays.id`，删除中转站会级联删除模型。
- `image_jobs.relay_id → image_relays.id`，已有任务会阻止直接删除仍被引用的中转站。

### 3.2 `image_models`：模型配置

一个中转站可以配置多个模型，同一个模型ID可以存在于不同中转站。

| 字段 | 类型 | 含义 |
|---|---|---|
| `id` | `uuid` | 主键 |
| `relay_id` | `uuid` | 所属中转站 |
| `model_id` | `text` | 用户选择和配置时使用的远端模型ID |
| `display_name` | `text` | 员工端展示名称 |
| `request_model_id` | `text` | 实际发给中转站的模型ID；可与`model_id`不同 |
| `remote_endpoint_types` | `text[]` | 中转站模型列表声明的端点能力 |
| `protocol` | `text` | 调用协议 |
| `enabled` | `boolean` | 是否对员工开放 |
| `supports_reference` | `boolean` | 是否支持参考图 |
| `supports_reverse` | `boolean` | 是否支持图片反推文字 |
| `supported_sizes` | `text[]` | 支持档位，如`1K/2K/4K` |
| `max_count` | `integer` | 单次允许数量，只能是1、2、4或8 |
| `created_at` / `updated_at` | `timestamptz` | 创建和更新时间 |

约束：

- `(relay_id, model_id)`唯一。
- `protocol`只能是`chat_completions`、`images_generations`或`gpt_image_2`。
- GPT Image 2的实际像素尺寸由服务端根据比例和1K/2K/4K档位计算。

### 3.3 `image_projects`：公司项目

项目用于控制功能按键内容、模型调用归属、费用统计和资源权限，不再和画布一对一绑定。

| 字段 | 类型 | 含义 |
|---|---|---|
| `id` | `uuid` | 主键 |
| `project_key` | `text` | 项目唯一标识 |
| `name` | `text` | 项目名称，支持中文 |
| `description` | `text` | 项目说明 |
| `acl` | `jsonb` | 全员、部门、人员、职位等资源ACL |
| `enabled` | `boolean` | 停用后禁止创建新任务 |
| `created_by` / `updated_by` | `uuid` | 管理员用户ID |
| `created_at` / `updated_at` | `timestamptz` | 创建和更新时间 |

示例ACL：

```json
{
  "scope": "departments",
  "department_ids": ["飞书部门ID"]
}
```

ACL必须由Go服务端检查，不能只依赖前端隐藏项目。

### 3.4 `image_prompt_actions`：功能按键

保存管理员配置的快捷提示词和悬浮预览图。

| 字段 | 类型 | 含义 |
|---|---|---|
| `id` | `uuid` | 主键 |
| `action_key` | `text` | 功能按键业务标识 |
| `name` | `text` | 员工端按钮名称 |
| `prompt_template` | `text` | 点击后写入文本框的提示词模板 |
| `project_id` | `uuid` | 旧版单项目作用范围，可为空 |
| `project_ids` | `uuid[]` | 当前多项目作用范围 |
| `enabled` | `boolean` | 是否启用 |
| `sort_order` | `integer` | 显示顺序 |
| `preview_object_key` | `text` | 预览图在MinIO中的对象键 |
| `preview_mime_type` | `text` | JPG、PNG或WEBP |
| `preview_size_bytes` | `bigint` | 预览图大小，最大5MB |
| `preview_width` / `preview_height` | `integer` | 预览图真实像素 |
| `created_by` / `updated_by` | `uuid` | 管理员用户ID |
| `created_at` / `updated_at` | `timestamptz` | 创建和更新时间 |

注意：`project_ids`是UUID数组，没有逐项数据库外键，删除项目时由业务服务同步清理作用范围。

## 4. 画布与图片资源

### 4.1 `image_canvases`：无限画布

每个用户可以创建多张私人画布。

| 字段 | 类型 | 含义 |
|---|---|---|
| `id` | `uuid` | 画布主键 |
| `user_id` | `uuid` | 画布所有者，关联`users.id` |
| `name` | `text` | 画布名称，去空格后1–80字符 |
| `project_id` | `uuid` | 旧版画布项目字段；新画布通常为空 |
| `viewport` | `jsonb` | 视野位置和缩放，如`{"x":0,"y":0,"zoom":1}` |
| `version` | `bigint` | 乐观锁版本，每次有效布局修改递增 |
| `deleted_at` | `timestamptz` | 软删除时间；为空表示活动画布 |
| `created_at` / `updated_at` | `timestamptz` | 创建和更新时间 |

关键约束：

- 同一用户的活动画布名称大小写不敏感且唯一。
- 删除进入30天回收站，不立即删除图片资产。
- 删除用户会级联删除该用户的画布。
- `version`用于避免多标签页互相覆盖节点位置。

### 4.2 `image_canvas_nodes`：画布节点

每一行表示画布中的一个图片节点；生成中时可能只有占位节点，成功后再关联资产。

| 字段 | 类型 | 含义 |
|---|---|---|
| `id` | `uuid` | 节点主键 |
| `canvas_id` | `uuid` | 所属画布，画布删除时级联删除节点 |
| `asset_id` | `uuid` | 成功图片对应的`image_assets.id`，生成中可为空 |
| `job_id` | `uuid` | 来源生图任务，任务消失时置空 |
| `output_index` | `integer` | 对应任务中的第几张输出 |
| `status` | `text` | `pending`、`ready`或`failed` |
| `x` / `y` | `double precision` | 无限画布世界坐标 |
| `width` / `height` | `double precision` | 画布显示尺寸，不是原图像素 |
| `z_index` | `integer` | 节点层级 |
| `error` | `text` | 节点失败原因 |
| `background_removal_job_id` | `uuid` | 来源Pixian抠图任务 |
| `source_node_id` | `uuid` | 抠图结果的原始节点，自关联 |
| `requested_size` | `text` | 请求档位/目标尺寸快照 |
| `actual_width` / `actual_height` | `integer` | 图片真实像素 |
| `resolution_warning` | `text` | 中转站没有达到目标尺寸时的非阻塞警告 |
| `generation_relay_name` | `text` | 生成时中转站名称快照 |
| `generation_model_name` | `text` | 生成时模型显示名称快照 |
| `generation_model_key` | `text` | 生成时实际模型ID快照 |
| `created_at` / `updated_at` | `timestamptz` | 创建和更新时间 |

删除节点只把图片从当前画布移除，不等于删除`image_assets`中的图片元数据或MinIO文件。

### 4.3 `image_assets`：图片资产元数据

| 字段 | 类型 | 含义 |
|---|---|---|
| `id` | `uuid` | 资产主键 |
| `owner_id` | `uuid` | 图片所有者 |
| `project_id` | `uuid` | 图片归属项目 |
| `object_key` | `text` | MinIO对象键，全局唯一 |
| `mime_type` | `text` | JPG、PNG或WEBP |
| `file_name` | `text` | 原始或生成文件名 |
| `width` / `height` | `integer` | 图片真实像素 |
| `size_bytes` | `bigint` | 文件字节数 |
| `source` | `text` | `upload`、`generated`或`background_removed` |
| `source_asset_id` | `uuid` | Pixian结果指向原始图片资产，自关联 |
| `created_at` | `timestamptz` | 创建时间 |

数据链路：

```text
PostgreSQL image_assets.object_key
        │
        └── MinIO Bucket中的真实图片文件
```

数据库备份不包含图片本体，交接和恢复时必须同时备份MinIO。

## 5. 生图任务

### 5.1 `image_jobs`：任务主表

一条记录代表一次“生成图片”或“图片反推描述”请求。

| 字段 | 类型 | 含义 |
|---|---|---|
| `id` | `uuid` | 任务主键 |
| `user_id` | `uuid` | 发起用户 |
| `project_id` | `uuid` | 本次任务所属公司项目 |
| `canvas_id` | `uuid` | 结果写入的画布；画布永久删除后置空 |
| `relay_id` | `uuid` | 使用的中转站 |
| `model_id` | `uuid` | 使用的模型配置 |
| `kind` | `text` | `generate`或`reverse_prompt` |
| `prompt` | `text` | 文本描述，当前业务上限15,000字符 |
| `aspect_ratio` | `text` | `original/1:1/16:9/9:16/4:3/3:4` |
| `image_size` | `text` | `1K/2K/4K`目标档位 |
| `count` | `integer` | 生成数量：1、2、4或8 |
| `reference_asset_ids` | `uuid[]` | 参考图资产ID，最多5张由服务校验 |
| `status` | `text` | 任务状态 |
| `attempts` | `integer` | Worker重试次数 |
| `next_attempt_at` | `timestamptz` | 下次允许领取时间 |
| `locked_until` | `timestamptz` | Worker租约截止时间 |
| `completed_count` | `integer` | 已成功输出数量 |
| `reversed_prompt` | `text` | 图片反推得到的文本 |
| `error` | `text` | 任务级错误信息 |
| `idempotency_key` | `text` | 客户端幂等键 |
| `created_at` / `updated_at` | `timestamptz` | 创建和更新时间 |

约束和索引：

- `(user_id, idempotency_key)`唯一，防止重复点击产生重复收费任务。
- `image_jobs_claim_idx(status, next_attempt_at, locked_until)`供Worker领取任务。
- 状态只能是`pending/running/retry/partial/succeeded/failed/cancelled`。

### 5.2 `image_job_outputs`：任务输出明细

一次任务生成多张图片时，每张结果一行。

| 字段 | 类型 | 含义 |
|---|---|---|
| `id` | `uuid` | 输出主键 |
| `job_id` | `uuid` | 所属任务，任务删除时级联删除 |
| `output_index` | `integer` | 第几张输出，与任务联合唯一 |
| `asset_id` | `uuid` | 成功结果图片资产 |
| `status` | `text` | `pending/running/succeeded/failed` |
| `error` | `text` | 单张输出错误 |
| `attempts` | `integer` | 单张处理尝试次数 |
| `requested_size` | `text` | 请求目标尺寸或档位 |
| `actual_width` / `actual_height` | `integer` | 中转站实际返回像素 |
| `resolution_warning` | `text` | 低于目标尺寸时的提示，不会使图片失败 |
| `created_at` / `updated_at` | `timestamptz` | 创建和更新时间 |

典型关系：

```text
image_jobs（count=4）
 ├─ image_job_outputs（output_index=0）→ image_assets → MinIO图片
 ├─ image_job_outputs（output_index=1）→ image_assets → MinIO图片
 ├─ image_job_outputs（output_index=2）→ 失败信息
 └─ image_job_outputs（output_index=3）→ image_assets → MinIO图片
```

这种情况下任务状态可以是`partial`。

## 6. Pixian智能抠图

### 6.1 `pixian_background_removal_config`：全局单例配置

该表固定只有一条`id=true`记录。

| 字段 | 类型 | 含义 |
|---|---|---|
| `id` | `boolean` | 固定为`true`，保证单例 |
| `enabled` | `boolean` | 是否启用抠图功能 |
| `test_mode` | `boolean` | 是否使用Pixian测试模式 |
| `encrypted_api_id` | `text` | 加密后的API ID |
| `encrypted_api_secret` | `text` | 加密后的API Secret |
| `api_id_hint` / `api_secret_hint` | `text` | 后台脱敏提示 |
| `timeout_seconds` | `integer` | 180–600秒 |
| `concurrency` | `integer` | Worker并发，1–5 |
| `max_pixels` | `integer` | 最大输入像素，最高25,000,000 |
| `account_state` | `text` | Pixian账户状态快照 |
| `account_credits` | `double precision` | 余额快照 |
| `account_checked_at` | `timestamptz` | 最近余额检查时间 |
| `updated_by` / `updated_at` | UUID/时间 | 最后修改人和时间 |

### 6.2 `background_removal_jobs`：抠图批次任务

| 字段组 | 含义 |
|---|---|
| `id`、`user_id`、`canvas_id` | 任务、发起用户和目标画布 |
| `status`、`attempts`、`next_attempt_at`、`locked_until` | 状态、重试和Worker租约 |
| `test_mode` | 创建任务时的模式快照 |
| `completed_count`、`failed_count` | 成功和失败图片数 |
| `credits_charged`、`credits_calculated` | Pixian实际/计算Credits合计 |
| `error` | 任务级错误 |
| `idempotency_key` | 用户维度幂等键 |
| `created_at`、`updated_at` | 时间 |

状态只能是`pending/running/retry/partial/succeeded/failed/cancelled`。

### 6.3 `background_removal_items`：逐图抠图明细

| 字段 | 含义 |
|---|---|
| `job_id` | 所属抠图批次 |
| `source_node_id` | 原始画布节点 |
| `source_asset_id` | 原始图片资产 |
| `project_id` | 继承原图所属项目 |
| `placeholder_node_id` | 原图右侧创建的透明图占位节点 |
| `result_asset_id` | 成功后的透明PNG资产 |
| `status` | `pending/running/succeeded/failed` |
| `attempts` | 调用尝试次数 |
| `credits_charged` / `credits_calculated` | 单图实际/计算Credits |
| `input_size` / `result_size` | Pixian响应的输入/输出尺寸信息 |
| `error` | 单图错误 |
| `created_at` / `updated_at` | 时间 |

同一个源节点同时最多存在一个`pending`或`running`抠图明细，防止用户重复点击造成重复扣费。

## 7. 一次生图的数据写入过程

```text
1. 用户选择项目、中转站、模型和画布
2. POST /api/v1/image-agent/jobs
3. 事务写入：
   - image_jobs任务主记录
   - image_job_outputs输出明细
   - image_canvas_nodes生成中占位节点
   - image_canvases.version + 1
4. Worker通过状态、next_attempt_at、locked_until领取任务
5. Worker调用XGAPI或Comfly
6. 返回图片通过MIME、图片解码、域名白名单和ClamAV检查
7. 图片写入MinIO
8. 事务写入：
   - image_assets图片元数据
   - image_job_outputs成功状态和真实分辨率
   - image_canvas_nodes关联asset_id并变为ready
   - image_jobs更新完成数量和最终状态
9. 前端轮询任务并刷新节点内容，保留用户本地拖动坐标
```

## 8. 一次Pixian抠图的数据写入过程

```text
1. 用户框选1–20张ready节点并点击橡皮擦
2. 事务写入：
   - background_removal_jobs批次
   - background_removal_items逐图明细
   - image_canvas_nodes透明图占位节点
3. Worker领取批次并从MinIO读取source_asset
4. 调用Pixian remove-background
5. 透明PNG通过安全检查后写入MinIO
6. 写入新的image_assets：
   source=background_removed
   source_asset_id=原图资产
7. 更新明细、Credits和占位节点
8. 原图保留，透明结果位于原图右侧
```

## 9. 交接人员常用查询

### 查看中转站和模型配置（不查询密钥密文）

```sql
SELECT
    r.name AS relay_name,
    r.base_url,
    r.enabled AS relay_enabled,
    m.display_name,
    m.model_id,
    m.request_model_id,
    m.protocol,
    m.enabled AS model_enabled,
    m.supported_sizes
FROM image_relays r
LEFT JOIN image_models m ON m.relay_id = r.id
ORDER BY r.name, m.display_name;
```

### 查看最近任务及实际分辨率

```sql
SELECT
    j.id,
    r.name AS relay,
    m.display_name AS model,
    j.image_size,
    j.aspect_ratio,
    j.status,
    o.output_index,
    o.actual_width,
    o.actual_height,
    o.resolution_warning,
    j.created_at
FROM image_jobs j
JOIN image_relays r ON r.id = j.relay_id
JOIN image_models m ON m.id = j.model_id
LEFT JOIN image_job_outputs o ON o.job_id = j.id
ORDER BY j.created_at DESC, o.output_index
LIMIT 100;
```

### 查看画布、节点和图片对象

```sql
SELECT
    c.name AS canvas_name,
    n.id AS node_id,
    n.status,
    n.x,
    n.y,
    n.actual_width,
    n.actual_height,
    a.object_key,
    a.source,
    a.mime_type,
    a.size_bytes
FROM image_canvases c
JOIN image_canvas_nodes n ON n.canvas_id = c.id
LEFT JOIN image_assets a ON a.id = n.asset_id
WHERE c.deleted_at IS NULL
ORDER BY c.updated_at DESC, n.z_index;
```

### 查看失败任务

```sql
SELECT id, status, attempts, error, created_at, updated_at
FROM image_jobs
WHERE status IN ('failed', 'partial', 'retry')
ORDER BY updated_at DESC
LIMIT 100;
```

### 查看Pixian调用和扣费

```sql
SELECT
    j.id,
    j.status,
    j.test_mode,
    j.completed_count,
    j.failed_count,
    j.credits_charged,
    j.credits_calculated,
    j.error,
    j.created_at
FROM background_removal_jobs j
ORDER BY j.created_at DESC
LIMIT 100;
```

## 10. 不要直接修改的字段

以下数据不能通过DBeaver手工修改，应通过后台或API操作：

- `encrypted_api_key`、`encrypted_api_id`、`encrypted_api_secret`：使用应用密钥加密，手工写值会导致无法解密。
- `image_canvases.version`：参与乐观锁，错误修改会造成画布保存冲突。
- 任务的`status/locked_until/attempts`：由Worker状态机维护。
- `completed_count`和输出状态：必须与`image_job_outputs`保持一致。
- `object_key`：必须与MinIO真实对象对应。
- 画布节点的`asset_id/job_id`：涉及任务、资产和画布三方关系。
- `project_ids`：应通过服务同步校验项目范围。

开发环境确需修复数据时，也应先备份并在事务中操作。

## 11. 本机查看命令

进入数据库：

```powershell
docker compose exec postgres psql -U ai_agent -d ai_agent
```

在`psql`中执行：

```sql
\dt image_*
\dt background_removal_*
\d+ image_jobs
\d+ image_canvas_nodes
\d+ image_assets
\d+ background_removal_jobs
```

退出：

```sql
\q
```

只导出当前数据库结构供交接查看：

```powershell
docker compose exec -T postgres pg_dump -U ai_agent -d ai_agent --schema-only > ai_agent_schema.sql
```

该命令不包含表数据和图片文件。

## 12. 迁移文件来源

生图数据库结构由以下迁移逐步形成：

```text
011_image_agent.sql
012_comfly_image_output_host.sql
015_comfly_apiproxy_output_host.sql
017_image_prompt_action_previews.sql
020_image_canvas_multi.sql
021_xgapi_image_output_host.sql
022_image_prompt_action_multi_projects.sql
023_image_original_ratio.sql
024_xgapi_apiproxy_output_host.sql
025_pixian_background_removal.sql
026_comfly_aiproxy_output_host.sql
027_gpt_image_2_resolution.sql
028_image_node_generation_source.sql
029_xgapi_base_url.sql
030_image_model_relay_routing.sql
031_gpt_image_2_pixel_sizes.sql
```

独立部署AI生图服务时必须按顺序执行这些迁移，并同时保留`users.id`的可信映射、MinIO对象和`AGENT_SECRET_ENCRYPTION_KEY`。
