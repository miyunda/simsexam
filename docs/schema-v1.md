# simsexam 数据库 Schema v1

本文档定义 `SE` 的第一版数据库设计。目标是覆盖以下能力：

- 科目与题库管理
- Markdown 导入与 seed 导入
- 考试与答题持久化
- 登录、角色、错题本、授权预留

当前建议：

- 开发期使用 SQLite
- schema 设计尽量贴近 PostgreSQL 风格
- 所有表使用 migration 管理

## 1. 设计原则

v1 schema 需要满足：

- 考试会话不依赖内存
- 题库导入有可追踪记录
- 题目支持稳定 key 和后台编辑
- 用户、登录、收费能自然扩展
- 未来 iOS App 可以直接复用同一套核心数据

本版不追求：

- 极度规范化到每个字段都拆表
- 一开始就支持所有支付平台
- 一开始就支持题库历史差异对比的完整 UI

## 2. 关键实体

核心实体如下：

- `users`
- `user_identities`
- `subjects`
- `question_sets`
- `questions`
- `question_options`
- `import_jobs`
- `question_revisions`
- `exams`
- `exam_questions`
- `exam_answers`
- `exam_answer_options`
- `user_question_stats`
- `subject_entitlements`

其中有一个关键建模决定：

- `questions` 表示“某个题库版本中的题目快照”
- 同一个逻辑题目在不同 `question_set` 中可以出现多次

这意味着：

- 历史考试永远绑定历史版本的题面
- 新导入不会覆盖旧版本题目行
- 用户错题统计不能只依赖某一行 `question_id`，而应基于稳定题目标识聚合

## 3. 枚举约定

SQLite 不原生支持 enum，v1 用 `TEXT CHECK (...)` 表达。

建议枚举：

`user_role`

- `user`
- `admin`

`user_status`

- `active`
- `disabled`

`subject_access_level`

- `free`
- `paid`
- `private`

`subject_status`

- `draft`
- `published`
- `archived`

`question_type`

- `single`
- `multiple`

`question_status`

- `active`
- `disabled`

`question_set_source_type`

- `seed`
- `markdown_import`
- `manual`

`import_job_status`

- `pending`
- `validated`
- `imported`
- `failed`

`exam_mode`

- `practice`
- `formal`

`exam_status`

- `in_progress`
- `submitted`
- `expired`

`mastery_status`

- `new`
- `weak`
- `mastered`

`entitlement_source`

- `free`
- `purchase`
- `gift`
- `admin`

## 4. 表设计

### 4.1 users

站内主用户表。

字段：

- `id` INTEGER PRIMARY KEY
- `email` TEXT NOT NULL
- `display_name` TEXT
- `avatar_url` TEXT
- `role` TEXT NOT NULL DEFAULT `user`
- `status` TEXT NOT NULL DEFAULT `active`
- `created_at` DATETIME NOT NULL
- `updated_at` DATETIME NOT NULL

约束：

- `email` UNIQUE
- `role` CHECK in (`user`, `admin`)
- `status` CHECK in (`active`, `disabled`)

说明：

- v1 保留 email 唯一，后续如果要支持更复杂账号合并，再单独设计流程

### 4.2 user_identities

第三方身份映射。

字段：

- `id` INTEGER PRIMARY KEY
- `user_id` INTEGER NOT NULL
- `provider` TEXT NOT NULL
- `provider_user_id` TEXT NOT NULL
- `provider_email` TEXT
- `created_at` DATETIME NOT NULL

约束：

- FOREIGN KEY (`user_id`) REFERENCES `users`(`id`)
- UNIQUE (`provider`, `provider_user_id`)

说明：

- `provider` 初期可取 `google`、`apple`、`email_link`

### 4.3 subjects

科目配置表，对应导入规范中的 manifest。

字段：

- `id` INTEGER PRIMARY KEY
- `slug` TEXT NOT NULL
- `title` TEXT NOT NULL
- `description` TEXT
- `duration_minutes` INTEGER NOT NULL
- `question_count` INTEGER NOT NULL
- `access_level` TEXT NOT NULL
- `status` TEXT NOT NULL
- `current_question_set_id` INTEGER
- `created_at` DATETIME NOT NULL
- `updated_at` DATETIME NOT NULL

约束：

- UNIQUE (`slug`)
- `duration_minutes > 0`
- `question_count > 0`
- `access_level` CHECK in (`free`, `paid`, `private`)
- `status` CHECK in (`draft`, `published`, `archived`)

说明：

- `current_question_set_id` 指向当前用于考试的题库版本
- 不建议把所有题都直接理解成“subject 当前唯一状态”，需要保留题库版本概念

