import { wasi, Fd, Inode } from "@bjorn3/browser_wasi_shim";

// Roughly https://developer.mozilla.org/en-US/docs/Web/API/FileSystemSyncAccessHandle
// but added open() and dropped ArrayBufferView as optional buffer type
export interface FileHandle {
  open(): void;

  // require open() to be called first
  close(): void;
  flush(): void;
  read(buffer: ArrayBuffer, options?: { at: number }): number;
  write(
    buffer: ArrayBuffer,
    options?: { at: number },
  ): number;

  // do not require open()
  getSize(): number;
  truncate(to: number): void;
}

export interface DirectoryHandle {
    readDir(): Map<string, Inode>;
    removeEntry(name: string): boolean;
    newEntry(name: string, isDir: boolean): Inode;
    createLink(name: string, entry: Inode): boolean;
    createFile(name: string, entry: Inode): boolean;
    createDirectory(name: string, entry: Inode): boolean;
}

// Synchronous access to an individual file in the origin private file system.
// Only allowed inside a WebWorker.
export class File extends Inode {
  handle: FileHandle;
  readonly: boolean;

  // FIXME needs a close() method to be called after start() to release the underlying handle
  constructor(
    handle: FileHandle,
    options?: Partial<{
      readonly: boolean;
    }>,
  ) {
    super();
    this.handle = handle;
    this.readonly = !!options?.readonly;
  }

  path_open(oflags: number, fs_rights_base: bigint, fd_flags: number) {
    if (
      this.readonly &&
      (fs_rights_base & BigInt(wasi.RIGHTS_FD_WRITE)) ==
        BigInt(wasi.RIGHTS_FD_WRITE)
    ) {
      // no write permission to file
      return { ret: wasi.ERRNO_PERM, fd_obj: null };
    }

    if ((oflags & wasi.OFLAGS_TRUNC) == wasi.OFLAGS_TRUNC) {
      if (this.readonly) return { ret: wasi.ERRNO_PERM, fd_obj: null };
      this.handle.truncate(0);
    }

    const file = new OpenFile(this);
    if (fd_flags & wasi.FDFLAGS_APPEND) file.fd_seek(0n, wasi.WHENCE_END);
    return { ret: wasi.ERRNO_SUCCESS, fd_obj: file };
  }

  get size(): bigint {
    return BigInt(this.handle.getSize());
  }

  stat(): wasi.Filestat {
    return new wasi.Filestat(this.ino, wasi.FILETYPE_REGULAR_FILE, this.size);
  }
}

export class OpenFile extends Fd {
  file: File;
  position: bigint = 0n;
  ino: bigint;

  constructor(file: File) {
    super();
    this.file = file;
    this.ino = Inode.issue_ino();
    
    this.file.handle.open();
  }

  fd_allocate(offset: bigint, len: bigint): number {
    if (BigInt(this.file.handle.getSize()) > offset + len) {
      // already big enough
    } else {
      // extend
      this.file.handle.truncate(Number(offset + len));
    }
    return wasi.ERRNO_SUCCESS;
  }


  fd_fdstat_get(): { ret: number; fdstat: wasi.Fdstat | null } {
    const size = this.file.handle.getSize();
    const fdstat = new wasi.Fdstat((size > 0) ? wasi.FILETYPE_REGULAR_FILE : wasi.FILETYPE_CHARACTER_DEVICE, 0);
    if (!this.file.readonly) {
      fdstat.fs_rights_base = BigInt(wasi.RIGHTS_FD_WRITE);
    }
    return { ret: 0, fdstat };
  }

  fd_filestat_get(): { ret: number; filestat: wasi.Filestat } {
    const size = this.file.handle.getSize();
    return {
      ret: 0,
      filestat: new wasi.Filestat(
        this.ino,
        (size > 0) ? wasi.FILETYPE_REGULAR_FILE : wasi.FILETYPE_CHARACTER_DEVICE,
        BigInt(size),
      ),
    };
  }

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  fd_filestat_set_size(size: bigint): number {
    this.file.handle.truncate(Number(size));
    return wasi.ERRNO_SUCCESS;
  }

  fd_read(size: number): { ret: number; data: Uint8Array } {
    const buf = new Uint8Array(size);
    const n = this.file.handle.read(buf, { at: Number(this.position) });
    this.position += BigInt(n);
    return { ret: 0, data: buf.slice(0, n) };
  }

