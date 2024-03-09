import { BrowserAuth0 } from "jazz-browser-auth0";
import {
  autoSub,
  autoSubResolution,
  createBrowserNode,
  parseInviteLink,
} from "jazz-browser";
import {cojsonInternals} from "cojson";
import * as fs from "./fs.ts";

export {
  autoSub,
  autoSubResolution,
};



export function createInviteLinkHash(value, role) {
  const coValueCore =
        "coValueType" in value ? value.meta.coValue.core : value.core;
  let currentCoValue = coValueCore;

  while (currentCoValue.header.ruleset.type === "ownedByGroup") {
      currentCoValue = currentCoValue.getGroup().core;
  }

  if (currentCoValue.header.ruleset.type !== "group") {
      throw new Error("Can't create invite link for object without group");
  }

  const group = cojsonInternals.expectGroup(
    currentCoValue.getCurrentContent()
  )
  return `#/invite/${value.id}/${group.createInvite(role)}`;
}

export async function setupFrameSpace(node, migration) {
  const listener = async () => {
    const invite = parseInviteLink(parent.location.href);
    if (invite) {
      try {
        await node.acceptInvite(invite.valueID, invite.inviteSecret);
      } catch {
        console.warn("invite not accepted");
        return;
      }
      parent.location.hash = invite.valueID;
    }
    
    let spaceID = parent.location.hash.slice(1) || undefined;
    if (!spaceID) {
      const group = node.createGroup();
      const space = group.createMap();
      if (migration) {
        await migration(space, group);
      }
      parent.location.hash = space.id;
    }
  };
  
  parent.addEventListener("hashchange", listener);
  await listener();

  return await autoSubResolution(parent.location.hash.slice(1), (s) => s, node);
}

export async function setupWorkerSpace(node, migration) {
  const hostHash = async (value) => {
    if (value) {
      await globalThis.sys.call("host.setHash", [value]);
      return;
    }
    return (await globalThis.sys.call("host.hash", [])).value;
  }
  const url = location.origin+"/#"+(await hostHash());

  const invite = parseInviteLink(url);
  if (invite) {
    try {
      await node.acceptInvite(invite.valueID, invite.inviteSecret);
    } catch {
      console.warn("invite not accepted");
      return;
    }
    await hostHash(invite.valueID);
  }

  const spaceID = await hostHash() || undefined;
  if (!spaceID) {
    const group = node.createGroup();
    const space = group.createMap();
    if (migration) {
      await migration(space, group);
    }
    await hostHash(space.id);
  }

  return await autoSubResolution(await hostHash(), (s) => s, node);
}

export async function initNode(name, domain, clientID, accessToken, migration) {
  const { node, done } = await createBrowserNode({
    auth: new BrowserAuth0({domain, clientID, accessToken}, domain, name),
    migration
  });
  return node;
}

export async function initJazz(globalObj) {
  let setupFn = undefined;
  let getItem = undefined;
  if (globalObj.hostURL) {
    // in worker
    setupFn = setupWorkerSpace;
    getItem = async (key) => {return (await globalObj.sys.call("host.getItem", [key])).value};
  } else {
    // in frame
    setupFn = setupFrameSpace
    getItem = async (key) => globalObj.localStorage.getItem(key);
  }

  const jazzEnabled = await getItem("jazz:enabled");
  if (!jazzEnabled) {
    console.warn("jazz not enabled");
    return null;
  }
  const settingsData = await getItem("auth:settings");
  if (!settingsData) {
    console.warn("auth settings not found")
    return null;
  }
  const loginData = await getItem("auth:login");
  if (!loginData) {
    console.warn("auth login not found")
    return null;
  }
  const settings = JSON.parse(settingsData);
  const login = JSON.parse(loginData);

  const node = await initNode(
    "wanix:"+location.origin.split("//")[1], 
    settings["domain"],
    settings["clientId"],
    login["access_token"],
    (account) => { },
  );

  // hack for fs api
  globalThis.node = node;
  
  const space = await setupFn(node, async (space, group) => {
    const root = await fs.makeDirNode("/", group);
    space.mutate(s => {
      s.set("fs", root.id);
    });
  });

  // hack for fs api
  globalThis.space = space;

  const username = await autoSubResolution("me", (me) => me.profile?.name, node);
  const inviteURL = location.origin+"/"+createInviteLinkHash(space, "admin");

  globalObj.jazz = {
    node,
    space,
    username,
    inviteURL,
    fsutil: fs,
    root: async () => await autoSubResolution(space.id, (s) => s?.fs, node)
  };
}