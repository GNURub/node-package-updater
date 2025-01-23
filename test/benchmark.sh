#!/bin/bash
hyperfine  --prepare 'sh ./prepare.sh' --warmup 3 --runs 10 --export-markdown ../benchmarks.md "npu -x -n" "ncu -u"