  fd_seek(
    offset: number | bigint,
    whence: number,
  ): { ret: number; offset: bigint } {
    let calculated_offset: bigint;
    switch (whence) {
      case wasi.WHENCE_SET:
        calculated_offset = BigInt(offset);
        break;
      case wasi.WHENCE_CUR:
        calculated_offset = this.position + BigInt(offset);
        break;
      case wasi.WHENCE_END:
        calculated_offset = BigInt(this.file.handle.getSize()) + BigInt(offset);
        break;
      default:
        return { ret: wasi.ERRNO_INVAL, offset: 0n };
    }
    if (calculated_offset < 0) {
      return { ret: wasi.ERRNO_INVAL, offset: 0n };
    }
    this.position = calculated_offset;
    return { ret: wasi.ERRNO_SUCCESS, offset: this.position };
  }

  fd_write(data: Uint8Array): { ret: number; nwritten: number } {
    if (this.file.readonly) return { ret: wasi.ERRNO_BADF, nwritten: 0 };

    // don't need to extend file manually, just write
    const n = this.file.handle.write(data, { at: Number(this.position) });
    this.position += BigInt(n);
    return { ret: wasi.ERRNO_SUCCESS, nwritten: n };
  }

  fd_sync(): number {
    this.file.handle.flush();
    return wasi.ERRNO_SUCCESS;
  }
}
  
export class OpenDirectory extends Fd {
    dir: Directory;
  
    constructor(dir: Directory) {
      super();
      this.dir = dir;
    }
  
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    fd_seek(offset: bigint, whence: number): { ret: number; offset: bigint } {
      return { ret: wasi.ERRNO_BADF, offset: 0n };
    }
  
    fd_tell(): { ret: number; offset: bigint } {
      return { ret: wasi.ERRNO_BADF, offset: 0n };
    }
  
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    fd_allocate(offset: bigint, len: bigint): number {
      return wasi.ERRNO_BADF;
    }
  
    fd_fdstat_get(): { ret: number; fdstat: wasi.Fdstat | null } {
      return { ret: 0, fdstat: new wasi.Fdstat(wasi.FILETYPE_DIRECTORY, 0) };
    }
  
    fd_readdir_single(cookie: bigint): {
      ret: number;
      dirent: wasi.Dirent | null;
    } {
    //   if (debug.enabled) {
    //     debug.log("readdir_single", cookie);
    //     debug.log(cookie, this.dir.contents.keys());
    //   }
  
      if (cookie == 0n) {
        return {
          ret: wasi.ERRNO_SUCCESS,
          dirent: new wasi.Dirent(1n, this.dir.ino, ".", wasi.FILETYPE_DIRECTORY),
        };
      } else if (cookie == 1n) {
        return {
          ret: wasi.ERRNO_SUCCESS,
          dirent: new wasi.Dirent(
            2n,
            this.dir.parent_ino(),
            "..",
            wasi.FILETYPE_DIRECTORY,
          ),
        };
      }
  
      if (cookie >= BigInt(this.dir.contents.size) + 2n) {
        return { ret: 0, dirent: null };
      }
  
      const [name, entry] = Array.from(this.dir.contents.entries())[
        Number(cookie - 2n)
      ];
  
      return {
        ret: 0,
        dirent: new wasi.Dirent(
          cookie + 1n,
          entry.ino,
          name,
          entry.stat().filetype,
        ),
      };
    }
  
    path_filestat_get(
      flags: number,
      path_str: string,
    ): { ret: number; filestat: wasi.Filestat | null } {
      const { ret: path_err, path } = Path.from(path_str);
      if (path == null) {
        return { ret: path_err, filestat: null };
      }
  
      const { ret, entry } = this.dir.get_entry_for_path(path);
      if (entry == null) {
        return { ret, filestat: null };
      }
  
      return { ret: 0, filestat: entry.stat() };
    }
  
    path_lookup(
      path_str: string,
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      dirflags: number,
    ): { ret: number; inode_obj: Inode | null } {
      const { ret: path_ret, path } = Path.from(path_str);
      if (path == null) {
        return { ret: path_ret, inode_obj: null };
      }
  
      const { ret, entry } = this.dir.get_entry_for_path(path);
      if (entry == null) {
        return { ret, inode_obj: null };
      }
  
      return { ret: wasi.ERRNO_SUCCESS, inode_obj: entry };
    }
  
