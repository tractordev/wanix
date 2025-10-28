// IDBFS - IndexedDB-based filesystem implementation
// Implements a POSIX-like filesystem interface using browser IndexedDB

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

class IDBFSError extends Error {
    constructor(message, code) {
        super(message);
        this.name = 'IDBFSError';
        this.code = code;
    }
}

export class IDBFS {
    constructor(name) {
        this.dbName = name;
        this.db = null;
    }

    async _initDB() {
        if (this.db) return this.db;

        await new Promise((resolve, reject) => {
            const request = indexedDB.open(this.dbName, 1);

            request.onerror = () => reject(new IDBFSError('Failed to open database', 'EIO'));

            request.onsuccess = () => {
                this.db = request.result;
                resolve(this.db);
            };

            request.onupgradeneeded = (event) => {
                const db = event.target.result;
                if (!db.objectStoreNames.contains('fs')) {
                    const objectStore = db.createObjectStore('fs', { keyPath: 'path' });
                    objectStore.createIndex('path', 'path', { unique: true });
                }
            };
        });

        // Ensure root directory exists
        await this._ensureRoot();
        
        return this.db;
    }

    async _ensureRoot() {
        const rootEntry = await this._getEntry('.');
        if (!rootEntry) {
            const now = Math.floor(Date.now() / 1000);
            await this._putEntry({
                path: '.',
                mode: S_IFDIR | 0o755,
                mtime: now,
                atime: now,
                size: 0,
                data: new Uint8Array(0)
            });
        }
    }

    async _getEntry(path) {
        const db = await this._initDB();
        return new Promise((resolve, reject) => {
            const transaction = db.transaction(['fs'], 'readonly');
            const store = transaction.objectStore('fs');
            const request = store.get(path);

            request.onsuccess = () => resolve(request.result);
            request.onerror = () => reject(new IDBFSError('Failed to get entry', 'EIO'));
        });
    }

    async _putEntry(entry) {
        const db = await this._initDB();
        return new Promise((resolve, reject) => {
            const transaction = db.transaction(['fs'], 'readwrite');
            const store = transaction.objectStore('fs');
            const request = store.put(entry);

            request.onsuccess = () => resolve();
            request.onerror = () => reject(new IDBFSError('Failed to put entry', 'EIO'));
        });
    }

    async _deleteEntry(path) {
        const db = await this._initDB();
        return new Promise((resolve, reject) => {
            const transaction = db.transaction(['fs'], 'readwrite');
            const store = transaction.objectStore('fs');
            const request = store.delete(path);

            request.onsuccess = () => resolve();
            request.onerror = () => reject(new IDBFSError('Failed to delete entry', 'EIO'));
        });
    }

    async _getAllEntries() {
        const db = await this._initDB();
        return new Promise((resolve, reject) => {
            const transaction = db.transaction(['fs'], 'readonly');
            const store = transaction.objectStore('fs');
            const request = store.getAll();

            request.onsuccess = () => resolve(request.result);
            request.onerror = () => reject(new IDBFSError('Failed to get all entries', 'EIO'));
        });
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
        
        const entry = await this._getEntry(parent);
        if (!entry) {
            throw new IDBFSError('Parent directory does not exist', 'ENOENT');
        }
        if (!this._isDir(entry.mode)) {
            throw new IDBFSError('Parent is not a directory', 'ENOTDIR');
        }
    }

    async _resolveSymlink(path, visited = new Set()) {
        if (visited.has(path)) {
            throw new IDBFSError('Too many levels of symbolic links', 'ELOOP');
        }
        visited.add(path);

        const entry = await this._getEntry(path);
        if (!entry) {
            return null;
        }

        if (this._isSymlink(entry.mode)) {
            const target = new TextDecoder().decode(entry.data);
            const resolvedPath = this._normalizePath(target);
            return this._resolveSymlink(resolvedPath, visited);
        }

        return entry;
    }

