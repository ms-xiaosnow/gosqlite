# 独立源码复核与增量证据适用

复核和执行日期2026-10-13 UTC。原始修改基点fd069d7a30b2e0ca8bdff67ca54c926aa118fc51。源码清单给逐文件SHA256；Git最终提交树绑定本目录，不把文档变化转作运行变化。

## PG风格CREATE INDEX

独立read-only复核CLEAR：parser只在ON table后新增USING分支，旧路径保持；严格仅dense HNSW/IVFFlat、单列opclass及既有配置边界，canonical metadata复用既有engine。语法不触达旧ANN V4报告修正或campaign。有效小数据库健康生命周期、真实索引结果/计数/Check、关闭重开与配置语义核对；无oracle/外部程序/manual/故障。

新5根加原5相关parser根，normal共104命名PASS（8parser根100，2healthy根4）；相同10根race104PASS；sourceguardPASS；全部零FAIL/SKIP。gofmt三文件零输出。完整命令、隔离env、固定Go1.27.2/gofmt SHA、日志和source-before压缩索引在pgvector。03:27:57 UTC自然闭合；代码执行前后相同。没有执行修前RED，原缺口由旧parser源码与goal7.3两例对照定位，不伪造baseline失败。

## 离线性能比较器

独立read-only复核CLEAR：6路径、57方法，45旧离线方法保留断言，新增5histogram与7bounded-input；旧9runner相关方法不带入，这些此前不在main，不减少已有default覆盖。stdlib+validation无subprocess/network/database/ANN。只在原check.sh的guard后加入offline unittest，无旧run.py/新runner/新workflow/测量。

原V3恢复来源Library libfile_30705dfdb9008191be17521369574232，26681613字节，SHA2560c925133d55dae1550f254e8d1a4f5ad7ad632a2e5aae7f0522b9a09d9d12941，旧treeb04c112944f042a1987b24692a2eeae3aecb1443；仅抽取已知regular源文档用于独立新切片，未执行旧runner，不继承旧PASS。

两处真实静态发现及修复：(1)旧reference读取先全量再限额、hash与parse可能分两次；改成32MiB+1 bounded单次JSON快照、同字节hash/parse、descriptor退出上下文，非JSON64KiB流式hash。(2)旧histogram桶与min/max/矩可能互相矛盾；新必要上下界与加权mean/variance校验、sample/population换算和浮点容差与native report.go阈值吻合，最终桶饱和。不是完整分布重建或敌对并发文件系统协议；保留原比较算法/置信判断，不凭syntheticPASS声称真实回归合格。

新57方法全部PASS，03:37:08 UTC，Python3.12.14、9.133秒，原始verbose日志/源码身份/命令见performance；shell仅bash -n解析。README独立审查发现“always prints”不精确，已修为输出写入成功才打印，失败为exception非PASS；只文案无代码变化。真实同机8–32对输入、控制环境、长期测量及性能regressionCI仍待外部输入/许可。普通CI仅检验离线逻辑。

## 复用与重跑原因

新parser分支及其两测试文件触发上述internal/sql与sqlite具名normal/race；新离线代码触发57Python方法和check.sh语法解析。既有fd/c5其他Go普通/oracle证据按未变源码保留，本地没有无理由全仓重跑。后续自动CI是既有push工作流的真实新组合验证，终态独立记录，未完成前不算PASS。精确bundle恢复只核Git/blob/mode/字节和fsck，不重复Go测试。

manual/native故障、退出、恢复、篡改/namespace及受阻ANN报告链仍NOT_RUN；编译、模型和固定seed不替代物理资格。整体未验收，M10未通过。
