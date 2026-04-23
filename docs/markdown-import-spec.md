# simsexam Markdown 导入规范 v1

本文档定义 `SE` 的题库 Markdown 导入格式、校验规则和幂等策略。目标是让以下场景共享同一套规范：

- 管理后台上传题库
- 命令行导入
- 首次运行导入内置测试题

本文档中的 MUST / SHOULD / MAY 含义如下：

- MUST：必须满足，否则导入失败
- SHOULD：建议满足，不满足时通常给出警告
- MAY：可选

## 1. 设计目标

v1 导入格式必须满足：

- 人类可编辑
- 纯文本、适合版本控制
- 能明确表达科目配置和题目内容
- 能稳定区分单选题、多选题
- 能支持导入校验和幂等更新
- 能作为后台与 seed 数据的统一输入格式

v1 不解决的事情：

- 图片、音频、视频等富媒体资源
- 题目分组、章节树、难度分层
- 复杂公式和附件引用

## 2. 文件级约束

一个导入文件表示一个科目的一个题库版本。

文件必须包含两个部分：

1. Manifest
2. 一个或多个 Question Block

文件编码要求：

- MUST 使用 UTF-8
- SHOULD 使用 LF 换行

推荐文件名：

- `<subject-slug>.md`
- `<subject-slug>.<version>.md`

## 3. 顶层结构

完整结构如下：

```md
# Subject: aws-clf-c02

## Meta
- slug: aws-clf-c02
- title: AWS Certified Cloud Practitioner
- description: AWS Cloud Practitioner 模拟题
- duration_minutes: 90
- question_count: 65
- access_level: free
- status: published
- version: 2026-04-01

---

## Question
key: clf-001
type: single

What does AWS Lambda allow users to do?

- [x] Run code without provisioning servers
- [ ] Replace IAM entirely
- [ ] Create relational schemas automatically
- [ ] Deploy physical network devices

### Explanation
AWS Lambda is a serverless compute service.

---

## Question
key: clf-002
type: multiple

Which services are storage services? Choose two.

- [x] Amazon S3
- [x] Amazon EBS
- [ ] Amazon CloudFront
- [ ] AWS Lambda

### Explanation
S3 and EBS are storage services.
```

## 4. Manifest 规范

Manifest 是文件头部的科目与导入元数据。

### 4.1 Header

第一行必须是：

```md
# Subject: <subject-slug>
```

规则：

- MUST 存在
- `<subject-slug>` MUST 与 `Meta.slug` 一致
- `slug` MUST 仅包含小写字母、数字和中划线

推荐正则：

```text
^[a-z0-9]+(?:-[a-z0-9]+)*$
```

### 4.2 Meta Section

Header 后必须紧跟：

```md
## Meta
```

Meta 内容为 Markdown 列表，每行一个键值：

```md
- key: value
```

v1 支持字段如下。

#### 必填字段

- `slug`
- `title`
- `duration_minutes`
- `question_count`
- `access_level`
- `status`

#### 可选字段

- `description`
- `version`

#### 字段定义

`slug`

- MUST 唯一标识一个科目
- MUST 匹配 slug 正则

`title`

- MUST 为 1 到 120 个字符

`description`

- MAY 为空
- SHOULD 不超过 1000 个字符

`duration_minutes`

- MUST 为正整数
- SHOULD 在 `1..600`

`question_count`

- MUST 为正整数
- 表示该科目每次正式考试默认抽题数量

`access_level`

- MUST 为以下之一：
  - `free`
  - `paid`
  - `private`

`status`

- MUST 为以下之一：
  - `draft`
  - `published`
  - `archived`

`version`

- MAY 为任意非空字符串
- SHOULD 使用稳定格式，例如 `2026-04-01` 或 `v1`

### 4.3 Manifest 示例

```md
# Subject: se-demo

## Meta
- slug: se-demo
- title: SE Demo Subject
- description: 用于首次运行时导入的演示题
- duration_minutes: 20
- question_count: 10
- access_level: free
- status: published
- version: 2026-04-23
```

## 5. Question Block 规范

每道题由 `---` 分隔。

每个题块必须以：

```md
## Question
```

开始。

### 5.1 题块结构

```md
## Question
key: demo-001
type: single

Question stem here.

- [x] Option A
- [ ] Option B
- [ ] Option C
- [ ] Option D

### Explanation
Optional explanation here.
```

### 5.2 题块字段

`key`

- MUST 存在
- MUST 在同一文件内唯一
- SHOULD 在同一科目内长期稳定
- 推荐格式：`<short-prefix>-<number>`

`type`

- MUST 为以下之一：
  - `single`
  - `multiple`

### 5.3 Stem

题干位于字段区之后、选项区之前。

规则：

- MUST 非空
- 可以包含多行 Markdown
- MUST 不包含 `### Explanation` 标题

