(async () => {
    // convert \n to \r\n since we have no line discipline in this setup
    const transform = new TransformStream({
        transform(chunk, controller) {
            const text = new TextDecoder().decode(chunk);
            const converted = text.replace(/\n/g, '\r\n');
            controller.enqueue(new TextEncoder().encode(converted));
        }
    });

    console.log("running golang init.js");
    const w = window.wanix.instance;        
    const tid = (await w.readText("task/new/wasi")).trim();
    await w.writeFile(`task/${tid}/cmd`, "#bundle/golangcheck.wasm");
    const stdout = await w.openReadable(`task/${tid}/fd/1`);
    const cons = await w.openWritable(`#console/data1`);
    await w.writeFile(`task/${tid}/ctl`, "start");
    stdout.pipeThrough(transform).pipeTo(cons);
})();