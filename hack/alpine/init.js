(async () => {
    console.log("running alpine init.js");
    window.makeScreen();
    const w = window.wanix.instance;        
    const vm = (await w.readText("vm/new/default")).trim();
    await w.writeFile("task/1/ctl", `bind #console/data1 vm/${vm}/ttyS0`);
    await w.writeFile("task/1/ctl", `bind . vm/${vm}/fsys`);
    await w.writeFile("task/1/ctl", `bind #bundle/rootfs vm/${vm}/fsys`);
    await w.writeFile(`vm/${vm}/ctl`, `start -append 'init=/bin/init rw root=host9p rootfstype=9p rootflags=trans=virtio,version=9p2000.L,aname=vm/${vm}/fsys,cache=none,msize=8192,access=client ramdisk_size=102400'`);
})();
