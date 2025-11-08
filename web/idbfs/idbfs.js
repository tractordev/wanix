// IDBFS - IndexedDB-based filesystem implementation
// Implements a POSIX-like filesystem interface using browser IndexedDB
// Optimized with metadata/data separation, LRU caching, and indexed queries

// File mode constants
const S_IFMT = 0o170000;   // bit mask for the file type bit field
const S_IFSOCK = 0o140000; // socket
const S_IFLNK = 0o120000;  // symbolic link
const S_IFREG = 0o100000;  // regular file
const S_IFBLK = 0o060000;  // block device
const S_IFDIR = 0o040000;  // directory
const S_IFCHR = 0o020000;  // character device
const S_IFIFO = 0o010000;  // FIFO

// File open flags
const O_RDONLY = 0;
const O_WRONLY = 1;
const O_RDWR = 2;
const O_CREAT = 0o100;
const O_EXCL = 0o200;
const O_TRUNC = 0o1000;
const O_APPEND = 0o2000;

// Seek whence values
const SEEK_SET = 0;
const SEEK_CUR = 1;
const SEEK_END = 2;

// Threshold for inline vs external data storage
const INLINE_DATA_THRESHOLD = 4096; // 4KB

class IDBFSError extends Error {
    constructor(message, code) {
        super(message);
        this.name = 'IDBFSError';
        this.code = code;
    }
}

function log(...args) {
    // Enable debug logging if ?debug-idbfs=true in the URL query params
    let _debugIDBFS;
    if (typeof window !== "undefined" && window.location && window.location.search) {
        const params = new URLSearchParams(window.location.search);
        _debugIDBFS = params.get("debug-idbfs") === "true";
    }
    if (_debugIDBFS) {
        console.log("[idbfs]", ...args);
    }
}

// LRU Cache for hot entries
class LRUCache {
    constructor(maxSize) {
        this.maxSize = maxSize;
        this.cache = new Map();
    }

    get(key) {
        if (!this.cache.has(key)) return undefined;
        
        // Move to end (most recently used)
        const value = this.cache.get(key);
        this.cache.delete(key);
        this.cache.set(key, value);
        return value;
    }

    set(key, value) {
        // Delete if exists (to update position)
        if (this.cache.has(key)) {
            this.cache.delete(key);
        }
        
        // Add to end
        this.cache.set(key, value);
        
        // Evict oldest if over size
        if (this.cache.size > this.maxSize) {
            const firstKey = this.cache.keys().next().value;
            this.cache.delete(firstKey);
        }
    }

    delete(key) {
        this.cache.delete(key);
    }

    clear() {
        this.cache.clear();
    }

    // Invalidate all entries with a given parent (for directory operations)
    invalidateByParent(parent) {
        for (const [key, value] of this.cache.entries()) {
            if (value && value.parent === parent) {
                this.cache.delete(key);
            }
        }
    }
}

export class IDBFS {
    constructor(name) {
        this.dbName = name;
        this.db = null;
        
        // Separate caches for metadata and data
        this.metadataCache = new LRUCache(1000);  // Cache 1000 metadata entries
        this.dataCache = new LRUCache(10);        // Cache 10 large data blobs
    }

