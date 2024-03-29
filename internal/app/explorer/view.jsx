function delay(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function dirname(path) {
  const dir = path.replace(/\\/g,'/').replace(/\/[^\/]*$/, '');
  if (dir === basename(path)) {
    return "";
  }
  if (dir === "") {
    return "/";
  }
  return dir;
}

function basename(path) {
  return path.replace(/\\/g,'/').split('/').pop();
}

async function save(path, goto) {
  const enc = new TextEncoder("utf-8");
  const contents = enc.encode(document.querySelector("textarea").value);
  await window.parent.sys.call("fs.writeFile", [path, contents, 0o644]);
  goto(path);
}

export async function load(path) {
  let dir = [];
  let contents = null
  const stat = (await window.parent.sys.call("fs.stat", [path.replace(/\/$/, "")])).value;
  if (stat.isDirectory) {
    dir = (await window.parent.sys.call("fs.readdir", [path])).value;
    for (const idx in dir) {
      const substat = (await window.parent.sys.call("fs.stat", [`${path}/${dir[idx]}`])).value;
      if (substat.isDirectory) {
        dir[idx] += "/"; 
      }
    }
  } else {
    const resp = await window.parent.sys.call("fs.readFile", [path]);
    const dec = new TextDecoder("utf-8");
    contents = dec.decode(resp.value);
  }
  return {dir, contents, path}
}

// this is all just kind of hacked together
// and should be rewritten/redesigned

export const Explorer = {
  view: ({attrs,state}) => {
    state.file = (state.file === undefined) ? attrs.file : state.file;
    attrs.file = state.file;

    attrs.goto = async (path) => {
      state.file = await load(path);
      m.redraw();
    }
    return attrs.file.path.endsWith("/") ? <DirView {...attrs} /> : <FileView {...attrs} />;
  }
}

const DirView = {
  view: ({attrs: {file, goto}}) => {
    const login = JSON.parse(localStorage.getItem("auth:login"));
    let username = "";
    if (login && login["user"]) {
      username = login["user"]["nickname"];
    }
    let nicePath = file.path.replace(/\/$/, "");
    if (nicePath === "") {
      nicePath = "/";
    }
    return (
      <main>
        <header class="p-4 pb-8 mb-4">
          <a href="javascript:void(0)" onclick={() => (file.path !== "/") ? goto((dirname(nicePath)+"/").replace("//", "/")) : null}>
            <div class="text-xs text-gray-400 mb-2">&lt;-- up</div>
          </a>
          <div class="username">{username}</div>
          <div class="project font-bold mb-2 text-xl">{nicePath}</div>

          <div class="button-bar flex justify-between">
            <div>
              <input class="bg-gray-800 py-1 px-2 rounded-sm w-96 border border-gray-700" type="search" placeholder="Filter files" />
              <button class="hidden bg-lime-300 py-1 px-3 rounded-sm text-lime-900">Search</button>
            </div>
            <button class="bg-gray-600 p-1 px-2 rounded-sm">+ New File</button>
          </div>

        </header>
        <div class="content">
          {file.dir.map(d => (
            <a href="javascript:void(0)" onclick={() => goto(`${file.path}${d}`)}>
              <div class="file-row flex justify-start items-center gap-4 p-4">
                <div class="filetype-image w-12 h-12 flex items-center justify-center bg-gray-800 rounded-sm">
                  {(d.endsWith("/")) ?
                    <svg class="icon-folder" width="240" height="240" viewBox="0 0 240 240" fill="none" xmlns="http://www.w3.org/2000/svg">
                      <path d="M95.8586 62.2518H208.569C216 62.2518 217.097 64.0745 220 67V62.2518C220 53.5 217 51 208.569 51H89L95.8586 62.2518Z" fill="currentColor"/>
                      <path d="M208.58 66.8788H93.3928L84.3478 51.7788C80.5 45 79 44 70.6433 44H30.4205C21.5 44 19 47 19 55.4394V183.561C19 192.5 19 195 30.4205 195H208.58C217.5 195 220 192 220 183.561V78.3182C220 70 217.5 66.8788 208.58 66.8788Z" fill="currentColor"/>
                    </svg>
                  : <svg id="icon-document" width="240" height="240" viewBox="0 0 240 240" fill="none" xmlns="http://www.w3.org/2000/svg">
                      <path d="M184.879 74.8787C186.769 76.7686 185.43 80 182.757 80H136C134.343 80 133 78.6569 133 77V30.2426C133 27.5699 136.231 26.2314 138.121 28.1213L184.879 74.8787Z" fill="currentColor"/>
                      <path fill-rule="evenodd" clip-rule="evenodd" d="M187.071 89.9674C187.071 87.2059 184.832 84.9674 182.071 84.9674H133.131C130.313 84.9674 127.898 82.5521 127.898 79.7344V30.7942C127.898 28.0328 125.659 25.7942 122.898 25.7942H54C51.2386 25.7942 49 28.0328 49 30.7942V212C49 214.761 51.2386 217 54 217H182.071C184.832 217 187.071 214.761 187.071 212V89.9674ZM69 117C67.3431 117 66 118.343 66 120C66 121.657 67.3431 123 69 123H170C171.657 123 173 121.657 173 120C173 118.343 171.657 117 170 117H69ZM66 144C66 142.343 67.3431 141 69 141H170C171.657 141 173 142.343 173 144C173 145.657 171.657 147 170 147H69C67.3431 147 66 145.657 66 144ZM69 165C67.3431 165 66 166.343 66 168C66 169.657 67.3431 171 69 171H170C171.657 171 173 169.657 173 168C173 166.343 171.657 165 170 165H69Z" fill="currentColor"/>
                    </svg>
                  }
                  
                </div>
                
                <div class="file-name">
                  <div class="file-name-header">
                      {d}
                  </div>
                </div>
              </div>
            </a>
          ))}          
        </div>
      </main>
    )
  }
}

const FileView = {
  view: ({attrs: {file,goto}}) => {
    return (
      <main>
        <header class="p-4 pb-8 mb-4 flex justify-between items-end">
          <div>
            <a href="javascript:void(0)" onclick={() => goto((dirname(file.path.replace(/\/$/, ""))+"/").replace("//", "/"))}>
              <div class="text-xs text-gray-400 mb-2">&lt;-- back</div>
            </a>
            <div class="project">{dirname(file.path)}</div>
            <div class="project font-bold leading-none text-xl">{basename(file.path)}</div>
          </div>

          <div class="flex gap-1">
            <button id="button-delete-file" class="bg-red-700 p-1 px-2 rounded-sm">Delete</button>
            <button onclick={() => save(file.path, goto)} class="bg-gray-600 p-1 px-2 rounded-sm">Save</button>
          </div>

        </header>

        <div class="content p-4">
          <textarea class="w-full h-96 bg-gray-800 border border-gray-700 box-border rounded-sm py-1 px-2">
            {file.contents||""}
          </textarea>
        </div>
      </main>
    )
  }
}


