-- literal <~> and ARRAY[9,9,9] are comment bytes.
CREATE TABLE public.documents(id INTEGER PRIMARY KEY, flags INTEGER NOT NULL, embedding REAL[]);
INSERT INTO public.documents(id,flags,embedding) VALUES (1,1,ARRAY[1,0,0]),(2,1,'{0,1,0}'::real[]),(3,1,ARRAY[1,2,3]),(4,1,NULL);
UPDATE public.documents SET embedding='{3,1,0}'::real[] WHERE id=2;
CREATE INDEX documents_manhattan ON public.documents USING hnsw(embedding legacy.ann_manhattan_ops) WITH(dims=3,m=16,efconstruction=64,efsearch=64);
SELECT id, embedding <~> ARRAY[1,2,3] AS distance, 'literal <~> ARRAY[9,9,9]' AS note FROM public.documents WHERE flags & 3 = 1 ORDER BY embedding <~> ARRAY[1,2,3], id LIMIT 10;