    async _initDB() {
        if (this.db) return this.db;

        await new Promise((resolve, reject) => {
            const request = indexedDB.open(this.dbName, 2);

            request.onerror = () => reject(new IDBFSError('Failed to open database', 'EIO'));

            request.onsuccess = () => {
                this.db = request.result;
                resolve(this.db);
            };

            request.onupgradeneeded = (event) => {
                console.log("idbfs onupgradeneeded");
                const db = event.target.result;
                const oldVersion = event.oldVersion;

                // Create metadata store
                if (!db.objectStoreNames.contains('metadata')) {
                    const metadata = db.createObjectStore('metadata', { keyPath: 'path' });
                    
                    // Critical indexes for performance
                    metadata.createIndex('parent', 'parent', { unique: false });
                    metadata.createIndex('type', 'type', { unique: false });
                    metadata.createIndex('parent_type', ['parent', 'type'], { unique: false });
                }

                // Create data store for large files
                if (!db.objectStoreNames.contains('data')) {
                    db.createObjectStore('data', { keyPath: 'path' });
                }

                // Migration from v1 to v2
                if (oldVersion === 1 && db.objectStoreNames.contains('fs')) {
                    console.log("migrating from v1 to v2");
                    const transaction = event.target.transaction;
                    const oldStore = transaction.objectStore('fs');
                    const metaStore = transaction.objectStore('metadata');
                    const dataStore = transaction.objectStore('data');

                    oldStore.openCursor().onsuccess = (e) => {
                        const cursor = e.target.result;
                        if (cursor) {
                            const entry = cursor.value;
                            const isLarge = entry.data && entry.data.length >= INLINE_DATA_THRESHOLD;
                            
                            // Determine type
                            let type = 'file';
                            if ((entry.mode & S_IFMT) === S_IFDIR) type = 'dir';
                            else if ((entry.mode & S_IFMT) === S_IFLNK) type = 'symlink';

                            // Compute parent
                            let parent = '.';
                            if (entry.path !== '.') {
                                const parts = entry.path.split('/');
                                parent = parts.length === 1 ? '.' : parts.slice(0, -1).join('/');
                            }

                            // Create metadata entry
                            const meta = {
                                path: entry.path,
                                parent: parent,
                                type: type,
                                mode: entry.mode,
                                mtime: entry.mtime,
                                atime: entry.atime,
                                size: entry.size,
                                dataExternal: isLarge,
                                data: isLarge ? null : entry.data
                            };
                            metaStore.put(meta);

                            // Create data entry if large
                            if (isLarge) {
                                dataStore.put({
                                    path: entry.path,
                                    data: entry.data
                                });
                            }

                            cursor.continue();
                        }
                    };
                }
            };
        });

        // Ensure root directory exists
        await this._ensureRoot();
        
        return this.db;
    }

    async _ensureRoot() {
        const rootMeta = await this._getMetadata('.');
        if (!rootMeta) {
            const now = Math.floor(Date.now() / 1000);
            const metadata = {
                path: '.',
                parent: '.',
                type: 'dir',
                mode: S_IFDIR | 0o755,
                mtime: now,
                atime: now,
                size: 0,
                dataExternal: false,
                data: null
            };
            await this._putEntry(metadata, null);
        }
    }

    // Get metadata with caching
    async _getMetadata(path) {
        // Check cache first
        const cached = this.metadataCache.get(path);
        if (cached) return cached;

        const db = await this._initDB();
        return new Promise((resolve, reject) => {
            const transaction = db.transaction(['metadata'], 'readonly');
            const store = transaction.objectStore('metadata');
            const request = store.get(path);

            request.onsuccess = () => {
                const result = request.result;
                if (result) {
                    this.metadataCache.set(path, result);
                }
                resolve(result);
            };
            request.onerror = () => reject(new IDBFSError('Failed to get metadata', 'EIO'));
        });
    }

    // Get data blob (only for large files)
    async _getDataBlob(path) {
        // Check cache first
        const cached = this.dataCache.get(path);
        if (cached) return cached;

        const db = await this._initDB();
        return new Promise((resolve, reject) => {
            const transaction = db.transaction(['data'], 'readonly');
            const store = transaction.objectStore('data');
            const request = store.get(path);

            request.onsuccess = () => {
                const result = request.result;
                if (result) {
                    this.dataCache.set(path, result);
                }
                resolve(result);
            };
            request.onerror = () => reject(new IDBFSError('Failed to get data', 'EIO'));
        });
    }

