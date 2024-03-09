import {
  createBinaryStreamFromBlob,
  readBlobFromBinaryStream,
  autoSubResolution,
  autoSub
} from "jazz-browser";
import { CoMap, BinaryCoStream } from "cojson";

export type FileNode = CoMap<{
  name: string;
  isDir: boolean;
  size: number;
  mtime: number;
  fileID: BinaryCoStream["id"];
  entries: CoMap<{[name: string]: FileNode["id"]}>;
}>;


// helper for jazzCall in fs/jazz.go
export function isPromise(value) {
  return value instanceof Promise;
}

export function cleanPath(path) {
  return path.replace(/^.\//, '');
}

export function dirname(path) {
  path = cleanPath(path);
  const dir = path.replace(/\\/g,'/').replace(/\/[^\/]*$/, '');
  if (dir === basename(path)) {
    return "/"
  }
  return dir;
}

export function basename(path) {
  path = cleanPath(path);
  return path.replace(/\\/g,'/').split('/').pop();
}

export function unixTime() {
  return Math.floor(Date.now() / 1000);
}

function delay(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function ensureUint8Array(data) {
  if (typeof data === "string") {
    const enc = new TextEncoder("utf-8");
    data = enc.encode(data);
  }
  return new Uint8Array(data);
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


export function blobToString(blob) {
  return new Promise((resolve, reject) => {
      const reader = new FileReader();

      reader.onloadend = function() {
          resolve(reader.result);
      };

      reader.onerror = function() {
          reject(new Error("Failed to read blob"));
      };

      reader.readAsText(blob);
  });
}


export async function waitOn(f) {
  let res = f();
  let count = 0;
  while (!res) {
    await delay(50);
    res = f();
    count++;
    if (count > 25) {
      throw new Error("value not resolving")
    }
  }
  return res;
}

// file data

export async function fetchFile(id) {
  const blob = await readBlobFromBinaryStream(id, window.node);
  return await blobToUint8Array(blob);
}

export async function makeFile(data, group) {
  if (!group) {
    group = window.space.meta.group;
  }
  const blob = new Blob([data], {type: "text/plain"});
  const bs = await createBinaryStreamFromBlob(blob, group);
  return bs;
}

// file node

export async function makeDirNode(name, group) {
  if (!group) {
    group = window.space.meta.group;
  }
  const entries = group.createMap();
  const dir = group.createMap<FileNode>({
    name: name,
    isDir: true,
    size: 0,
    mtime: unixTime(),
    entries: entries
  });
  return await autoSubResolution(dir.id, (d) => d, window.node);
}

export async function makeFileNode(name, data, group) {
  if (!group) {
    group = window.space.meta.group;
  }
  data = ensureUint8Array(data);
  const file = await makeFile(data, group);
  return group.createMap<FileNode>({
    name: name,
    isDir: false,
    size: data.byteLength,
    mtime: unixTime(),
    fileID: file.id
  });
}

export async function nodeReload(n) {
  return await autoSubResolution(n.id, (n) => n, window.node);
}

export async function nodeDir(n) {
  const entries = await waitOn(() => n.entries);
  return Object.keys(entries);
}

export async function nodeFileGet(n) {
  if (n.meta) {
    return await fetchFile(n.meta.coValue.get("fileID"));
  } else {
    return await fetchFile(n.get("fileID"));
  }
}

export async function nodeFilePut(n, data) {
  data = ensureUint8Array(data);
  const file = await makeFile(data);
  n.mutate(n => {
    n.set("size", data.byteLength);
    n.set("mtime", unixTime());
    n.set("fileID", file.id);
  });
}

export async function nodeAdd(n, node) {
  const entries = await waitOn(() => n.entries);
  entries.mutate(n => {
    n.set(node.get("name"), node.id);
  });
  await nodeTouch(n);
}

export async function nodeTouch(n) {
  n.mutate(n => {
    n.set("mtime", unixTime());
  });
}

export async function nodeRemove(n, name) {
  const entries = await waitOn(() => n.entries);
  entries.mutate(n => {
    n.delete(name);
  });
  await nodeTouch(n);
}



export async function walk(path: string): any {
  path = cleanPath(path);
  let cur = await window.jazz.root();
  if (path === "." || path === "" || path === "/") {
    return cur;
  }
  let parts = path.replace(/^\//, '').split('/');
  for (const name of parts) {
    if (!name) {
      continue;
    }
    let names = await nodeDir(cur);
    if (!names.includes(name)) {
      return null;
    }
    cur = await waitOn(() => cur.entries[name]);
    if (cur === undefined) {
      return undefined;
    }
  }
  if (!cur.isDir && !mtimes[cur.id]) {
    // setup change tracking
    autoSub(cur.id, globalThis.node, (file) => {
      if (!mtimes[file.id]) {
        mtimes[file.id] = file?.meta.coValue.get("mtime");
        return;
      }
      if (mtimes[file.id] < file?.meta.coValue.get("mtime")) {
        const event = new CustomEvent("change", {detail: {path, ...file?.meta.coValue.toJSON()}});
        watches.dispatchEvent(event);
        mtimes[file.id] = file?.meta.coValue.get("mtime");
      }
    });
  }
  return cur;
}

// path operations

export async function stat(path) {
  const node = await walk(path);
  if (!node) {
    return null;
  }
  return node.meta.coValue.toJSON();
}

export async function readdir(path) {
  return await nodeDir(await walk(path));
}

export async function mkdir(path) {
  const node = await walk(dirname(path));
  if (!node) {
    console.log("mkdir: no parent dir", dirname(path));
    return;
  }
  const dir = await makeDirNode(basename(path));
  await nodeAdd(node, dir);
}

export async function mkdirAll(path) {
  const parts = cleanPath(path).split("/");
  let fullPath = "";
  const fullPaths = [];
  for (const part of parts) {
    fullPath += `/${part}`;
    fullPaths.push(fullPath);
  }
  let curParent = await window.jazz.root();;
  for (const dirpath of fullPaths) {
    const n = await walk(dirpath.slice(1));
    if (n) {
      curParent = n;
      continue;
    }
    const dir = await makeDirNode(basename(dirpath));
    await nodeAdd(curParent, dir);
    curParent = dir;
  }
}

export async function remove(path) {
  const n = await walk(dirname(path));
  if (!n) {
    return null;
  }
  await nodeRemove(n, basename(path));
}

export async function rename(oldpath, newpath) {
  const node = await walk(oldpath);
  if (!node) {
    return null;
  }
  if (basename(oldpath) !== basename(newpath)) {
    node.mutate(n => {
      n.set("name", basename(newpath));
    });
  }
  const olddir = await walk(dirname(oldpath));
  if (!olddir) {
    return null;
  }
  const newdir = await walk(dirname(newpath));
  if (!newdir) {
    return null;
  }
  await nodeAdd(newdir, node);
  await nodeRemove(olddir, basename(oldpath));
}

export async function readFile(path) {
  const node = await walk(path);
  if (!node) {
    return null;
  }
  return await nodeFileGet(node);
}


export async function writeFile(path, content) {
  const node = await walk(path);
  if (!node) {
    const dir = await walk(dirname(path));
    if (!dir) {
      return null;
    }
    const f = await makeFileNode(basename(path), content);
    await nodeAdd(dir, f);
    return true;
  }
  await nodeFilePut(node, content);
  return true;
}

// file watching

const mtimes = {};
const watches = new EventTarget();

export function watch(path, cb) {
  const listener = (e) => {
    if (e.detail.path.startsWith(path)) {
      cb(e);
    }
  }
  watches.addEventListener("change", listener);
  return listener;
}

export function unwatch(listener) {
  watches.removeEventListener("change", listener);
}