    path_open(
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      dirflags: number,
      path_str: string,
      oflags: number,
      fs_rights_base: bigint,
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      fs_rights_inheriting: bigint,
      fd_flags: number,
    ): { ret: number; fd_obj: Fd | null } {
      const { ret: path_ret, path } = Path.from(path_str);
      if (path == null) {
        return { ret: path_ret, fd_obj: null };
      }
  
      // eslint-disable-next-line prefer-const
      let { ret, entry } = this.dir.get_entry_for_path(path);
      if (entry == null) {
        if (ret != wasi.ERRNO_NOENT) {
          return { ret, fd_obj: null };
        }
        if ((oflags & wasi.OFLAGS_CREAT) == wasi.OFLAGS_CREAT) {
          // doesn't exist, but shall be created
          const { ret, entry: new_entry } = this.dir.create_entry_for_path(
            path_str,
            (oflags & wasi.OFLAGS_DIRECTORY) == wasi.OFLAGS_DIRECTORY,
          );
          if (new_entry == null) {
            return { ret, fd_obj: null };
          }
          entry = new_entry;
        } else {
          // doesn't exist, no such file
          return { ret: wasi.ERRNO_NOENT, fd_obj: null };
        }
      } else if ((oflags & wasi.OFLAGS_EXCL) == wasi.OFLAGS_EXCL) {
        // was supposed to be created exclusively, but exists already
        return { ret: wasi.ERRNO_EXIST, fd_obj: null };
      }
      if (
        (oflags & wasi.OFLAGS_DIRECTORY) == wasi.OFLAGS_DIRECTORY &&
        entry.stat().filetype !== wasi.FILETYPE_DIRECTORY
      ) {
        // expected a directory but the file is not a directory
        return { ret: wasi.ERRNO_NOTDIR, fd_obj: null };
      }
      return entry.path_open(oflags, fs_rights_base, fd_flags);
    }
  
    path_create_directory(path: string): number {
      return this.path_open(
        0,
        path,
        wasi.OFLAGS_CREAT | wasi.OFLAGS_DIRECTORY,
        0n,
        0n,
        0,
      ).ret;
    }
  
    path_link(path_str: string, inode: Inode, allow_dir: boolean): number {
      const { ret: path_ret, path } = Path.from(path_str);
      if (path == null) {
        return path_ret;
      }
  
      if (path.is_dir) {
        return wasi.ERRNO_NOENT;
      }
  
      const {
        ret: parent_ret,
        parent_entry,
        filename,
        entry,
      } = this.dir.get_parent_dir_and_entry_for_path(path, true);
      if (parent_entry == null || filename == null) {
        return parent_ret;
      }
  
      if (entry != null) {
        const source_is_dir = inode.stat().filetype == wasi.FILETYPE_DIRECTORY;
        const target_is_dir = entry.stat().filetype == wasi.FILETYPE_DIRECTORY;
        if (source_is_dir && target_is_dir) {
          if (allow_dir && entry instanceof Directory) {
            if (entry.contents.size == 0) {
              // Allow overwriting empty directories
            } else {
              return wasi.ERRNO_NOTEMPTY;
            }
          } else {
            return wasi.ERRNO_EXIST;
          }
        } else if (source_is_dir && !target_is_dir) {
          return wasi.ERRNO_NOTDIR;
        } else if (!source_is_dir && target_is_dir) {
          return wasi.ERRNO_ISDIR;
        } else if (
          inode.stat().filetype == wasi.FILETYPE_REGULAR_FILE &&
          entry.stat().filetype == wasi.FILETYPE_REGULAR_FILE
        ) {
          // Overwriting regular files is fine
        } else {
          return wasi.ERRNO_EXIST;
        }
      }
  
      if (!allow_dir && inode.stat().filetype == wasi.FILETYPE_DIRECTORY) {
        return wasi.ERRNO_PERM;
      }
  
      parent_entry.createLink(filename, inode);
  
      return wasi.ERRNO_SUCCESS;
    }
  
    path_unlink(path_str: string): { ret: number; inode_obj: Inode | null } {
      const { ret: path_ret, path } = Path.from(path_str);
      if (path == null) {
        return { ret: path_ret, inode_obj: null };
      }
  
      const {
        ret: parent_ret,
        parent_entry,
        filename,
        entry,
      } = this.dir.get_parent_dir_and_entry_for_path(path, true);
      if (parent_entry == null || filename == null) {
        return { ret: parent_ret, inode_obj: null };
      }
  
      if (entry == null) {
        return { ret: wasi.ERRNO_NOENT, inode_obj: null };
      }
  
      parent_entry.removeEntry(filename);
  
      return { ret: wasi.ERRNO_SUCCESS, inode_obj: entry };
    }
  
