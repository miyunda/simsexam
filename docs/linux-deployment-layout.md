# simsexam Linux 部署规范

本文档定义 `simsexam` 在单机 Linux 服务器上的标准目录布局、配置位置、日志方式和部署步骤。

当前目标：

- 单机部署
- `systemd` 托管
- 应用仅监听 `127.0.0.1:6080`
- SQLite 作为本地持久化数据库
- 反向代理负责公网入口

适用前提：

- 你从 GitHub Release 下载的是 `simsexam-<version>-linux-amd64.tar.gz`
- 该压缩包内包含：
  - `simsexam`
  - `simsexam-migrate`
  - `simsexam-bootstrapv1`

## 1. 标准目录布局

建议统一采用以下目录：

```text
/opt/simsexam/
  bin/
    simsexam -> /opt/simsexam/releases/v0.1.0/simsexam
    simsexam-migrate -> /opt/simsexam/releases/v0.1.0/simsexam-migrate
    simsexam-bootstrapv1 -> /opt/simsexam/releases/v0.1.0/simsexam-bootstrapv1
  releases/
    v0.1.0/
      simsexam
      simsexam-migrate
      simsexam-bootstrapv1

/etc/simsexam/
  simsexam.env

/var/lib/simsexam/
  simsexam_v1.db

/var/log/simsexam/
  README
```

职责约定：

- `/opt/simsexam/releases/<version>/`
  - 存放每个版本对应的只读二进制文件
- `/opt/simsexam/bin/simsexam`
  - 指向当前启用版本的稳定软链接
- `/opt/simsexam/bin/simsexam-migrate`
  - 指向当前启用版本的数据库 migration 工具
- `/opt/simsexam/bin/simsexam-bootstrapv1`
  - 指向当前启用版本的数据库初始化与 seed 导入工具
- `/etc/simsexam/simsexam.env`
  - 存放运行时环境变量
- `/var/lib/simsexam/simsexam_v1.db`
  - 存放 SQLite 数据文件
- `/var/log/simsexam/`
  - 预留日志目录；当前主日志仍以 `journald` 为准

## 2. 为什么这样布局

这样做有几个直接好处：

- 升级时只需要替换版本目录或切换软链接
- 程序和数据分离，避免升级误伤数据库
- 配置独立于程序目录，便于审计和备份
- 回滚时可以快速切回旧版本二进制

## 3. 标准环境变量文件

建议在 `/etc/simsexam/simsexam.env` 中写入：

```bash
SIMSEXAM_ADDR=127.0.0.1:6080
SIMSEXAM_DB_PATH=/var/lib/simsexam/simsexam_v1.db
SIMSEXAM_IMPORT_SOURCE_TYPE=manual
```

说明：

- `SIMSEXAM_ADDR`
  - 固定监听回环地址，不直接暴露公网端口
- `SIMSEXAM_DB_PATH`
  - 指向标准数据库位置
- `SIMSEXAM_IMPORT_SOURCE_TYPE`
  - 当前保留为可配置项

## 4. systemd 单元文件位置

建议系统服务文件路径为：

```text
/etc/systemd/system/simsexam.service
```

标准模板见：

- [deploy/systemd/simsexam.service](/Users/yu/repos/simsexam/deploy/systemd/simsexam.service:1)

## 5. 标准首次部署步骤

### 5.1 创建目录和用户

```bash
sudo useradd --system --home /opt/simsexam --shell /usr/sbin/nologin simsexam || true

sudo mkdir -p /opt/simsexam/bin
sudo mkdir -p /opt/simsexam/releases/v0.1.0
sudo mkdir -p /etc/simsexam
sudo mkdir -p /var/lib/simsexam
sudo mkdir -p /var/log/simsexam

sudo chown -R simsexam:simsexam /opt/simsexam /var/lib/simsexam /var/log/simsexam
sudo chown root:root /etc/simsexam
```

### 5.2 安装 release 包

将 release 页面下载的压缩包上传到服务器后：

```bash
tar -xzf simsexam-v0.1.0-linux-amd64.tar.gz
sudo mv simsexam /opt/simsexam/releases/v0.1.0/simsexam
sudo mv simsexam-migrate /opt/simsexam/releases/v0.1.0/simsexam-migrate
sudo mv simsexam-bootstrapv1 /opt/simsexam/releases/v0.1.0/simsexam-bootstrapv1
sudo chmod 0755 /opt/simsexam/releases/v0.1.0/simsexam
sudo chmod 0755 /opt/simsexam/releases/v0.1.0/simsexam-migrate
sudo chmod 0755 /opt/simsexam/releases/v0.1.0/simsexam-bootstrapv1
sudo ln -sfn /opt/simsexam/releases/v0.1.0/simsexam /opt/simsexam/bin/simsexam
sudo ln -sfn /opt/simsexam/releases/v0.1.0/simsexam-migrate /opt/simsexam/bin/simsexam-migrate
sudo ln -sfn /opt/simsexam/releases/v0.1.0/simsexam-bootstrapv1 /opt/simsexam/bin/simsexam-bootstrapv1
```

