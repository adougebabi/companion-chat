let tbox;
export function toast(text, type = '') {
  if (!tbox) { tbox = document.createElement('div'); tbox.className = 'toasts'; document.body.append(tbox); }
  const t = document.createElement('div'); t.className = 'toast ' + type; t.setAttribute('role', type === 'err' ? 'alert' : 'status'); t.textContent = text; tbox.append(t);
  setTimeout(() => t.remove(), type === 'err' ? 6500 : 4200);
}

