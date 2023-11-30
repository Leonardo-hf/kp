云计算Github消息生产者，周期性向kafka内写入消息

```
.
├── Dockerfile...........................打包镜像
├── github_msg.proto.....................定义消息格式
├── go.mod
├── go.sum
├── kpjob.yaml...........................k8s JOB
├── main.go..............................生产者逻辑
├── msg..................................消息结构体
│   └── github_msg.pb.go
└── README.md
```

镜像：`applerodite/kp:latest`

接受以下参数（用环境变量的形式配置）：

| 参数      | 类型   | 默认  | 备注                                                         |
| --------- | ------ | ----- | ------------------------------------------------------------ |
| Datasets  | string |       | 标注 Github json 数据集的绝对路径，逻辑里直接 io.Read 读该数据集 |
| Addr      | string |       | kafka broker 域名                                            |
| Topic     | string |       | kafka topic                                                  |
| CInterval | int64  |       | 每周期生产 CInterval 秒内产生的 github 项目，例如：设置为 86400 表示，每周期生产一天产生的 github 项目 |
| PInterval | int64  |       | 每周期的实际秒数，例如：设置为 600 表示，每 10 min 作为一个周期生产一批 github 项目 |
| NotSleep  | bool   | false | 代码里默认从下一个整点开始第一次生产。设置为 true 可以取消这一特征 |

备注：

package.json 在描述依赖时，对于被依赖的 npm 包的版本，通常会使用范围表示依赖某个版本及更新版本，例如：“>= 1.0”，“~1.0”，“^1.0”，“1.X”等，生产者在处理时，将其中的特殊字符全都替换为了空格，X 替换为 0。

