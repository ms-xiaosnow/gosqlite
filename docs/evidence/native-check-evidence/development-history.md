# Retained development observations

- First build of new memory tests failed at compile time because DeleteIndex/DeleteTable return two values. Corrected assignments; no test body ran.
- First memory test run failed two new assertions: fixture-construction reads were included in the independent counter, and its absolute cancellation trigger had already been passed during fixture construction. Reset the PageReader read count only after construction; integrity assertions and budgets were unchanged. Failed output retained in memory-development-v1.txt, passing output in memory-development-v2.txt.
- First complete new native/API/official fixture run passed (normal-development-v1.txt).
- Added transaction ownership/counter and CLI/provider tests plus empty-schema-format and classified full-WAL CLI boundary. Expanded scoped run passed (normal-development-v2.txt).
- No native failure or prohibited physical scenario was executed in development.
