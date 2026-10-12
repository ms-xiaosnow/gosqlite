//go:build linux && manual_fault_rollback && !manual_restart_rollback

package pager

// The fault-only selection contains no restart state, environment, IPC or exit.
func rollbackRestartBefore(ticket rollbackFaultTicket) {}
func rollbackRestartAfter(ticket rollbackFaultTicket)  {}