    async open(path) {
        path = this._normalizePath(path);
        const entry = await this._resolveSymlink(path);
        if (!entry) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }
        return new File(this, path, entry, O_RDWR);
    }

    async create(path) {
        path = this._normalizePath(path);
        await this._ensureParentExists(path);

        const existing = await this._getEntry(path);
        if (existing) {
            throw new IDBFSError('File exists', 'EEXIST');
        }

        const now = Math.floor(Date.now() / 1000);
        const entry = {
            path: path,
            mode: S_IFREG | 0o644,
            mtime: now,
            atime: now,
            size: 0,
            data: new Uint8Array(0)
        };
        await this._putEntry(entry);
        return new File(this, path, entry, O_RDWR);
    }

    async openfile(path, flags) {
        path = this._normalizePath(path);
        
        const shouldCreate = flags & O_CREAT;
        const exclusive = flags & O_EXCL;
        const truncate = flags & O_TRUNC;

        let entry = await this._getEntry(path);

        // Follow symlinks
        if (entry && this._isSymlink(entry.mode)) {
            entry = await this._resolveSymlink(path);
        }

        if (!entry) {
            if (shouldCreate) {
                await this._ensureParentExists(path);
                const now = Math.floor(Date.now() / 1000);
                entry = {
                    path: path,
                    mode: S_IFREG | 0o644,
                    mtime: now,
                    atime: now,
                    size: 0,
                    data: new Uint8Array(0)
                };
                await this._putEntry(entry);
            } else {
                throw new IDBFSError('No such file or directory', 'ENOENT');
            }
        } else {
            if (shouldCreate && exclusive) {
                throw new IDBFSError('File exists', 'EEXIST');
            }
            if (truncate && !this._isDir(entry.mode)) {
                entry.data = new Uint8Array(0);
                entry.size = 0;
                entry.mtime = Math.floor(Date.now() / 1000);
                await this._putEntry(entry);
            }
        }

        return new File(this, path, entry, flags);
    }

    async mkdir(path, perm) {
        path = this._normalizePath(path);
        await this._ensureParentExists(path);

        const existing = await this._getEntry(path);
        if (existing) {
            throw new IDBFSError('File exists', 'EEXIST');
        }

        const now = Math.floor(Date.now() / 1000);
        const entry = {
            path: path,
            mode: S_IFDIR | (perm & 0o777),
            mtime: now,
            atime: now,
            size: 0,
            data: new Uint8Array(0)
        };
        await this._putEntry(entry);
    }

    async symlink(oldpath, newpath) {
        newpath = this._normalizePath(newpath);
        await this._ensureParentExists(newpath);

        const existing = await this._getEntry(newpath);
        if (existing) {
            throw new IDBFSError('File exists', 'EEXIST');
        }

        const now = Math.floor(Date.now() / 1000);
        const entry = {
            path: newpath,
            mode: S_IFLNK | 0o777,
            mtime: now,
            atime: now,
            size: oldpath.length,
            data: new TextEncoder().encode(oldpath)
        };
        await this._putEntry(entry);
    }

    async chtimes(path, atime, mtime) {
        path = this._normalizePath(path);
        const entry = await this._getEntry(path);
        if (!entry) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }

        entry.atime = atime;
        entry.mtime = mtime;
        await this._putEntry(entry);
    }

    async chmod(path, mode) {
        path = this._normalizePath(path);
        const entry = await this._getEntry(path);
        if (!entry) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }

        // Preserve file type bits, only update permission bits
        entry.mode = (entry.mode & S_IFMT) | (mode & 0o777);
        await this._putEntry(entry);
    }

    async stat(path) {
        path = this._normalizePath(path);
        const entry = await this._getEntry(path);
        if (!entry) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }

        return {
            name: this._basename(path),
            mode: entry.mode,
            mtime: entry.mtime,
            atime: entry.atime,
            size: entry.size
        };
    }

    async truncate(path, size) {
        path = this._normalizePath(path);
        const entry = await this._resolveSymlink(path);
        if (!entry) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }
        if (this._isDir(entry.mode)) {
            throw new IDBFSError('Is a directory', 'EISDIR');
        }

        if (size < entry.size) {
            entry.data = entry.data.slice(0, size);
        } else if (size > entry.size) {
            const newData = new Uint8Array(size);
            newData.set(entry.data);
            entry.data = newData;
        }
        entry.size = size;
        entry.mtime = Math.floor(Date.now() / 1000);
        await this._putEntry(entry);
    }

    async remove(path) {
        path = this._normalizePath(path);
        const entry = await this._getEntry(path);
        if (!entry) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }

        // If it's a directory, check if it's empty
        if (this._isDir(entry.mode)) {
            const entries = await this._getAllEntries();
            const prefix = path === '.' ? '' : path + '/';
            const hasChildren = entries.some(e => 
                e.path !== path && 
                e.path.startsWith(prefix) && 
                e.path.slice(prefix.length).indexOf('/') === -1
            );
            if (hasChildren) {
                throw new IDBFSError('Directory not empty', 'ENOTEMPTY');
            }
        }

        await this._deleteEntry(path);
    }

    async rename(oldpath, newpath) {
        oldpath = this._normalizePath(oldpath);
        newpath = this._normalizePath(newpath);

        const oldEntry = await this._getEntry(oldpath);
        if (!oldEntry) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }

        await this._ensureParentExists(newpath);

        const newEntry = await this._getEntry(newpath);
        if (newEntry) {
            // If newpath exists, remove it first
            await this.remove(newpath);
        }

        // If moving a directory, update all child paths
        if (this._isDir(oldEntry.mode)) {
            const entries = await this._getAllEntries();
            const prefix = oldpath === '.' ? '' : oldpath + '/';
            const newPrefix = newpath === '.' ? '' : newpath + '/';
            
            for (const entry of entries) {
                if (entry.path === oldpath) {
                    entry.path = newpath;
                    await this._putEntry(entry);
                } else if (entry.path.startsWith(prefix)) {
                    const relativePath = entry.path.slice(prefix.length);
                    entry.path = newPrefix + relativePath;
                    await this._putEntry(entry);
                }
            }
            await this._deleteEntry(oldpath);
        } else {
            // For files and symlinks, just update the path
            oldEntry.path = newpath;
            await this._putEntry(oldEntry);
            await this._deleteEntry(oldpath);
        }
    }

    async readlink(path) {
        path = this._normalizePath(path);
        const entry = await this._getEntry(path);
        if (!entry) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }
        if (!this._isSymlink(entry.mode)) {
            throw new IDBFSError('Not a symbolic link', 'EINVAL');
        }

        return new TextDecoder().decode(entry.data);
    }

    async readdir(path) {
        path = this._normalizePath(path);
        const entry = await this._resolveSymlink(path);
        if (!entry) {
            throw new IDBFSError('No such file or directory', 'ENOENT');
        }
        if (!this._isDir(entry.mode)) {
            throw new IDBFSError('Not a directory', 'ENOTDIR');
        }

        const entries = await this._getAllEntries();
        const prefix = path === '.' ? '' : path + '/';
        const result = [];

        for (const entry of entries) {
            if (entry.path === path) continue;
            
            if (path === '.') {
                // Root directory: only include top-level entries
                if (entry.path.indexOf('/') === -1) {
                    result.push({
                        name: this._basename(entry.path),
                        mode: entry.mode,
                        mtime: entry.mtime,
                        atime: entry.atime,
                        size: entry.size
                    });
                }
            } else if (entry.path.startsWith(prefix)) {
                // Subdirectory: only include direct children
                const relativePath = entry.path.slice(prefix.length);
                if (relativePath.indexOf('/') === -1) {
                    result.push({
                        name: this._basename(entry.path),
                        mode: entry.mode,
                        mtime: entry.mtime,
                        atime: entry.atime,
                        size: entry.size
                    });
                }
            }
        }

        return result;
    }
}

