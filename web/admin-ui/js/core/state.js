/* ─── App State ──────────────────────────────────────────────────────────────
   Global application state. Single source of truth for current session.
──────────────────────────────────────────────────────────────────────────── */

let state = {
  loggedIn: false,
  currentUser: null,
  node: null,   // 'HQ' | 'FACTORY' | 'STORE'
  page: null,   // current page id string
};
