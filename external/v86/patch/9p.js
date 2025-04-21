// -------------------------------------------------
// --------------------- 9P ------------------------
// -------------------------------------------------
// Implementation of the 9p filesystem device following the
// 9P2000.L protocol ( https://code.google.com/p/diod/wiki/protocol )

import { LOG_9P } from "./../src/const.js";
import { VirtIO, VIRTIO_F_VERSION_1, VIRTIO_F_RING_EVENT_IDX, VIRTIO_F_RING_INDIRECT_DESC } from "../src/virtio.js";
import { S_IFREG, S_IFDIR, STATUS_UNLINKED } from "./filesystem.js";
import * as marshall from "../lib/marshall.js";
import { dbg_log, dbg_assert } from "../src/log.js";
import { h } from "../src/lib.js";

// For Types Only
import { CPU } from "../src/cpu.js";
import { BusConnector } from "../src/bus.js";
import { FS } from "./filesystem.js";

/**
 * @const
 * More accurate filenames in 9p debug messages at the cost of performance.
 */
const TRACK_FILENAMES = false;

// Feature bit (bit position) for mount tag.
const VIRTIO_9P_F_MOUNT_TAG = 0;
// Assumed max tag length in bytes.
const VIRTIO_9P_MAX_TAGLEN = 254;

const MAX_REPLYBUFFER_SIZE = 16 * 1024 * 1024;

// TODO
// flush

export const EPERM = 1;       /* Operation not permitted */
export const ENOENT = 2;      /* No such file or directory */
export const EEXIST = 17;      /* File exists */
export const EINVAL = 22;     /* Invalid argument */
export const EOPNOTSUPP = 95;  /* Operation is not supported */
export const ENOTEMPTY = 39;  /* Directory not empty */
export const EPROTO    = 71;  /* Protocol error */

var P9_SETATTR_MODE = 0x00000001;
var P9_SETATTR_UID = 0x00000002;
var P9_SETATTR_GID = 0x00000004;
var P9_SETATTR_SIZE = 0x00000008;
var P9_SETATTR_ATIME = 0x00000010;
var P9_SETATTR_MTIME = 0x00000020;
var P9_SETATTR_CTIME = 0x00000040;
var P9_SETATTR_ATIME_SET = 0x00000080;
var P9_SETATTR_MTIME_SET = 0x00000100;

var P9_STAT_MODE_DIR = 0x80000000;
var P9_STAT_MODE_APPEND = 0x40000000;
var P9_STAT_MODE_EXCL = 0x20000000;
var P9_STAT_MODE_MOUNT = 0x10000000;
var P9_STAT_MODE_AUTH = 0x08000000;
var P9_STAT_MODE_TMP = 0x04000000;
var P9_STAT_MODE_SYMLINK = 0x02000000;
var P9_STAT_MODE_LINK = 0x01000000;
var P9_STAT_MODE_DEVICE = 0x00800000;
var P9_STAT_MODE_NAMED_PIPE = 0x00200000;
var P9_STAT_MODE_SOCKET = 0x00100000;
var P9_STAT_MODE_SETUID = 0x00080000;
var P9_STAT_MODE_SETGID = 0x00040000;
var P9_STAT_MODE_SETVTX = 0x00010000;

export const P9_LOCK_TYPE_RDLCK = 0;
export const P9_LOCK_TYPE_WRLCK = 1;
export const P9_LOCK_TYPE_UNLCK = 2;
const P9_LOCK_TYPES = ["shared", "exclusive", "unlock"];

const P9_LOCK_FLAGS_BLOCK = 1;
const P9_LOCK_FLAGS_RECLAIM = 2;

export const P9_LOCK_SUCCESS = 0;
export const P9_LOCK_BLOCKED = 1;
export const P9_LOCK_ERROR = 2;
export const P9_LOCK_GRACE = 3;

var FID_NONE = -1;
var FID_INODE = 1;
var FID_XATTR = 2;

