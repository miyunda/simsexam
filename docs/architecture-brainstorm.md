# simsexam 架构草案

本文档基于当前仓库中的原型实现整理，目标不是一次性定死所有细节，而是先把接下来 2-3 个迭代的技术方向、数据模型和开发顺序定清楚。

## 1. 当前原型的现状判断

当前项目已经具备最小可运行能力：

- Go 单体 Web 服务
- SQLite 数据库
- 题库、考试流程、成绩页的基本页面
- 初始测试题自动导入
- 一个命令行导入器

但从产品目标看，当前实现仍然是原型：

- 领域模型过薄，无法支撑管理员题库维护、错题本、收费、登录
- `exam_sessions` 和内存态 `activeSessions` 不适合多实例和长期运行
- 题库 schema 无法表达导入来源、版本、题目状态、科目配置
- Markdown 导入格式还没有被正式定义
- 模板与 handler 耦合较紧，不利于后续扩展后台管理和 API
- 前端主题目前只有一套静态样式，没有真正的浅色/深色/跟随系统切换

结论：现在最合适的方向不是继续补丁式加功能，而是把项目提升为“可持续演进的单体应用”。

## 2. 建议的总体架构

第一阶段建议继续保持单体架构，不要过早拆微服务。

推荐结构：

- `cmd/server`: Web 入口
- `internal/app`: 应用装配、配置、依赖注入
- `internal/domain`: 领域模型和核心规则
- `internal/service`: 用例层，组织业务流程
- `internal/repository`: 数据访问层
- `internal/http`: 页面路由、API 路由、中间件
- `internal/auth`: 登录、用户身份、权限
- `internal/importer`: Markdown 题库解析与导入
- `internal/bootstrap`: 初始化、迁移、内置测试题导入

建议坚持三条边界：

- handler 不直接拼业务流程，只负责 HTTP 输入输出
- service 不感知模板和 HTTP，只处理业务规则
- repository 不夹带业务决策，只做持久化

这会让你之后同时支持：

- Web 页面
- Web API
- 将来的 iOS App

## 3. Web 界面建议

产品上建议明确拆成 3 个界面区域：

- 用户端
- 管理后台
- 公共登录/账户页面

### 用户端

核心页面：

- 首页：展示可考试科目、免费/付费状态、说明
- 科目详情页：考试时长、题量、说明、是否需要购买
- 考试页：逐题作答、进度、剩余时间、提交
- 成绩页：分数、错题、正确答案、解析
- 错题本：按科目查看历史错题，可标记已掌握
- 个人中心：登录方式、订阅/购买状态、考试记录

### 管理后台

核心页面：

- 科目列表
- 新建/编辑科目
- 导入题库
- 题目列表
- 单题编辑
- 导入记录与错误报告

管理员实际最常用的能力不是“重新导入全部”，而是：

- 看某个科目当前配置
- 搜索题目
- 修改单题
- 下线题目
- 重新导入并比对变更

所以后台一定要把“题库浏览和单题编辑”作为一等功能，而不只是上传文件。

### 主题与设计方向

建议继续保留你当前偏 rose-pine 的视觉气质，但结构上改为真正的 design token：

- `light`
- `dark`
- `system`

实现上建议：

- 用 CSS 变量管理色板、边框、阴影、语义色
- 服务端模板先输出基础结构
- 用少量原生 JS 处理主题切换和本地持久化
- 组件风格统一，不要在模板里写内联样式

兼容性目标：

- 桌面端：Chrome、Safari、Firefox、Edge 最新两个主版本
- 移动端：iPhone Safari、Android Chrome

## 4. API 与 iOS 预留

即使当前先做服务端渲染页面，也应该从现在开始把“用户端能力”按 API 可复用方式组织。

建议路线：

- 第一阶段：SSR 页面优先，后台也先用 SSR
- 同时在 service 层保证不依赖模板
- 第二阶段：补用户端 JSON API
- 第三阶段：iOS App 直接复用认证、题库、考试、错题、购买相关 API

建议 API 边界：

- `/api/v1/auth/...`
- `/api/v1/subjects/...`
- `/api/v1/exams/...`
- `/api/v1/review/...`
- `/api/v1/admin/...`

