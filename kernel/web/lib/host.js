const baseURL = window.location.pathname.replace("index.html", "").replace(/\/$/, '');

globalThis.sys.pipe.handle("host.currentUser", duplex.handlerFrom(() => {
    const login = localStorage.getItem("auth:login");
    if (!login) {
        return null;
    }
    return JSON.parse(login)["user"] || null;
}));
globalThis.sys.pipe.handle("host.login", duplex.handlerFrom(() => {
    localStorage.setItem("auth:redirect", window.location.href);
    window.location.href = "/auth/";  
}));
globalThis.sys.pipe.handle("host.logout", duplex.handlerFrom(() => {
    localStorage.setItem("auth:redirect", window.location.href);
    window.location.href = "/auth/?logout";
}));


globalThis.sys.pipe.handle("host.loadStylesheet", duplex.handlerFrom((path) => {
  const style = document.createElement("link");
  style.rel = "stylesheet";
  style.href = baseURL+path;
  document.body.appendChild(style);
}));
globalThis.sys.pipe.handle("host.loadApp", duplex.handlerFrom((target, path, focus) => {
  let frame = document.querySelector("#"+target);
  if (frame) {
      if (!path) {
          frame.onload = (e) => {
              frame.contentDocument.addEventListener("keydown", visorKeydown);
          }
          frame.contentWindow.location.reload();
          return;
      }
  } else {
      frame = document.createElement("iframe");
      frame.setAttribute("id", target);
      document.body.appendChild(frame);
  }
  frame.onload = (e) => {
      if (target !== "terminal") {
          const tclass = document.querySelector("#terminal").classList;
          tclass.add("visor");
          if (!tclass.replace("open", "closed")) {
              tclass.add("closed");
          }
      }
      frame.contentDocument.addEventListener("keydown", visorKeydown);
      if (focus) {
          frame.focus();
      }
      frame.onload = null;
  }
  frame.setAttribute("src", baseURL+path);
}));

globalThis.sys.pipe.handle("host.download", duplex.handlerFrom((filename, data) => {
  const blob = new Blob([data], {type: "application/octet-stream"});
  const url = URL.createObjectURL(blob);
  
  const elem = document.createElement("a");
  elem.setAttribute("download", filename);
  elem.href = url;
  elem.setAttribute("target", "_blank");
  elem.click();

  elem.remove();
  URL.revokeObjectURL(url);
}));


const visorKeydown = (e) => {
  const el = document.querySelector("#terminal");
  if (e.code === "Backquote" && e.ctrlKey && el.classList.contains("visor")) {   
      if (el.classList.contains("open")) {
          el.classList.replace("open", "closed");
          const app = document.querySelector("#main");
          app.focus();
      } else {
          el.classList.replace("closed", "open");
          setTimeout(() => el.focus(), 250);
      }
      e.stopPropagation();
      e.preventDefault();
      return false;
  }
  return true
}
