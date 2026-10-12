package sqlite

import (
	"fmt"
	"runtime"
)

// WALTailPublicationOptions 显式选择一次有限的独占 WAL 尾部发布。
// 本实现属于实验性源码：原生恢复、故障和物理持久性尚未验收。
// Options 中的 nil 保持既有尾部拒绝；非 nil 只允许 Linux、可写明文、
// WALExclusive，不能与共享 WAL、加密、RollbackRecovery 或 powersafe
// overwrite 组合。该策略没有扩大格式、文件系统或命名空间范围的参数。
// 仅在已验证的完整未提交尾部出现时才获取发布能力；干净或不存在的 WAL
// 继续走既有路径。该配置不是原生试验的执行许可或生产安全声明。
type WALTailPublicationOptions struct{}

// 仅检查标量策略，不解析路径、不预留 native owner、不调用文件系统。
// 未启用时保持原有选项错误与行为，由各既有检查继续负责。
func validateWALTailPublication(enabled, readOnly, exclusive, shared, encrypted, recovery, powersafe bool, platform string) error {
	if !enabled {
		return nil
	}
	if readOnly {
		return fmt.Errorf("%w: WAL tail publication requires writable access", ErrReadOnly)
	}
	if !exclusive || shared || encrypted || recovery || powersafe {
		return fmt.Errorf("%w: WAL tail publication requires exclusive plaintext WAL without recovery or powersafe overwrite", ErrUnsupported)
	}
	if platform != "linux" {
		return fmt.Errorf("%w: WAL tail publication currently requires Linux", ErrUnsupported)
	}
	return nil
}

// 值复制固定本次连接的策略；空结构体不承诺独立指针地址。
func snapshotWALTailPublication(opts Options) (Options, error) {
	if err := validateWALTailPublication(opts.WALTailPublication != nil, opts.ReadOnly,
		opts.WALExclusive, opts.WALShared, opts.Encryption != nil,
		opts.RollbackRecovery != nil, opts.WALPowersafeOverwrite, runtime.GOOS); err != nil {
		return opts, err
	}
	if opts.WALTailPublication != nil {
		copy := *opts.WALTailPublication
		opts.WALTailPublication = &copy
	}
	return opts, nil
}
