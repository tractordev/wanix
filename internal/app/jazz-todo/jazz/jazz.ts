import { BrowserAuth0 } from "jazz-browser-auth0";
import {
  autoSub,
  autoSubResolution,
  createBrowserNode,
  createBinaryStreamFromBlob,
  readBlobFromBinaryStream,
  createInviteLink,
  consumeInviteLinkFromWindowLocation,
} from "jazz-browser";

export {
  autoSub,
  autoSubResolution,
  createInviteLink
};


export async function setupSpace(node, migration) {
  const listener = async () => {
    const acceptedInvitation = await consumeInviteLinkFromWindowLocation(node);

    if (acceptedInvitation) {
        parent.location.hash = acceptedInvitation.valueID;
        return;
    }

    let spaceID = parent.location.hash.slice(1) || undefined;
    if (!spaceID) {
      const group = node.createGroup();
      const space = group.createMap();
      
      if (migration) {
        migration(space);
      }

      parent.location.hash = space.id;
    }
  };
  
  parent.addEventListener("hashchange", listener);
  await listener();

  return await autoSubResolution(parent.location.hash.slice(1), (s) => s, node);
}

export async function initNode(name, domain, clientID, accessToken, migration) {
  const { node, done } = await createBrowserNode({
    auth: new BrowserAuth0({domain, clientID, accessToken}, domain, name),
    migration
  });
  return node;
}
