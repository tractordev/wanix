// path, perms, size, isdir, ctime (create time), mtime (modified), atime (last accessed), blob
const fileData = [
	{ path: ".", perms: 0, size: 0, isdir: true, ctime: 0, mtime: 0, atime: 0, blob: null },
	{ path: "home", perms: 0, size: 0, isdir: true, ctime: 0, mtime: 0, atime: 0, blob: null },
	{ path: "home/hello.txt", perms: 0, size: 12, isdir: false, ctime: 0, mtime: 0, atime: 0, blob: new Blob(["Hello World!"], {type: "text/plain"}) },
	{ path: "home/goodbye.txt", perms: 0, size: 17, isdir: false, ctime: 0, mtime: 0, atime: 0, blob: new Blob(["Sayonara Suckers!"], {type: "text/plain"}) },
];

export function initialize() {
	return new Promise((resolve, reject) => {
		const OpenDBRequest = indexedDB.open("indexedDBFS");

		OpenDBRequest.onerror = (event) => {
			reject(new Error(`Unable to open IndexedDB: ${event.target.errorCode}`));
		};
		OpenDBRequest.onsuccess = (reqEvent) => {
			console.log("OpenDBRequest.onsuccess");
			const db = reqEvent.target.result;

			db.onerror = (dbEvent) => {
				// Generic error handler for all errors targeted at this database's
				// requests!
				console.error(`Unhandled IndexedDB error: ${dbEvent.target.errorCode}`);
			};
			resolve(db)
		};
		
		// This event is only implemented in recent browsers
		OpenDBRequest.onupgradeneeded = (upgradeEvent) => {
			console.log("OpenDBRequest.onupgradeneeded");
			const db = upgradeEvent.target.result;

			const objectStore = db.createObjectStore("files", {
				autoIncrement: true
			});

			objectStore.createIndex("path", "path", {
				unique: true
			});

			objectStore.transaction.oncomplete = () => {
				// TODO: fill data from disk instead of using sample data
				const fileStore = db.transaction("files", "readwrite").objectStore("files");
				fileData.forEach((file) => {
					fileStore.add(file);
				});
			};
		};
	})
}

export function addFile(db, path, perms, isdir) {
	return new Promise((resolve, reject) => {
		const transaction = db.transaction("files", "readwrite");

		transaction.onerror = () => {
			reject(new Error(`addFile transaction failed: ${transaction.error}`));
		};

		const fileStore = transaction.objectStore("files");
		const addRequest = fileStore.add({
			path: path,
			perms: perms,
			size: 0,
			isdir: isdir,
			ctime: 0,
			mtime: 0,
			atime: 0,
			blob: new Blob([""], {
				type: "text/plain"
			}),
		});

		addRequest.onsuccess = (event) => {
			resolve(event.target.result) // return key
		};
	});
}

export function getFileKey(db, path) {
	return new Promise((resolve, reject) => {
		const getRequest =
			db.transaction("files", "readonly")
			.objectStore("files")
			.index("path")
			.getKey(path);

		getRequest.onsuccess = (event) => {
			// The success callbacks are triggered even if the function 
			// didn't actually return any data... So we have to do error
			// handling in here too. >:|
			if(event.target.result) {
				resolve(getRequest.result);
			} else {
				reject(new Error(`ErrNotExist: Failed to find file at path: ${path}`));
			}
		};

		getRequest.onerror = () => {
			reject(new Error(`Failed to find file at path ${path}: ${getRequest.error}`));
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
				reject(new Error(`ErrNotExist: Failed to find file at path: ${path}`));
			}
		};

		getRequest.onerror = () => {
			reject(new Error(`Failed to find file at path ${path}: ${getRequest.error}`));
		};
	});
}

export function readFile(db, key) {
	return new Promise((resolve, reject) => {
		const getRequest = 
			db.transaction("files", "readonly")
			.objectStore("files")
			.get(key);

		getRequest.onsuccess = (event) => {
			if(event.target.result) {
				resolve(blobToUint8Array(event.target.result.blob));
			} else {
				reject(new Error(`ErrNotExist: Failed to read file with key ${key}`))
			}
		};

		getRequest.onerror = () => {
			reject(new Error(`Failed to read file with key ${key}: ${getRequest.error}`))
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