    // Put metadata and data together (atomic)
    // If data is undefined, preserve existing data storage (metadata-only update)
    // If data is null, remove data (for directories, symlinks)
    // If data is Uint8Array, update data storage
    async _putEntry(metadata, data = undefined) {
        const db = await this._initDB();
        
        // Check if this is a metadata-only update
        const isMetadataOnly = data === undefined;
        
        if (!isMetadataOnly) {
            // Determine if data should be external
            const isLarge = data && data.length >= INLINE_DATA_THRESHOLD;
            
            // Get old metadata to check if we need to delete old external data
            const oldMeta = await this._getMetadata(metadata.path);
            const hadExternalData = oldMeta && oldMeta.dataExternal;
            const needsDataStoreAccess = isLarge || hadExternalData;
            
            metadata.dataExternal = isLarge;
            metadata.data = isLarge ? null : data;

            return new Promise((resolve, reject) => {
                const transaction = db.transaction(
                    needsDataStoreAccess ? ['metadata', 'data'] : ['metadata'], 
                    'readwrite'
                );
                
                transaction.oncomplete = () => {
                    this.metadataCache.set(metadata.path, metadata);
                    if (isLarge && data) {
                        this.dataCache.set(metadata.path, { path: metadata.path, data });
                    } else if (hadExternalData) {
                        this.dataCache.delete(metadata.path);
                    }
                    resolve();
                };
                transaction.onerror = () => reject(new IDBFSError('Failed to put entry', 'EIO'));

                // Write metadata
                const metaStore = transaction.objectStore('metadata');
                metaStore.put(metadata);

                if (needsDataStoreAccess) {
                    const dataStore = transaction.objectStore('data');
                    
                    // Write new external data
                    if (isLarge) {
                        dataStore.put({ path: metadata.path, data });
                    }
                    
                    // Delete old external data if transitioning to inline or null
                    if (hadExternalData && !isLarge) {
                        dataStore.delete(metadata.path);
                    }
                }
            });
        } else {
            // Metadata-only update - don't touch data storage
            return new Promise((resolve, reject) => {
                const transaction = db.transaction(['metadata'], 'readwrite');
                
                transaction.oncomplete = () => {
                    this.metadataCache.set(metadata.path, metadata);
                    resolve();
                };
                transaction.onerror = () => reject(new IDBFSError('Failed to put entry', 'EIO'));

                const metaStore = transaction.objectStore('metadata');
                metaStore.put(metadata);
            });
        }
    }

    // Delete entry (both metadata and data)
    async _deleteEntry(path) {
        const db = await this._initDB();
        
        // Get metadata to know if we need to delete data too
        const meta = await this._getMetadata(path);
        const hasExternalData = meta && meta.dataExternal;

        return new Promise((resolve, reject) => {
            const transaction = db.transaction(
                hasExternalData ? ['metadata', 'data'] : ['metadata'],
                'readwrite'
            );
            
            transaction.oncomplete = () => {
                this.metadataCache.delete(path);
                if (hasExternalData) {
                    this.dataCache.delete(path);
                }
                resolve();
            };
            transaction.onerror = () => reject(new IDBFSError('Failed to delete entry', 'EIO'));

            const metaStore = transaction.objectStore('metadata');
            metaStore.delete(path);

            if (hasExternalData) {
                const dataStore = transaction.objectStore('data');
                dataStore.delete(path);
            }
        });
    }

    // Get children of a directory using index
    async _getChildren(parentPath) {
        const db = await this._initDB();
        return new Promise((resolve, reject) => {
            const transaction = db.transaction(['metadata'], 'readonly');
            const store = transaction.objectStore('metadata');
            const index = store.index('parent');
            const range = IDBKeyRange.only(parentPath);
            const request = index.getAll(range);

            request.onsuccess = () => resolve(request.result);
            request.onerror = () => reject(new IDBFSError('Failed to get children', 'EIO'));
        });
    }

    // Check if directory has any children (faster than getting all)
    async _hasChildren(parentPath) {
        const db = await this._initDB();
        return new Promise((resolve, reject) => {
            const transaction = db.transaction(['metadata'], 'readonly');
            const store = transaction.objectStore('metadata');
            const index = store.index('parent');
            const range = IDBKeyRange.only(parentPath);
            const request = index.openCursor(range);

            request.onsuccess = () => {
                // If cursor has any result, directory has children
                resolve(request.result !== null);
            };
            request.onerror = () => reject(new IDBFSError('Failed to check children', 'EIO'));
        });
    }

