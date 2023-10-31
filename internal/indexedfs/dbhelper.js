// path, perms, size, isdir, ctime (create time), mtime (modified), atime (last accessed), blob

// only implement MutableFS
// don't try to stream blobs (you can't lol)
// use jazzFS as a reference

const fileData = [
	{ path: "/", perms: 0, size: 0, isdir: true, ctime: 0, mtime: 0, atime: 0, blob: 0 },
	{ path: "home", perms: 0, size: 0, isdir: true, ctime: 0, mtime: 0, atime: 0, blob: 0 },
	{ path: "home/hello.txt", perms: 0, size: 0, isdir: false, ctime: 0, mtime: 0, atime: 0, blob: 1 },
	{ path: "home/goodbye.txt", perms: 0, size: 0, isdir: false, ctime: 0, mtime: 0, atime: 0, blob: 2 },
];

// const OpenFlags = { O_WRONLY: 1, O_RDWR: 2, O_CREAT: 64, O_TRUNC: 512, O_APPEND: 1024, O_EXCL: 128 };

export function initialize() {
	return new Promise((resolve) => {
		const OpenDBRequest = indexedDB.open("indexedDBFS");
		OpenDBRequest.onerror = (event) => {
			console.error(`Unable to open IndexedDB: ${event.target.errorCode}`);
		};
		OpenDBRequest.onsuccess = (event) => {
			let db = event.target.result;

			db.onerror = (event) => {
				// Generic error handler for all errors targeted at this database's
				// requests!
				console.error(`Unhandled IndexedDB error: ${event.target.errorCode}`);
			};

			// This event is only implemented in recent browsers
			OpenDBRequest.onupgradeneeded = (event) => {
				// Save the IDBDatabase interface
				db = event.target.result;

				// Create an objectStore for this database
				const objectStore = db.createObjectStore("files", {
					autoIncrement: true
				});

				objectStore.createIndex("path", "path", {
					unique: true
				});

				objectStore.transaction.oncomplete = (event) => {
					// TODO: fill data from disk instead of using sample data
					const fileStore = db.transaction("files", "readwrite").objectStore("files");
					fileData.forEach((file) => {
						fileStore.add(file);
					});
				};
			};

			resolve(db);
		};
	})
}
export function getFileKey(db, path) {
	return new Promise((resolve, reject) => {
		let pathIndex = db.transaction("files", "readonly")
			.objectStore("files")
			.index("path");

		let getRequest = pathIndex.getKey(path);

		getRequest.onsuccess = (event) => {
			resolve(event.target.result)
		};

		getRequest.onsuccess = () => {
			reject(getRequest.error)
		};
	});
}

export function readFile(db, key) {
	return new Promise((resolve, reject) => {
		const getRequest = db
			.transaction("files", "readonly")
			.objectStore("files")
			.get(key);

		getRequest.onsuccess = (event) => {
			resolve(blobToUint8Array(event.target.result.blob));
		};

		getRequest.onerror = () => {
			reject(getRequest.error);
		};
	})
}

export function blobToUint8Array(blob) {
	return new Promise((resolve, reject) => {
		const reader = new FileReader();

		reader.onloadend = function() {
			resolve(new Uint8Array(reader.result));
		};

		reader.onerror = function() {
			reject(new Error("Failed to read blob"));
		};

		reader.readAsArrayBuffer(blob);
	});
}

// function writefile(path, content) {
// 	path = cleanPath(path);
// 	let node = makeNode(path);
// 	if (typeof content === "string") {
// 		content = new Blob([content], {
// 			type: "text/plain"
// 		});
// 	}
// 	let bs = await createBinaryStreamFromBlob(content, window.wanix.collab.group);
// 	node.mutate(n => {
// 		n.set("dataID", bs.id);
// 		n.set("dataSize", content.size);
// 	})
// }
