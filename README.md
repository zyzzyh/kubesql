# KubeSQL

KubeSQL 是一个用 Go 编写的命令行工具，命令名为 ksql。它将 Kubernetes 资源映射成 SQL 表：资源类型是表，一个资源对象是一行，对象字段是列。

当前实现支持 SELECT、UPDATE、DELETE、INSERT；固定资源 namespaces、deployments、ingresses；Metrics API 的 pod_metrics、node_metrics 只读表；SQL WHERE 和三值逻辑；API Discovery、dynamic client、unstructured 对象和 JSON Pointer；CRD 注册等待；JSON 输出、结构化错误及 kubeconfig/context/namespace 配置。

Metrics 表读取实际观测值，不把 Pod requests/limits 当成使用量。Metrics API 不可用时返回 E_METRICS_UNAVAILABLE，未采到样本的 Pod 不会伪造为零。动态资源根据 Discovery 返回的 GVR、scope 和 verbs 执行操作；创建 CRD 后会等待 Established 条件和 served version 出现在 Discovery 中。

## 1. 项目定位

一条 SQL 的执行链路：

~~~text
SQL
 -> Lexer 逐字符扫描
 -> Parser 递归下降生成 AST
 -> catalog 做语义检查和投影解析
 -> query 或 write 执行 AST
 -> client-go typed/dynamic/Discovery/Metrics client 访问 Kubernetes
 -> WHERE 过滤和列投影
 -> JSON 输出
~~~

Parser 不访问 Kubernetes。语法分析、语义检查、表达式求值和资源访问各有独立职责。

| SQL 概念 | KubeSQL 概念 |
| --- | --- |
| 表 | 一种 Kubernetes 资源，例如 deployments |
| 行 | 一个 Kubernetes 对象 |
| 列 | 对象字段，例如 name、namespace、replicas |
| SELECT | 列举对象并选择字段 |
| WHERE | 按字段值筛选对象 |
| INSERT | 调用 API Server 创建对象 |
| UPDATE | 对匹配对象发送 JSON Patch |
| DELETE | 删除匹配对象 |
| pod_metrics | Metrics API 中每个 Pod 的实际使用量 |
| node_metrics | Metrics API 中每个 Node 的实际使用量 |

## 2. 当前功能

固定资源使用 Kubernetes typed client：

| 表 | API 资源 | 当前公开列 |
| --- | --- | --- |
| namespaces | core/v1 Namespace | name、labels、annotations、manifest |
| deployments | apps/v1 Deployment | name、namespace、replicas、labels、annotations、manifest |
| ingresses | networking.k8s.io/v1 Ingress | name、namespace、default_backend_service、labels、annotations、manifest |

支持 SELECT、UPDATE、DELETE、INSERT。UPDATE 和 DELETE 必须有 WHERE。Ingress 没有默认 Service 后端时，default_backend_service 为 JSON null。

WHERE 支持 =、<>、>、>=、<、<=、AND、OR、NOT、括号、IS NULL、IS NOT NULL、字符串、整数、小数、布尔值、NULL 和列引用。优先级由高至低是比较、NOT、AND、OR。数字按数值比较；SQL NULL 依照 TRUE/FALSE/UNKNOWN 三值逻辑处理，WHERE 只保留 TRUE。

动态资源通过 API Discovery 解析。普通表名由 Discovery 选择 API group 和 preferred version；双引号标识符可以精确指定资源，例如：

~~~sql
SELECT name, "/spec/replicas" AS replicas
FROM "apps/v1/statefulsets";
~~~

双引号列名是 JSON Pointer。AS 只重命名输出列。动态操作通过 dynamic client 和 unstructured.Unstructured 完成；SELECT/INSERT/UPDATE/DELETE 会检查资源 scope 和 Discovery 返回的 verbs。namespaced 资源使用 CLI namespace，cluster-scoped 资源忽略它。UPDATE 只接受双引号 JSON Pointer，并拒绝修改服务器管理字段。