function range(size)
{
    return Array.from(Array(size).keys());
}

/**
 * @constructor
 *
 * @param {FS} filesystem
 * @param {CPU} cpu
 */
export function Virtio9p(filesystem, cpu, bus) {
    this.tagBufchain = new Map();
    
    /** @type {FS} */
    this.fs = filesystem;

    /** @const @type {BusConnector} */
    this.bus = bus;

    //this.configspace = [0x0, 0x4, 0x68, 0x6F, 0x73, 0x74]; // length of string and "host" string
    //this.configspace = [0x0, 0x9, 0x2F, 0x64, 0x65, 0x76, 0x2F, 0x72, 0x6F, 0x6F, 0x74 ]; // length of string and "/dev/root" string
    this.configspace_tagname = [0x68, 0x6F, 0x73, 0x74, 0x39, 0x70]; // "host9p" string
    this.configspace_taglen = this.configspace_tagname.length; // num bytes
    this.VERSION = "9P2000.L";
    this.BLOCKSIZE = 8192; // Let's define one page.
    this.msize = 8192; // maximum message size
    this.replybuffer = new Uint8Array(this.msize*2); // Twice the msize to stay on the safe site
    this.replybuffersize = 0;

    this.fids = [];

    /** @type {VirtIO} */
    this.virtio = new VirtIO(cpu,
    {
        name: "virtio-9p",
        pci_id: 0x06 << 3,
        device_id: 0x1049,
        subsystem_device_id: 9,
        common:
        {
            initial_port: 0xA800,
            queues:
            [
                {
                    size_supported: 32,
                    notify_offset: 0,
                },
            ],
            features:
            [
                VIRTIO_9P_F_MOUNT_TAG,
                VIRTIO_F_VERSION_1,
                VIRTIO_F_RING_EVENT_IDX,
                VIRTIO_F_RING_INDIRECT_DESC,
            ],
            on_driver_ok: () => {},
        },
        notification:
        {
            initial_port: 0xA900,
            single_handler: false,
            handlers:
            [
                (queue_id) =>
                {
                    if(queue_id !== 0)
                    {
                        dbg_assert(false, "Virtio9P Notified for non-existent queue: " + queue_id +
                            " (expected queue_id of 0)");
                        return;
                    }
                    while(this.virtqueue.has_request())
                    {
                        const bufchain = this.virtqueue.pop_request();
                        this.ReceiveRequest(bufchain);
                    }
                    this.virtqueue.notify_me_after(0);
                    // Don't flush replies here: async replies are not completed yet.
                },
            ],
        },
        isr_status:
        {
            initial_port: 0xA700,
        },
        device_specific:
        {
            initial_port: 0xA600,
            struct:
            [
                {
                    bytes: 2,
                    name: "mount tag length",
                    read: () => this.configspace_taglen,
                    write: data => { /* read only */ },
                },
            ].concat(range(VIRTIO_9P_MAX_TAGLEN).map(index =>
                ({
                    bytes: 1,
                    name: "mount tag name " + index,
                    // Note: configspace_tagname may have changed after set_state
                    read: () => this.configspace_tagname[index] || 0,
                    write: data => { /* read only */ },
                })
            )),
        },
    });
    this.virtqueue = this.virtio.queues[0];
}

Virtio9p.prototype.get_state = function()
{
    var state = [];

    state[0] = this.configspace_tagname;
    state[1] = this.configspace_taglen;
    state[2] = this.virtio;
    state[3] = this.VERSION;
    state[4] = this.BLOCKSIZE;
    state[5] = this.msize;
    state[6] = this.replybuffer;
    state[7] = this.replybuffersize;
    state[8] = this.fids.map(function(f) { return [f.inodeid, f.type, f.uid, f.dbg_name]; });
    state[9] = this.fs;

    return state;
};

