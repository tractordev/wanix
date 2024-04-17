
export function login(redirect) {
  if (redirect) {
    localStorage.setItem("auth:redirect", redirect);
  } else {
    localStorage.setItem("auth:redirect", window.location.href);
  }
  window.location.href = "/auth/";
}

export function logout(redirect) {
  if (redirect) {
    localStorage.setItem("auth:redirect", redirect);
  } else {
    localStorage.setItem("auth:redirect", window.location.href);
  }
  window.location.href = "/auth/?logout";
}

export function isAuthenticated() {
  const login = localStorage.getItem("auth:login");
  if (login) {
    return true;
  } else {
    return false;
  }
}

export function currentUser() {
  const login = localStorage.getItem("auth:login");
  if (!login) {
    return null;
  }
  return JSON.parse(login)["user"];
}

export function accessToken() {
  const login = localStorage.getItem("auth:login");
  if (!login) {
    return null;
  }
  return JSON.parse(login)["access_token"];
}