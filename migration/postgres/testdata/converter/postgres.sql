-- literal <~> and ARRAY[9,9,9] are comment bytes.
CREATE TABLE "public"."documents"(id INTEGER PRIMARY KEY, flags INTEGER NOT NULL, embedding "public".vector(3));
INSERT INTO "public"."documents"(id,flags,embedding) VALUES (1,1,'[1,0,0]'::"public".vector(3)),(2,1,'[0,1,0]'::"public".vector(3)),(3,1,'[1,2,3]'::"public".vector(3)),(4,1,NULL);
UPDATE "public"."documents" SET embedding='[3,1,0]'::"public".vector(3) WHERE (id=2);
CREATE INDEX "documents_manhattan" ON "public"."documents" USING hnsw ("embedding" "public"."vector_l1_ops") WITH (m=16,ef_construction=64)        ;
SELECT id, (embedding OPERATOR("public".<+>) '[1,2,3]'::"public".vector(3)) AS distance, 'literal <~> ARRAY[9,9,9]' AS note FROM "public"."documents" WHERE ((flags & 3) = 1) ORDER BY (embedding OPERATOR("public".<+>) '[1,2,3]'::"public".vector(3)), id LIMIT 10;
