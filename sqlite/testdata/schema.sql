-- Oracle fixture: official SQLite 3.54.0, built from the repository-pinned source.
-- Regenerate: sqlite3 schema.db < schema.sql (destination must not exist).
PRAGMA page_size=4096;
PRAGMA journal_mode=DELETE;
CREATE TABLE shadow(rowid TEXT,oid INTEGER,_rowid_ TEXT,id INTEGER PRIMARY KEY);
INSERT INTO shadow VALUES('shadow',17,'internal',9);
CREATE TABLE shadow_one(rowid TEXT,x INTEGER);
INSERT INTO shadow_one VALUES('named',7);
CREATE TABLE defaults(id INTEGER PRIMARY KEY);
INSERT INTO defaults VALUES(3);
ALTER TABLE defaults ADD COLUMN n INTEGER DEFAULT '7';
ALTER TABLE defaults ADD COLUMN t TEXT DEFAULT 7;
ALTER TABLE defaults ADD COLUMN r REAL DEFAULT 7;
CREATE TABLE noninteger(id INT PRIMARY KEY,value TEXT);
INSERT INTO noninteger VALUES(7,'primary');
CREATE TABLE descpk(id INTEGER PRIMARY KEY DESC);
INSERT INTO descpk VALUES(8);
CREATE TABLE empty(x INTEGER);
CREATE TABLE unicode_names("İ" TEXT,"i" TEXT);
INSERT INTO unicode_names VALUES('capital dotted','ascii');
CREATE TABLE flags("TRUE" INTEGER,"FALSE" INTEGER,x INTEGER);
INSERT INTO flags VALUES(7,8,7);
PRAGMA integrity_check;
