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
  const dir = (await window.parent.sys.call("fs.readdir", [path])).value;
  for (const idx in dir) {
    const stat = (await window.parent.sys.call("fs.stat", [`${path}/${dir[idx]}`])).value;
    if (stat.isDirectory) {
      dir[idx] += "/"; 
    }
  }
  let contents = null
  if (!path.endsWith("/")) {
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
                      <path d="M182.071 84.9674C184.832 84.9674 187.071 87.206 187.071 89.9674V212C187.071 214.761 184.832 217 182.071 217H54C51.2386 217 49 214.761 49 212V30.7942C49 28.0328 51.2386 25.7942 54 25.7942H122.898C125.659 25.7942 127.898 28.0328 127.898 30.7942V79.7344C127.898 82.5521 130.313 84.9674 133.131 84.9674H182.071Z" fill="currentColor"/>
                      <path d="M184.879 74.8787C186.769 76.7686 185.43 80 182.757 80H136C134.343 80 133 78.6569 133 77V30.2426C133 27.5699 136.231 26.2314 138.121 28.1213L184.879 74.8787Z" fill="currentColor"/>
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


