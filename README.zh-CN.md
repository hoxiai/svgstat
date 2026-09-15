# SVGStat

> 把每一次访问，变成看得见的增长信号。

[English](./README.md)

SVGStat 是一个基于 Go 构建、面向开发者展示场景的 SVG 数据统计平台。它不是为传统网站后台而生，而是为 GitHub README、文档站、官网落地页、变更日志和内部工作台这些高曝光位置而设计。

你不需要接入前端埋点脚本，不需要维护截图，也不需要额外做组件封装。SVGStat 直接把 SVG 图片请求次数变成可嵌入的实时地址，让你的项目在任何支持图片的地方都能展示活跃度。

## 预览

![SVGStat 控制台预览](./preview_zh.png)

## 为什么选择 SVGStat

大多数统计工具关注的是“站点后台”。

SVGStat 关注的是“项目展示面”：

- GitHub README
- Markdown 文档
- 文档门户
- 静态网站
- 开源项目主页
- 内部工程工作台

它把展示能力和分析能力放进同一条工作流里：

- 用 SVG 地址发布实时计数器
- 生成更适合公开展示的高质量徽章
- 在 GitHub 隐藏来源页时，通过 `page_id` 做页面归因
- 查看 PV、UV、来源页、国家地区、设备、浏览器和最近访客
- 在一个多项目工作区里实时预览并复制可直接上线的嵌入代码

## 你能获得什么

### 实时 SVG 计数器

把 SVG 图片请求次数发布成轻量计数器，适合任何支持 Markdown 图片语法或 `<img>` 标签的环境。它统计的是图片端点请求，不等同于 GitHub 的真实访客、下载量或 Star 数。

### 更适合公开展示的 SVG 徽章

生成可直接投入生产使用的徽章，支持文案、颜色、样式和首页跳转链接，适合 README、官网、产品文档和公开状态展示位。

### 真正可用的分析工作区

在一个聚焦的 SPA 控制台里看到每个徽章和计数器背后的真实数据，目前已经覆盖：

- 页面访问量
- 独立访客
- 来源页与访问页面
- 国家地区
- 设备类型
- 浏览器分布
- 匿名访客明细（不存储原始 IP）

### 面向 GitHub 的页面归因

GitHub 的图片代理经常会隐藏原始来源页。SVGStat 已支持 `page_id`，即使徽章嵌入在 README 或其他 Markdown 页面里，你也依然可以把访问归因到正确页面，而不是只看到一团模糊流量。

### 免费公共徽章节点

SVGStat 还内置了一个无需注册即可使用的公共徽章节点。每个 `page_id` 都会独立计数，非常适合快速生成 GitHub 访客徽章。

## 实际演示

