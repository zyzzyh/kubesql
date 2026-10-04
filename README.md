# KubeSQL

KubeSQL 是一个使用 Go 编写的命令行工具，暂定命令名为 `ksql`。它把 Kubernetes 资源映射成可以用 SQL 查询和修改的对象：资源类型对应表，资源对象对应行，资源字段对应列。

本项目按照实习题目的要求逐阶段实现。项目目标是先完成清晰、可测试的 SQL 到 Kubernetes API 的完整链路，再逐步扩展资源类型和操作能力。

## 项目目标

KubeSQL 的基本执行链路如下：

```text
SQL 文本
  -> 词法分析（lexer）
  -> 语法分析（递归下降 parser）
  -> 抽象语法树（AST）
  -> 语义检查
  -> Kubernetes API 执行
  -> 结果投影和 JSON 输出
```

SQL 与 Kubernetes 概念的对应关系：

| SQL 概念 | KubeSQL 概念 |
| --- | --- |
| 表 | 一种 Kubernetes 资源，例如 `deployments` |
| 行 | 一个资源对象，例如某个 namespace 中的 `web` Deployment |
| 列 | 资源对象的字段，例如 `name`、`namespace`、`replicas` |
| `SELECT` | 从 API Server 读取对象并投影列 |
| `WHERE` | 在客户端或 API Server 侧筛选对象 |
| `INSERT` | 请求 API Server 创建对象 |
| `UPDATE` | 请求 API Server 修改指定字段 |
| `DELETE` | 请求 API Server 删除对象 |

本项目不要求实现关系型数据库的事务、ACID、一致性保证、并发控制或多表连接。每个操作直接通过 Kubernetes API 完成，API Server 负责其自身的校验和持久化。

## 技术约束

- 全程使用 Go。
- 词法分析器必须逐字符扫描输入。
- 语法分析器必须使用递归下降。
- 不使用正则表达式、按空格切分或关键词切片来实现 SQL 解析。
- Kubernetes 访问使用官方 `client-go`。
- 程序本身不通过执行 `kubectl` 完成功能。
- SQL 解析和 Kubernetes 访问保持解耦，解析阶段不请求集群。
- 错误需要包含稳定的错误码；解析错误需要报告行号和列号。
- 代码保持 `gofmt` 格式，包职责清楚，优先使用标准库和简单的接口。

## 功能阶段

### 阶段 1：基础解析器（easy）

先实现以下语法：

```ebnf
statement  = selectStmt, [";"], EOF ;
selectStmt = "SELECT", ("*" | columnList), "FROM", identifier ;
columnList = identifier, {",", identifier} ;
```

关键字匹配不区分大小写。`SELECT *` 在 AST 中保存为明确的星号节点，不在解析阶段展开列。

需要覆盖：

- `SELECT name, replicas FROM deployments;` 的 AST 生成。
- 缺少列名等语法错误的行列位置。
- 表名和列名的语法解析与语义检查分离。

### 阶段 2：基础 Kubernetes 查询（easy）

使用 `client-go` 读取以下资源：

| 表名 | API 资源 | 基础列 |
| --- | --- | --- |
| `namespaces` | `core/v1` Namespace | `name` |
| `deployments` | `apps/v1` Deployment | `name`、`namespace`、`replicas` |
| `ingresses` | `networking.k8s.io/v1` Ingress | `name`、`namespace`、`default_backend_service` |

字段规则：

- `name` 和 `namespace` 来自 `metadata`。
- `replicas` 来自 `spec.replicas`。
- `default_backend_service` 来自 `spec.defaultBackend.service.name`。
- 没有默认后端或默认后端不是 Service 时返回 JSON `null`。
- `SELECT *` 返回当前表公开的全部列。
- 表名使用题目规定的复数形式，不自动兼容 `deploy`、`ns` 等缩写。
- 未知表或未知列必须在请求集群前报错。

### 阶段 3：WHERE 过滤（easy）

支持：

- 比较：`=`、`<>`、`>`、`>=`、`<`、`<=`。
- 逻辑：`AND`、`OR`、`NOT`。
- 括号。
- 空值判断：`IS NULL`、`IS NOT NULL`。
- 字符串、整数、小数、`TRUE`、`FALSE`、`NULL` 和列引用。

运算优先级从高到低：

1. 比较和 `IS NULL`。
2. `NOT`。
3. `AND`。
4. `OR`。

过滤必须遵循 SQL 三值逻辑：`TRUE`、`FALSE`、`UNKNOWN`。`WHERE` 只保留结果为 `TRUE` 的行。数字按数字比较，不能把所有值先转换成字符串；不兼容的类型需要返回类型错误。

