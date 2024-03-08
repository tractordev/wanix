
build:
	npm install
	esbuild jazz.ts --bundle --outfile=jazz.min.js --format=esm --minify
	echo "var window = globalThis; var localStorage = {};" | cat - jazz.min.js > ../../kernel/jazz/jazz.min.js