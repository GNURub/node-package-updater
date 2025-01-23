#!/bin/bash
hyperfine  --prepare 'sh ./prepare' --warmup 3 --runs 10 --export-markdown ../benchmarks.md "npu -dryRun" "ncu --jsonUpgraded"