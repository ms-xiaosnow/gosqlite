-- literal <~> and ARRAY[9,9,9] are comment bytes.
CREATE TABLE "documents"(id INTEGER PRIMARY KEY, flags INTEGER NOT NULL, embedding VECTOR(3));
INSERT INTO "documents"(id,flags,embedding) VALUES (1,1,'[1,0,0]'),(2,1,'[0,1,0]'),(3,1,'[1,2,3]'),(4,1,NULL);
UPDATE "documents" SET embedding='[3,1,0]' WHERE (id=2);
CREATE INDEX "documents_manhattan" ON "documents"("embedding") USING hnsw WITH(metric='l1',opclass='vector_l1_ops',m=16,ef_construction=64)        ;
SELECT id, (embedding <+> '[1,2,3]') AS distance, 'literal <~> ARRAY[9,9,9]' AS note FROM "documents" WHERE ((flags & 3) = 1) ORDER BY ((embedding <+> '[1,2,3]')) IS NULL ASC, (embedding <+> '[1,2,3]'), id LIMIT 10;