这比一开始就上前后端完全分离更稳，也更适合你目前的人力。

## 5. 认证与收费建议

登录建议从一开始就预留，但不要一开始自己做密码体系。

推荐优先级：

1. Google 登录
2. Apple 登录
3. 邮箱魔法链接，作为兜底

原因：

- 自建密码登录的安全和找回成本高
- iOS 方向会逼近 Apple 登录
- Google/Apple 足够覆盖大部分用户

建议身份模型：

- 用户表存站内主身份
- OAuth 账号表存第三方身份映射
- Session/Cookie 做 Web 登录态
- 后续为移动端补 token 机制

收费建议先做“授权模型”，暂不急着真正接支付：

- 科目是否免费
- 用户是否拥有某个科目访问权
- 后续再接 Stripe、Apple IAP 或兑换码

不要把“是否收费”硬编码在 `subjects` 上的一个布尔值里就结束，至少要能表达：

- 免费
- 付费
- 隐藏/未发布
- 下线

## 6. 题库导入格式建议

你前面说的那部分内容，本质上可以叫：

- `subject manifest`
- 或 `subject config`

如果是跟题目一起放在一个 markdown 文件里，我建议正式叫：

- `import manifest`

它描述的是“这次导入包”的元数据，而不是单纯的科目字段。

建议一个 Markdown 文件结构如下：

```md
# Subject: AWS CLF-C02

## Meta
- slug: aws-clf-c02
- title: AWS Certified Cloud Practitioner
- description: 入门模拟题
- duration_minutes: 90
- question_count: 65
- price_type: free
- published: true

---

## Question
type: single

What does AWS Lambda allow users to do?

- [x] Run code without provisioning servers
- [ ] Create relational schemas automatically
- [ ] Replace IAM entirely
- [ ] Deploy physical network devices

### Explanation
AWS Lambda is a serverless compute service.

---

## Question
type: multiple

Which services are storage services? Choose two.

- [x] Amazon S3
- [x] Amazon EBS
- [ ] Amazon CloudFront
- [ ] AWS Lambda

### Explanation
S3 and EBS are storage services.
```

这样比当前“靠粗糙正则和加粗答案”稳得多，也更方便后台校验。

导入器应支持：

- 解析 manifest
- 校验题目格式
- 事务导入
- 输出导入报告
- 幂等策略

幂等策略建议不要靠题目文本直接覆盖，而是引入稳定标识：

- `external_key` 或 `source_ref`

如果源文件没有显式 key，可以先用题干归一化哈希，但长期看最好允许题目自己带一个 key。

## 7. 建议的数据库 schema

数据库建议继续从 SQLite 起步，但 schema 按 PostgreSQL 风格设计，避免以后迁移太痛。

### 7.1 用户与认证

#### users

- `id`
- `email`
- `display_name`
- `avatar_url`
- `status` (`active`, `disabled`)
- `role` (`user`, `admin`)
- `created_at`
- `updated_at`

#### user_identities

- `id`
- `user_id`
- `provider` (`google`, `apple`, `email_link`)
- `provider_user_id`
- `provider_email`
- `created_at`

唯一约束：

- `(provider, provider_user_id)`

### 7.2 科目与题库

#### subjects

- `id`
- `slug`
- `title`
- `description`
- `duration_minutes`
- `question_count`
- `access_level` (`free`, `paid`, `private`)
- `status` (`draft`, `published`, `archived`)
- `created_at`
- `updated_at`

#### question_sets

表示某个科目的一次题库版本或导入版本。

- `id`
- `subject_id`
- `version`
- `source_type` (`seed`, `markdown_import`, `manual`)
- `source_name`
- `is_active`
- `created_by`
- `created_at`

#### questions

- `id`
- `subject_id`
- `question_set_id`
- `external_key`
- `type` (`single`, `multiple`)
- `stem_markdown`
- `explanation_markdown`
- `status` (`active`, `disabled`)
- `created_at`
- `updated_at`

#### question_options

- `id`
- `question_id`
- `option_key` (`A`, `B`, `C`, `D`, ...)
- `content_markdown`
- `sort_order`
- `is_correct`

### 7.3 导入与后台维护

#### import_jobs

