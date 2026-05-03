# Sims Exam Application

A web application for simulating exams.

## Getting Started

### Prerequisites
- Go 1.26+

### Common Commands
```bash
make test
make build
make version
make bootstrap
make run
```

默认运行地址为 `127.0.0.1:6080`，默认数据库为 `./simsexam_v1.db`。

也可以覆盖变量：

```bash
make run ADDR=127.0.0.1:6090 DB=./tmp/dev.db
make import IMPORT_FILE=./docs/examples/se-demo.md DB=./tmp/dev.db
```

### Importing Markdown

```bash
make validate IMPORT_FILE=./docs/examples/se-demo.md
make import IMPORT_FILE=./docs/examples/se-demo.md
```

## Versioning

`simsexam` embeds build metadata into the server binary.

```bash
make version
./bin/simsexam --version
```

Official releases are controlled by Git tags such as `vX.Y.Z`. Current release assets use the `simsexam-${VERSION}-${OS}-${ARCH}.tar.gz` naming pattern and the official build target is currently `linux-amd64`. See [versioning-and-releases.md](/Users/yu/repos/simsexam/docs/versioning-and-releases.md:1).

For the intended PR validation, staging, and release promotion flow, see [pr-testing-and-release-flow.md](/Users/yu/repos/simsexam/docs/pr-testing-and-release-flow.md:1).

For current deployment policy, including manual `Deploy Staging` usage and staging environment requirements, see [testing-and-deployment.md](/Users/yu/repos/simsexam/docs/testing-and-deployment.md:1).

## License
[License Info]
