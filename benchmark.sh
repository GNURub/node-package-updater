#!/bin/bash
cd test
hyperfine --warmup 3 --runs 10 --export-markdown ../benchmarks.md "npu -x" "ncu -u"
cd ..