    path_unlink_file(path_str: string): number {
      const { ret: path_ret, path } = Path.from(path_str);
      if (path == null) {
        return path_ret;
      }
  
      const {
        ret: parent_ret,
        parent_entry,
        filename,
        entry,
      } = this.dir.get_parent_dir_and_entry_for_path(path, false);
      if (parent_entry == null || filename == null || entry == null) {
        return parent_ret;
      }
      if (entry.stat().filetype === wasi.FILETYPE_DIRECTORY) {
        return wasi.ERRNO_ISDIR;
      }
      parent_entry.removeEntry(filename);
      return wasi.ERRNO_SUCCESS;
    }
  
    path_remove_directory(path_str: string): number {
      const { ret: path_ret, path } = Path.from(path_str);
      if (path == null) {
        return path_ret;
      }
  
      const {
        ret: parent_ret,
        parent_entry,
        filename,
        entry,
      } = this.dir.get_parent_dir_and_entry_for_path(path, false);
      if (parent_entry == null || filename == null || entry == null) {
        return parent_ret;
      }
  
      if (
        !(entry instanceof Directory) ||
        entry.stat().filetype !== wasi.FILETYPE_DIRECTORY
      ) {
        return wasi.ERRNO_NOTDIR;
      }
      entry.syncEntries();
      if (entry.contents.size !== 0) {
        return wasi.ERRNO_NOTEMPTY;
      }
      if (!parent_entry.removeEntry(filename)) {
        return wasi.ERRNO_NOENT;
      }
      return wasi.ERRNO_SUCCESS;
    }
  
    fd_filestat_get(): { ret: number; filestat: wasi.Filestat } {
      return { ret: 0, filestat: this.dir.stat() };
    }
  
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    fd_filestat_set_size(size: bigint): number {
      return wasi.ERRNO_BADF;
    }
  
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    fd_read(size: number): { ret: number; data: Uint8Array } {
      return { ret: wasi.ERRNO_BADF, data: new Uint8Array() };
    }
  
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    fd_pread(size: number, offset: bigint): { ret: number; data: Uint8Array } {
      return { ret: wasi.ERRNO_BADF, data: new Uint8Array() };
    }
  
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    fd_write(data: Uint8Array): { ret: number; nwritten: number } {
      return { ret: wasi.ERRNO_BADF, nwritten: 0 };
    }
  
