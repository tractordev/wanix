import { 
    WanixHandle
} from "./lib.js";

self.onmessage = async (e) => {
    if (!e.data.worker) return;

    console.log("gojs worker started");
    const fs = new WanixHandle(e.data.worker.sys);
    globalThis.sys = fs;
    const pid = e.data.worker.env.pid;
    const env = (await fs.readText(`task/${pid}/env`)).trim().split("\n");
    const args = (await fs.readText(`task/${pid}/cmd`)).trim().split(" ");
    globalThis.cwd = (await fs.readText(`task/${pid}/dir`)).trim();
    globalThis.stdin = await fs.open(`task/${pid}/.sys/fd0`);
    globalThis.stdout = await fs.open(`task/${pid}/.sys/fd1`);
    globalThis.stderr = await fs.open(`task/${pid}/.sys/fd2`);
    const bin = await fs.readFile(args[0]); 

    const go = new Go();
    go.argv = args || [];
    go.env = Object.fromEntries(env.map(line => {
        const idx = line.indexOf("=");
        if (idx === -1) return [line, ""];
        return [line.slice(0, idx), line.slice(idx + 1)];
    }));
    go.exit = async (code) => {
        await fs.writeFile(`task/${pid}/exit`, code.toString());
    };
    const result = await WebAssembly.instantiate(bin, go.importObject);
    const start = performance.now();
    await go.run(result.instance);
    const end = performance.now();
    console.log(`gojs execution took ${end - start}ms`);
}

function log(...args) {
    // console.log(...args);
}


function errback(cb, e) {
    const err = new Error(e);
    if (e.includes("does not exist")) {
        err.code = "ENOENT";
    }
    if (e.includes("permission denied")) {
        err.code = "EPERM";
        console.warn(err);
    }
    if (e.includes("not a directory")) {
        err.code = "ENOTDIR";
    }
    if (e.includes("file already exists")) {
        err.code = "EEXIST";
    }
    if (e.includes("invalid argument")) {
        err.code = "EINVAL";
    }
    if (!err.code) {
        console.warn(err);
    }
    cb(err);
}

// todo: support .. and ~
function cleanpath(path) {
    // console.log("cleanpath", path);
    if (path.startsWith("./")) {
        path = path.slice(2);
    }
    if (path === ".") {
        path = "";
    }
    if (!path.startsWith("/")) {
        path = [globalThis.cwd, path].join("/");
    }
    path = path.replace(/\/+/g, '/'); // collapse multiple slashes
    path = path.replace(/^\/+/, ''); // remove leading slash
    path = "vm/1/fsys/"+path;
    path = path.replace(/\/+$/, ''); // remove trailing slash(es)
    return path;
}

// below is based on wasm_exec.js from go 1.25.0

// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

"use strict";