## 3. 运行环境和启动

需要 Go 和一个由 kubeconfig 描述的 Kubernetes API Server。kubectl 用于准备和检查测试资源；minikube 可用于本地集成测试。

~~~powershell
go mod download
go build -o bin/ksql ./cmd/ksql
~~~

运行程序，从标准输入读取一条 SQL：

~~~powershell
@'
SELECT name, namespace, replicas FROM deployments;
'@ | go run ./cmd/ksql --context minikube --namespace default --output json
~~~

每次调用解析一条 SQL，可带一个结尾分号。

## 4. 命令行参数

| 参数 | 作用 |
| --- | --- |
| --kubeconfig <path> | 指定 kubeconfig 文件 |
| --context <name> | 选择 kubeconfig context |
| --namespace <name> | 指定 namespaced 操作的 namespace |
| --all-namespaces | SELECT 查询所有 namespace 中的 namespaced 资源 |
| --output json | JSON 输出，目前唯一支持的格式 |

namespace 顺序：显式 --namespace、当前 context namespace、default。集群级资源不受 namespace 过滤。namespaced 写操作需要有效 namespace。

## 5. 查询示例

查询 Deployment：

~~~powershell
@'
SELECT name, namespace, replicas
FROM deployments;
'@ | go run ./cmd/ksql --namespace demo --output json
~~~

结果始终是 JSON 数组，数字保持 JSON 数字：

~~~json
[{"name":"web","namespace":"demo","replicas":2}]
~~~

WHERE 和运算优先级：

~~~powershell
@'
SELECT name
FROM deployments
WHERE name = 'web' OR name = 'worker' AND replicas >= 3;
'@ | go run ./cmd/ksql --namespace demo --output json
~~~

AND 优先于 OR。括号可以改变结合顺序。空值必须使用 IS NULL：

~~~sql
SELECT name FROM ingresses WHERE default_backend_service IS NULL;
~~~

name = NULL 的比较结果为 UNKNOWN，不会保留在 WHERE 结果中。

跨 namespace 查询：

~~~powershell
@'
SELECT name, namespace FROM deployments;
'@ | go run ./cmd/ksql --all-namespaces --output json
~~~

动态资源和 JSON Pointer：

~~~powershell
@'
SELECT name, "/spec/replicas" AS replicas
FROM "apps/v1/statefulsets"
WHERE "/spec/replicas" >= 1;
'@ | go run ./cmd/ksql --namespace demo --output json
~~~

Metrics 查询：

~~~powershell
@'
SELECT name, cpu_millicores, memory_bytes
FROM pod_metrics
WHERE name = 'measure';
'@ | go run ./cmd/ksql --namespace demo --output json
~~~

`cpu_millicores` 和 `memory_bytes` 分别是整数 millicores 和 bytes。Pod 的 CPU、内存值是所有容器样本的累加。

## 6. 写操作示例

UPDATE：

~~~powershell
@'
UPDATE deployments SET replicas = 2 WHERE name = 'web';
'@ | go run ./cmd/ksql --namespace demo --output json
~~~

成功时输出：

~~~json
{"affected_rows":1}
~~~

labels 和 annotations 接受代表整个映射的 JSON 字符串，例如：

~~~sql
UPDATE deployments
SET annotations = '{"owner":"alice"}'
WHERE name = 'web';
~~~

UPDATE 用 JSON Patch 只改指定路径，并先 test resourceVersion，避免基于旧对象覆盖新值。

DELETE：

~~~powershell
@'
DELETE FROM deployments WHERE name = 'worker';
'@ | go run ./cmd/ksql --namespace demo --output json
~~~

使用普通 Kubernetes Delete，不自动移除 finalizer。

INSERT 要指定 manifest 列，值是完整 Kubernetes JSON：

~~~powershell
@'
INSERT INTO namespaces (manifest)
VALUES ('{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"demo-new"}}');
'@ | go run ./cmd/ksql --output json
~~~