### 阶段 4：修改和删除（medium）

支持：

```sql
UPDATE deployments SET replicas = 3 WHERE name = 'web';
DELETE FROM deployments WHERE name = 'web';
```

规则：

- `UPDATE` 支持多个赋值。
- `UPDATE` 和 `DELETE` 必须包含 `WHERE`，否则返回 `E_WHERE_REQUIRED`。
- 先查询指定 namespace 中的候选对象，再复用阶段 3 的表达式求值逻辑。
- 只修改用户指定字段，不能用简化对象覆盖整个资源。
- 推荐使用带 `resourceVersion` 检查的 JSON Patch。
- `labels` 和 `annotations` 的整列赋值必须替换整个映射。
- `name`、`namespace`、`status` 和服务器管理的元数据不可修改。
- `replicas` 必须是非负整数。
- 多对象操作逐个执行；某个对象失败不能阻止其余对象继续处理。
- 所有成功时输出 `{"affected_rows":N}`；部分失败时输出失败数和错误对象，并返回非零退出码。
- `DELETE` 使用普通删除，不自动移除 finalizer。

### 阶段 5：创建（medium）

支持一次创建一个资源：

```sql
INSERT INTO deployments (manifest)
VALUES ('{"apiVersion":"apps/v1","kind":"Deployment",...}');
```

规则：

- `manifest` 必须是合法 JSON 对象。
- `apiVersion`、`kind` 和 `metadata.name` 必须与目标表匹配。
- namespaced 资源未指定 namespace 时使用 CLI 选择的 namespace。
- 明确指定的 namespace 必须与 CLI namespace 一致。
- Namespace 是集群级资源，manifest 不得带非空 namespace。
- 拒绝用户写入 `status`、`uid`、`resourceVersion`、`managedFields` 等服务器管理字段。
- 同名对象已存在时返回 `AlreadyExists`，不能隐式更新或删除重建。
- SQL 字符串由 lexer 处理单引号和两个单引号转义，JSON 内容交给 Go 的 `encoding/json`。

### 阶段 6：动态资源和内置资源（hard）

通过 API Discovery 获取资源的 group、version、resource、scope 和 verbs，并使用 `dynamic.Interface` 与 `unstructured.Unstructured` 支持集群实际提供的资源。

目标包括：

- Pod、Node、Service、ReplicaSet、StatefulSet、DaemonSet、Job、CronJob、ConfigMap、Secret、PV、PVC、StorageClass、ServiceAccount、Role、ClusterRole 及其 Binding。
- 通过双引号精确指定资源，例如 `"apps/v1/statefulsets"`。
- 通过 JSON Pointer 访问深层字段，例如 `"/spec/replicas"`。
- 支持 `CAST('...' AS JSON)` 进行明确 JSON 类型转换。
- 根据 Discovery 返回的 `scope` 判断 namespace 是否适用。
- 根据资源实际 `verbs` 判断 `SELECT`、`INSERT`、`UPDATE`、`DELETE` 是否支持。
- 资源不存在或 Discovery 失败时返回可区分的错误，而不是统一伪装成 SQL 错误。

### 阶段 7：Metrics 和 CRD（hard）

Metrics 阶段增加只读表 `pod_metrics` 和 `node_metrics`，通过 Metrics API 读取实际 CPU 和内存观测值。未安装 Metrics Server、权限不足或暂时没有样本时必须报告明确错误，不能伪造为零。

CRD 阶段要求：

- 通过 Discovery 发现新资源，不为每种 CRD 增加专用 Go 类型。
- 正确区分 CRD 定义和 CR 实例。
- 支持 namespaced 与 cluster-scoped CR。
- 创建 CRD 和创建 CR 分步执行，并等待 CRD 建立完成。
- 将 API Schema、准入、RBAC、不可变字段和版本冲突错误向上返回。

## 连接和输出

CLI 需要支持：

```text
--kubeconfig <path>
--context <name>
--namespace <name>
--all-namespaces
--output json
```

namespace 选择规则：

1. 显式传入 `--namespace` 时使用它。
2. 没有传入时使用当前 context 的 namespace。
3. 当前 context 没有配置 namespace 时使用 `default`。
4. `--all-namespaces` 只影响支持跨 namespace 查询的 `SELECT`。
5. 写操作必须在一个明确的 namespace 中执行。
6. 集群级资源不受 `--namespace` 过滤影响。

使用 `--output json` 时，查询结果始终为 JSON 数组；数字保持 JSON 数字，空值输出为 JSON `null`。测试按行集合比较，不依赖行顺序和 JSON 对象键顺序，除非具体测试另有说明。