- 产品站点：[https://svgstat.com](https://svgstat.com)
- 免费公共徽章：`https://svgstat.com/svg/free/badge/visitor.svg?label=visitors&page_id=github.com/svgstat/demo`
- 演示项目标识：`demo`
- 计数器地址：`https://svgstat.com/svg/demo/counter/visits.svg?label=Visits&color=7c3aed&page_id=github.com/svgstat/demo`
- 徽章地址：`https://svgstat.com/svg/demo/badge/requests.svg?label=Requests&color=0ea5e9&style=flat&page_id=github.com/svgstat/demo`
- Markdown 嵌入：

```markdown
![Visits](https://svgstat.com/svg/demo/counter/visits.svg?label=Visits&color=7c3aed&page_id=github.com/svgstat/demo)
```

![Visits](https://svgstat.com/svg/demo/counter/visits.svg?label=Visits&color=7c3aed&page_id=github.com/svgstat/demo)

## 快速开始

### 环境要求

- Go `1.25.1`
- Podman 或 Docker
- Podman Compose 或 Docker Compose

### 1. 准备环境变量

```bash
cp .env.example .env
```

### 2. 启动 PostgreSQL 与 Redis

```bash
make up
```

### 3. 执行数据库迁移

```bash
make migrate-up
```

### 4. 启动应用

使用热重载开发模式：

```bash
make watch
```

或者直接启动 API：

```bash
go run cmd/api/main.go
```

### 5. 打开应用

- SPA 首页：[http://localhost:8080](http://localhost:8080)
- 健康检查：[http://localhost:8080/health](http://localhost:8080/health)
- 就绪检查：[http://localhost:8080/ready](http://localhost:8080/ready)
- Prometheus 指标：[http://localhost:8080/metrics](http://localhost:8080/metrics)

### 可选：初始化本地测试数据

```bash
go run scripts/init_test_data.go
```

## API 与嵌入示例

### 网站访问统计

在普通 HTML 网站、WordPress 主题或 SPA 的 `</head>` 前加入一行脚本，并把 `my-project` 替换为控制台显示的项目标识：

```html
<script defer src="https://svgstat.com/sdk.js" data-project="my-project"></script>
```

SDK 会记录首次页面访问，并自动识别 History API、前进后退、查询参数及 hash 路由变化。它不使用 Cookie，匿名访客标识只保存在 `sessionStorage`。自定义虚拟页面可以调用 `window.svgstatTrack('/virtual-page')`。

无需再写代码，SDK 还会通过事件委托自动采集当前及动态插入元素上的四类实用行为：`outbound_click`（外链）、`file_download`（文件下载）、`contact_click`（邮件/电话联系）和 `form_submit`（表单提交）。自动 URL 明细只保留域名和路径，不包含查询参数或 hash；联系事件只记录 `email` 或 `phone` 类型；表单事件不会读取输入值或按钮文字。

同一自动模式还会通过浏览器 Performance Observer API 测量真实访客的 LCP、INP 和 CLS，并统计 JavaScript 与资源加载失败的安全类别。控制台使用标准 Core Web Vitals 阈值评级，并显示受影响匿名访客数。系统不会采集报错消息、Promise 内容、调用堆栈、DOM 选择器或 URL 查询参数/hash；每次页面加载最多上报 20 个错误，避免错误风暴，不支持相关 API 的浏览器会安全跳过。

需要业务名称或明确属性时，可以直接在元素上声明自定义事件：

```html
<button data-svgstat-event="signup">注册</button>
<button data-svgstat-event="purchase" data-svgstat-value="99" data-svgstat-currency="CNY" data-svgstat-property-plan="pro">购买</button>
```

在容器上添加 `data-svgstat-ignore` 可以跳过其中的行为事件；在 SDK 脚本上添加 `data-auto-track="false"` 可以关闭自动行为和质量监控，同时保留页面浏览和手动事件。`data-svgstat-property-*` 的值由站点主动配置，请勿写入个人信息或敏感数据。

网站事件发送到 `POST /api/v1/collect`。控制台支持精确域名、`*.example.com` 通配子域名和本地开发地址。域名列表留空时，为方便快速接入会允许任意 HTTP(S) 来源；正式上线前建议配置域名。

### 计数器 SVG

```text
GET /svg/{projectSlug}/counter/{name}.svg
```

示例：

```text
https://svgstat.com/svg/demo/counter/visits.svg?label=Visits&color=brightgreen
```

### 徽章 SVG

```text
GET /svg/{projectSlug}/badge/{name}.svg
```

示例：

```text
https://svgstat.com/svg/demo/badge/requests.svg?label=Requests&style=flat
```

### 项目统计

```text
GET /api/v1/projects/{id}/stats
```

### 历史趋势

```text
GET /api/v1/projects/{id}/stats/trend?days=30
```

`days` 支持 `7`、`30` 或 `90`。响应包含连续日期的 PV、UV、SVG 请求数和机器人请求数。

### 实时与周期分析

```text
GET /api/v1/projects/{id}/stats/realtime
GET /api/v1/projects/{id}/analysis?days=30
GET /api/v1/projects/{id}/session-quality?days=30
GET /api/v1/projects/{id}/issues?days=30
```

实时统计包含最近 5 分钟和 30 分钟的页面浏览量与独立访客。周期分析支持 7、30、90 天，与紧邻的上一周期对比，并返回页面、来源页、国家、设备、浏览器、UTM 来源、媒介和活动分布。访问质量返回入口页、退出页、页面流转和渠道质量。周期 UV 是每日独立访客数之和；页面分布会移除查询参数，以减少敏感信息泄露与维度膨胀。

问题报告把这些聚合数据转成有优先级的行动建议，不保存原始事件或访客轨迹。为减少误报，流量下降要求上一周期至少 100 PV；转化下降只检查已经配置、且上一周期至少有 50 名访客和 5 名转化者的目标；核心体验指标要求当前至少 20 个样本；采集中断要求过去确有流量且连续两天没有访问。JavaScript 与资源错误按受影响匿名访客比例判断，而不是只看错误次数。

### 安装状态

```text
GET /api/v1/projects/{id}/installation
```

在 SVGStat 收到项目首个真实网站、计数器或徽章请求前返回 `pending`，收到后返回 `installed`，并包含首次及最近请求时间。

控制台预览会附加 `preview=1`。预览请求只渲染 SVG，不增加计数、不记录分析数据，也不会把项目标记为已安装。公开嵌入时请使用生成的不含 `preview=1` 的地址。

### 认证接口

```text
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/auth/me
```

## 架构概览

SVGStat 将渲染、统计与项目管理聚合在一个高聚焦的 Go 服务中，并配套轻量 SPA 前端。

```text
README / 文档站 / 官网 / 控制台
              │
              ▼
         SVGStat HTTP 层
       ┌──────┼───────┐
       ▼      ▼       ▼
      SPA    API   SVG 渲染
              │
              ▼
            Redis
              │
              ▼
         PostgreSQL
```

设计原则：

- 性能优先
- Redis 优先处理高频事件
- SVG 渲染尽量无状态
- API First
- 渲染与统计职责清晰分离

## 项目结构

```text
cmd/
  api/        # API 服务入口
  migrate/    # 数据库迁移命令

internal/
  analytics/  # 统计聚合
  api/        # 路由与处理器
  auth/       # 认证与会话逻辑
  cache/      # 缓存层
  config/     # 配置加载
  counter/    # 计数器 SVG 生成
  database/   # 数据库初始化
  geoip/      # GeoIP 查询
  migrate/    # 迁移执行器
  project/    # 项目数据访问
  renderer/   # 通用 SVG 渲染
  worker/     # 统计落库 worker

migrations/   # SQL 迁移文件
scripts/      # 辅助脚本
web/          # Alpine.js SPA 前端
resource/     # 静态资源
```

## 技术栈

- Go `1.25.1`
- PostgreSQL `16`
- Redis `7`
- Gorilla Mux
- pgx `v5`
- go-redis `v9`
- Alpine.js
- UnoCSS Runtime

## 管理端

管理端提供平台概览、用户启停、项目状态与能力开关管理。它使用独立页面，不会进入 SVG 渲染热路径。

1. 在 `.env` 中设置管理员邮箱：

   ```text
   ADMIN_EMAILS=admin@example.com
   ```

2. 执行 `make migrate-up` 并重启服务。已有同邮箱账号会自动获得管理员角色；也可以在配置后使用该邮箱注册。
3. 访问 [http://localhost:8080/admin](http://localhost:8080/admin)。

管理端的用户和项目状态修改会写入 `admin_audit_logs`。停用用户会立即撤销其全部会话；项目停用会同步刷新运行缓存。

## 生产安全配置

- `HTTP_TRUSTED_PROXIES` 只配置允许提供 `X-Forwarded-*` 请求头的代理 IP 或 CIDR。
- Session 凭据以 SHA-256 哈希保存；迁移 `021` 会让旧 Session 保持有效直到正常过期。
- Redis 负责跨实例认证/采集限流，并广播项目运行缓存失效消息。
- PostgreSQL advisory lock 保证每轮只有一个启用的 Worker 实例落库；无需运行 Worker 的实例设置 `WORKER_ENABLED=false`。
- `ANALYTICS_MAX_DAILY_VISITORS` 与 `ANALYTICS_MAX_DIMENSION_VALUES` 限制每个项目每天的精确 Redis 数据规模；达到上限后仍继续记录聚合总量。

## 路线图

### 当前已具备

- 动态 SVG 计数器
- 动态 SVG 徽章
- 项目统计面板
- 流量分析
- 用户认证与项目管理

### 下一阶段

- 更丰富的 SVG Widgets
- 趋势视图与图表能力
- 项目公开统计页
- 团队协作支持

### 后续阶段

- SVG 原生评论能力
- 模板与市场能力
- 更完善的自托管体验

## 文档

- [GETTING_STARTED.md](./GETTING_STARTED.md)
- [API.md](./API.md)
- [ARCHITECTURE.md](./ARCHITECTURE.md)
- [DATABASE.md](./DATABASE.md)
- [MIGRATIONS.md](./MIGRATIONS.md)
- [ANALYTICS.md](./ANALYTICS.md)
- [RENDERER.md](./RENDERER.md)
- [REDIS.md](./REDIS.md)
- [INTEGRATION.md](./INTEGRATION.md)
- [CONTRIBUTING.md](./CONTRIBUTING.md)
- [AGENT.md](./AGENT.md)

## 参与贡献

欢迎提交 Issue 和 PR。

在开始贡献前，建议先阅读：

- [CONTRIBUTING.md](./CONTRIBUTING.md)
- [AGENT.md](./AGENT.md)

### 自定义事件与转化分析

除元素声明外，也可以用 JavaScript 采集注册、购买等产品行为：

```js
window.svgstat('event', 'signup')
window.svgstat('event', 'purchase', { value: 99, currency: 'CNY' })
```

控制台可以将事件配置为转化目标和有序漏斗，查看每日趋势，并按来源、媒介、活动、页面、设备和国家比较转化率与漏斗完成率。分群转化率按所选周期内“每日匿名转化人数之和 / 对应分群每日匿名访客之和”计算；系统不保存原始事件流水或长期身份。

访问质量分析提供 30 分钟会话、跳出率、每次访问页数、估算参与时长、入口页、退出页、相邻页面流转，以及按首次触达渠道和设备的质量对比。仅浏览一页的会话记为跳出；参与时长只累计页面浏览之间的时间，不虚构单页停留时长。页面流转只保存聚合边，不保存可回放的访客完整轨迹。

接入验收时可以使用测试模式。测试流量会在实时调试器中保留 30 分钟，但绝不会影响正式统计：

```html
<script defer src="https://svgstat.com/sdk.js" data-project="my-project" data-mode="test"></script>
```

## 许可证

MIT License.

## 愿景

SVGStat 不只是一个访问计数器。

它想成为面向开发者展示场景的 SVG 数据基础设施，让统计能力像静态资源一样易缓存、像图片一样易嵌入、像产品指标一样持续产生说服力。
