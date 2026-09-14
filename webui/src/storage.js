const PREFIX = "lousentry";
const LEGACY = "watchvuln";

export function getStore(name) {
  const current = localStorage.getItem(`${PREFIX}-${name}`);
  if (current != null) return current;
  const old = localStorage.getItem(`${LEGACY}-${name}`);
  if (old != null) {
    localStorage.setItem(`${PREFIX}-${name}`, old);
    localStorage.removeItem(`${LEGACY}-${name}`);
    return old;
  }
  return null;
}

export function setStore(name, value) {
  localStorage.setItem(`${PREFIX}-${name}`, value);
  localStorage.removeItem(`${LEGACY}-${name}`);
}

export function removeStore(name) {
  localStorage.removeItem(`${PREFIX}-${name}`);
  localStorage.removeItem(`${LEGACY}-${name}`);
}
