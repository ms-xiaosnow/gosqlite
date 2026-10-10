package sqlite

import (
 "context"
 "errors"
 "testing"
)

// This focused strict repair expectation is a deliberate baseline negative.
func TestReviewPreparationRequiredNegative(t *testing.T){
 ctx:=context.Background()
 db,path:=reviewTransactionDB(t)
 reviewTransactionExec(t,db,"CREATE TABLE t(id INTEGER PRIMARY KEY,v)")
 reviewTransactionExec(t,db,"INSERT INTO t VALUES(1,11),(2,12)")
 peer,err:=Open(path,Options{});if err!=nil{t.Fatal(err)};defer peer.Close()
 reviewTransactionExec(t,peer,"BEGIN EXCLUSIVE")
 _,err=db.Exec(ctx,"UPDATE t SET missing=1")
 if err==nil||errors.Is(err,ErrBusy){t.Fatalf("required cached preparation error, got %v",err)}
 if db.counters.Changes!=2{t.Fatal(db.counters)}
 _,err=db.Exec(ctx,"UPDATE t SET v=v")
 if !errors.Is(err,ErrBusy)||db.counters.Changes!=0{t.Fatalf("required valid execution BUSY changes=0, got %v %#v",err,db.counters)}
 reviewTransactionExec(t,peer,"ROLLBACK")
}
