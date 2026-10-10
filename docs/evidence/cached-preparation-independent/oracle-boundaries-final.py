import subprocess,tempfile,pathlib
oracle='/workspace/scratch/5e06a74e5f65/durable-restart/toolchains/sqlite-build-3540000/sqlite3'
counters='SELECT changes(),total_changes(),last_insert_rowid();'
with tempfile.TemporaryDirectory() as directory:
 ps=[]
 def start(name):
  p=subprocess.Popen([oracle,'-batch','-header','-quote',str(pathlib.Path(directory)/'ordinary.db')],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True);ps.append((name,p))
  p.stdin.write('.print __ready__\n');p.stdin.flush()
  if p.stdout.readline().strip()!='__ready__':raise RuntimeError('CLI initialization handshake failed')
  return p
 def run(p,q):
  print('SQL',next(n for n,x in ps if x is p),repr(q))
  p.stdin.write(q+'\n.print __done__\n');p.stdin.flush()
  while True:
   s=p.stdout.readline()
   if not s:raise RuntimeError('unexpected normal-stream EOF')
   if s.strip()=='__done__':break
   print(s,end='')
 a=start('setup');b=start('lock')
 run(a,'CREATE TABLE t(id INTEGER PRIMARY KEY,v); INSERT INTO t VALUES(1,1),(2,2);')
 for label,warm in [('cold',None),('constant','SELECT 1;'),('counter',counters),('read','SELECT * FROM t;'),('invalid','DELETE FROM absent;'),('schema','SELECT name FROM sqlite_schema;')]:
  p=start(label)
  if warm:run(p,warm)
  run(b,'BEGIN EXCLUSIVE;')
  for q in ['DELETE FROM absent;','UPDATE t SET missing=1;','DELETE FROM t;',counters]:run(p,q)
  run(b,'ROLLBACK;')
 # Set nonzero counts before normal DDL rollback variants.
 for label,sql in [('full','BEGIN; CREATE TABLE own(x); ROLLBACK;'),('savepoint','SAVEPOINT s; CREATE TABLE own(x); ROLLBACK TO s; RELEASE s;'),('emptyrollback','BEGIN; ROLLBACK;'),('failedcreate','BEGIN; CREATE TABLE t(x); ROLLBACK;'),('createindex','BEGIN; CREATE INDEX own_ix ON t(v); ROLLBACK;')]:
  p=start(label)
  run(p,'UPDATE t SET v=v;')
  for statement in sql.split(';'):
   if statement.strip():run(p,statement+';')
  run(b,'BEGIN EXCLUSIVE;')
  for q in ['DELETE FROM absent;', 'DELETE FROM t;', counters]:run(p,q)
  run(b,'ROLLBACK;')
 # A stale negative preparation must refresh before returning an error once it can lock.
 p=start('refresh');run(p,'UPDATE t SET v=v;')
 run(b,'ALTER TABLE t ADD COLUMN newer; CREATE TABLE fresh(x);')
 run(p,'UPDATE t SET newer=7;');run(p,'INSERT INTO fresh VALUES(8);');run(p,counters)
 # Stale successful binding must be rechecked under snapshot and must not write wrong table layout.
 run(b,'DROP TABLE t; CREATE TABLE t(id INTEGER PRIMARY KEY,different); INSERT INTO t VALUES(1,11),(2,12);')
 run(p,'UPDATE t SET newer=7;');run(p,counters);run(p,'SELECT * FROM t;')
 # Committed schema updates on one connection remain warm on the same connection.
 run(p,'CREATE TABLE own(x);');run(p,'INSERT INTO own VALUES(9),(10);');run(b,'BEGIN EXCLUSIVE;')
 run(p,'UPDATE own SET x=x;');run(p,counters);run(b,'ROLLBACK;')
 for n,p in ps:
  p.stdin.write('.quit\n');p.stdin.close();print('NORMAL END',n,p.wait())
