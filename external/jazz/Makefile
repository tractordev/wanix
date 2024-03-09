
build:
	npm install
	esbuild jazz.ts --bundle --outfile=jazz.min.js --format=esm --minify
	echo "var window = globalThis; if (typeof WorkerGlobalScope !== 'undefined' && self instanceof WorkerGlobalScope) { var localStorage = {}; } else { var localStorage = globalThis.localStorage; }" | cat - jazz.min.js > ../../kernel/jazz/jazz.min.js