    // Get all descendants recursively (for directory rename)
    async _getAllDescendants(rootPath) {
        const db = await this._initDB();
        const descendants = [];
        
        return new Promise((resolve, reject) => {
            const transaction = db.transaction(['metadata'], 'readonly');
            const store = transaction.objectStore('metadata');
            const request = store.openCursor();

            request.onsuccess = (event) => {
                const cursor = event.target.result;
                if (cursor) {
                    const entry = cursor.value;
                    const prefix = rootPath === '.' ? '' : rootPath + '/';
                    
                    // Check if this entry is a descendant
                    if (entry.path === rootPath || entry.path.startsWith(prefix)) {
                        descendants.push(entry);
                    }
                    cursor.continue();
                } else {
                    resolve(descendants);
                }
            };
            request.onerror = () => reject(new IDBFSError('Failed to get descendants', 'EIO'));
        });
    }

    // Batch update multiple metadata entries in a single transaction
    async _batchUpdateMetadata(updates) {
        if (updates.length === 0) return;
        
        const db = await this._initDB();
        return new Promise((resolve, reject) => {
            const transaction = db.transaction(['metadata'], 'readwrite');
            
            transaction.oncomplete = () => {
                // Update cache
                for (const meta of updates) {
                    this.metadataCache.set(meta.path, meta);
                }
                resolve();
            };
            transaction.onerror = () => reject(new IDBFSError('Failed to batch update', 'EIO'));

            const store = transaction.objectStore('metadata');
            for (const meta of updates) {
                store.put(meta);
            }
        });
    }

    // Batch delete multiple entries
    async _batchDelete(paths) {
        if (paths.length === 0) return;
        
        const db = await this._initDB();
        
        // Get metadata to know which have external data
        const metaList = await Promise.all(paths.map(p => this._getMetadata(p)));
        const hasAnyExternalData = metaList.some(m => m && m.dataExternal);
        
        return new Promise((resolve, reject) => {
            const transaction = db.transaction(
                hasAnyExternalData ? ['metadata', 'data'] : ['metadata'],
                'readwrite'
            );
            
            transaction.oncomplete = () => {
                // Clear cache
                for (const path of paths) {
                    this.metadataCache.delete(path);
                    this.dataCache.delete(path);
                }
                resolve();
            };
            transaction.onerror = () => reject(new IDBFSError('Failed to batch delete', 'EIO'));

            const metaStore = transaction.objectStore('metadata');
            const dataStore = hasAnyExternalData ? transaction.objectStore('data') : null;
            
            for (let i = 0; i < paths.length; i++) {
                metaStore.delete(paths[i]);
                if (metaList[i] && metaList[i].dataExternal && dataStore) {
                    dataStore.delete(paths[i]);
                }
            }
        });
    }

    _getTypeFromMode(mode) {
        if ((mode & S_IFMT) === S_IFDIR) return 'dir';
        if ((mode & S_IFMT) === S_IFLNK) return 'symlink';
        return 'file';
    }

    _isDir(mode) {
        return (mode & S_IFMT) === S_IFDIR;
    }

    _isSymlink(mode) {
        return (mode & S_IFMT) === S_IFLNK;
    }

    _isFile(mode) {
        return (mode & S_IFMT) === S_IFREG;
    }

    _basename(path) {
        if (path === '.') return '.';
        const parts = path.split('/');
        return parts[parts.length - 1];
    }

    _dirname(path) {
        if (path === '.') return '.';
        const parts = path.split('/');
        if (parts.length === 1) return '.';
        return parts.slice(0, -1).join('/');
    }

    _normalizePath(path) {
        // Remove leading slash if present
        if (path.startsWith('/')) {
            path = path.slice(1);
        }
        // Empty path becomes root
        if (path === '' || path === '/') {
            return '.';
        }
        return path;
    }

    async _ensureParentExists(path) {
        if (path === '.') return;
        const parent = this._dirname(path);
        if (parent === '.') return;
        
        const meta = await this._getMetadata(parent);
        if (!meta) {
            throw new IDBFSError('Parent directory does not exist', 'ENOENT');
        }
        if (meta.type !== 'dir') {
            throw new IDBFSError('Parent is not a directory', 'ENOTDIR');
        }
    }

    async _resolveSymlink(path, visited = new Set()) {
        if (visited.has(path)) {
            throw new IDBFSError('Too many levels of symbolic links', 'ELOOP');
        }
        visited.add(path);

        const meta = await this._getMetadata(path);
        if (!meta) {
            return null;
        }

        if (meta.type === 'symlink') {
            const target = new TextDecoder().decode(meta.data);
            const resolvedPath = this._normalizePath(target);
            return this._resolveSymlink(resolvedPath, visited);
        }

        return meta;
    }

