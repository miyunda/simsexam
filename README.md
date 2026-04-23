# Sims Exam Application

A web application for simulating exams.

## Getting Started

### Prerequisites
- Go 1.26+

### Common Commands
```bash
make test
make build
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

## License
[License Info]