class File {
    constructor(fs, path, entry, flags) {
        this.fs = fs;
        this.path = path;
        this.entry = entry;
        this.flags = flags;
        this.position = 0;
        this.readdirOffset = 0;
        this.closed = false;
    }

    async close() {
        if (this.closed) {
            throw new IDBFSError('File already closed', 'EBADF');
        }
        this.closed = true;
    }

    async stat() {
        if (this.closed) {
            throw new IDBFSError('File closed', 'EBADF');
        }

        // Refresh entry data
        this.entry = await this.fs._getEntry(this.path);
        if (!this.entry) {
            throw new IDBFSError('File removed', 'ENOENT');
        }

        return {
            name: this.fs._basename(this.path),
            mode: this.entry.mode,
            mtime: this.entry.mtime,
            atime: this.entry.atime,
            size: this.entry.size
        };
    }

    async read(buf) {
        if (this.closed) {
            throw new IDBFSError('File closed', 'EBADF');
        }

        const accessMode = this.flags & 3;
        if (accessMode === O_WRONLY) {
            throw new IDBFSError('File not open for reading', 'EBADF');
        }

        // Refresh entry data to get latest content
        this.entry = await this.fs._getEntry(this.path);
        if (!this.entry) {
            throw new IDBFSError('File removed', 'ENOENT');
        }

        const available = this.entry.size - this.position;
        const toRead = Math.min(buf.length, available);

        if (toRead <= 0) {
            return 0;
        }

        buf.set(this.entry.data.slice(this.position, this.position + toRead));
        this.position += toRead;

        // Update access time
        this.entry.atime = Math.floor(Date.now() / 1000);
        await this.fs._putEntry(this.entry);

        return toRead;
    }