## 推荐架构

目录按职责拆分，避免在单个文件堆积解析、业务和 API 逻辑：

```text
cmd/ksql/main.go       命令行参数、输入读取、退出码
internal/lexer/        逐字符词法分析
internal/parser/       递归下降语法分析
internal/ast/           AST 数据结构
internal/semantic/      表、列、类型和语义检查
internal/eval/          WHERE 表达式和三值逻辑求值
internal/kube/          client-go、dynamic client、资源适配
internal/engine/        查询和写操作的流程编排
internal/output/        JSON 输出和错误格式化
fixtures/               集成测试用 YAML 和 SQL
```

模块之间通过小型接口和明确的数据结构通信：

- lexer 不依赖 Kubernetes。
- parser 不访问集群。
- eval 不知道 API Client 的具体实现。
- kube 包负责 Kubernetes 类型转换和 API 错误处理。
- engine 组合各模块，但不重复实现解析或字段映射。
- CLI 只负责输入、配置和进程退出状态。

## 本地开发环境

推荐使用 Go、kubectl 和 minikube：

```powershell
git clone https://github.com/zyzzyh/kubesql.git
cd kubesql
go mod init github.com/zyzzyh/kubesql
go get k8s.io/api k8s.io/apimachinery k8s.io/client-go
minikube start
kubectl config current-context
```

开发过程中不要直接把题目目标 YAML 全部作为现成数据提交到集群；按章节说明准备 fixture，并使用 `ksql` 进行测试。

## 测试和质量检查

纯逻辑优先写单元测试：

- lexer：关键字大小写、字符串转义、数字、运算符和错误位置。
- parser：AST 结构、优先级、括号和语法错误。
- eval：类型比较、NULL 三值逻辑和 WHERE 保留规则。
- semantic：未知表、未知列、非法赋值和写操作约束。

需要真实 API Server 的行为使用 minikube 集成测试，并按题目提供的 YAML fixture 验证。

集成测试位于 `test/integration`，fixture 位于 `fixtures/integration-base.yaml`。普通 `go test ./...` 不会连接 Kubernetes；准备好集群后显式设置环境变量运行：

```powershell
$env:KUBESQL_INTEGRATION = "1"
go test ./test/integration -v
```

每个测试会创建唯一的 `sql-it-*` namespace，使用 `kubectl` 准备测试资源，再启动当前项目编译出的 `ksql` 二进制验证真实 API Server 行为；测试结束后会删除该 namespace。集成测试只使用这个测试专用 namespace，不读取或提交 kubeconfig、Token 等凭据。

提交前运行：

```powershell
gofmt -w .
go test ./...
go vet ./...
staticcheck ./...
go fix ./...
```

如果项目使用的 Go 版本支持 `modernize` 和 `deadcode`，也应运行对应检查，并确认每项改动确实必要。

## Git 提交约定

按功能少量多次提交。一个提交通常不超过 200 行，不为了凑数量拆分不可分割的改动。提交信息使用不带 emoji 的 Angular 风格：

```text
feat(lexer): tokenize select statements
feat(parser): parse select statements
feat(kube): list deployments
test(query): cover null filtering
fix(output): preserve JSON null values
docs(readme): describe project requirements
```

每次提交应保持项目处于可理解状态，并在提交说明或 PR 中写明验证命令和结果。不要提交 kubeconfig、访问令牌、集群凭据、编辑器临时文件或构建产物。

## 推荐开发顺序

1. 先完成 lexer 和 parser，再连接 Kubernetes。
2. 用固定输入测试 AST，不要用集群状态测试解析器。
3. 实现一个最小的 Deployment 查询闭环。
4. 扩展 namespaces 和 ingresses。
5. 加入 WHERE 和三值逻辑。
6. 再实现 UPDATE、DELETE 和 INSERT。
7. 最后考虑 dynamic client、Metrics 和 CRD。

每完成一个阶段，都运行对应的单元测试和质量检查，并用 README 中的示例记录实际行为。

## AI 辅助开发原则

可以使用 AI 辅助设计、查资料和生成初稿，但提交前必须理解并能解释所有代码。至少应掌握：

- token、词法分析、递归下降和 AST。
- Kubernetes API、namespace、resourceVersion、JSON Patch 和 Discovery。
- typed client、dynamic client 和 unstructured 对象的区别。
- SQL NULL 的三值逻辑和类型比较规则。

AI 生成的代码需要经过测试、人工阅读和静态检查，不能把无法解释的代码直接提交。
