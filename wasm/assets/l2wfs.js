/**
 * @constructor
 * @extends {FS}
 * @param {!FileStorageInterface} storage
 * @param {{ last_qidnumber: number }=} qidcounter Another fs's qidcounter to synchronise with.
 */
function LinuxToWanixFS(storage, qidcounter) {
    FS.call(this, storage, qidcounter);

    // let root = this.SearchPath("/");
    // if (window.wanix) {
    //     window.wanix.readDir(".").then(dir => {
    //         dir.forEach(d => {
    //             if (d.endsWith("/")) {
    //                 this.CreateDirectory(d.slice(0,-1), root.id);
    //             } else {
    //                 this.CreateFile(d, root.id);
    //             }
    //         });
    //     });
    // }
}

LinuxToWanixFS.prototype = Object.create(FS.prototype);
LinuxToWanixFS.prototype.constructor = FS;

LinuxToWanixFS.prototype.wanixOpenInode = async function(id, name) {
    if (Math.floor(Date.now() / 1000) -this.inodes[id].wtime < 1) {
        return;
    }
    const wnode = await window.wanix.openInode(name);
    if (!wnode) {
        console.warn("openInode failed:", name);
        return;
    }
    if (wnode.Error) {
        console.warn("openInode error:", name, wnode.Error);
        return;
    }
    if (!this.inodes[id].wnode) {
        if (wnode.IsDir) {
            wnode.Entries.forEach(e => {
                if (e.IsDir) {
                    this.CreateDirectory(e.Name, id);
                } else {
                    const idx = this.CreateFile(e.Name, id);
                    this.inodes[idx].size = e.Size; // || Math.pow(2, 31) - 1; // max int32 for 0 size files
                    this.inodes[idx].wpath = [name, e.Name].join("/");
                }
            }); 
        }
        this.inodes[id].wnode = wnode;
    } else if (wnode.IsDir) {
        const entries = Array.from(this.inodes[id].direntries.keys())
                            .filter(e => ![".", ".."].includes(e));
        wnode.Entries.forEach(e => {
            if (entries.includes(e.Name)) {
                return;
            }
            if (e.IsDir) {
                this.CreateDirectory(e.Name, id);
            } else {
                const idx = this.CreateFile(e.Name, id);
                this.inodes[idx].size = e.Size; // || Math.pow(2, 31) - 1; // max int32 for 0 size files
                this.inodes[idx].wpath = [name, e.Name].join("/");
            }
        });
    }
    this.inodes[id].wtime = Math.floor(Date.now() / 1000);
}

LinuxToWanixFS.prototype.OpenInodeAsync = async function(id, mode)
{
    const inode = this.inodes[id];

    // console.log("OpenInodeAsync", id, inode.wnode);   

    if (this.IsDirectory(id)) {
        await this.wanixOpenInode(id, this.GetFullPath(id) || ".");
    } else if (inode.wpath) {
        await this.wanixOpenInode(id, inode.wpath);
    }
    
    // console.log(`open ${this.GetFullPath(id)}:`, wnode)
    return this.OpenInode(id, mode);
};

LinuxToWanixFS.prototype.get_data = async function(idx, offset, count)
{
    const inode = this.inodes[idx];
    if (!inode.wnode || inode.wnode.IsDir) {
        return FS.prototype.get_data.call(this, idx, offset, count);
    }
    return await window.wanix.read(inode.wpath, offset, count);
};

LinuxToWanixFS.prototype.SearchAsync = async function(parentid, name) {
    if (name === "MAILPATH") {
        return -1;
    }
    // console.log("SearchAsync", parentid, name);
    const ret = this.Search(parentid, name);    
    if (ret == -1) {
        await this.wanixOpenInode(parentid, this.GetFullPath(parentid));
        return this.Search(parentid, name);
    }
    return ret
};

if (window) {
    window.LinuxToWanixFS = LinuxToWanixFS;
}