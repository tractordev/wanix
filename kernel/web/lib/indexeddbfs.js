// path, perms, size, isdir, ctime (create time), mtime (modified), atime (last accessed), blob

const fileData = [
	{ path: "/", perms: 0, size: 0, isdir: true, ctime: 0, mtime: 0, atime: 0, blob: 0 },
	{ path: "home", perms: 0, size: 0, isdir: true, ctime: 0, mtime: 0, atime: 0, blob: 0 },
	{ path: "home/hello.txt", perms: 0, size: 0, isdir: false, ctime: 0, mtime: 0, atime: 0, blob: 1 },
	{ path: "home/goodbye.txt", perms: 0, size: 0, isdir: false, ctime: 0, mtime: 0, atime: 0, blob: 2 },
];


const OpenFlags = { O_WRONLY: 1, O_RDWR: 2, O_CREAT: 64, O_TRUNC: 512, O_APPEND: 1024, O_EXCL: 128 };

const IDBFS = class {
	constructor() {
		const OpenDBRequest = window.indexedDB.open("indexedDBFS");
		OpenDBRequest.onerror = (event) => {
			console.error("Why didn't you allow my web app to use IndexedDB?!");
		};
		OpenDBRequest.onsuccess = (event) => {
			this.db = event.target.result;
			console.log("yip");
		};

		this.db.onerror = (event) => {
			// Generic error handler for all errors targeted at this database's
			// requests!
			console.error(`Database error: ${event.target.errorCode}`);
		};

		// This event is only implemented in recent browsers
		OpenDBRequest.onupgradeneeded = (event) => {
			// Save the IDBDatabase interface
			this.db = event.target.result;

			// Create an objectStore for this database
			const objectStore = this.db.createObjectStore("files", {
				keyPath: "path"
			});

			// objectStore.createIndex("isdir", "isdir", {unique: false});

			objectStore.transaction.oncomplete = (event) => {
				const fileStore = this.db.transaction("files", "readwrite").objectStore("files");
				fileData.forEach((file) => {
					fileStore.add(file);
				});
			};
		};

		this.nextFd = 1000;
		this.fileDescriptors = new Map()
	}

	write(fd, buf, offset, length, position, callback) {
		if(fd === 1) {
			globalThis.stdout(buf);
			callback(null, length);
		}
		if(fd === 2) {
			globalThis.stderr(buf);
			callback(null, length);
		} 

		const f = this.fileDescriptors.get(fd);
		if(f === undefined) {
			callback(Error("EBADF"));
			return;
		}

		console.error("TODO write");
	}
	chmod(path, mode, callback) {
	
	}
	chown(path, uid, gid, callback) {
	
	}
	close(fd, callback) {
		if(!this.fileDescriptors.has(fd)) {
			callback(Error("EBADF"));
			return;
		}

		this.fileDescriptors.delete(fd);
	}
	fchmod(fd, mode, callback) {
	
	}
	fchown(fd, uid, gid, callback) {
	
	}
	fstat(fd, callback) {
	}
	fsync(fd, callback) {
	
	}
	ftruncate(fd, length, callback) {
	
	}
	lchown(path, uid, gid, callback) {
	
	}
	link(path, link, callback) {
	
	}
	lstat(path, callback) {
		
	}
	mkdir(path, perm, callback) {
	
	}
	open(path, flags, mode, callback) {
		// TODO: use flags & mode
		const getRq = this.db.transaction("files").objectStore("files").get(path);
		getRq.onsuccess = (event) => {
			const fd = this.nextFd;
			this.fileDescriptors.set(fd, {file: event.target.result, fpath: path});
			this.nextFd++;

			callback(null, fd);
		}
		getRq.onerror = (event) => {
			callback(event.target.errorCode); // TODO: convert dbErrorCode to Errno
		}
	}

	read(fd, buffer, offset, length, position, callback) {
		if(fd === 0) {
			globalThis.stdin(buffer, callback);
			return;
		}

		const f = this.fileDescriptors.get(fd);
		if(f === undefined) {
			callback(Error("EBADF"));
			return;
		}

		if(position >= f.file.size || position < 0) {
			callback(Error("EINVAL"));
			return;	
		}

		if(f.file.isdir) {
			callback(Error("EISDIR"));
			return;
		}

		// TODO: rewrite this part
		// Why can't I just read binary data?
		// Why is a simple memcpy buried so deep behind a crappy API?
		const end = position + length < f.file.size ? position + length : f.file.size;
		const slice = f.file.blob.slice(position,  f.file.blob.type);
		buffer = slice.arrayBuffer()

		callback(null, end - position);
	}
		
	readdir(path, callback) {
	
	}
	readlink(path, callback) {
	
	}
	rename(from, to, callback) {
	
	}
	rmdir(path, callback) {
	
	}
	stat(path, callback) {
		
	}
	symlink(path, link, callback) {
	
	}
	truncate(path, length, callback) {
	
	}
	unlink(path, callback) {
	
	}
	utimes(path, atime, mtime, callback) {
	
	}
}
