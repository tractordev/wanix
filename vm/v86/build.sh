#!/bin/bash
esbuild v86.worker.js --bundle --outfile=v86.worker.min.js --external:perf_hooks --external:node:fs/promises --external:node:crypto --external:crypto --minify