// path, perms, size, isdir, ctime (create time), mtime (modified), atime (last accessed), blob
const fileData = [
	{ path: ".", perms: 0, size: 0, isdir: true, ctime: 0, mtime: 0, atime: 0, blob: null },
	{ path: "home", perms: 0, size: 0, isdir: true, ctime: 0, mtime: 0, atime: 0, blob: null },
	{ path: "home/hello.txt", perms: 0, size: 12, isdir: false, ctime: 0, mtime: 0, atime: 0, blob: new Blob(["Hello World!"], {type: "application/octet-stream"}) },
	{ path: "home/goodbye.txt", perms: 0, size: 17, isdir: false, ctime: 0, mtime: 0, atime: 0, blob: new Blob(["Sayonara Suckers!"], {type: "application/octet-stream"}) },
	{ path: "home/subdir", perms: 0, size: 0, isdir: true, ctime: 0, mtime: 0, atime: 0, blob: null },
	{ path: "home/subdir/hello2.txt", perms: 0, size: 12, isdir: false, ctime: 0, mtime: 0, atime: 0, blob: new Blob(["Hello World!"], {type: "application/octet-stream"}) },
];

export function reset() {
	indexedDB.deleteDatabase("indexedFS");
}

export function initialize() {
	return new Promise((resolve, reject) => {
		const OpenDBRequest = indexedDB.open("indexedFS");

		OpenDBRequest.onerror = (event) => {
			reject(goError(`Unable to open IndexedDB: ${event.target.error}`), null);
		};
		OpenDBRequest.onsuccess = (reqEvent) => {
			const db = reqEvent.target.result;
			globalThis.indexedfs = db;

			db.onerror = (dbEvent) => {
				// Generic error handler for all errors targeted at this database's
				// requests!
				console.error(`Unhandled IndexedDB error: ${dbEvent.target.error}`);
			};
			resolve(db)
		};
		
		// This event is only implemented in recent browsers
		OpenDBRequest.onupgradeneeded = (upgradeEvent) => {
			const db = upgradeEvent.target.result;

			const objectStore = db.createObjectStore("files", {
				autoIncrement: true
			});

			objectStore.createIndex("path", "path", {
				unique: true
			});

			objectStore.transaction.oncomplete = () => {
				// TODO: fill data from disk instead of using sample data
				console.log("Loading debug indexedfs files...");
				const fileStore = db.transaction("files", "readwrite").objectStore("files");
				fileData.forEach((file) => {
					fileStore.add(file);
				});
			};
		};
	})
}

export function addFile(db, path, perms, isdir, unixTime) {
	return new Promise((resolve, reject) => {
		const req = 
			db.transaction("files", "readwrite")
			.objectStore("files")
			.add({
				path: path,
				perms: perms,
				size: 0,
				isdir: isdir,
				ctime: unixTime,
				mtime: unixTime,
				atime: unixTime,
				blob: new Blob([""], {
					type: "text/plain"
				}),
			});

		req.onsuccess = (event) => {
			resolve(event.target.result) // return key
		};

		req.onerror = (event) => {
			reject(goError(`Failed to add file at path ${path}: ${event.target.error}`), "ErrExist");
		};
	});
}

// updateCallback takes a file object, modifies, and returns it.
export function updateFile(db, pathOrKey, updateCallback) {
	return new Promise((resolve, reject) => {
		const fileStore = db.transaction("files", "readwrite").objectStore("files");
		const request =
			typeof pathOrKey === "string" ?
			fileStore.index("path").openCursor(pathOrKey) :
			fileStore.openCursor(pathOrKey);

		const onFailure = (event) => {
			event.stopPropagation();
			reject(goError(`Couldn't find file with key ${pathOrKey}: ${event.target.error}`, "ErrNotExist"));
		};

		request.onsuccess = (event) => {
			const cursor = event.target.result;
			if(cursor) {
				const file = updateCallback(cursor.value);
				const updateReq = cursor.update(file)
				updateReq.onsuccess = () => resolve();
				updateReq.onerror = onFailure;
			} else {
				reject(goError(`Couldn't find file with key ${pathOrKey}`, "ErrNotExist"));
			}
		};

		request.onerror = onFailure;
	});
}