Deployment manifest 示例：

~~~powershell
@'
INSERT INTO deployments (manifest)
VALUES ('{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"api"},"spec":{"replicas":2,"selector":{"matchLabels":{"app":"api"}},"template":{"metadata":{"labels":{"app":"api"}},"spec":{"containers":[{"name":"api","image":"nginx:1.27"}]}}}}');
'@ | go run ./cmd/ksql --namespace demo --output json
~~~

Lexer 先解析 SQL 外层单引号和转义，再由 encoding/json 解析内部 manifest。namespaced 资源缺少 namespace 时注入 CLI namespace；若二者冲突则报错。拒绝 status 和服务器管理元数据。同名资源返回 AlreadyExists，不会转为 UPDATE。

## 7. 总体架构

~~~text
输入与选项
  -> cli
  -> parser -> lexer -> token
  -> ast
  -> catalog / eval
  -> query / write
  -> kube / discovery
  -> output / apperror
~~~

typed client 适合固定资源类型；dynamic client 通过 GVR 和 unstructured 对象处理运行时发现的资源；Metrics REST client 访问 `metrics.k8s.io/v1beta1`；Discovery Resolver 负责 GVR、scope、kind 和 verbs。query 和 write 负责 SQL 操作流程，不让 Parser 依赖 Kubernetes API。

## 8. Lexer 原理

internal/lexer/lexer.go 将输入转成 []rune，逐字符扫描并维护 position、line、column。NextToken 识别空白、标点、组合运算符、字符串、双引号标识符、数字、普通标识符和关键字。未知字符变成 ILLEGAL token。

Lexer 只负责字符到 Token 的转换。它记录 token 起始行列，所以 Parser 能报告具体错误位置。单引号字符串中的两个单引号表示转义；双引号用于可能含斜线或点号的标识符。

## 9. Token 设计

internal/token/token.go 定义 lexer 和 parser 共享的 Token：

~~~go
type Token struct {
    Type    Type
    Literal string
    Line    int
    Column  int
}
~~~

Type 表示类别，Literal 保留词面值，Line/Column 记录起点。关键字大小写不敏感，普通标识符保留原文。双引号标识符用于资源路径或 JSON Pointer，例如 "/spec/replicas"。

## 10. AST 和递归下降 Parser

internal/ast/select.go 定义 SELECT、UPDATE、DELETE、INSERT 和表达式节点。表达式节点包括 ColumnReference、Literal、BinaryExpression、UnaryExpression 和 IsNullExpression。SELECT * 保留为 Star 节点，由语义阶段按表展开。

递归下降 Parser 将语法规则映射到函数。WHERE 的优先级链：

~~~text
parseOr
  -> parseAnd
      -> parseNot
          -> parseComparison
              -> parsePrimary
~~~

低优先级函数调用高优先级函数，生成的 AST 就按 SQL 规则分组。括号递归调用表达式解析。Parser 只判断语法，不判断表列是否存在；语法错误由 parser.Error 保存 E_PARSE、行和列。

## 11. Catalog 和语义检查

internal/catalog/catalog.go 定义固定表、公开列和 Projection。Projection 将字段 Source 与输出键 Output 分开，使 AS 只改变 JSON 键。catalog 检查固定表、列、WHERE 引用，并将 SELECT * 展开为固定表列。动态资源的字段在运行时才知道，因此先建立延迟投影，再由 Discovery 和实际对象解析。

语义检查先于 API List 请求；拼错表或列会先返回语义错误，不会访问集群。

## 12. Kubernetes Client

internal/kube/client.go 从一份 kubeconfig 创建 typed、dynamic、Discovery 和 Metrics client，并解析 context 的默认 namespace。