    async write(data) {
        if (this.closed) {
            throw new IDBFSError('File closed', 'EBADF');
        }

        const accessMode = this.flags & 3;
        if (accessMode === O_RDONLY) {
            throw new IDBFSError('File not open for writing', 'EBADF');
        }

        // Refresh entry data
        this.entry = await this.fs._getEntry(this.path);
        if (!this.entry) {
            throw new IDBFSError('File removed', 'ENOENT');
        }

        // Handle append mode
        if (this.flags & O_APPEND) {
            this.position = this.entry.size;
        }

        const endPosition = this.position + data.length;
        const newSize = Math.max(this.entry.size, endPosition);

        // Resize data if needed
        if (newSize > this.entry.data.length) {
            const newData = new Uint8Array(newSize);
            newData.set(this.entry.data);
            this.entry.data = newData;
        }

        // Write data
        this.entry.data.set(data, this.position);
        this.position = endPosition;
        this.entry.size = newSize;
        this.entry.mtime = Math.floor(Date.now() / 1000);
        this.entry.atime = this.entry.mtime;

        await this.fs._putEntry(this.entry);

        return data.length;
    }

    async seek(offset, whence) {
        if (this.closed) {
            throw new IDBFSError('File closed', 'EBADF');
        }

        // Refresh entry data
        this.entry = await this.fs._getEntry(this.path);
        if (!this.entry) {
            throw new IDBFSError('File removed', 'ENOENT');
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
                newPosition = this.entry.size + offset;
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
        if (this.closed) {
            throw new IDBFSError('File closed', 'EBADF');
        }

        // Refresh entry data
        this.entry = await this.fs._getEntry(this.path);
        if (!this.entry) {
            throw new IDBFSError('File removed', 'ENOENT');
        }

        if (!this.fs._isDir(this.entry.mode)) {
            throw new IDBFSError('Not a directory', 'ENOTDIR');
        }

        // Get all entries in this directory
        const entries = await this.fs._getAllEntries();
        const prefix = this.path === '.' ? '' : this.path + '/';
        const childEntries = [];

        for (const entry of entries) {
            if (entry.path === this.path) continue;
            
            if (this.path === '.') {
                // Root directory: only include top-level entries
                if (entry.path.indexOf('/') === -1) {
                    childEntries.push({
                        name: this.fs._basename(entry.path),
                        mode: entry.mode,
                        mtime: entry.mtime,
                        atime: entry.atime,
                        size: entry.size
                    });
                }
            } else if (entry.path.startsWith(prefix)) {
                // Subdirectory: only include direct children
                const relativePath = entry.path.slice(prefix.length);
                if (relativePath.indexOf('/') === -1) {
                    childEntries.push({
                        name: this.fs._basename(entry.path),
                        mode: entry.mode,
                        mtime: entry.mtime,
                        atime: entry.atime,
                        size: entry.size
                    });
                }
            }
        }

        // Return entries starting from current offset, up to count
        const startIdx = this.readdirOffset;
        let endIdx;
        if (count === -1) {
            // Return all remaining entries
            endIdx = childEntries.length;

            this.readdirOffset = 0;
        } else {
            endIdx = Math.min(startIdx + count, childEntries.length);

            // Increment offset by the number of entries returned
            this.readdirOffset += endIdx - startIdx;
        }
        const result = childEntries.slice(startIdx, endIdx);
        return result;
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
