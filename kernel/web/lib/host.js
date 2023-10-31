
globalThis.sys.pipe.handle("host.loadStylesheet", duplex.handlerFrom((url) => {
  const style = document.createElement("link");
  style.rel = "stylesheet";
  style.href = url;
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
  frame.setAttribute("src", path);
}));

const visorKeydown = (e) => {
  const el = document.querySelector("#terminal");
  if (e.code === "Backquote" && (e.ctrlKey || e.altKey) && el.classList.contains("visor")) {   
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