Virtio9p.prototype.set_state = function(state)
{
    this.configspace_tagname = state[0];
    this.configspace_taglen = state[1];
    this.virtio.set_state(state[2]);
    this.virtqueue = this.virtio.queues[0];
    this.VERSION = state[3];
    this.BLOCKSIZE = state[4];
    this.msize = state[5];
    this.replybuffer = state[6];
    this.replybuffersize = state[7];
    this.fids = state[8].map(function(f)
    {
        return { inodeid: f[0], type: f[1], uid: f[2], dbg_name: f[3] };
    });
    this.fs.set_state(state[9]);
};

// Note: dbg_name is only used for debugging messages and may not be the same as the filename,
// since it is not synchronised with renames done outside of 9p. Hard-links, linking and unlinking
// operations also mean that having a single filename no longer makes sense.
// Set TRACK_FILENAMES = true to sync dbg_name during 9p renames.
Virtio9p.prototype.Createfid = function(inodeid, type, uid, dbg_name) {
    return {inodeid, type, uid, dbg_name};
};

Virtio9p.prototype.update_dbg_name = function(idx, newname)
{
    for(const fid of this.fids)
    {
        if(fid.inodeid === idx) fid.dbg_name = newname;
    }
};

Virtio9p.prototype.reset = function() {
    this.fids = [];
    this.virtio.reset();
};


Virtio9p.prototype.BuildReply = function(id, tag, payloadsize) {
    dbg_assert(payloadsize >= 0, "9P: Negative payload size");
    marshall.Marshall(["w", "b", "h"], [payloadsize+7, id+1, tag], this.replybuffer, 0);
    if((payloadsize+7) >= this.replybuffer.length) {
        dbg_log("Error in 9p: payloadsize exceeds maximum length", LOG_9P);
    }
    //for(var i=0; i<payload.length; i++)
    //    this.replybuffer[7+i] = payload[i];
    this.replybuffersize = payloadsize+7;
};

Virtio9p.prototype.SendError = function (tag, errormsg, errorcode) {
    //var size = marshall.Marshall(["s", "w"], [errormsg, errorcode], this.replybuffer, 7);
    var size = marshall.Marshall(["w"], [errorcode], this.replybuffer, 7);
    this.BuildReply(6, tag, size);
};

Virtio9p.prototype.SendReply = function (bufchain) {
    dbg_assert(this.replybuffersize >= 0, "9P: Negative replybuffersize");
    bufchain.set_next_blob(this.replybuffer.subarray(0, this.replybuffersize));
    this.virtqueue.push_reply(bufchain);
    this.virtqueue.flush_replies();
};

Virtio9p.prototype.ReceiveRequest = async function (bufchain) {
    // TODO: split into header + data blobs to avoid unnecessary copying.
    const buffer = new Uint8Array(bufchain.length_readable);
    bufchain.get_next_blob(buffer);

    const state = { offset : 0 };
    var header = marshall.Unmarshall(["w", "b", "h"], buffer, state);
    // var size = header[0];
    // var id = header[1];
    var tag = header[2];
    //message.Debug("size:" + size + " id:" + id + " tag:" + tag);

    this.tagBufchain.set(tag, bufchain);

    if (window.wanix) {
        window.wanix.virtioHandle(buffer, (buffer) => {
            var replyState = { offset: 0 };
            var replyHeader = marshall.Unmarshall(["w", "b", "h"], buffer, replyState);
            var replySize = replyHeader[0];
            var replyId = replyHeader[1];
            var replyTag = replyHeader[2];

            // Create a new buffer for each response instead of reusing the same one
            this.replybuffer = new Uint8Array(buffer.byteLength);
            this.replybuffer.set(buffer);
            this.replybuffersize = buffer.byteLength;

            const bufchain = this.tagBufchain.get(replyTag);
            if (!bufchain) {
                console.error("No bufchain found for tag: " + replyTag);
                return;
            }

            bufchain.set_next_blob(buffer);
            this.virtqueue.push_reply(bufchain);
            this.virtqueue.flush_replies();

            this.tagBufchain.delete(replyTag);
        });
        return;
    }

};