### 4.4 question_sets

题库版本表。一个科目可有多个题库版本。

字段：

- `id` INTEGER PRIMARY KEY
- `subject_id` INTEGER NOT NULL
- `version` TEXT
- `source_type` TEXT NOT NULL
- `source_name` TEXT
- `source_checksum` TEXT
- `is_active` INTEGER NOT NULL DEFAULT 0
- `created_by_user_id` INTEGER
- `created_at` DATETIME NOT NULL

约束：

- FOREIGN KEY (`subject_id`) REFERENCES `subjects`(`id`)
- FOREIGN KEY (`created_by_user_id`) REFERENCES `users`(`id`)
- `source_type` CHECK in (`seed`, `markdown_import`, `manual`)

索引建议：

- INDEX (`subject_id`, `created_at`)
- INDEX (`subject_id`, `is_active`)

说明：

- 每次成功导入都可以生成一个新的 `question_set`
- `is_active=1` 的版本通常应与 `subjects.current_question_set_id` 一致

### 4.5 questions

题目快照表。

字段：

- `id` INTEGER PRIMARY KEY
- `subject_id` INTEGER NOT NULL
- `question_set_id` INTEGER NOT NULL
- `external_key` TEXT NOT NULL
- `type` TEXT NOT NULL
- `stem_markdown` TEXT NOT NULL
- `explanation_markdown` TEXT
- `status` TEXT NOT NULL DEFAULT `active`
- `created_at` DATETIME NOT NULL
- `updated_at` DATETIME NOT NULL

约束：

- FOREIGN KEY (`subject_id`) REFERENCES `subjects`(`id`)
- FOREIGN KEY (`question_set_id`) REFERENCES `question_sets`(`id`)
- UNIQUE (`question_set_id`, `external_key`)
- `type` CHECK in (`single`, `multiple`)
- `status` CHECK in (`active`, `disabled`)

说明：

- `external_key` 对应导入规范中的 `key`
- `external_key` 在业务上应“按科目稳定”，但数据库唯一性只约束在同一个 `question_set` 内
- 这允许同一逻辑题目在不同版本中保留历史快照
- 导入新版本时，不应覆盖旧版本题目，而是创建属于新 `question_set` 的新快照行

### 4.6 question_options

题目选项表。

字段：

- `id` INTEGER PRIMARY KEY
- `question_id` INTEGER NOT NULL
- `option_key` TEXT NOT NULL
- `content_markdown` TEXT NOT NULL
- `sort_order` INTEGER NOT NULL
- `is_correct` INTEGER NOT NULL DEFAULT 0

约束：

- FOREIGN KEY (`question_id`) REFERENCES `questions`(`id`)
- UNIQUE (`question_id`, `option_key`)
- UNIQUE (`question_id`, `sort_order`)
- `sort_order > 0`

说明：

- `option_key` 可以存 `A`、`B`、`C`、`D`
- `sort_order` 决定展示顺序

### 4.7 import_jobs

导入记录表。

字段：

- `id` INTEGER PRIMARY KEY
- `subject_id` INTEGER
- `question_set_id` INTEGER
- `source_type` TEXT NOT NULL
- `source_filename` TEXT NOT NULL
- `source_checksum` TEXT
- `status` TEXT NOT NULL
- `manifest_json` TEXT
- `error_report` TEXT
- `warning_report` TEXT
- `created_by_user_id` INTEGER
- `created_at` DATETIME NOT NULL

约束：

- FOREIGN KEY (`subject_id`) REFERENCES `subjects`(`id`)
- FOREIGN KEY (`question_set_id`) REFERENCES `question_sets`(`id`)
- FOREIGN KEY (`created_by_user_id`) REFERENCES `users`(`id`)
- `status` CHECK in (`pending`, `validated`, `imported`, `failed`)

说明：

- `manifest_json` 保存当次解析出的 manifest 快照
- `error_report` / `warning_report` 初期可用 JSON 字符串

### 4.8 question_revisions

后台改单题留痕。

字段：

- `id` INTEGER PRIMARY KEY
- `question_id` INTEGER NOT NULL
- `editor_user_id` INTEGER
- `change_summary` TEXT
- `snapshot_json` TEXT NOT NULL
- `created_at` DATETIME NOT NULL

约束：

- FOREIGN KEY (`question_id`) REFERENCES `questions`(`id`)
- FOREIGN KEY (`editor_user_id`) REFERENCES `users`(`id`)

说明：

- `snapshot_json` 记录改单前或改单后的完整快照，具体策略可在实现时统一