    fd_pwrite(
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      data: Uint8Array,
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      offset: bigint,
    ): { ret: number; nwritten: number } {
      return { ret: wasi.ERRNO_BADF, nwritten: 0 };
    }
  }
  
  export class PreopenDirectory extends OpenDirectory {
    prestat_name: string;
  
    constructor(name: string, dir: Directory) {
      super(dir);
      this.prestat_name = name;
    }
  
    fd_prestat_get(): { ret: number; prestat: wasi.Prestat | null } {
      return {
        ret: 0,
        prestat: wasi.Prestat.dir(this.prestat_name),
      };
    }
  }
  
  class Path {
    parts: string[] = [];
    is_dir: boolean = false;
  
    static from(path: string): { ret: number; path: Path | null } {
      const self = new Path();
      self.is_dir = path.endsWith("/");
  
      if (path.startsWith("/")) {
        return { ret: wasi.ERRNO_NOTCAPABLE, path: null };
      }
      if (path.includes("\0")) {
        return { ret: wasi.ERRNO_INVAL, path: null };
      }
  
      for (const component of path.split("/")) {
        if (component === "" || component === ".") {
          continue;
        }
        if (component === "..") {
          if (self.parts.pop() == undefined) {
            return { ret: wasi.ERRNO_NOTCAPABLE, path: null };
          }
          continue;
        }
        self.parts.push(component);
      }
  
      return { ret: wasi.ERRNO_SUCCESS, path: self };
    }
  
    to_path_string(): string {
      let s = this.parts.join("/");
      if (this.is_dir) {
        s += "/";
      }
      return s;
    }
  }
  
  export class Directory extends Inode {
    contents: Map<string, Inode>;
    private parent: Directory | null = null;
    private handle: DirectoryHandle;
  
    constructor(handle: DirectoryHandle) {
      super();
      this.handle = handle;
    }

    syncEntries() {
        this.contents = this.handle.readDir();
        for (const entry of this.contents.values()) {
            if (entry instanceof Directory) {
              entry.parent = this;
            }
        }
    }

    removeEntry(name: string): boolean {
        if (this.handle.removeEntry(name)) {
            return this.contents.delete(name);
        }
        return false;
    }

    createLink(name: string, entry: Inode): void {
        if (this.handle.createLink(name, entry)) {
            this.contents.set(name, entry);
        }
    }

    createFile(name: string, entry: Inode) {
        if (this.handle.createFile(name, entry)) {
            this.contents.set(name, entry);
        }
    }

    createDirectory(name: string, entry: Inode) {
        if (this.handle.createDirectory(name, entry)) {
            this.contents.set(name, entry);
        }
    }
  
    parent_ino(): bigint {
      if (this.parent == null) {
        return Inode.root_ino();
      }
      return this.parent.ino;
    }
  
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    path_open(oflags: number, fs_rights_base: bigint, fd_flags: number) {
        this.syncEntries();
        return { ret: wasi.ERRNO_SUCCESS, fd_obj: new OpenDirectory(this) };
    }
  
    stat(): wasi.Filestat {
      return new wasi.Filestat(this.ino, wasi.FILETYPE_DIRECTORY, 0n);
    }
  
    get_entry_for_path(path: Path): { ret: number; entry: Inode | null } {
        let entry: Inode = this;
        for (const component of path.parts) {
            if (!(entry instanceof Directory)) {
                return { ret: wasi.ERRNO_NOTDIR, entry: null };
            }
            entry.syncEntries();
            const child = entry.contents.get(component);
            if (child !== undefined) {
                entry = child;
            } else {
                //   debug.log(component);
                return { ret: wasi.ERRNO_NOENT, entry: null };
            }
        }
  
        if (path.is_dir) {
            if (entry.stat().filetype != wasi.FILETYPE_DIRECTORY) {
                return { ret: wasi.ERRNO_NOTDIR, entry: null };
            }
        }
  
      return { ret: wasi.ERRNO_SUCCESS, entry };
    }
  
    get_parent_dir_and_entry_for_path(
      path: Path,
      allow_undefined: boolean,
    ): {
      ret: number;
      parent_entry: Directory | null;
      filename: string | null;
      entry: Inode | null;
    } {
      const filename = path.parts.pop();
  
      if (filename === undefined) {
        return {
          ret: wasi.ERRNO_INVAL,
          parent_entry: null,
          filename: null,
          entry: null,
        };
      }
  
      const { ret: entry_ret, entry: parent_entry } =
        this.get_entry_for_path(path);
      if (parent_entry == null) {
        return {
          ret: entry_ret,
          parent_entry: null,
          filename: null,
          entry: null,
        };
      }
      if (!(parent_entry instanceof Directory)) {
        return {
          ret: wasi.ERRNO_NOTDIR,
          parent_entry: null,
          filename: null,
          entry: null,
        };
      }
      parent_entry.syncEntries();
      const entry: Inode | undefined | null = parent_entry.contents.get(filename);
      if (entry === undefined) {
        if (!allow_undefined) {
          return {
            ret: wasi.ERRNO_NOENT,
            parent_entry: null,
            filename: null,
            entry: null,
          };
        } else {
          return { ret: wasi.ERRNO_SUCCESS, parent_entry, filename, entry: null };
        }
      }
  
      if (path.is_dir) {
        if (entry.stat().filetype != wasi.FILETYPE_DIRECTORY) {
          return {
            ret: wasi.ERRNO_NOTDIR,
            parent_entry: null,
            filename: null,
            entry: null,
          };
        }
      }
  
      return { ret: wasi.ERRNO_SUCCESS, parent_entry, filename, entry };
    }
  
    create_entry_for_path(
      path_str: string,
      is_dir: boolean,
    ): { ret: number; entry: Inode | null } {
      const { ret: path_ret, path } = Path.from(path_str);
      if (path == null) {
        return { ret: path_ret, entry: null };
      }
  
      let {
        // eslint-disable-next-line prefer-const
        ret: parent_ret,
        // eslint-disable-next-line prefer-const
        parent_entry,
        // eslint-disable-next-line prefer-const
        filename,
        entry,
      } = this.get_parent_dir_and_entry_for_path(path, true);
      if (parent_entry == null || filename == null) {
        return { ret: parent_ret, entry: null };
      }
  
      if (entry != null) {
        return { ret: wasi.ERRNO_EXIST, entry: null };
      }
  
    //   debug.log("create", path);
      let new_child;
      if (!is_dir) {
        new_child = parent_entry.handle.newEntry(filename, false);
        parent_entry.createFile(filename, new_child);
      } else {
        new_child = parent_entry.handle.newEntry(filename, true);
        parent_entry.createDirectory(filename, new_child);
      }
      entry = new_child;
  
      return { ret: wasi.ERRNO_SUCCESS, entry };
    }
  }
  