export function getFileKey(db, path) {
	return new Promise((resolve, reject) => {
		const req =
			db.transaction("files", "readonly")
			.objectStore("files")
			.index("path")
			.getKey(path);

		req.onsuccess = (event) => {
			// The success callbacks are triggered even if the function 
			// didn't actually return any data... So we have to do error
			// handling in here too. >:|
			if(event.target.result) {
				resolve(req.result);
			} else {
				reject(goError(`Failed to find file at path: ${path}`, "ErrNotExist"));
			}
		};

		req.onerror = (event) => {
			reject(goError(`Failed to find file at path ${path}: ${event.target.error}`, "ErrNotExist"));
		};
	});
}

export function getDirEntries(db, path) {
	if (path === ".") {
		path = "";
	}
	if (path && path[path.length - 1] !== '/') {
		path = path + "/";
	}
	return new Promise((resolve, reject) => {
		const range = IDBKeyRange.bound(path, path + '\uffff', false, true);

		const getRequest =
			db.transaction("files", "readonly")
			.objectStore("files")
			.index("path")
			.openCursor(range);

		const entries = [];

		getRequest.onsuccess = (event) => {
			const cursor = event.target.result;
			if (cursor) {
				// Check if the key is directly under the path, which means it should not have any
				// more slashes beyond the given path
				const key = cursor.key;
				if (key && key.startsWith(path) && key.slice(path.length).indexOf('/') === -1 && key !== ".") {
					entries.push(cursor.value); // Store the value in the results array
				}
				cursor.continue();
			} else {
				resolve(entries);
			}
		};

		getRequest.onerror = (event) => {
			reject(goError(`Failed to find dir at path ${path}: ${event.target.error}`, "ErrNotExist"));
		};
	});
}

export function getFileByPath(db, path) {
	return new Promise((resolve, reject) => {
		const getRequest =
			db.transaction("files", "readonly")
			.objectStore("files")
			.index("path")
			.get(path);

		getRequest.onsuccess = (event) => {
			if(event.target.result) {
				resolve(getRequest.result);
			} else {
				reject(goError(`Failed to find file at path: ${path}`, "ErrNotExist"));
			}
		};

		getRequest.onerror = (event) => {
			reject(goError(`Failed to find file at path ${path}: ${event.target.error}`, "ErrNotExist"));
		};
	});
}

export function readFile(db, key) {
	return new Promise((resolve, reject) => {
		const getRequest = 
			db.transaction("files", "readonly")
			.objectStore("files")
			.get(key);

		getRequest.onsuccess = async (event) => {
			if(event.target.result) {
				try {
					const data = await event.target.result.blob.arrayBuffer();
					resolve(new Uint8Array(data));
				} catch (rejected) {
					reject(goError(rejected instanceof Error ? rejected.message : String(rejected), "ErrReadingBlob"));
				}
			} else {
				reject(goError(`Failed to read file with key ${key}`, "ErrNotExist"));
			}
		};

		getRequest.onerror = (event) => {
			reject(goError(`Failed to read file with key ${key}: ${event.target.error}`, "ErrNotExist"));
		};
	})
}

export function deleteFile(db, key) {
	return new Promise((resolve, reject) => {
		const req = 
			db.transaction("files", "readwrite")
			.objectStore("files")
			.delete(key);

		req.onsuccess = () => resolve();
		req.onerror = (event) => {
			reject(goError(`Failed to delete file with key ${key}: ${event.target.error}`, "ErrNotExist"));
		};
	})
}


export function deleteAll(db, path) {
	// Assumes path !== "." or ""
	// This is asserted by the caller.

	let dirpath = path;
	if (path && path[path.length - 1] !== '/') {
		dirpath = path + "/";
	}

	return new Promise((resolve, reject) => {
		const range = IDBKeyRange.lowerBound(path);

		const tx = db.transaction("files", "readwrite");
		const req =	tx.objectStore("files").index("path").openCursor(range);

		req.onsuccess = (event) => {
			const cursor = event.target.result;
			if (cursor) {
				if ((cursor.key === path || cursor.key.startsWith(dirpath)) && cursor.key !== ".") {
					cursor.delete();
					cursor.continue();
				}
			}
		};

		tx.oncomplete = () => resolve();
		tx.onerror = (event) => {
			reject(goError(`DeleteAll failure: ${path}: ${event.target.errorCode}`, "ErrNotExist"));
		};
	});
}

function goError(message, goErrorName) {
	let err = new Error(message);
	if(goErrorName) {
		err.goErrorName = goErrorName;
	}
	return err;
}