### 5.3 写入环境变量文件

```bash
sudo tee /etc/simsexam/simsexam.env >/dev/null <<'EOF'
SIMSEXAM_ADDR=127.0.0.1:6080
SIMSEXAM_DB_PATH=/var/lib/simsexam/simsexam_v1.db
SIMSEXAM_IMPORT_SOURCE_TYPE=manual
EOF
```

### 5.4 安装 systemd 服务

将仓库中的模板复制到系统目录：

```bash
sudo cp deploy/systemd/simsexam.service /etc/systemd/system/simsexam.service
sudo systemctl daemon-reload
```

### 5.5 初始化数据库

首次部署建议显式执行一次初始化，而不是直接盲目启动服务：

```bash
sudo -u simsexam /opt/simsexam/bin/simsexam-migrate -dsn /var/lib/simsexam/simsexam_v1.db
sudo -u simsexam /opt/simsexam/bin/simsexam-bootstrapv1 -dsn /var/lib/simsexam/simsexam_v1.db
```

如果你只想做最小首装，一般直接执行 `simsexam-bootstrapv1` 也可以，因为它本身会先准备 v1 数据库。

### 5.6 启动服务

```bash
sudo systemctl enable --now simsexam
sudo systemctl status simsexam
curl http://127.0.0.1:6080/
```

## 6. 首次启动时数据库放在哪里

数据库文件由 `SIMSEXAM_DB_PATH` 控制。

按本规范部署后，数据库文件固定在：

```text
/var/lib/simsexam/simsexam_v1.db
```

如果该文件不存在，推荐先用 release 包内自带的 `simsexam-migrate` 和 `simsexam-bootstrapv1` 显式完成初始化，再启动服务。

## 7. 日志规范

当前推荐把 `journald` 作为主日志入口。

查看方法：

```bash
sudo systemctl status simsexam
sudo journalctl -u simsexam -n 100 --no-pager
sudo journalctl -u simsexam -f
```

`/var/log/simsexam/` 目前主要是预留目录，不作为首选日志入口。

## 8. 升级步骤

以升级到 `v0.1.1` 为例：

```bash
sudo mkdir -p /opt/simsexam/releases/v0.1.1
tar -xzf simsexam-v0.1.1-linux-amd64.tar.gz
sudo mv simsexam /opt/simsexam/releases/v0.1.1/simsexam
sudo mv simsexam-migrate /opt/simsexam/releases/v0.1.1/simsexam-migrate
sudo mv simsexam-bootstrapv1 /opt/simsexam/releases/v0.1.1/simsexam-bootstrapv1
sudo chmod 0755 /opt/simsexam/releases/v0.1.1/simsexam
sudo chmod 0755 /opt/simsexam/releases/v0.1.1/simsexam-migrate
sudo chmod 0755 /opt/simsexam/releases/v0.1.1/simsexam-bootstrapv1
sudo ln -sfn /opt/simsexam/releases/v0.1.1/simsexam /opt/simsexam/bin/simsexam
sudo ln -sfn /opt/simsexam/releases/v0.1.1/simsexam-migrate /opt/simsexam/bin/simsexam-migrate
sudo ln -sfn /opt/simsexam/releases/v0.1.1/simsexam-bootstrapv1 /opt/simsexam/bin/simsexam-bootstrapv1
sudo -u simsexam /opt/simsexam/bin/simsexam-migrate -dsn /var/lib/simsexam/simsexam_v1.db
sudo systemctl restart simsexam
```

升级后建议立即检查：

```bash
sudo systemctl status simsexam
curl http://127.0.0.1:6080/
```

## 9. 回滚步骤

如果新版本异常，可以把软链接切回旧版本：

```bash
sudo ln -sfn /opt/simsexam/releases/v0.1.0/simsexam /opt/simsexam/bin/simsexam
sudo systemctl restart simsexam
```

## 10. 反向代理边界

`simsexam` 自身只监听：

```text
127.0.0.1:6080
```

公网访问应通过 Nginx 或 Caddy 转发到该地址。

这意味着：

- `simsexam` 不直接暴露在公网
- TLS 终止放在反向代理层
- 未来如果改为多实例部署，再重新评估入口层设计