- kubernetes.Interface：固定 Kubernetes API 类型。
- dynamic.Interface：运行时 GVR 和 unstructured 对象。
- discovery.DiscoveryInterface：资源 group/version/scope/verbs。
- metrics.Client：Pod/Node 实际资源使用量。
- Namespace：context 默认 namespace，没有配置时为 default。

使用 client 接口便于测试注入 fake client。三个 client 共用同一个 REST 配置、凭据和 context。

## 13. Query 执行流程

internal/query/executor.go 先调用 catalog 做语义检查，再选择 typed 或 Discovery/dynamic 路径。查询对象经 internal/query/resources.go 转成 resourceRow，然后对每一行求值 WHERE，最后按 Projection 生成输出列。

固定资源的映射：

- Namespace：metadata.name。
- Deployment：metadata.name、metadata.namespace、spec.replicas。
- Ingress：metadata.name、metadata.namespace 和默认后端 Service 名称。

动态资源使用 unstructured.Unstructured。name/namespace 从 metadata 读取，JSON Pointer 字段可用于投影和 WHERE；cluster-scoped 资源的 namespace 输出为 JSON null。先过滤再投影，因此 WHERE 字段不必出现在 SELECT 列表中。Metrics 表先读取专用 API，再复用相同的过滤和投影流程。

## 14. WHERE 求值和三值逻辑

internal/eval 将值归一为 NULL、STRING、INTEGER、DECIMAL 或 BOOLEAN。比较先处理 NULL，再做类型匹配；不兼容类型报 E_EVAL，数字不会按字符串顺序比较。

SQL 条件结果为 TRUE、FALSE、UNKNOWN：

~~~text
FALSE AND UNKNOWN = FALSE
TRUE  AND UNKNOWN = UNKNOWN
TRUE  OR  UNKNOWN  = TRUE
FALSE OR  UNKNOWN  = UNKNOWN
NOT UNKNOWN       = UNKNOWN
~~~

普通比较涉及 NULL 会得到 UNKNOWN；IS NULL/IS NOT NULL 单独判断空值。WHERE 只接受 TRUE。

## 15. UPDATE、DELETE 和 JSON Patch

固定表 UPDATE 的可写列：

| 表 | 可写列 |
| --- | --- |
| namespaces | labels、annotations |
| deployments | replicas、labels、annotations |
| ingresses | default_backend_service、labels、annotations |

internal/write/update.go 校验语句和赋值，List 候选对象，复用 eval 匹配，再让 patch.go 生成 JSON Patch。Patch 首先 test metadata.resourceVersion，成功后只修改 SET 指定路径。这样不会拿一份简化对象覆盖 Deployment 的 selector、Pod template 或其他字段。

DELETE 执行相同的 WHERE 校验和候选匹配，然后调用普通 Delete。多对象操作逐个执行，单个失败会写入错误汇总，其他对象仍继续处理。

dynamic_update.go、dynamic_delete.go 使用 Discovery 返回的 GVR、scope 和 verbs。动态 UPDATE 要求双引号 JSON Pointer，并使用带 resourceVersion test 的 JSON Patch。

Discovery 会区分部分发现失败：健康 API group 仍可用，失败 group 会返回 E_DISCOVERY。`/status`、`/scale`、`exec`、`logs` 等子资源不作为普通 SQL 表。创建 `CustomResourceDefinition` 后，`discovery.WaitForCRD` 轮询 Established 条件，并确认每个 served version 已经可被 Discovery 解析。

## 16. INSERT、错误和输出

固定 manifest 由 internal/write/manifest.go 解码和校验；动态 manifest 在 dynamic_insert.go 中校验。检查表和 manifest 匹配、apiVersion/kind、metadata.name、namespace 规则和服务器管理字段，然后调用 Create。API Server 负责资源 schema 及其他服务端校验。

internal/apperror/error.go 将错误转换为稳定错误码，保留 cause 以支持 errors.Is/errors.As，并把常见 Kubernetes API 错误分类。常见代码：

