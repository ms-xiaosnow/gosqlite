from pathlib import Path
import subprocess,os,hashlib,json
r=Path('/workspace/scratch/5e06a74e5f65/durable-restart/gosqlite');p=r.parent;base='a9fea96211d4aecfc98e0da02321b24daf9342b8';env=os.environ.copy();env['GIT_INDEX_FILE']=str(p/'grouped-counters-final-index')
def git(*a,**kw):return subprocess.check_output(['git',*a],cwd=r,env=env,**kw)
git('read-tree',base);patch=p/'sql-preparation-independent-evidence/independent-tests.patch';assert hashlib.sha256(patch.read_bytes()).hexdigest()=='3c37c3e236eaba5565066e592974414b2209e7b6c412a3c9a14931d2c23f5d11';git('apply','--cached','--check',str(patch));git('apply','--cached',str(patch))
def edit(rel,fn):
 data=git('show',':'+rel).decode();new=fn(data);assert new!=data,rel
 sha=git('hash-object','-w','--stdin',input=new.encode()).decode().strip();git('update-index','--cacheinfo','100644,'+sha+','+rel)
def native(s):
 old='''\t\t\t// Native snapshot admission precedes preparation: EXCLUSIVE BUSY is
\t\t\t// explicitly outside SQLite's cached-schema parity, so preserve prior
\t\t\t// counters. The strict official expectation is retained as negative evidence.
\t\t\tchanges := int64(0)
\t\t\tif mode == "BEGIN EXCLUSIVE" {
\t\t\t\tchanges = 2
\t\t\t}
\t\t\treviewCounterWant(t, db, changes, 2, last)'''
 new='''\t\t\t// The independently verified cached-schema repair now classifies this
\t\t\t// warm ordinary valid INSERT as execution BUSY, matching the retained
\t\t\t// strict official control: changes resets but totals and source IDs do not.
\t\t\treviewCounterWant(t, db, 0, 2, last)'''
 assert old in s;s=s.replace(old,new)
 old='\tt.Logf("native error: %v", err)';new='\tif err == nil || errors.Is(err, sqlite.ErrBusy) {\n\t\tt.Fatalf("cached missing table must fail preparation: %v", err)\n\t}'
 assert old in s;s=s.replace(old,new)
 old='\tfor _, q := range []string{"SELECT changes(),count(*) FROM t GROUP BY id", "SELECT changes() OVER () FROM t"} {'
 new='''\t// Retain the original now-supported GROUP BY statement as a complete
\t// value/type positive; its official counterpart is in the combined history.
\tif got := reviewCounterQuery(t, db, "SELECT changes(),count(*) FROM t GROUP BY id"); !reflect.DeepEqual(got, [][]any{{int64(2), int64(1)}, {int64(2), int64(1)}}) {
\t\tt.Fatal(got)
\t}
\tfor _, q := range []string{"SELECT changes() OVER () FROM t"} {'''
 assert old in s;s=s.replace(old,new);return s
edit('sqlite/connection_counters_independent_test.go',native)
def oracle(s):
 old='\t\t{q: "SELECT COUNT(*),SUM(changes()),MAX(total_changes()),MIN(last_insert_rowid()) FROM t", read: true},'
 new=old+'''\n\t\t{q: "SELECT changes(),count(*) FROM t GROUP BY id", read: true},
\t\t{q: "SELECT id,COUNT(*),SUM(changes()),MAX(total_changes()),MIN(last_insert_rowid()) FROM t GROUP BY id ORDER BY id DESC LIMIT 2", read: true},
\t\t{q: "SELECT changes() AS c,count(*) AS n FROM t GROUP BY changes() HAVING count(*)>0 ORDER BY n DESC", read: true},'''
 assert old in s;return s.replace(old,new)
edit('internal/testkit/connection_counters_independent_test.go',oracle)
tree=git('write-tree').decode().strip();dst=p/'grouped-counters-final-verify';dst.mkdir(exist_ok=False)
a=subprocess.Popen(['git','archive',tree],cwd=r,stdout=subprocess.PIPE);subprocess.run(['tar','-x','-C',str(dst)],stdin=a.stdout,check=True);assert a.wait()==0
(p/'grouped-counters-final-tree.txt').write_text(tree+'\n');(p/'grouped-counters-final-refinement.patch').write_bytes(git('diff','--cached','--binary',base))
(p/'grouped-counters-final-all-sha256.txt').write_text(''.join(hashlib.sha256(f.read_bytes()).hexdigest()+'  '+str(f.relative_to(dst))+'\n' for f in sorted(dst.rglob('*')) if f.is_file()))
print('Final composition',tree,'adds reviewed cached-preparation regressions and retains original promoted GROUP BY/strict BUSY positive controls. Original assertions were identified before execution, not reported as a failed run.')