    async open(path) {
        log("idbfs open:", path);
        path = this._normalizePath(path);
        const meta = await this._resolveSymlink(path);
        if (!meta) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }
        return new File(this, path, meta, O_RDWR);
    }

    async create(path) {
        log("idbfs create:", path);
        path = this._normalizePath(path);
        await this._ensureParentExists(path);

        const now = Math.floor(Date.now() / 1000);
        const metadata = {
            path: path,
            parent: this._dirname(path),
            type: 'file',
            mode: S_IFREG | 0o644,
            mtime: now,
            atime: now,
            size: 0,
            dataExternal: false,
            data: new Uint8Array(0)
        };
        
        await this._putEntry(metadata, new Uint8Array(0));
        this.metadataCache.invalidateByParent(metadata.parent);
        
        return new File(this, path, metadata, O_RDWR);
    }

    async openfile(path, flags) {
        log("idbfs openfile:", path, flags);
        path = this._normalizePath(path);
        
        const shouldCreate = flags & O_CREAT;
        const exclusive = flags & O_EXCL;
        const truncate = flags & O_TRUNC;

        let meta = await this._getMetadata(path);

        // Follow symlinks
        if (meta && meta.type === 'symlink') {
            meta = await this._resolveSymlink(path);
        }

        if (!meta) {
            if (shouldCreate) {
                await this._ensureParentExists(path);
                const now = Math.floor(Date.now() / 1000);
                meta = {
                    path: path,
                    parent: this._dirname(path),
                    type: 'file',
                    mode: S_IFREG | 0o644,
                    mtime: now,
                    atime: now,
                    size: 0,
                    dataExternal: false,
                    data: new Uint8Array(0)
                };
                await this._putEntry(meta, new Uint8Array(0));
                this.metadataCache.invalidateByParent(meta.parent);
            } else {
                throw new IDBFSError('No such file or directory', 'ENOENT');
            }
        } else {
            if (shouldCreate && exclusive) {
                throw new IDBFSError('File exists', 'EEXIST');
            }
            if (truncate && meta.type !== 'dir') {
                meta.size = 0;
                meta.mtime = Math.floor(Date.now() / 1000);
                meta.dataExternal = false;
                meta.data = new Uint8Array(0);
                await this._putEntry(meta, new Uint8Array(0));
            }
        }

        return new File(this, path, meta, flags);
    }

    async mkdir(path, perm) {
        log("idbfs mkdir:", path, perm);
        path = this._normalizePath(path);
        await this._ensureParentExists(path);

        const existing = await this._getMetadata(path);
        if (existing) {
            throw new IDBFSError('File exists', 'EEXIST');
        }

        const now = Math.floor(Date.now() / 1000);
        const metadata = {
            path: path,
            parent: this._dirname(path),
            type: 'dir',
            mode: S_IFDIR | (perm & 0o777),
            mtime: now,
            atime: now,
            size: 0,
            dataExternal: false,
            data: null
        };
        
        await this._putEntry(metadata, null);
        this.metadataCache.invalidateByParent(metadata.parent);
    }

    async symlink(oldpath, newpath) {
        log("idbfs symlink:", oldpath, newpath);
        newpath = this._normalizePath(newpath);
        await this._ensureParentExists(newpath);

        const existing = await this._getMetadata(newpath);
        if (existing) {
            throw new IDBFSError('File exists', 'EEXIST');
        }

        const now = Math.floor(Date.now() / 1000);
        const target = new TextEncoder().encode(oldpath);
        const metadata = {
            path: newpath,
            parent: this._dirname(newpath),
            type: 'symlink',
            mode: S_IFLNK | 0o777,
            mtime: now,
            atime: now,
            size: target.length,
            dataExternal: false,
            data: target
        };
        
        await this._putEntry(metadata, target);
        this.metadataCache.invalidateByParent(metadata.parent);
    }

    async chtimes(path, atime, mtime) {
        log("idbfs chtimes:", path, atime, mtime);
        path = this._normalizePath(path);
        const meta = await this._getMetadata(path);
        if (!meta) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }

        meta.atime = atime;
        meta.mtime = mtime;
        await this._putEntry(meta); // metadata-only update
    }

    async chmod(path, mode) {
        log("idbfs chmod:", path, mode);
        path = this._normalizePath(path);
        const meta = await this._getMetadata(path);
        if (!meta) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }

        // Preserve file type bits, only update permission bits
        meta.mode = (meta.mode & S_IFMT) | (mode & 0o777);
        await this._putEntry(meta); // metadata-only update
    }

    async stat(path) {
        log("idbfs stat:", path);
        path = this._normalizePath(path);
        const meta = await this._getMetadata(path);
        if (!meta) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }

        return {
            name: this._basename(path),
            mode: meta.mode,
            mtime: meta.mtime,
            atime: meta.atime,
            size: meta.size
        };
    }

    async truncate(path, size) {
        log("idbfs truncate:", path, size);
        path = this._normalizePath(path);
        const meta = await this._resolveSymlink(path);
        if (!meta) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }
        if (meta.type === 'dir') {
            throw new IDBFSError('Is a directory', 'EISDIR');
        }

        // Get current data
        let currentData;
        if (meta.dataExternal) {
            const dataEntry = await this._getDataBlob(path);
            currentData = dataEntry ? dataEntry.data : new Uint8Array(0);
        } else {
            currentData = meta.data || new Uint8Array(0);
        }

        let newData;
        if (size < currentData.length) {
            newData = currentData.slice(0, size);
        } else if (size > currentData.length) {
            newData = new Uint8Array(size);
            newData.set(currentData);
        } else {
            newData = currentData;
        }

        meta.size = size;
        meta.mtime = Math.floor(Date.now() / 1000);
        await this._putEntry(meta, newData);
    }

    async remove(path) {
        log("idbfs remove:", path);
        path = this._normalizePath(path);
        const meta = await this._getMetadata(path);
        if (!meta) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }

        // If it's a directory, check if it's empty using index
        if (meta.type === 'dir') {
            const hasChildren = await this._hasChildren(path);
            if (hasChildren) {
                throw new IDBFSError('Directory not empty', 'ENOTEMPTY');
            }
        }

        await this._deleteEntry(path);
        
        // Invalidate parent directory cache
        this.metadataCache.invalidateByParent(meta.parent);
    }

    async rename(oldpath, newpath) {
        log("idbfs rename:", oldpath, newpath);
        oldpath = this._normalizePath(oldpath);
        newpath = this._normalizePath(newpath);

        const oldMeta = await this._getMetadata(oldpath);
        if (!oldMeta) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }

        await this._ensureParentExists(newpath);

        const newMeta = await this._getMetadata(newpath);
        if (newMeta) {
            if (newMeta.type === 'dir') {
                const hasChildren = await this._hasChildren(newpath);
                if (hasChildren) {
                    throw new IDBFSError('Directory not empty', 'ENOTEMPTY');
                }
            }
            await this.remove(newpath);
        }

        // If moving a directory, update all descendant paths in batch
        if (oldMeta.type === 'dir') {
            const descendants = await this._getAllDescendants(oldpath);
            const prefix = oldpath === '.' ? '' : oldpath + '/';
            const newPrefix = newpath === '.' ? '' : newpath + '/';
            
            const updates = [];
            const oldPaths = [];
            
            for (const entry of descendants) {
                oldPaths.push(entry.path);
                
                if (entry.path === oldpath) {
                    entry.path = newpath;
                    entry.parent = this._dirname(newpath);
                } else {
                    const relativePath = entry.path.slice(prefix.length);
                    entry.path = newPrefix + relativePath;
                    entry.parent = this._dirname(entry.path);
                }
                updates.push(entry);
            }
            
            // Batch update all in single transaction
            await this._batchUpdateMetadata(updates);
            await this._batchDelete(oldPaths);
            
            // Handle external data if any
            const db = await this._initDB();
            const hasExternalData = updates.some(u => u.dataExternal);
            
            if (hasExternalData) {
                // Batch rename data blobs too
                await new Promise((resolve, reject) => {
                    const transaction = db.transaction(['data'], 'readwrite');
                    const store = transaction.objectStore('data');
                    
                    transaction.oncomplete = () => {
                        // Clear caches
                        for (const path of oldPaths) {
                            this.dataCache.delete(path);
                        }
                        resolve();
                    };
                    transaction.onerror = () => reject(new IDBFSError('Failed to rename data', 'EIO'));
                    
                    for (let i = 0; i < oldPaths.length; i++) {
                        if (updates[i].dataExternal) {
                            const getRequest = store.get(oldPaths[i]);
                            getRequest.onsuccess = () => {
                                if (getRequest.result) {
                                    getRequest.result.path = updates[i].path;
                                    store.put(getRequest.result);
                                    store.delete(oldPaths[i]);
                                }
                            };
                        }
                    }
                });
            }
        } else {
            // For files and symlinks, create new entry at newpath and delete old
            const newMeta = {
                path: newpath,
                parent: this._dirname(newpath),
                type: oldMeta.type,
                mode: oldMeta.mode,
                mtime: oldMeta.mtime,
                atime: oldMeta.atime,
                size: oldMeta.size,
                dataExternal: oldMeta.dataExternal,
                data: oldMeta.data
            };
            
            // Get data if external
            let data;
            if (oldMeta.dataExternal) {
                const dataEntry = await this._getDataBlob(oldpath);
                data = dataEntry ? dataEntry.data : new Uint8Array(0);
            } else {
                data = oldMeta.data;
            }
            
            // Write new entry
            await this._putEntry(newMeta, data);
            
            // Delete old entry (both metadata and external data if any)
            await this._deleteEntry(oldpath);
        }
        
        // Invalidate caches
        this.metadataCache.invalidateByParent(oldMeta.parent);
        this.metadataCache.invalidateByParent(this._dirname(newpath));
    }

    async readlink(path) {
        log("idbfs readlink:", path);
        path = this._normalizePath(path);
        const meta = await this._getMetadata(path);
        if (!meta) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }
        if (meta.type !== 'symlink') {
            throw new IDBFSError('Not a symbolic link', 'EINVAL');
        }

        return new TextDecoder().decode(meta.data);
    }

    async readdir(path) {
        log("idbfs readdir:", path);
        path = this._normalizePath(path);
        const meta = await this._getMetadata(path);
        if (!meta) {
            // Try to resolve symlink
            const resolved = await this._resolveSymlink(path);
            if (!resolved) {
                throw new IDBFSError('No such file or directory', 'ENOENT');
            }
            if (resolved.type !== 'dir') {
                throw new IDBFSError('Not a directory', 'ENOTDIR');
            }
        } else if (meta.type !== 'dir') {
            throw new IDBFSError('Not a directory', 'ENOTDIR');
        }

        // Use index to get only direct children
        const children = await this._getChildren(path);
        
        return children.map(entry => ({
            name: this._basename(entry.path),
            mode: entry.mode,
            mtime: entry.mtime,
            atime: entry.atime,
            size: entry.size
        }));
    }
}

