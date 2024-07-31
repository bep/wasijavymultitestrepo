javy compile js/greet.bundle.js -d -o wasm/greet.wasm
javy compile js/renderkatex.bundle.js -d -o wasm/renderkatex.wasm
javy emit-provider -o wasm/quickjs.wasm
touch warpc_test.go