### 4.9 exams

一次考试实例。

字段：

- `id` INTEGER PRIMARY KEY
- `user_id` INTEGER
- `subject_id` INTEGER NOT NULL
- `question_set_id` INTEGER NOT NULL
- `mode` TEXT NOT NULL
- `status` TEXT NOT NULL
- `started_at` DATETIME NOT NULL
- `submitted_at` DATETIME
- `expires_at` DATETIME
- `score` INTEGER

约束：

- FOREIGN KEY (`user_id`) REFERENCES `users`(`id`)
- FOREIGN KEY (`subject_id`) REFERENCES `subjects`(`id`)
- FOREIGN KEY (`question_set_id`) REFERENCES `question_sets`(`id`)
- `mode` CHECK in (`practice`, `formal`)
- `status` CHECK in (`in_progress`, `submitted`, `expired`)
- `score IS NULL OR (score >= 0 AND score <= 100)`

说明：

- 匿名用户允许 `user_id` 为空
- 一次考试必须绑定启动时的 `question_set_id`，避免题库更新后结果失真

### 4.10 exam_questions

一次考试中实际抽到的题及其顺序。

字段：

- `id` INTEGER PRIMARY KEY
- `exam_id` INTEGER NOT NULL
- `question_id` INTEGER NOT NULL
- `position` INTEGER NOT NULL

约束：

- FOREIGN KEY (`exam_id`) REFERENCES `exams`(`id`)
- FOREIGN KEY (`question_id`) REFERENCES `questions`(`id`)
- UNIQUE (`exam_id`, `position`)
- UNIQUE (`exam_id`, `question_id`)
- `position > 0`

说明：

- 这是替代当前内存 `activeSessions.QuestionIDs` 的关键表

### 4.11 exam_answers

每道题在一次考试中的作答记录。

字段：

- `id` INTEGER PRIMARY KEY
- `exam_id` INTEGER NOT NULL
- `question_id` INTEGER NOT NULL
- `answered_at` DATETIME NOT NULL
- `is_correct` INTEGER NOT NULL DEFAULT 0

约束：

- FOREIGN KEY (`exam_id`) REFERENCES `exams`(`id`)
- FOREIGN KEY (`question_id`) REFERENCES `questions`(`id`)
- UNIQUE (`exam_id`, `question_id`)

说明：

- 每题只保留最终答案
- 如果后续要保留“改答案轨迹”，可再加 answer_events 表

### 4.12 exam_answer_options

存储用户在某题选择了哪些选项。

字段：

- `id` INTEGER PRIMARY KEY
- `exam_answer_id` INTEGER NOT NULL
- `option_id` INTEGER NOT NULL

约束：

- FOREIGN KEY (`exam_answer_id`) REFERENCES `exam_answers`(`id`)
- FOREIGN KEY (`option_id`) REFERENCES `question_options`(`id`)
- UNIQUE (`exam_answer_id`, `option_id`)

说明：

- 单选和多选统一存储

### 4.13 user_question_stats

错题本和学习追踪的聚合表。

字段：

- `id` INTEGER PRIMARY KEY
- `user_id` INTEGER NOT NULL
- `subject_id` INTEGER NOT NULL
- `question_key` TEXT NOT NULL
- `wrong_count` INTEGER NOT NULL DEFAULT 0
- `correct_count` INTEGER NOT NULL DEFAULT 0
- `last_answered_at` DATETIME
- `last_wrong_at` DATETIME
- `mastery_status` TEXT NOT NULL DEFAULT `new`

约束：

- FOREIGN KEY (`user_id`) REFERENCES `users`(`id`)
- FOREIGN KEY (`subject_id`) REFERENCES `subjects`(`id`)
- UNIQUE (`user_id`, `subject_id`, `question_key`)
- `wrong_count >= 0`
- `correct_count >= 0`
- `mastery_status` CHECK in (`new`, `weak`, `mastered`)

说明：

- 这是读优化表，不是事实来源
- 事实来源仍然是 `exams / exam_answers / exam_answer_options`
- 聚合键使用 `(user_id, subject_id, question_key)`，这样题目跨版本更新后，用户学习轨迹仍能连续

### 4.14 subject_entitlements

科目访问授权。

字段：

- `id` INTEGER PRIMARY KEY
- `user_id` INTEGER NOT NULL
- `subject_id` INTEGER NOT NULL
- `source` TEXT NOT NULL
- `starts_at` DATETIME
- `ends_at` DATETIME
- `created_at` DATETIME NOT NULL

约束：

