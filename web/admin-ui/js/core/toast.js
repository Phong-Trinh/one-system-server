/* ─── Toast Notifications ────────────────────────────────────────────────────
   Lightweight toast system. Appends to #toast-container in the DOM.
──────────────────────────────────────────────────────────────────────────── */

/**
 * Show a toast notification.
 * @param {string} msg   - Message to display.
 * @param {'info'|'success'|'error'} type - Visual style.
 */
function toast(msg, type = 'info') {
  const el = document.createElement('div');
  el.className = `toast ${type}`;
  el.textContent = msg;
  document.getElementById('toast-container').appendChild(el);
  setTimeout(() => el.remove(), 4000);
}
