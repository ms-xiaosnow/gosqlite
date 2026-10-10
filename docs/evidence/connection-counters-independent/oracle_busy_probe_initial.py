import subprocess, tempfile, pathlib, json
cli=str(pathlib.Path('../toolchains/sqlite-build-3540000/sqlite3').resolve())
class CLI:
 def __init__(self,path):
  self.p=subprocess.Popen([cli,'-batch','-noheader','-quote',path],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True,bufsize=1)
  self.n=0
 def run(self,sql):
  self.n+=1;m='MARK'+str(self.n)
  self.p.stdin.write(sql+'\n.print '+m+'\n'); self.p.stdin.flush(); out=[]
  while True:
   v=self.p.stdout.readline().rstrip('\n')
   if v==m: break
   if not v and self.p.poll()!=None: raise RuntimeError('early EOF')
   out.append(v)
  print(json.dumps({'sql':sql,'out':out}),flush=True)
  return out
 def close(self):
  self.p.stdin.close(); print('normal EOF status',self.p.wait())
with tempfile.TemporaryDirectory() as tmp:
 a=CLI(tmp+'/t.db'); b=CLI(tmp+'/t.db')
 try:
  a.run('SELECT sqlite_version(),sqlite_source_id();')
  a.run('CREATE TABLE t(id INTEGER PRIMARY KEY); INSERT INTO t VALUES(1),(2);')
  b.run('BEGIN EXCLUSIVE;')
  a.run('INSERT INTO t VALUES(3); SELECT changes(),total_changes(),last_insert_rowid();')
  a.run('DELETE FROM missing; SELECT changes(),total_changes(),last_insert_rowid();')
  b.run('ROLLBACK;')
  a.run('INSERT INTO t VALUES(4),(5);')
  b.run('BEGIN IMMEDIATE;')
  a.run('INSERT INTO t VALUES(3); SELECT changes(),total_changes(),last_insert_rowid();')
  b.run('ROLLBACK;')
  a.run('INSERT INTO t VALUES(6),(7); BEGIN;')
  b.run('BEGIN EXCLUSIVE;')
  a.run('INSERT INTO t VALUES(3); SELECT changes(),total_changes(),last_insert_rowid();')
  b.run('ROLLBACK;')
  a.run('ROLLBACK; INSERT INTO t VALUES(8),(9);')
  b.run('BEGIN; SELECT * FROM t;')
  a.run('INSERT INTO t VALUES(10); SELECT changes(),total_changes(),last_insert_rowid();')
  b.run('ROLLBACK;')
  a.run('SELECT * FROM t;')
 finally: a.close(); b.close()
