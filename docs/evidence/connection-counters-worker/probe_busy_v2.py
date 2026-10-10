import subprocess,tempfile,pathlib
oracle=str(pathlib.Path('../toolchains/sqlite-build-3540000/sqlite3').resolve())
with tempfile.TemporaryDirectory() as directory:
    path=str(pathlib.Path(directory)/'ordinary.db')
    ps=[]
    def start():
        p=subprocess.Popen([oracle,'-batch','-noheader','-list',path],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True)
        ps.append(p)
        return p
    def run(p,q):
        p.stdin.write(q+'\n.print __done__\n');p.stdin.flush()
        print('SQL:',q)
        while True:
            line=p.stdout.readline()
            if not line: raise RuntimeError('unexpected EOF')
            if line.strip()=='__done__': break
            print(line,end='')
    a=start();b=start()
    run(a,'CREATE TABLE t(id INTEGER PRIMARY KEY); INSERT INTO t VALUES(1),(2);')
    run(b,'BEGIN IMMEDIATE;')
    run(a,'UPDATE t SET id=id;\nSELECT changes(),total_changes(),last_insert_rowid();')
    run(b,'ROLLBACK;')
    run(a,'UPDATE t SET id=id;')
    run(b,'BEGIN; SELECT * FROM t;')
    run(a,'INSERT INTO t VALUES(3);\nSELECT changes(),total_changes(),last_insert_rowid();')
    run(b,'ROLLBACK;')
    run(a,'SELECT * FROM t;')
    for p in ps:
        p.stdin.write('.quit\n');p.stdin.close();print('normal EOF exit',p.wait())
