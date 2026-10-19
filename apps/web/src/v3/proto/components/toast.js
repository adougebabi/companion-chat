let tbox;
export function toast(text, type = '') {
  if (!tbox) { tbox = document.createElement('div'); tbox.className = 'toasts'; document.body.append(tbox); }
  const t = document.createElement('div'); t.className = 'toast ' + type; t.textContent = text; tbox.append(t);
  setTimeout(() => t.remove(), 2600);
}

