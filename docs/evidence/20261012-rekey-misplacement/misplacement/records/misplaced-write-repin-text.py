# Source text and byte-hash maintenance only. Does not execute a Go tool/parser.
from pathlib import Path
import hashlib,json,re,sys
R=Path('/workspace/scratch/5e06a74e5f65/restart-20261012T2033')
O=Path(sys.argv[1]) if len(sys.argv)>1 else R/'misplaced-write-overlay'
for guard,table,items in [
 ('internal/testscope/rollback_fault.go','rollbackFaultOwnedShapes',{
 'rollbackFaultManual':'internal/pager/rollback_io_manual_linux.go',
 'rollbackFaultContract':'internal/testkit/rollbackfault/contract.go',
 'rollbackFaultContractTest':'internal/testkit/rollbackfault/contract_test.go'}),
 ('internal/testscope/rollback_fault_sql.go','rollbackFaultSQLShapes',{
 'rollbackFaultSQLBridge':'internal/pager/rollback_fault_sql_manual_linux.go',
 'rollbackFaultSQLNative':'sqlite/rollback_fault_sql_manual_linux_test.go',
 'rollbackFaultSQLModel':'internal/testkit/rollbackfault/sql_ledger.go',
 'rollbackFaultSQLModelTest':'internal/testkit/rollbackfault/sql_ledger_test.go'})]:
 p=O/guard;s=p.read_text();at=s.index('var '+table+' =');prefix=s[:at];tail=s[at:]
 for key,f in items.items():
  raw=(O/f).read_bytes();text=raw.decode();h=hashlib.sha256(raw).hexdigest()
  pattern=r'(^\t'+key+r':\s+)"[0-9a-f]{64}"'
  prefix,n=re.subn(pattern,lambda m:m.group(1)+json.dumps(h),prefix,count=1,flags=re.M);assert n==1,(key,'pin')
  pattern=r'(^\t'+key+r':\s+)(`[^`]*`|"(?:[^"\\]|\\.)*")(,\n)'
  value='`'+text+'`' if table=='rollbackFaultOwnedShapes' and '`' not in text else json.dumps(text,ensure_ascii=True)
  tail,n=re.subn(pattern,lambda m:m.group(1)+value+m.group(3),tail,count=1,flags=re.M);assert n==1,(key,'shape')
 p.write_text(prefix+tail)
print('Updated seven source pins and matching complete-source snapshots; no semantic expectation changed.')
