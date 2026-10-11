# 中文验收文档的静态复核

日期：2026-10-11 UTC。源码基点：
`776b23eb59ce9613bf4498d352a716a193cdc9c3`。
结论：**限定文档审阅 PASS**，不是数据库运行或最终验收 PASS。

## 范围与身份

- `docs/manual-verification-zh.md`：840 行，SHA-256
  `7a6559bbf76e702443b9f1d5ac4a559c6f2f0b78b07baf655bad87d3aa3e5e14`。
- 独立检查全部命令的现有源码入口、flag、测试包/声明、JSON 字段、
  标签/环境门、隔离副本、日志和清理语义；没有运行任何文档命令、
  测试、构建、fuzz、故障或 namespace 操作。
- 39 个 focused 普通测试选择均在相应 package 中声明；73 个 capture
  名称无重复且有失败停止。较广静态引用检查找到 48 个真实测试名；
  仅 TestCrash 不存在，但它是明确禁止虚构入口的反例。
- 新文档的相对链接存在、代码围栏配对；原 goal 哈希不变；
  checkpoints.json 只新增 776b 既有证据，没有改写旧条目语义。

## 首轮发现与修正

1. sparse-exact 示例错误可能仍退出 0：补充空 stderr、完整唯一 JSON、
   具体维数/计数/命中断言，不能仅据 capture 返回 0 判通过。
2. write 示例忽略 Close 且 log.Fatal 绕过 defer：明确只能证明写入 smoke。
3. 部分加密/native-check 示例失败也无条件删除 fixture：明确日志与完整
   失败现场不是同一证据；需要现场保全的正式规程仍缺实现。
4. maintenance 和 manual 命令使用独立 capture 名及失败停止。
5. 源清单启用 pipefail，身份/哈希输出检查错误，避免不完整证据继续使用。
6. 本地开发空间资料不是仓库依赖；修正相对路径和 progress 入口。

首轮对验收矩阵的独立审阅还要求将 /tmp 余量与 fsync=volatile 明确为
10 日针对拟议长测的历史预检；未来须重新核验，不概括成当前普遍事实。

## 没有改变的边界

- main 与未发布候选分开；Native Check V2 已合入，不再写作缺实现。
- 保留代码缺口、FAIL/HOLD、NOT_RUN、平台阻塞与真实环境需求。
- Windows/Darwin 两项 raw namespace 测试在当前 main 尚未门控；不能
  直接开启这些 OS 的默认全套。ANN V4、新 41 目标 fuzz 和未知中断
  链路保持暂停，没有提供重启或换路命令。
- ANN V3 的缺陷属于未发布报告准入工具，不凭此推断生产算法错误。
- 文档通过不证明物理恢复、安全、跨平台、长期压力或 M10 完整验收。