- FOREIGN KEY (`user_id`) REFERENCES `users`(`id`)
- FOREIGN KEY (`subject_id`) REFERENCES `subjects`(`id`)
- `source` CHECK in (`free`, `purchase`, `gift`, `admin`)

索引建议：

- INDEX (`user_id`, `subject_id`)

说明：

- 免费科目也可以通过逻辑直接放行，不一定要求每个免费科目都生成一行 entitlement
- 但该表为后续收费和赠送能力预留了统一模型

## 5. 关系摘要

主要关系：

- 一个 `subject` 有多个 `question_set`
- 一个 `question_set` 包含多道 `question`
- 一道 `question` 有多个 `question_option`
- 一个 `user` 有多个 `user_identity`
- 一个 `user` 有多次 `exam`
- 一次 `exam` 有多条 `exam_question`
- 一次 `exam` 有多条 `exam_answer`
- 一条 `exam_answer` 有多条 `exam_answer_option`

## 6. 与导入规范的映射

`docs/markdown-import-spec.md` 中字段与数据库的对应关系：

- `Meta.slug` -> `subjects.slug`
- `Meta.title` -> `subjects.title`
- `Meta.description` -> `subjects.description`
- `Meta.duration_minutes` -> `subjects.duration_minutes`
- `Meta.question_count` -> `subjects.question_count`
- `Meta.access_level` -> `subjects.access_level`
- `Meta.status` -> `subjects.status`
- `Meta.version` -> `question_sets.version`
- `Question.key` -> `questions.external_key`
- `Question.type` -> `questions.type`
- `Question stem` -> `questions.stem_markdown`
- `Explanation` -> `questions.explanation_markdown`
- `Option text` -> `question_options.content_markdown`
- `Option checked` -> `question_options.is_correct`

## 7. 删除与停用策略

v1 建议默认不做硬删除。

规则：

- 科目下线：更新 `subjects.status`
- 题目失效：更新 `questions.status = disabled`
- 导入文件缺失旧题：默认不删除，后续可通过同步模式停用

原因：

- 保护历史考试数据
- 降低导入失误造成的数据损失风险

对于新导入版本，建议流程是：

1. 创建新的 `question_set`
2. 为该版本写入一批新的 `questions` 和 `question_options`
3. 校验成功后切换 `subjects.current_question_set_id`
4. 旧版本继续保留，供历史考试引用

## 8. 索引建议

除主键和唯一约束外，建议补这些索引：

- `subjects(status)`
- `questions(subject_id, status)`
- `questions(question_set_id)`
- `question_options(question_id, sort_order)`
- `import_jobs(subject_id, created_at)`
- `exams(user_id, started_at)`
- `exams(subject_id, started_at)`
- `exam_answers(exam_id, is_correct)`
- `user_question_stats(user_id, mastery_status)`

## 9. 迁移顺序建议

Phase 1 migration 可以按这个顺序：

1. `users`
2. `user_identities`
3. `subjects`
4. `question_sets`
5. `questions`
6. `question_options`
7. `import_jobs`
8. `question_revisions`
9. `exams`
10. `exam_questions`
11. `exam_answers`
12. `exam_answer_options`
13. `user_question_stats`
14. `subject_entitlements`

## 10. 首批必须实现的最小子集

如果想尽快从原型切到可演进结构，最先必须落地的是：

- `subjects`
- `question_sets`
- `questions`
- `question_options`
- `import_jobs`
- `exams`
- `exam_questions`
- `exam_answers`
- `exam_answer_options`

这套已经足够支撑：

- 导入题库
- seed 题导入
- 题目抽取
- 考试持久化
- 成绩页和错题回顾

## 11. 暂缓实现的能力

以下表可以在 schema 中先定义，但实现时可以晚一点接入：

- `user_identities`
- `question_revisions`
- `user_question_stats`
- `subject_entitlements`

原因不是它们不重要，而是当前关键路径首先是：

- 把题库结构做稳
- 把考试从内存移到数据库
- 把管理员导入和修改路径打通

## 12. 实现提醒

有几个实现层面的决定需要提前注意：

1. `questions.external_key` 必须稳定，否则导入幂等会失效。
2. `exam_questions` 是必需表，不能继续只把抽题结果放内存。
3. 计算分数时要基于 `question_options.is_correct` 与 `exam_answer_options` 做集合比较。
4. 如果题目被后台修改，历史考试仍应保留当时引用的 `question_set_id`，避免成绩回放漂移。
5. 由于 `questions` 已按 `question_set` 存快照，v1 已能回放历史题面，不必急着把题面 JSON 再复制进考试表。
