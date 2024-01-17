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

	const inflateStat = (stat) => {
		if (stat) {
			const isDir = stat.isDirectory;
			stat.isDirectory = () => isDir;
		}
		return stat;
	}

	const inflateErr = (err) => {
		if (!err.includes(";")) {
			return err;
		}
		const [msg, params] = err.split(";");
		const obj = params.split(",").reduce((obj, pair) => {
			const [key, value] = pair.split('=');
			obj[key.trim()] = value.trim();
			return obj;
		}, {});
		const e = new Error(msg);
		return Object.assign(e, obj);
	}

  if (!globalThis.stdin) {
    globalThis.stdin = (buf, cb) => cb(null, 0);
  }

  if (!globalThis.stdout) {
    globalThis.stdout = (buf) => globalThis.fs.writeSync(1, buf);
  }

  if (!globalThis.stderr) {
    globalThis.stderr = (buf) => globalThis.fs.writeSync(2, buf);
  }

	if (!globalThis.fs) {
		let outputBuf = "";
		globalThis.fs = {
			constants: { O_WRONLY: 1, O_RDWR: 2, O_CREAT: 64, O_TRUNC: 512, O_APPEND: 1024, O_EXCL: 128 },
			// writeFile helper for working with fs in js
			writeFile(path, buf, perm) {
				return new Promise((resolve, reject) => {
					fs.open(path, fs.constants.O_WRONLY|fs.constants.O_CREAT|fs.constants.O_TRUNC, perm, (err, fd) => {
						if (err) return reject(err);
						fs.write(fd, buf, 0, buf.byteLength, 0, (err, n) => {
							if (err) return reject(err);
							// TODO: dont assume full buf was written
							fs.close(fd, (err) => {
								if (err) return reject(err);
								resolve();
							});
						});
					});
				});
			},
			// readFile helper for working with fs in js
			readFile(path) {
				return new Promise((resolve, reject) => {
					fs.stat(path, (err, stat) => {
						if (err) return reject(err);
						const buf = new Uint8Array(stat.size);
						fs.open(path, 0, 0, async (err, fd) => {
							if (err) return reject(err);

							const pinkyPromiseRead = function(fd, buffer, length) {
								return new Promise((res, rej) => {
									fs.read(fd, buffer, 0, length, 0, (err, n) => {
										if (err)
											rej(err)
										else
											res(n);
									});
								});
							};

							let cursor = 0;
							while (cursor < buf.byteLength) {
								const result = await pinkyPromiseRead(fd, buf.subarray(cursor), buf.byteLength - cursor);
								if (typeof result !== 'number') return reject(result);
								cursor += result;
							}

							fs.close(fd, (err) => {
								if (err) return reject(err);
								resolve(buf);
							});
						});
					});
				});
			},
			// writeSync used by runtime.wasmWrite
			writeSync(fd, buf) {
				outputBuf += decoder.decode(buf);
				const nl = outputBuf.lastIndexOf("\n");
				if (nl != -1) {
					console.log(outputBuf.substring(0, nl));
					outputBuf = outputBuf.substring(nl + 1);
				}
				return buf.length;
			},
			// the actual fs api
			write(fd, buf, offset, length, position, callback) {
        switch (fd) {
          case 1:
						globalThis.stdout(buf);
						callback(null, length);
						return;
          case 2:
						globalThis.stderr(buf);
						callback(null, length);
						return;
          default:
						globalThis.sys.call("fs.write", [fd, buf, offset, length, position])
							.then(res => callback(null, res.value))
							.catch(err => callback(inflateErr(err)));
        }
			},
			chmod(path, mode, callback) {
				globalThis.sys.call("fs.chmod", [path, mode])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			chown(path, uid, gid, callback) {
				globalThis.sys.call("fs.chown", [path, uid, gid])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			close(fd, callback) {
				globalThis.sys.call("fs.close", [fd])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			fchmod(fd, mode, callback) {
				globalThis.sys.call("fs.fchmod", [fd, mode])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			fchown(fd, uid, gid, callback) {
				globalThis.sys.call("fs.fchown", [fd, uid, gid])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			fstat(fd, callback) {
				globalThis.sys.call("fs.fstat", [fd])
					.then(res => callback(null, inflateStat(res.value)))
					.catch(err => callback(inflateErr(err)));
			},
			fsync(fd, callback) {
				globalThis.sys.call("fs.fsync", [fd])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			ftruncate(fd, length, callback) {
				globalThis.sys.call("fs.ftruncate", [fd])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			lchown(path, uid, gid, callback) {
				globalThis.sys.call("fs.lchown", [path, uid, gid])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			link(path, link, callback) {
				globalThis.sys.call("fs.link", [path, link])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			lstat(path, callback) {
				globalThis.sys.call("fs.lstat", [path])
					.then(res => callback(null, inflateStat(res.value)))
					.catch(err => callback(inflateErr(err)));
			},
			mkdir(path, perm, callback) {
				globalThis.sys.call("fs.mkdir", [path, perm])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			open(path, flags, mode, callback) {
				globalThis.sys.call("fs.open", [path, flags, mode])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			read(fd, buffer, offset, length, position, callback) { 
				if (fd === 0) {
          globalThis.stdin(buffer, callback);
          return;
        }
				globalThis.sys.call("fs.read", [fd, buffer.length, offset, length, position])
					.then(res => {
						buffer.set(res.value.buf);
						callback(res.value.err, res.value.n);
					});
      },
			readdir(path, callback) {
				globalThis.sys.call("fs.readdir", [path])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			readlink(path, callback) {
				globalThis.sys.call("fs.readlink", [path])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			rename(from, to, callback) {
				globalThis.sys.call("fs.rename", [from, to])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			rmdir(path, callback) {
				globalThis.sys.call("fs.rmdir", [path])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			stat(path, callback) {
				globalThis.sys.call("fs.stat", [path])
					.then(res => callback(null, inflateStat(res.value)))
					.catch(err => callback(inflateErr(err)));
			},
			symlink(path, link, callback) {
				globalThis.sys.call("fs.symlink", [path, link])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			truncate(path, length, callback) {
				globalThis.sys.call("fs.truncate", [path, length])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			unlink(path, callback) {
				globalThis.sys.call("fs.unlink", [path])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
			},
			utimes(path, atime, mtime, callback) {
				globalThis.sys.call("fs.utimes", [path, atime, mtime])
					.then(res => callback(null, res.value))
					.catch(err => callback(inflateErr(err)));
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
			umask() { throw enosys(); },
			pid: -1,
			ppid: -1,
			dir: "/",
			cwd() { return globalThis.process.dir },
			chdir(path) {
				// TODO: handle relative paths etc
				globalThis.process.dir = path;
			},
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
			this.exitcode = undefined;
			this.exit = (code) => {
				this.exitcode = code;
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

			const timeOrigin = Date.now() - performance.now();
			this.importObject = {
				_gotest: {
					add: (a, b) => a + b,
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
						const fd = getInt64(sp + 8);
						const p = getInt64(sp + 16);
						const n = this.mem.getInt32(sp + 24, true);
						fs.writeSync(fd, new Uint8Array(this._inst.exports.mem.buffer, p, n));
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
//# sourceURL=wasm.js
