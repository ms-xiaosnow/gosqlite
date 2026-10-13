# ANN 报告三项修正与离线入口：精确增量证据

2026-10-13 06:11:06 UTC，六门全部 PASS。代码基点：
40e287dc50f8c282f2003ca16475af5eb73a9400。
[被测新增文件身份](tested-scope.json)绑定实际执行；本提交保存相同 Go/JSON 字节及记录。
原4,029条全仓路径摘要未发布，只在原本地记录保留；没有换名或内嵌到其他文件。
当前公开证明采用上述基点提交与受测15个Go/JSON文件身份。
后续文档同步不改变受测代码，不重复无关整套。

## 实测

- 29个顶层根：reportcheck 23、CLI 6。
- normal：467个命名项PASS（23+438个报告项、6个CLI根），零FAIL/SKIP。
- race：同样467个命名项PASS，零FAIL/SKIP。
- 新两包vet、build、整仓源码guard、格式检查均PASS。
- 11个Go文件格式化零字节变化，资源闩锁无越界，自然退出均0。
- CLI完整SYNTHETIC输入确实经 run → DecodeReport → ValidatePass →
  JSON输出，VALID且performance_accepted=false；状态改为incomplete被拒绝。
  未执行CLI的main/os.Exit，也未启动native测量。

[命令和环境](commands.json)、[逐项终态](completion.json)、
[normal原始事件](02_normal.stdout)、[race原始事件](03_race.stdout)、
[记录器](recorder.py)、[独立审查](independent-review.md)保留。
每门stdout/stderr、资源采样、起止/退出码与result.json配套。

固定Go1.27.2 SHA-256：
548d3c32f83fa81152f543eb1526be7884cfe0526eb5e9ce29d3eca14786e7e0。
Linux/amd64、默认tags；普通CGO=0、race工具CGO=1；显式
GOTOOLCHAIN=local、GOPROXY/GOSUMDB/GOAUTH=off，单Go租约。
测试仅嵌入JSON、内存值/缓冲区与合成I/O错误。
sourceguard另读项目源码；工具链可读源码/缓存，不能声称runtime/loader无系统访问。

## 三处修正

1. HNSW：NodeReads−BaseRowFetches=Visited、Visited≥ExactRescores、
   DistanceEvaluations≥Visited；非负/配额和受保护减法避免溢出。
2. IVF：自适应打开全部列表时MemberKeysRead=Config.Rows。
3. 同query/RowID的exact与最终rescore距离必须数值相等，
   同快照dense float32与同L2实现，无人为epsilon。

[原V3最小补丁](historical-v3-three-fix.patch)、
[来源](three-fix-source-provenance.json)保留。
新入口仅离线reportcheck与stdin CLI；旧native benchmark未改，
没有直接安装整份V4。

## 原中断与历史FAIL

已确认2026-10-10 22:39整轮安全中断，精确调用和目标未知。
独立约22:30的41-target活动记录原文：
“This content was flagged for possible cybersecurity risk.”
原始中断说明Library：
libfile_0958d62d04dc8191bdb2ea277785b416，
SHA-256 13092da4ed0e6ca3d7633dc4951a9ac5d0ab18060f6bbd55a4330d39a3a2a5d2。
其全面暂停是事后谨慎决定，未定位到具体ANN源码编辑拒绝。
本轮按用户新请求，限定三项源码/离线入口与有限fixture；
没有重发旧被拒操作、旧探针、campaign、未知整链或物理故障。

V3 b13158709a57ce175437bb48a0d66104b5eff68d历史FAIL/HOLD；
V4 3f36a1130ea62df0b6688d6ebd793ecf8bb4442b只多未修探针。
静态归档确认此差异，原15/16份参考文件不变。
原V3独立审查全文与完整旧运行日志未在当前恢复件中核验恢复；
新结果不补造旧日志，本轮不声称重新取得V3 RED运行。

held source Library：
libfile_276312ea452881918463dcb19db1e1b8，
SHA-256 62426f18782f2f39d25b95e9a98448649163399015da5f64d1c66da571811423。
本轮只静态提取ANN选择。[原中断](../../interruption-20261010.md)、
[溯源](../../restriction-provenance-20261011.md)。

## 复用与未验收

40e287未变引擎、其他包、oracle与手工门控证据继续适用。
其CI [38346611629](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38346611629)
两作业SUCCESS，终态档案libfile_15e09e6d493481919edf232265c66e0d，
SHA-256 c737063610ede5632cb3211531ddaf7032fd1336ec7875e6b6a5c999983a4233。
本地只测新增两包/必要guard，未重跑全仓、oracle或跨平台。
现有Ubuntu工作流随新提交自动运行，按精确提交另监控；
本记录不预报终态，也不为结果递归造docs提交。

三项准入代码与有限回归已闭合；真实固定报告来源、性能、recall未验收。
未在此接入或运行schema3 native生产器；旧聚合报告不能改版本后冒充兼容。
[统一手工清单](../../testing/finite-acceptance-summary-20261012-zh.md)
17组/82核心场景仍NOT_RUN，M10未通过。
[证据文件索引](evidence-manifest.json)不自含摘要，避免循环身份。

文档准备时一次JS字符串语法错误在调用前返回
SyntaxError: Unexpected identifier 'cmd'，没有执行该批文件写入。
后续静态写入修正完成；这不是Go测试失败或安全拒绝。

## 获准缩小的发布范围

2026-10-13，完整progress更新和全仓路径摘要在用户授权后仍遇发布限制。
按用户随后批准的缩小方案，保留原progress、acceptance-status及历史余缺文档
不变，只发布ANN源码、专属记录和更新后的权威手工总MD。
被省去的正文、全仓摘要及含其base64的旧请求参数不进入其它文件、压缩包或备份。
原记录器代码如实保留：它当时仅在本地生成snapshot，没有内嵌摘要数据或上传器。
六门PASS和15个受测文件字节未变，没有重跑测试。