class File {
    constructor(fs, path, metadata, flags) {
        this.fs = fs;
        this.path = path;
        this.metadata = metadata;  // Only metadata, not full data
        this.flags = flags;
        this.position = 0;
        this.readdirOffset = 0;
        this.closed = false;
        this.dirty = false;
        this.dataCache = null;  // Cache data in file handle
    }

    async _ensureData() {
        if (this.dataCache !== null) return this.dataCache;
        
        if (this.metadata.dataExternal) {
            const dataEntry = await this.fs._getDataBlob(this.path);
            this.dataCache = dataEntry ? dataEntry.data : new Uint8Array(0);
        } else {
            this.dataCache = this.metadata.data || new Uint8Array(0);
        }
        return this.dataCache;
    }

    async close() {
        log("idbfs fclose:", this.path);
        if (this.closed) {
            throw new IDBFSError('File already closed', 'EBADF');
        }
        this.closed = true;
        
        if (this.dirty) {
            // Write back both metadata and data
            await this.fs._putEntry(this.metadata, this.dataCache);
        }
    }

    async stat() {
        log("idbfs fstat:", this.path);
        if (this.closed) {
            throw new IDBFSError('File closed', 'EBADF');
        }

        return {
            name: this.fs._basename(this.path),
            mode: this.metadata.mode,
            mtime: this.metadata.mtime,
            atime: this.metadata.atime,
            size: this.metadata.size
        };
    }

