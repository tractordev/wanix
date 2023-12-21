
globalThis.api = {
  host: {
    respondRPC: async (resp, call) => {
      // forward upstream to host page
      const args = await call.receive();
      const r = await sys.call(call.selector, args);
      resp.return(r.value);
    },
  },
  fs: {
    write(fd, buf, offset, length, position) {
      return new Promise((ok, err) => globalThis.fs.write(fd, buf, offset, length, position, cb(ok, err)));
    },
    chmod(path, mode) { 
      return new Promise((ok, err) => globalThis.fs.chmod(path, mode, cb(ok, err)));
    },
    chown(path, uid, gid) { 
      return new Promise((ok, err) => globalThis.fs.chown(path, uid, gid, cb(ok, err)));
    },
    close(fd) { 
      return new Promise((ok, err) => globalThis.fs.close(fd, cb(ok, err)));
    },
    fchmod(fd, mode) { 
      return new Promise((ok, err) => globalThis.fs.fchmod(fd, mode, cb(ok, err)));
    },
    fchown(fd, uid, gid) { 
      return new Promise((ok, err) => globalThis.fs.fchown(fd, uid, gid, cb(ok, err)));
    },
    fstat(fd) { 
      return new Promise((ok, err) => globalThis.fs.fstat(fd, (e,res) => {
        if (e !== null) {
          err(encodeErrCode(e));
        } else {
          ok(marshalizeStat(res));
        }
      }));
    },
    fsync(fd) { 
      return new Promise((ok, err) => globalThis.fs.fsync(fd, cb(ok, err)));
    },
    ftruncate(fd, length) { 
      return new Promise((ok, err) => globalThis.fs.ftruncate(fd, length, cb(ok, err)));
    },
    lchown(path, uid, gid) { 
      return new Promise((ok, err) => globalThis.fs.lchown(path, uid, gid, cb(ok, err)));
    },
    link(path, link) { 
      return new Promise((ok, err) => globalThis.fs.link(path, link, cb(ok, err)));
    },
    lstat(path) { 
      return new Promise((ok, err) => globalThis.fs.lstat(path, (e,res) => {
        if (e !== null) {
          err(encodeErrCode(e));
        } else {
          ok(marshalizeStat(res));
        }
      }));
    },
    mkdir(path, perm) { 
      return new Promise((ok, err) => globalThis.fs.mkdir(path, perm, cb(ok, err)));
    },
    open(path, flags, mode) { 
      return new Promise((ok, err) => globalThis.fs.open(path, flags, mode, cb(ok, err)));
    },
    // only significant signature change
    read(fd, bufsize, offset, length, position) {
      const buf = new Uint8Array(bufsize);
      return new Promise(resolve => globalThis.fs.read(fd, buf, offset, length, position, (err, n) => {
        resolve({err, buf, n});
      }));
    },
    readdir(path) { 
      return new Promise((ok, err) => globalThis.fs.readdir(path, cb(ok, err)));
    },
    readlink(path) { 
      return new Promise((ok, err) => globalThis.fs.readlink(path, cb(ok, err)));
    },
    rename(from, to) { 
      return new Promise((ok, err) => globalThis.fs.rename(from, to, cb(ok, err)));
    },
    rmdir(path) { 
      return new Promise((ok, err) => globalThis.fs.rmdir(path, cb(ok, err)));
    },
    stat(path) { 
      return new Promise((ok, err) => globalThis.fs.stat(path, (e,res) => {
        if (e !== null) {
          err(encodeErrCode(e));
        } else {
          ok(marshalizeStat(res));
        }
      }));
    },
    symlink(path, link) { 
      return new Promise((ok, err) => globalThis.fs.symlink(path, link, cb(ok, err)));
    },
    truncate(path, length) { 
      return new Promise((ok, err) => globalThis.fs.truncate(path, length, cb(ok, err)));
    },
    unlink(path) { 
      return new Promise((ok, err) => globalThis.fs.unlink(path, cb(ok, err)));
    },
    utimes(path, atime, mtime) { 
      return new Promise((ok, err) => globalThis.fs.utimes(path, atime, mtime, cb(ok, err)));
    },
    watch(path, recursive, eventMask, ignores) {
      return new Promise((ok, err) => globalThis.fs.watch(path, recursive, eventMask, ignores, cb(ok, err)));
    },
    unwatch(handle) {
      return new Promise((ok, err) => globalThis.fs.unwatch(handle, cb(ok, err)))
    }
  }
}


// the stat struct returned includes a function
// isDirectory, but this is never dynamic, so we'll
// evalutate the function in place and make it a
// function again on the other side.
function marshalizeStat(stat) {
  if (stat) {
    stat.isDirectory = stat.isDirectory();
  }
  return stat;
}

// error handler callback
function cb(ok, err) {
  return (e, ret) => {
    if (e !== null) {
      err(encodeErrCode(e));
    } else {
      ok(ret);
    }
  }
}

function encodeErrCode(err) {
  if (err.code) {
    err.message += `;code=${err.code}`;
  }
  return err;
}


//# sourceURL=syscall.js
