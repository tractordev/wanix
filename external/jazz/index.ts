import { BrowserLocalAuth } from "jazz-browser-auth-local";
import {
  //AuthProvider,
  createBinaryStreamFromBlob,
  readBlobFromBinaryStream,
  createInviteLink,
  // parseInviteLink,
  consumeInviteLinkFromWindowLocation,
  createBrowserNode,
} from "jazz-browser";
import { CoMap, BinaryCoStream } from "cojson";

export type Todo = CoMap<{
  title: string;
  completed: boolean;
  creator: string;
}>;
export type Todos = CoList<Todo["id"]>;

export type Collab = CoMap<{
  messages: Messages["id"];
  fs: Node["id"];
  todos: Todos["id"];
}>;
export type Messages = CoList<Message["id"]>;
export type Message = CoMap<{
  from: string;
  to: string;
  text: string;
  seen: boolean;
}>;
export type Node = CoMap<{ [name: string]: Node["id"]; dataID: BinaryCoStream["id"]; dataSize: number; }>;

// helper for jazzCall in fs/jazz.go
export function isPromise(value) {
  return value instanceof Promise;
}

function cleanPath(path) {
  return path.replace(/^.\//, '');
}

function dirname(path) {
  const dir = path.replace(/\\/g,'/').replace(/\/[^\/]*$/, '');
  if (dir === basename(path)) {
    return ""
  }
  return dir;
}

function basename(path) {
  return path.replace(/\\/g,'/').split('/').pop();
}

export function walkTo(v: any, path: string): any {
  path = cleanPath(path);
  if (path === "." || path === "") {
    return v;
  }
  let parts = path.replace(/^\//, '').split('/');
  let cur = v;
  for (const name of parts) {
    if (!name) {
      continue;
    }
    cur = cur[name];
    if (cur === undefined) {
      //console.log("not found:", path);
      return null;
    }
    if (cur.coMap.get("dataID")) {
      return cur;
    }
  }
  return cur;
}

export function root() {
  return window.wanix.collab.fs;
}

export function readdir(path) {
  path = cleanPath(path);
  const node = walkTo(root(), path);
  return Object.keys(node);
}

export async function mkdir(path) {
  path = cleanPath(path);
  if (path === "" || path === ".") {
    console.log("skipping jazz mkdir:", path);
    return
  }
  console.log("jazz mkdir:", path);
  let node;
  if (dirname(path) === "") {
    node = root();
  } else {
    node = walkTo(root(), dirname(path));
  }
  if (!node) {
    //mkdir(dirname(path));
    console.log("mkdir: no parent dir", dirname(path));
    return;
  }
  const dir = window.wanix.collab.group.createMap<Node>();
  node.mutate(n => {
    n.set(basename(path), dir.id);
  });
  return await waitFor(path);
}

export function remove(path) {
  path = cleanPath(path);
  const dir = walkTo(root(), dirname(path));
  if (!dir) {
    return null;
  }
  dir.mutate(n => {
    n.delete(basename(path));
  });
}

export function rename(oldpath, newpath) {
  oldpath = cleanPath(oldpath);
  newpath = cleanPath(newpath);
  const olddir = walkTo(root(), dirname(oldpath));
  if (!olddir) {
    return null;
  }
  const oldnode = olddir[basename(oldpath)];
  const newdir = walkTo(root(), dirname(newpath));
  if (!newdir) {
    return null;
  }
  newdir.mutate(n => {
    n.set(basename(newpath), oldnode);
  });
  olddir.mutate(n => {
    n.delete(basename(oldpath));
  });
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

export async function fetchFile(id) {
  const blob = await readBlobFromBinaryStream(id, window.localNode);
  return await blobToUint8Array(blob);
}

export async function makeFile(data) {
  const blob = new Blob([data], {type: "text/plain"});
  const bs = await createBinaryStreamFromBlob(blob, window.wanix.collab.group);
  return bs;
}

export async function readfile(path) {
  path = cleanPath(path);
  const node = walkTo(root(), path);
  if (!node) {
    return null;
  }
  const id = node.coMap.get("dataID");
  return await readBlobFromBinaryStream(id, window.localNode);
}

// export {createBinaryStreamFromBlob, readBlobFromBinaryStream }

function delay(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

export async function waitFor(path) {
  let node = walkTo(root(), path);
  let count = 0;
  while (!node) {
    await delay(50);
    node = walkTo(root(), path);
    count++;
    if (count > 20) {
      throw new Error("path not showing up")
    }
  }
  return node;
}

export async function makeNode(path) {
  path = cleanPath(path);
  let node = walkTo(root(), path);
  if (!node) {
    const dir = walkTo(root(), dirname(path));
    if (!dir) {
      throw new Error("makenode needs parent dir to exist");
    }
    node = window.wanix.collab.group.createMap<Node>();
    dir.mutate(d => {
      d.set(basename(path), node.id);
    })
  }
  node.mutate(n => {
    n.set("dataID", "");
    n.set("dataSize", 0);
  })
  return await waitFor(path);
}

export async function writefile(path, content) {
  path = cleanPath(path);
  let node = makeNode(path);
  if (typeof content === "string") {
    content = new Blob([content], {type: "text/plain"});
  }
  let bs = await createBinaryStreamFromBlob(content, window.wanix.collab.group);
  node.mutate(n => {
    n.set("dataID", bs.id);
    n.set("dataSize", content.size);
  })
}

var auth;

export function login() {
  if (auth.logIn) {
    auth.logIn();
  }
}
export function logout() {
  if (auth.logOut) {
    auth.logOut();
  }
}
export function signup(username) {
  if (auth.signUp) {
    auth.signUp(username);
  }
}

export function invite() {
  return createInviteLink(window.wanix.collab, "admin");
}

export function sendMessage(to, text) {
  window.wanix.collab.messages.mutate(m => m.append({from: window.account.profile.name, text, to}));
}

export function init(appname) {
  if (!appname) {
    appname = "dev";
  }
  const handlePromise = createBrowserNode({
    syncAddress: "ws://localhost:4200",
    auth: new BrowserLocalAuth(
      {
          onReady(next) {
            auth = next;
            console.log("jazz ready");
          },
          async onSignedIn(next) {
            auth = next;
            const handle = await handlePromise;
            window.account = {};
            window.localNode = handle.node;
            const ready = new Promise((resolve) => {
              handle.node.query(handle.node.account.id, (update) => {
                window.account = update;
                if (update.profile) {
                  resolve();
                }
              })
            });
            
  
            const listener = async () => {
              const acceptedInvitation = await consumeInviteLinkFromWindowLocation(handle.node);
        
              if (acceptedInvitation) {
                  window.location.hash = acceptedInvitation.valueID;
                  return;
              }
        
              let collabID = window.location.hash.slice(1) || undefined;
              if (!collabID) {
                const group = handle.node.createGroup();
                const root = group.createMap<Node>();
                const messages = group.createList<Messages>();
                const collab = group.createMap<Collab>();
                
                collab.mutate(c => {
                  c.set("messages", messages.id);
                  c.set("fs", root.id);
                })
          
                window.location.hash = collab.id;
              }
            };
            
            window.addEventListener("hashchange", listener);
            await listener();
  
            let collabID = window.location.hash.slice(1);
            
            window.wanix.collab = {};

            handle.node.query(collabID, (update) => {
              window.wanix.collab = update;
            });
  
            await ready;

            setTimeout(() => {
              let lastLength = window.wanix.collab.messages.length;
              handle.node.query(window.wanix.collab.messages.id, (messages) => {
                if (messages.length > lastLength) {
                  const m = messages[messages.length-1];
                  if (m && m.to === account.profile.name && m.seen !== true) {
                    alert(m.text);
                    lastLength = messages.length;
                  }
                }
              })
            }, 1000)
  
            console.log("jazz signed in as ", account.profile.name);
            
          },
      },
      appname
    )
  });
}

if (window.location.hash.slice(1).startsWith("co_") || window.location.hash.slice(1).startsWith("/invite/")) {
  init();
}