    async read(buf) {
        log("idbfs fread:", this.path, buf.length);
        if (this.closed) {
            throw new IDBFSError('File closed', 'EBADF');
        }

        const accessMode = this.flags & 3;
        if (accessMode === O_WRONLY) {
            throw new IDBFSError('File not open for reading', 'EBADF');
        }

        const data = await this._ensureData();
        const available = this.metadata.size - this.position;
        const toRead = Math.min(buf.length, available);

        if (toRead <= 0) {
            return null;
        }

        buf.set(data.slice(this.position, this.position + toRead));
        this.position += toRead;

        // Update access time
        this.metadata.atime = Math.floor(Date.now() / 1000);
        this.dirty = true;

        return toRead;
    }

    async write(data) {
        log("idbfs fwrite:", this.path, data.length);
        if (this.closed) {
            throw new IDBFSError('File closed', 'EBADF');
        }

        const accessMode = this.flags & 3;
        if (accessMode === O_RDONLY) {
            throw new IDBFSError('File not open for writing', 'EBADF');
        }

        const currentData = await this._ensureData();

        // Handle append mode
        if (this.flags & O_APPEND) {
            this.position = this.metadata.size;
        }

        const endPosition = this.position + data.length;
        const newSize = Math.max(this.metadata.size, endPosition);

        // Resize data if needed
        if (newSize > currentData.length) {
            const newData = new Uint8Array(newSize);
            newData.set(currentData);
            this.dataCache = newData;
        }

        // Write data
        this.dataCache.set(data, this.position);
        this.position = endPosition;
        this.metadata.size = newSize;
        this.metadata.mtime = Math.floor(Date.now() / 1000);
        this.metadata.atime = this.metadata.mtime;
        this.dirty = true;

        return data.length;
    }

