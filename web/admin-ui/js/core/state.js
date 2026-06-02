/* ─── App State ──────────────────────────────────────────────────────────────
   Global application state. Single source of truth for current session.
──────────────────────────────────────────────────────────────────────────── */

let state = {
  loggedIn: false,
  currentUser: null,
  node: null,   // 'HQ' | 'FACTORY' | 'STORE'
  page: null,   // current page id string
};

// Define missing mock data globals to prevent ReferenceErrors in legacy/unmigrated views
window.PRODUCTION_ORDERS = [];
window.ITEMS = [];
window.B2B_ORDERS = [];
window.ITORDERS = [];
window.POS_ORDERS = [];
window.PURCHASE_ORDERS = [];
window.DISC_TICKETS = [];
window.GOODS_RECEIPTS = [];
window.INVOICES = [];
window.ASSETS = [];
window.EQUIPMENT_TYPES = [];
window.PURCHASE_REQS = [];
window.SUPPLIERS = [];
window.COST_RECORDS = [];