| 错误码 | 含义 |
| --- | --- |
| E_USAGE | 参数或输出格式错误 |
| E_PARSE | SQL 语法错误 |
| E_SEMANTIC | 表、列或值不合法 |
| E_EVAL | WHERE 类型或求值错误 |
| E_WHERE_REQUIRED | UPDATE/DELETE 缺少 WHERE |
| E_NAMESPACE_REQUIRED | 缺少 namespace |
| E_KUBE | Kubernetes 请求错误 |
| E_ALREADY_EXISTS | 资源已存在 |
| E_NOT_FOUND | 资源不存在 |
| E_FORBIDDEN | 权限不足 |
| E_CONFLICT | 资源冲突 |
| E_DISCOVERY | Discovery 错误 |
| E_UNSUPPORTED_VERB | 资源不支持所需 API 操作 |
| E_UNSUPPORTED_SUBRESOURCE | `/status`、`/scale` 等不是普通 SQL 表 |
| E_METRICS_UNAVAILABLE | Metrics API 不可用或响应无法解析 |
| E_OUTPUT | JSON 输出错误 |

internal/output/json.go 将 SELECT 输出为 JSON 数组，并输出写操作结果。internal/output/error.go 将错误写到 stderr，避免序列化底层 cause。

退出码：0 表示成功；1 表示 Kubernetes/输出错误或写操作部分失败；2 表示参数、解析、语义或表达式错误。写操作部分失败仍输出 JSON 汇总，再返回 1。

## 17. CLI、文件和测试

cmd/ksql/main.go 是薄入口，只将进程参数和标准流传给 cli.Run。internal/cli/options.go 用标准库 flag 解析参数。internal/cli/app.go 负责读 stdin、解析 AST、创建 clients、按语句类型调用 query/write、写 JSON 和返回退出码。

Run 接收 io.Reader/io.Writer，因此 CLI 测试可以使用内存 buffer，无需启动进程。命令入口与业务流程分开，便于独立测试。

### 文件职责