### 5.4 Options

选项使用 checklist 语法：

```md
- [x] Correct option
- [ ] Incorrect option
```

规则：

- 每题 MUST 至少有 2 个选项
- 每题 SHOULD 有 4 个选项
- 每个选项文本 MUST 非空
- 选项顺序即展示顺序
- `single` 题 MUST 恰好有 1 个正确选项
- `multiple` 题 MUST 至少有 2 个正确选项

### 5.5 Explanation

解释区可选：

```md
### Explanation
Explanation markdown here.
```

规则：

- MAY 缺省
- 如果存在，标题 MUST 为 `### Explanation`
- 内容可以是多行 Markdown

## 6. 解析顺序

导入器应按以下步骤解析：

1. 解析 Header
2. 解析 `## Meta`
3. 以 `---` 分割 Question Block
4. 对每个 Question Block 解析字段、题干、选项、解析
5. 对整份文件执行交叉校验
6. 生成导入计划
7. 在事务中写入数据库

## 7. 校验规则

### 7.1 文件级错误

以下情况 MUST 失败：

- 缺少 `# Subject: ...`
- 缺少 `## Meta`
- Manifest 缺失必填字段
- 没有任何题目
- Header slug 与 Meta.slug 不一致
- `question_count` 大于有效题目总数

### 7.2 题目级错误

以下情况 MUST 失败：

- 缺少 `## Question`
- 缺少 `key`
- 缺少 `type`
- `type` 非法
- 题干为空
- 选项少于 2 个
- `single` 没有且仅有 1 个正确答案
- `multiple` 正确答案少于 2 个
- 同一文件内 `key` 重复

### 7.3 警告

以下情况 SHOULD 产生 warning，但不一定阻止导入：

- 选项数不是 4
- 题干或解析过长
- 某些选项文本高度相似
- 缺少 `Explanation`

## 8. 幂等与更新策略

v1 导入以 `(subject_slug, question_key)` 作为稳定身份。

规则：

- 若 `subject` 不存在，则创建
- 若 `subject` 已存在，则更新 manifest 中允许覆盖的字段
- 每次成功导入都创建新的 `question_set`
- 导入器按本次文件内容为该 `question_set` 写入一批题目快照
- 题目的业务稳定标识仍然是 `(subject_slug, key)`
- 历史 `question_set` 中的旧题快照不被覆盖

这意味着：

- 幂等判断依赖 `subject_slug + version + source_checksum` 或等价规则
- 学习统计与后台比对应以稳定 `key` 为准，而不是某一行数据库 `question_id`

v1 不自动删除数据库中“导入文件里缺失”的旧题版本，避免误删。

对于导入文件里缺失的旧题，后台应提供两种后续处理方式：

- 手工停用
- 明确执行“同步删除/停用”模式

因此导入模式建议支持：

- `new_version`，默认
- `sync_disable`，后续版本再实现

## 9. 后台导入行为建议

后台上传导入时，建议分成两步：

1. Validate
2. Import

Validate 输出：

- 解析后的 manifest 预览
- 题目总数
- 错误列表
- warning 列表
- 本次将新增/更新多少题

Import 输出：

- 导入记录 ID
- 实际新增题数
- 实际更新题数
- 跳过题数
- 失败题数

## 10. 与 seed 数据的关系

首次运行时内置测试题必须使用同一格式。

建议：

- 将 seed 题存为 `docs/examples` 或 `seed/` 下的 Markdown 文件
- 通过同一导入器导入
- 在导入记录中标记 `source_type = seed`

## 11. v1 限制

以下能力不纳入 v1：

- 题目图片
- 选项图片
- 数学公式专门语法
- 题目标签
- 多语言题目
- 题目难度分级
- 子题/材料题

这些能力如果以后要做，建议在 v2 扩展，而不是让 v1 语法先变得松散。

## 12. 规范化示例

### 12.1 单选题

```md
## Question
key: demo-001
type: single

What color is the sky on a clear day?

- [x] Blue
- [ ] Green
- [ ] Red
- [ ] Yellow

### Explanation
Under ordinary daytime conditions, the sky usually appears blue.
```

### 12.2 多选题

```md
## Question
key: demo-002
type: multiple

Which of the following are planets? Choose two.

- [x] Mars
- [x] Jupiter
- [ ] Moon
- [ ] Sun

### Explanation
Mars and Jupiter are planets. The Moon is a natural satellite and the Sun is a star.
```

## 13. 实现建议

代码上建议分成三层：

- `parser`: 负责文本解析
- `validator`: 负责规则校验
- `importer`: 负责数据库写入

解析结果建议先落成内存结构：

```text
ImportDocument
  Manifest
  Questions[]
```

不要在解析过程中直接写数据库。