- `id`
- `subject_id`
- `source_filename`
- `source_checksum`
- `status` (`pending`, `validated`, `imported`, `failed`)
- `manifest_json`
- `error_report`
- `created_by`
- `created_at`

#### question_revisions

给管理员后续改单题留痕。

- `id`
- `question_id`
- `editor_user_id`
- `change_summary`
- `snapshot_json`
- `created_at`

### 7.4 考试与答题

#### exams

一次考试实例。

- `id`
- `user_id` 可空
- `subject_id`
- `question_set_id`
- `mode` (`practice`, `formal`)
- `status` (`in_progress`, `submitted`, `expired`)
- `started_at`
- `submitted_at`
- `expires_at`
- `score`

#### exam_questions

固化当次考试抽到的题序，避免只放内存。

- `id`
- `exam_id`
- `question_id`
- `position`

唯一约束：

- `(exam_id, position)`

#### exam_answers

- `id`
- `exam_id`
- `question_id`
- `answered_at`
- `is_correct`

#### exam_answer_options

- `id`
- `exam_answer_id`
- `option_id`

这样单选和多选都能统一表达。

### 7.5 错题本与权限

#### user_question_stats

给错题本和学习追踪用。

- `id`
- `user_id`
- `question_id`
- `wrong_count`
- `correct_count`
- `last_answered_at`
- `last_wrong_at`
- `mastery_status` (`new`, `weak`, `mastered`)

#### subject_entitlements

用户是否可以访问某科目。

- `id`
- `user_id`
- `subject_id`
- `source` (`free`, `purchase`, `gift`, `admin`)
- `starts_at`
- `ends_at`

## 8. 关于内置测试题

这部分应该保留，但不要继续写死在裸 SQL seed 里。

建议改成：

- 内置一份正式的 `seed` 题库文件
- 首次运行自动执行导入
- 在数据库里标记 `source_type = seed`

这样好处是：

- 测试题走和正式题库一致的导入链路
- 能顺便验证导入器
- 管理员可以观察完整流程

## 9. 技术选型建议

先给一个务实组合：

- 后端：Go
- Router: Chi
- DB：SQLite 起步，预留 PostgreSQL
- Migration: `golang-migrate` 或 `goose`
- HTML：服务端模板
- CSS：原生 CSS + design tokens
- JS：原生少量增强
- Auth：OAuth 2.0 / OIDC

现阶段不建议立刻引入：

- 微服务
- GraphQL
- 前后端完全分离 SPA
- Redis
- 复杂消息队列

如果以后并发和部署规模上来，再考虑：

- PostgreSQL
- Redis 会话/缓存
- 对象存储保存导入原文件

## 10. 分阶段开发路线

### Phase 0：定规格

- 确认领域术语
- 定 Markdown 导入格式
- 定数据库 schema
- 定用户端和后台的最小页面范围

### Phase 1：重构地基

- 引入 migration
- 重建 schema
- 把考试会话从内存迁到数据库
- 把 seed 题改为走导入器
- 分离 handler/service/repository

### Phase 2：管理员题库能力

- 科目管理
- Markdown 导入
- 导入校验报告
- 题目列表
- 单题编辑

### Phase 3：用户学习闭环

- 登录
- 考试历史
- 错题本
- 科目访问控制

### Phase 4：商业化与 App 预留

- 支付/授权接入
- JSON API 补齐
- iOS 客户端对接

## 11. 我建议你现在优先确认的决定

当前最值得尽快拍板的，不是 UI 细节，而是这 6 件事：

1. 先坚持单体架构，不拆服务。
2. 题库导入格式要正式化，不再靠隐式约定。
3. 考试会话必须持久化到数据库，不能只放内存。
4. 后台题目编辑必须是正式能力，不依赖重复导入修正。
5. 登录优先第三方 OAuth，不自建密码体系。
6. schema 从一开始就预留用户、授权、版本化题库、错题统计。

## 12. 下一步建议

最合适的下一步不是直接写页面，而是先完成一份更严格的技术规格：

- 题库 Markdown 格式规范
- 第一版数据库 ERD
- Phase 1 重构任务拆分

这三样定下来以后，再开始改代码，返工会少很多。
