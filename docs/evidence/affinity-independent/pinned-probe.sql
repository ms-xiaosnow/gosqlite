SELECT sqlite_version(),sqlite_source_id();
CREATE TABLE f(t TEXT,b BLOB,u,n NUMERIC);
INSERT INTO f VALUES(1.0,1.0,1.0,1.0);
.parameter init
.parameter set ?1 "1.0"
SELECT 'physical',t=b,b=t,t=u,u=t,t=n,n=t,typeof(t),typeof(b),typeof(u) FROM f;
SELECT 'paren',((t))=((b)),((t))=((u)),t=+b,+b=t,t=?1,?1=t,typeof(?1) FROM f;
SELECT 'forms',t IN(b),CASE t WHEN b THEN 1 ELSE 0 END,t BETWEEN b AND b,t BETWEEN +b AND +b,CAST(b AS TEXT) FROM f;
EXPLAIN SELECT t=b,t=u,t=+b,t=?1 FROM f;