| 路径 | 作用 |
| --- | --- |
| cmd/ksql/main.go | 进程入口，调用 cli.Run |
| go.mod | Go 模块和依赖版本 |
| go.sum | 依赖校验和 |
| README.md | 当前实现、用法、架构和文件说明 |
| internal/token/token.go | Token 类型和词面位置 |
| internal/lexer/lexer.go | SQL 逐字符扫描 |
| internal/lexer/lexer_test.go | lexer 关键字、字符串、运算符和位置测试 |
| internal/ast/select.go | SQL 语句和表达式 AST |
| internal/parser/parser.go | 递归下降语法分析 |
| internal/parser/error.go | E_PARSE 和行列错误位置 |
| internal/parser/parser_test.go | AST、语法优先级和语法错误测试 |
| internal/catalog/catalog.go | 表列定义、语义验证、投影和别名 |
| internal/eval/value.go | SQL/Kubernetes 值归一化 |
| internal/eval/compare.go | 类型安全的比较 |
| internal/eval/logic.go | 三值逻辑运算 |
| internal/eval/eval.go | WHERE AST 求值 |
| internal/eval/eval_test.go | 表达式和 NULL 逻辑测试 |
| internal/kube/client.go | typed/dynamic/Discovery client 构造 |
| internal/discovery/reference.go | 精确 GVR 引用解析 |
| internal/discovery/resolver.go | Discovery 资源、scope、kind、verbs 解析 |
| internal/discovery/crd.go | CRD Established 和 Discovery 就绪等待 |
| internal/discovery/resolver_test.go | 版本选择、部分发现失败和子资源测试 |
| internal/discovery/crd_test.go | CRD 就绪等待测试 |
| internal/metrics/client.go | Metrics API REST client |
| internal/metrics/quantity.go | CPU/内存 Quantity 解析和累加 |
| internal/metrics/quantity_test.go | Metrics API、数量和权限测试 |
| internal/query/resources.go | Kubernetes 对象到查询行的映射 |
| internal/query/executor.go | SELECT、WHERE、投影和动态查询 |
| internal/query/executor_test.go | fake client 查询测试 |
| internal/write/executor.go | 写操作路由 |
| internal/write/update.go | 固定资源 UPDATE |
| internal/write/delete.go | 固定资源 DELETE |
| internal/write/insert.go | 固定资源 INSERT |
| internal/write/manifest.go | 固定 manifest 校验 |
| internal/write/patch.go | 固定资源 JSON Patch 构建 |
| internal/write/validate.go | 写列、类型和 WHERE 校验 |
| internal/write/values.go | AST 字面量和 JSON map 转换 |
| internal/write/dynamic_update.go | 动态资源 JSON Pointer UPDATE |
| internal/write/dynamic_delete.go | 动态资源 DELETE 和动态字段映射 |
| internal/write/dynamic_insert.go | 动态资源 manifest 创建 |
| internal/write/result.go | affected/failed rows 和对象错误结构 |
| internal/write/errors.go | 逐对象错误及 WHERE 匹配辅助 |
| internal/write/executor_test.go | UPDATE、DELETE、where 和 Patch 测试 |
| internal/write/insert_test.go | typed INSERT 和 manifest 校验测试 |
| internal/apperror/error.go | 错误分类、稳定错误码和退出码 |
| internal/apperror/error_test.go | 错误转换与退出码测试 |
| internal/output/json.go | 查询和写操作 JSON 输出 |
| internal/output/error.go | 结构化错误 JSON |
| internal/output/json_test.go | JSON 成功结果测试 |
| internal/output/error_test.go | JSON 错误和 cause 隔离测试 |
| internal/cli/app.go | CLI 流程编排 |
| internal/cli/options.go | 命令行参数解析 |
| internal/cli/app_test.go | CLI 参数与错误输出测试 |
| fixtures/integration-base.yaml | 集成测试的 Kubernetes 对象模板 |
| test/integration/helpers_test.go | fixture 准备、二进制构建、命令执行和清理 |
| test/integration/integration_test.go | 真实 API Server 查询和写入测试 |
| test/integration/hard_test.go | CRD/dynamic CRUD 和 Metrics 集成测试 |

### 测试命令

本地单元测试：

~~~powershell
go test -count=1 ./...
~~~

静态检查和构建：

~~~powershell
go vet ./...
go build ./...
staticcheck ./...
gofmt -l .
git diff --check
~~~

真实 Kubernetes 集成测试要求集群运行：

~~~powershell
$env:KUBESQL_INTEGRATION = "1"
go test -count=1 -v ./test/integration
~~~

集成测试创建唯一 sql-it-* namespace，应用 fixtures/integration-base.yaml，构建 ksql 并检查真实 API Server 行为，结束后删除该 namespace。hard 测试还创建临时 CRD，验证 CRD 注册等待、namespaced/cluster-scoped Custom Resource CRUD；Metrics 测试要求 Metrics Server 可用，否则跳过。kubectl 只由测试 harness 准备和检查 fixture；KubeSQL 本身使用 client-go 访问 API Server。

## 当前代码约束

- Lexer 逐字符扫描，Parser 使用递归下降。
- Parser 不依赖 Kubernetes。
- 固定资源用 typed client，其他资源通过 Discovery 和 dynamic client。
- Metrics 使用 `metrics.k8s.io/v1beta1`，CPU 和内存使用 `resource.Quantity` 转换。
- CRD 创建成功必须同时满足 Established 和 Discovery 可见。
- 查询先做语义检查，再请求 API Server。
- UPDATE/DELETE 要求 WHERE。
- UPDATE 只 Patch 指定字段并检查 resourceVersion。
- INSERT 分开处理 SQL 字符串和 JSON manifest。
- SELECT 输出 JSON 数组，保留数字和 JSON null 类型。
- 错误使用稳定错误码及约定退出码。