(() => {
	const enosys = () => {
		const err = new Error("not implemented");
		err.code = "ENOSYS";
		return err;
	};

	if (!globalThis.fs) {
		// let outputBuf = "";
		globalThis.fs = {
			constants: { O_WRONLY: 1, O_RDWR: 2, O_CREAT: 64, O_TRUNC: 512, O_APPEND: 1024, O_EXCL: 128, O_DIRECTORY: 0 }, 
			// writeSync(fd, buf) {
			// 	outputBuf += decoder.decode(buf);
			// 	const nl = outputBuf.lastIndexOf("\n");
			// 	if (nl != -1) {
			// 		console.log(outputBuf.substring(0, nl));
			// 		outputBuf = outputBuf.substring(nl + 1);
			// 	}
			// 	return buf.length;
			// },
			async write(fd, buf, offset, length, position, callback) {
                log("write", fd, buf.length, offset, length, position);
				if (offset !== 0 || length !== buf.length) {
					callback(enosys());
					return;
				}
                try {
                    if (fd === 1) {
                        fd = stdout;
                    } else if (fd === 2) {
                        fd = stderr;
                    }
                    if (position !== null) {
                        callback(null, await sys.writeAt(fd, buf, position));
                        return;
                    }
                    callback(null, await sys.write(fd, buf));
                } catch (e) {
                    errback(callback, e);
                }
			},
			async chmod(path, mode, callback) {
                path = cleanpath(path);
                log("chmod", path, mode);
                try {
                    await sys.chmod(path, mode);
                    callback(null);
                } catch (e) {
                    errback(callback, e);
                }
            },
			async chown(path, uid, gid, callback) {
                path = cleanpath(path);
                log("chown", path, uid, gid);
                try {
                    await sys.chown(path, uid, gid);
                    callback(null);
                } catch (e) {
                    errback(callback, e);
                }
            },
			async close(fd, callback) { 
                log("close", fd);
                try {
                    await sys.close(fd);
                    callback(null);
                } catch (e) {
                    errback(callback, e);
                }
            },
			async fchmod(fd, mode, callback) {
                log("fchmod", fd, mode);
                try {
                    await sys.fchmod(fd, mode);
                    callback(null);
                } catch (e) {
                    errback(callback, e);
                }
            },
			async fchown(fd, uid, gid, callback) {
                log("fchown", fd, uid, gid);
                try {
                    await sys.fchown(fd, uid, gid);
                    callback(null);
                } catch (e) {
                    errback(callback, e);
                }
            },
			async fstat(fd, callback) { 
                log("fstat", fd);
                try {
                    if (fd === 1) {
                        fd = stdout;
                    } else if (fd === 2) {
                        fd = stderr;
                    }
                    const stat = await sys.fstat(fd);
                    callback(null, {
                        "mode":    stat.Mode,
                        "dev":     0,
                        "ino":     0,
                        "nlink":   0,
                        "uid":     0,
                        "gid":     0,
                        "rdev":    0,
                        "size":    stat.Size,
                        "blksize": 0,
                        "blocks":  0,
                        "atimeMs": 0,
                        "mtimeMs": stat.ModTime * 1000,
                        "ctimeMs": 0,
                        "isDirectory": () => stat.IsDir,
                    });
                } catch (e) {
                    errback(callback, e);
                }
            },
			fsync(fd, callback) { callback(null); },
			async ftruncate(fd, length, callback) {
                log("ftruncate", fd, length);
                try {
                    await sys.ftruncate(fd, length);
                    callback(null);
                } catch (e) {
                    errback(callback, e);
                }
            },
			lchown(path, uid, gid, callback) { callback(enosys()); },
			link(path, link, callback) { callback(enosys()); },
			async lstat(path, callback) {
                path = cleanpath(path);
                log("lstat", path);
                try {
                    const stat = await sys.lstat(path);
                    callback(null, {
                        "mode":    stat.Mode,
                        "dev":     0,
                        "ino":     0,
                        "nlink":   0,
                        "uid":     0,
                        "gid":     0,
                        "rdev":    0,
                        "size":    stat.Size,
                        "blksize": 0,
                        "blocks":  0,
                        "atimeMs": 0,
                        "mtimeMs": stat.ModTime * 1000,
                        "ctimeMs": 0,
                        "isDirectory": () => stat.IsDir,
                    });
                } catch (e) {
                    errback(callback, e);
                }
            },
			async mkdir(path, perm, callback) {
                path = cleanpath(path);
                log("mkdir", path, perm);
                try {
                    await sys.makeDir(path);
                    callback(null);
                } catch (e) {
                    errback(callback, e);
                }
            },
			async open(path, flags, mode, callback) {
                path = cleanpath(path);
                log("open", path, flags, mode);
                try {
                    callback(null, await sys.openFile(path, flags, mode));
                } catch (e) {
                    errback(callback, e);
                }
            },
			async read(fd, buffer, offset, length, position, callback) { 
                log("read", fd, buffer.length, offset, length, position);
                try {   
                    if (fd === 0) {
                        fd = stdin;
                    }
                    const buf = await sys.read(fd, length);
                    if (buf === null) {
                        callback(null, 0);
                        return;
                    }
                    buffer.set(buf);
                    callback(null, buf.length);
                } catch (e) {
                    errback(callback, e);
                }
            },
			async readdir(path, callback) { 
                path = cleanpath(path);
                log("readdir", path);
                try {
                    const entries = await sys.readDir(path);
                    callback(null, entries.map(e => e.replace("/", "")));
                } catch (e) {
                    errback(callback, e);
                }
            },
			async readlink(path, callback) {
                path = cleanpath(path);
                log("readlink", path);
                try {
                    const target = await sys.readlink(path);
                    callback(null, target);
                } catch (e) {
                    errback(callback, e);
                }
            },
			async rename(from, to, callback) {
                from = cleanpath(from);
                to = cleanpath(to);
                log("rename", from, to);
                try {
                    await sys.rename(from, to);
                    callback(null);
                } catch (e) {
                    errback(callback, e);
                }
            },
			async rmdir(path, callback) {
                path = cleanpath(path);
                log("rmdir", path);
                try {
                    await sys.remove(path);
                    callback(null);
                } catch (e) {
                    errback(callback, e);
                }
            },
			async stat(path, callback) {
                path = cleanpath(path);
                log("stat", path);
                try {
                    const stat = await sys.stat(path);
                    callback(null, {
                        "mode":    stat.Mode,
                        "dev":     0,
                        "ino":     0,
                        "nlink":   0,
                        "uid":     0,
                        "gid":     0,
                        "rdev":    0,
                        "size":    stat.Size,
                        "blksize": 0,
                        "blocks":  0,
                        "atimeMs": 0,
                        "mtimeMs": stat.ModTime * 1000,
                        "ctimeMs": 0,
                        "isDirectory": () => stat.IsDir,
                    });
                } catch (e) {
                    errback(callback, e);
                }
            },
			async symlink(path, link, callback) {
                path = cleanpath(path);
                link = cleanpath(link);
                log("symlink", path, link);
                try {
                    await sys.symlink(path, link);
                    callback(null);
                } catch (e) {
                    errback(callback, e);
                }
            },
			async truncate(path, length, callback) {
                path = cleanpath(path);
                log("truncate", path, length);
                try {
                    await sys.truncate(path, length);
                    callback(null);
                } catch (e) {
                    errback(callback, e);
                }
            },
			async unlink(path, callback) {
                path = cleanpath(path);
                log("unlink", path);
                try {
                    await sys.removeAll(path);
                    callback(null);
                } catch (e) {
                    errback(callback, e);
                }
            },
			async utimes(path, atime, mtime, callback) {
                path = cleanpath(path);
                log("utimes", path, atime, mtime);
                try {
                    await sys.chtimes(path, atime, mtime);
                    callback(null);
                } catch (e) {
                    errback(callback, e);
                }
            },
		};
	}

	if (!globalThis.process) {
		globalThis.process = {
			getuid() { return -1; },
			getgid() { return -1; },
			geteuid() { return -1; },
			getegid() { return -1; },
			getgroups() { throw enosys(); },
			pid: -1,
			ppid: -1,
			umask() { throw enosys(); },
			cwd() { return globalThis.cwd; },
			chdir(dir) {
                globalThis.cwd = dir;
            },
		}
	}

	if (!globalThis.path) {
		globalThis.path = {
			resolve(...pathSegments) {
				return cleanpath(pathSegments.join("/"));
			}
		}
	}

	if (!globalThis.crypto) {
		throw new Error("globalThis.crypto is not available, polyfill required (crypto.getRandomValues only)");
	}

	if (!globalThis.performance) {
		throw new Error("globalThis.performance is not available, polyfill required (performance.now only)");
	}

	if (!globalThis.TextEncoder) {
		throw new Error("globalThis.TextEncoder is not available, polyfill required");
	}

	if (!globalThis.TextDecoder) {
		throw new Error("globalThis.TextDecoder is not available, polyfill required");
	}

	const encoder = new TextEncoder("utf-8");
	const decoder = new TextDecoder("utf-8");

	globalThis.Go = class {
		constructor() {
			this.argv = ["js"];
			this.env = {};
			this.exit = (code) => {
				if (code !== 0) {
					console.warn("exit code:", code);
				}
			};
			this._exitPromise = new Promise((resolve) => {
				this._resolveExitPromise = resolve;
			});
			this._pendingEvent = null;
			this._scheduledTimeouts = new Map();
			this._nextCallbackTimeoutID = 1;

			const setInt64 = (addr, v) => {
				this.mem.setUint32(addr + 0, v, true);
				this.mem.setUint32(addr + 4, Math.floor(v / 4294967296), true);
			}

			const setInt32 = (addr, v) => {
				this.mem.setUint32(addr + 0, v, true);
			}

			const getInt64 = (addr) => {
				const low = this.mem.getUint32(addr + 0, true);
				const high = this.mem.getInt32(addr + 4, true);
				return low + high * 4294967296;
			}

			const loadValue = (addr) => {
				const f = this.mem.getFloat64(addr, true);
				if (f === 0) {
					return undefined;
				}
				if (!isNaN(f)) {
					return f;
				}

				const id = this.mem.getUint32(addr, true);
				return this._values[id];
			}

			const storeValue = (addr, v) => {
				const nanHead = 0x7FF80000;

				if (typeof v === "number" && v !== 0) {
					if (isNaN(v)) {
						this.mem.setUint32(addr + 4, nanHead, true);
						this.mem.setUint32(addr, 0, true);
						return;
					}
					this.mem.setFloat64(addr, v, true);
					return;
				}

				if (v === undefined) {
					this.mem.setFloat64(addr, 0, true);
					return;
				}

				let id = this._ids.get(v);
				if (id === undefined) {
					id = this._idPool.pop();
					if (id === undefined) {
						id = this._values.length;
					}
					this._values[id] = v;
					this._goRefCounts[id] = 0;
					this._ids.set(v, id);
				}
				this._goRefCounts[id]++;
				let typeFlag = 0;
				switch (typeof v) {
					case "object":
						if (v !== null) {
							typeFlag = 1;
						}
						break;
					case "string":
						typeFlag = 2;
						break;
					case "symbol":
						typeFlag = 3;
						break;
					case "function":
						typeFlag = 4;
						break;
				}
				this.mem.setUint32(addr + 4, nanHead | typeFlag, true);
				this.mem.setUint32(addr, id, true);
			}

			const loadSlice = (addr) => {
				const array = getInt64(addr + 0);
				const len = getInt64(addr + 8);
				return new Uint8Array(this._inst.exports.mem.buffer, array, len);
			}

			const loadSliceOfValues = (addr) => {
				const array = getInt64(addr + 0);
				const len = getInt64(addr + 8);
				const a = new Array(len);
				for (let i = 0; i < len; i++) {
					a[i] = loadValue(array + i * 8);
				}
				return a;
			}

			const loadString = (addr) => {
				const saddr = getInt64(addr + 0);
				const len = getInt64(addr + 8);
				return decoder.decode(new DataView(this._inst.exports.mem.buffer, saddr, len));
			}

			const testCallExport = (a, b) => {
				this._inst.exports.testExport0();
				return this._inst.exports.testExport(a, b);
			}

			const timeOrigin = Date.now() - performance.now();
			this.importObject = {
				_gotest: {
					add: (a, b) => a + b,
					callExport: testCallExport,
				},
				gojs: {
					// Go's SP does not change as long as no Go code is running. Some operations (e.g. calls, getters and setters)
					// may synchronously trigger a Go event handler. This makes Go code get executed in the middle of the imported
					// function. A goroutine can switch to a new stack if the current stack is too small (see morestack function).
					// This changes the SP, thus we have to update the SP used by the imported function.

					// func wasmExit(code int32)
					"runtime.wasmExit": (sp) => {
						sp >>>= 0;
						const code = this.mem.getInt32(sp + 8, true);
						this.exited = true;
						delete this._inst;
						delete this._values;
						delete this._goRefCounts;
						delete this._ids;
						delete this._idPool;
						this.exit(code);
					},

					// func wasmWrite(fd uintptr, p unsafe.Pointer, n int32)
					"runtime.wasmWrite": (sp) => {
						sp >>>= 0;
						let fd = getInt64(sp + 8);
						if (fd === 1) {
							fd = stdout;
						} else if (fd === 2) {
							fd = stderr;
						}
						const p = getInt64(sp + 16);
						const n = this.mem.getInt32(sp + 24, true);
						sys.write(fd, new Uint8Array(this._inst.exports.mem.buffer, p, n));
					},

					// func resetMemoryDataView()
					"runtime.resetMemoryDataView": (sp) => {
						sp >>>= 0;
						this.mem = new DataView(this._inst.exports.mem.buffer);
					},

					// func nanotime1() int64
					"runtime.nanotime1": (sp) => {
						sp >>>= 0;
						setInt64(sp + 8, (timeOrigin + performance.now()) * 1000000);
					},

					// func walltime() (sec int64, nsec int32)
					"runtime.walltime": (sp) => {
						sp >>>= 0;
						const msec = (new Date).getTime();
						setInt64(sp + 8, msec / 1000);
						this.mem.setInt32(sp + 16, (msec % 1000) * 1000000, true);
					},

					// func scheduleTimeoutEvent(delay int64) int32
					"runtime.scheduleTimeoutEvent": (sp) => {
						sp >>>= 0;
						const id = this._nextCallbackTimeoutID;
						this._nextCallbackTimeoutID++;
						this._scheduledTimeouts.set(id, setTimeout(
							() => {
								this._resume();
								while (this._scheduledTimeouts.has(id)) {
									// for some reason Go failed to register the timeout event, log and try again
									// (temporary workaround for https://github.com/golang/go/issues/28975)
									console.warn("scheduleTimeoutEvent: missed timeout event");
									this._resume();
								}
							},
							getInt64(sp + 8),
						));
						this.mem.setInt32(sp + 16, id, true);
					},

					// func clearTimeoutEvent(id int32)
					"runtime.clearTimeoutEvent": (sp) => {
						sp >>>= 0;
						const id = this.mem.getInt32(sp + 8, true);
						clearTimeout(this._scheduledTimeouts.get(id));
						this._scheduledTimeouts.delete(id);
					},

					// func getRandomData(r []byte)
					"runtime.getRandomData": (sp) => {
						sp >>>= 0;
						crypto.getRandomValues(loadSlice(sp + 8));
					},

					// func finalizeRef(v ref)
					"syscall/js.finalizeRef": (sp) => {
						sp >>>= 0;
						const id = this.mem.getUint32(sp + 8, true);
						this._goRefCounts[id]--;
						if (this._goRefCounts[id] === 0) {
							const v = this._values[id];
							this._values[id] = null;
							this._ids.delete(v);
							this._idPool.push(id);
						}
					},

					// func stringVal(value string) ref
					"syscall/js.stringVal": (sp) => {
						sp >>>= 0;
						storeValue(sp + 24, loadString(sp + 8));
					},

					// func valueGet(v ref, p string) ref
					"syscall/js.valueGet": (sp) => {
						sp >>>= 0;
						const result = Reflect.get(loadValue(sp + 8), loadString(sp + 16));
						sp = this._inst.exports.getsp() >>> 0; // see comment above
						storeValue(sp + 32, result);
					},

					// func valueSet(v ref, p string, x ref)
					"syscall/js.valueSet": (sp) => {
						sp >>>= 0;
						Reflect.set(loadValue(sp + 8), loadString(sp + 16), loadValue(sp + 32));
					},

					// func valueDelete(v ref, p string)
					"syscall/js.valueDelete": (sp) => {
						sp >>>= 0;
						Reflect.deleteProperty(loadValue(sp + 8), loadString(sp + 16));
					},

					// func valueIndex(v ref, i int) ref
					"syscall/js.valueIndex": (sp) => {
						sp >>>= 0;
						storeValue(sp + 24, Reflect.get(loadValue(sp + 8), getInt64(sp + 16)));
					},

					// valueSetIndex(v ref, i int, x ref)
					"syscall/js.valueSetIndex": (sp) => {
						sp >>>= 0;
						Reflect.set(loadValue(sp + 8), getInt64(sp + 16), loadValue(sp + 24));
					},

					// func valueCall(v ref, m string, args []ref) (ref, bool)
					"syscall/js.valueCall": (sp) => {
						sp >>>= 0;
						try {
							const v = loadValue(sp + 8);
							const m = Reflect.get(v, loadString(sp + 16));
							const args = loadSliceOfValues(sp + 32);
							const result = Reflect.apply(m, v, args);
							sp = this._inst.exports.getsp() >>> 0; // see comment above
							storeValue(sp + 56, result);
							this.mem.setUint8(sp + 64, 1);
						} catch (err) {
							sp = this._inst.exports.getsp() >>> 0; // see comment above
							storeValue(sp + 56, err);
							this.mem.setUint8(sp + 64, 0);
						}
					},

					// func valueInvoke(v ref, args []ref) (ref, bool)
					"syscall/js.valueInvoke": (sp) => {
						sp >>>= 0;
						try {
							const v = loadValue(sp + 8);
							const args = loadSliceOfValues(sp + 16);
							const result = Reflect.apply(v, undefined, args);
							sp = this._inst.exports.getsp() >>> 0; // see comment above
							storeValue(sp + 40, result);
							this.mem.setUint8(sp + 48, 1);
						} catch (err) {
							sp = this._inst.exports.getsp() >>> 0; // see comment above
							storeValue(sp + 40, err);
							this.mem.setUint8(sp + 48, 0);
						}
					},

					// func valueNew(v ref, args []ref) (ref, bool)
					"syscall/js.valueNew": (sp) => {
						sp >>>= 0;
						try {
							const v = loadValue(sp + 8);
							const args = loadSliceOfValues(sp + 16);
							const result = Reflect.construct(v, args);
							sp = this._inst.exports.getsp() >>> 0; // see comment above
							storeValue(sp + 40, result);
							this.mem.setUint8(sp + 48, 1);
						} catch (err) {
							sp = this._inst.exports.getsp() >>> 0; // see comment above
							storeValue(sp + 40, err);
							this.mem.setUint8(sp + 48, 0);
						}
					},

					// func valueLength(v ref) int
					"syscall/js.valueLength": (sp) => {
						sp >>>= 0;
						setInt64(sp + 16, parseInt(loadValue(sp + 8).length));
					},

					// valuePrepareString(v ref) (ref, int)
					"syscall/js.valuePrepareString": (sp) => {
						sp >>>= 0;
						const str = encoder.encode(String(loadValue(sp + 8)));
						storeValue(sp + 16, str);
						setInt64(sp + 24, str.length);
					},

					// valueLoadString(v ref, b []byte)
					"syscall/js.valueLoadString": (sp) => {
						sp >>>= 0;
						const str = loadValue(sp + 8);
						loadSlice(sp + 16).set(str);
					},

					// func valueInstanceOf(v ref, t ref) bool
					"syscall/js.valueInstanceOf": (sp) => {
						sp >>>= 0;
						this.mem.setUint8(sp + 24, (loadValue(sp + 8) instanceof loadValue(sp + 16)) ? 1 : 0);
					},

					// func copyBytesToGo(dst []byte, src ref) (int, bool)
					"syscall/js.copyBytesToGo": (sp) => {
						sp >>>= 0;
						const dst = loadSlice(sp + 8);
						const src = loadValue(sp + 32);
						if (!(src instanceof Uint8Array || src instanceof Uint8ClampedArray)) {
							this.mem.setUint8(sp + 48, 0);
							return;
						}
						const toCopy = src.subarray(0, dst.length);
						dst.set(toCopy);
						setInt64(sp + 40, toCopy.length);
						this.mem.setUint8(sp + 48, 1);
					},

					// func copyBytesToJS(dst ref, src []byte) (int, bool)
					"syscall/js.copyBytesToJS": (sp) => {
						sp >>>= 0;
						const dst = loadValue(sp + 8);
						const src = loadSlice(sp + 16);
						if (!(dst instanceof Uint8Array || dst instanceof Uint8ClampedArray)) {
							this.mem.setUint8(sp + 48, 0);
							return;
						}
						const toCopy = src.subarray(0, dst.length);
						dst.set(toCopy);
						setInt64(sp + 40, toCopy.length);
						this.mem.setUint8(sp + 48, 1);
					},

					"debug": (value) => {
						console.log(value);
					},
				}
			};
		}

		async run(instance) {
			if (!(instance instanceof WebAssembly.Instance)) {
				throw new Error("Go.run: WebAssembly.Instance expected");
			}
			this._inst = instance;
			this.mem = new DataView(this._inst.exports.mem.buffer);
			this._values = [ // JS values that Go currently has references to, indexed by reference id
				NaN,
				0,
				null,
				true,
				false,
				globalThis,
				this,
			];
			this._goRefCounts = new Array(this._values.length).fill(Infinity); // number of references that Go has to a JS value, indexed by reference id
			this._ids = new Map([ // mapping from JS values to reference ids
				[0, 1],
				[null, 2],
				[true, 3],
				[false, 4],
				[globalThis, 5],
				[this, 6],
			]);
			this._idPool = [];   // unused ids that have been garbage collected
			this.exited = false; // whether the Go program has exited

			// Pass command line arguments and environment variables to WebAssembly by writing them to the linear memory.
			let offset = 4096;

			const strPtr = (str) => {
				const ptr = offset;
				const bytes = encoder.encode(str + "\0");
				new Uint8Array(this.mem.buffer, offset, bytes.length).set(bytes);
				offset += bytes.length;
				if (offset % 8 !== 0) {
					offset += 8 - (offset % 8);
				}
				return ptr;
			};

			const argc = this.argv.length;

			const argvPtrs = [];
			this.argv.forEach((arg) => {
				argvPtrs.push(strPtr(arg));
			});
			argvPtrs.push(0);

			const keys = Object.keys(this.env).sort();
			keys.forEach((key) => {
				argvPtrs.push(strPtr(`${key}=${this.env[key]}`));
			});
			argvPtrs.push(0);

			const argv = offset;
			argvPtrs.forEach((ptr) => {
				this.mem.setUint32(offset, ptr, true);
				this.mem.setUint32(offset + 4, 0, true);
				offset += 8;
			});

			// The linker guarantees global data starts from at least wasmMinDataAddr.
			// Keep in sync with cmd/link/internal/ld/data.go:wasmMinDataAddr.
			const wasmMinDataAddr = 4096 + 8192;
			if (offset >= wasmMinDataAddr) {
				throw new Error("total length of command line and environment variables exceeds limit");
			}

			this._inst.exports.run(argc, argv);
			if (this.exited) {
				this._resolveExitPromise();
			}
			await this._exitPromise;
		}

		_resume() {
			if (this.exited) {
				throw new Error("Go program has already exited");
			}
			this._inst.exports.resume();
			if (this.exited) {
				this._resolveExitPromise();
			}
		}

		_makeFuncWrapper(id) {
			const go = this;
			return function () {
				const event = { id: id, this: this, args: arguments };
				go._pendingEvent = event;
				go._resume();
				return event.result;
			};
		}
	}
})();
