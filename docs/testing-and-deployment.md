# simsexam 测试与部署约定

本文档定义 `SE` 当前阶段的测试、提交流程和部署方法。

## 1. 当前部署形态

当前约定：

- 服务运行在一台 Linux 服务器上
- 应用只监听回环地址：`127.0.0.1:6080`
- 外部流量通过反向代理接入，例如 Nginx 或 Caddy

当前不做的事情：

- 不直接暴露公网监听端口
- 不假设多实例部署
- 不假设 Kubernetes

备注：

- 如果以后业务规模增长，可以再评估容器化和 K8s
- 当前阶段优先保证可维护、可回滚、易排障

## 2. 分支与提交流程

强制约定：

- 所有提交和推送都不得直接进入 `main`
- 所有开发工作必须在分支上进行
- 只有通过测试并完成 review 的改动才允许发起 PR
- `main` 只接受 PR 合并

推荐分支命名：

- `codex/<topic>`
- `feature/<topic>`
- `fix/<topic>`

## 3. PR 合并前最低要求

每个 PR 在合并前至少必须满足：

- `make test` 通过
- `make build` 通过

建议逐步补充：

- `gofmt` 检查
- lint
- 更细的集成测试

## 4. GitHub Actions 的使用原则

当前结论：

- 使用 GitHub Actions 做 CI
- 暂不使用 GitHub Actions 做生产自动部署

原因：

- 当前项目还在快速重构阶段
- 先把测试和编译自动化，比先上自动部署更重要
- 部署先保持人工可控，更利于排查和回滚

当前阶段 GitHub Actions 负责：

- 拉取代码
- 配置 Go 环境
- 运行 `make test`
- 运行 `make build`

当前阶段 GitHub Actions 不负责：

- 自动发布到服务器
- 自动替换生产进程
- 自动执行生产数据库操作

## 5. 当前推荐部署流程

当前推荐使用：

- Linux
- `systemd`
- 反向代理
- SQLite 数据库文件

推荐部署步骤：

1. 在服务器上获取目标版本代码
2. 执行 `make build`
3. 执行 `make migrate` 或 `make bootstrap`
4. 如有需要，执行 `make import`
5. 重启 `systemd` 服务
6. 做最小冒烟验证

## 6. 部署前人工检查

即使 CI 通过，部署前仍建议人工确认：

- 首页可打开
- 能开始考试
- 能提交答案
- 后台可导入 Markdown
- 后台可编辑单题

## 7. 运行方式建议

应用进程建议由 `systemd` 管理。

建议：

- 二进制放在固定路径，例如 `/opt/simsexam/bin/simsexam`
- 工作目录固定，例如 `/opt/simsexam/current`
- 数据库文件放在固定目录，例如 `/var/lib/simsexam/simsexam_v1.db`
- 日志通过 `journalctl` 查看

## 8. 配置建议

虽然当前代码里还有部分固定值，部署层面建议尽快收敛到配置项：

- 监听地址
- 数据库路径
- 环境类型
- 日志级别

当前部署基线应保持：

- 默认监听 `127.0.0.1:6080`

当前代码中的运行配置入口：

- `SIMSEXAM_ADDR`
- `SIMSEXAM_DB_PATH`
- `SIMSEXAM_IMPORT_SOURCE_TYPE`

它们由 `internal/config` 统一加载，`server`、`migrate`、`bootstrapv1`、`importer` 共用这套默认值体系。

## 9. 未来演进原则

如果以后要升级部署体系，推荐顺序：

1. 先完善 CI
2. 再补发布包或版本化产物
3. 再评估自动部署
4. 最后才考虑容器编排或 K8s

不建议当前阶段就引入：

- 复杂 CD 流程
- Kubernetes 专用部署
- 多环境自动编排

## 10. 当前结论摘要

当前正式约定如下：

1. 生产服务运行在 Linux 单机。
2. 应用监听 `127.0.0.1:6080`。
3. 所有改动必须走分支和 PR。
4. 禁止直接向 `main` 提交。
5. PR 合并前必须通过 `make test` 和 `make build`。
6. GitHub Actions 当前只做 CI，不做生产自动部署。
7. 生产部署当前采用人工、可审计、可回滚的单机流程。