    async seek(offset, whence) {
        log("idbfs fseek:", this.path, offset, whence);
        if (this.closed) {
            throw new IDBFSError('File closed', 'EBADF');
        }

        let newPosition;
        switch (whence) {
            case SEEK_SET:
                newPosition = offset;
                break;
            case SEEK_CUR:
                newPosition = this.position + offset;
                break;
            case SEEK_END:
                newPosition = this.metadata.size + offset;
                break;
            default:
                throw new IDBFSError('Invalid whence value', 'EINVAL');
        }

        if (newPosition < 0) {
            throw new IDBFSError('Invalid seek position', 'EINVAL');
        }

        this.position = newPosition;
        return this.position;
    }

    async readdir(count) {
        log("idbfs freaddir:", this.path, count);
        if (this.closed) {
            throw new IDBFSError('File closed', 'EBADF');
        }

        if (this.metadata.type !== 'dir') {
            throw new IDBFSError('Not a directory', 'ENOTDIR');
        }

        // Use optimized index query
        const children = await this.fs._getChildren(this.path);
        const childEntries = children.map(entry => ({
            name: this.fs._basename(entry.path),
            mode: entry.mode,
            mtime: entry.mtime,
            atime: entry.atime,
            size: entry.size
        }));

        // Return entries starting from current offset
        const startIdx = this.readdirOffset;
        let endIdx;
        if (count === -1) {
            endIdx = childEntries.length;
            this.readdirOffset = 0;
        } else {
            endIdx = Math.min(startIdx + count, childEntries.length);
            this.readdirOffset += endIdx - startIdx;
        }
        
        return childEntries.slice(startIdx, endIdx);
    }
}

// Export constants for external use
export { 
    S_IFMT, S_IFSOCK, S_IFLNK, S_IFREG, S_IFBLK, S_IFDIR, S_IFCHR, S_IFIFO,
    O_RDONLY, O_WRONLY, O_RDWR, O_CREAT, O_EXCL, O_TRUNC, O_APPEND,
    SEEK_SET, SEEK_CUR, SEEK_END,
    IDBFSError
};

if (typeof window !== 'undefined') {
    window.IDBFS = IDBFS;
}
