export default async (w) => {
    console.log("running alpine init.js");
    window.makeScreen();
    const vm = (await w.readText("vm/new/default")).trim();
    await w.writeFile("task/1/ctl", `bind #console/data1 vm/${vm}/ttyS0`);
    await w.writeFile("task/1/ctl", `bind . vm/${vm}/fsys`);
    await w.writeFile("task/1/ctl", `bind #bundle/rootfs vm/${vm}/fsys`);
    const cmdline = [
        "init=/bin/init",
        "rw",
        "root=host9p",
        "rootfstype=9p",
        `rootflags=trans=virtio,version=9p2000.L,aname=vm/${vm}/fsys,cache=loose`, //msize=524288
    ];
    const ctlcmd = ["start", "-m", "1024M", "-append", `'${cmdline.join(" ")}'`];
    if (w.config.network) {
        ctlcmd.push("-netdev");
        ctlcmd.push(`user,type=virtio,relay_url=${w.config.network}`);
    }
    await w.writeFile(`vm/${vm}/ctl`, ctlcmd.join(" "));
}
