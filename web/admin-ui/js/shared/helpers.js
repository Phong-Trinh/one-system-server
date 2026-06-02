/* ─── Shared Helpers ─────────────────────────────────────────────────────────
   Pure utility functions used across all feature modules.
   Depends on: mock-data.js (ITEMS, PURCHASE_REQS), toast.js, core/router.js (renderPage).
──────────────────────────────────────────────────────────────────────────── */

/* ── Status Badge ─────────────────────────────────────────── */
/**
 * Returns an HTML badge string for a given status string.
 * @param {string} s - Status value (e.g. 'COMPLETED', 'PENDING').
 * @returns {string} HTML string.
 */
function statusBadge(s) {
  const map = {
    'COMPLETED':           'badge-green',
    'AUTO_APPROVED':       'badge-green',
    'APPROVED':            'badge-green',
    'IN_PROGRESS':         'badge-blue',
    'IN_REVIEW':           'badge-blue',
    'GOODS_ISSUED':        'badge-blue',
    'ASSIGNED':            'badge-blue',
    'PENDING_HQ_APPROVAL': 'badge-amber',
    'PENDING':             'badge-amber',
    'AUTO_DRAFT':          'badge-amber',
    'DRAFT':               'badge-amber',
    'QUEUED':              'badge-amber',
    'CONFIRMED':           'badge-purple',
    'SHIPPED':             'badge-purple',
    'DELIVERED':           'badge-purple',
    'OPEN':                'badge-amber',
    'CANCELLED':           'badge-red',
    'REJECTED':            'badge-red',
    'FAILED':              'badge-red',
    'IDLE':                'badge-green',
    'BUSY':                'badge-amber',
    'UNDER_MAINTENANCE':   'badge-red',
    'DECOMMISSIONED':      'badge-dim',
  };
  return `<span class="badge ${map[s] || 'badge-dim'}">${s.replace(/_/g, ' ')}</span>`;
}

/* ── Currency Formatter ───────────────────────────────────── */
/**
 * Formats a number as Vietnamese Dong.
 * @param {number} n
 * @returns {string}
 */
function fmt(n) {
  return Number(n).toLocaleString('vi-VN') + ' ₫';
}

/* ── ROP Helpers ──────────────────────────────────────────── */
/**
 * Returns a progress bar color class based on stock vs ROP.
 * @param {number} qty  - Current qty on hand.
 * @param {number} rop  - Reorder point.
 * @returns {'red'|'amber'|'green'}
 */
function ropColor(qty, rop) {
  if (qty <= rop * 0.5) return 'red';
  if (qty <= rop)       return 'amber';
  return 'green';
}

/**
 * Returns a percentage (0–100) for stock bar width.
 * Treats 1.5× ROP as 100%.
 * @param {number} qty
 * @param {number} rop
 * @returns {number}
 */
function ropPct(qty, rop) {
  return Math.min(100, Math.round((qty / (rop * 1.5)) * 100));
}

/* ── Page Header ──────────────────────────────────────────── */
/**
 * Returns HTML for a standardized page header row.
 * @param {string} title
 * @param {string} [sub]     - Optional subtitle.
 * @param {string} [actions] - Optional HTML for action buttons (right side).
 * @returns {string}
 */
function pageHeader(title, sub = '', actions = '') {
  return `
    <div class="page-header flex ai-c jc-sb">
      <div>
        <h2>${title}</h2>
        ${sub ? `<p class="dim small mt-4">${sub}</p>` : ''}
      </div>
      <div class="flex gap-8">${actions}</div>
    </div>
  `;
}

/* ── KPI Card ─────────────────────────────────────────────── */
/**
 * Returns HTML for a KPI metric card.
 * @param {string} icon
 * @param {string} label
 * @param {number|string} val
 * @param {string} color - CSS color value.
 * @param {string} sub   - Sub-label text.
 * @returns {string}
 */
function kpi(icon, label, val, color, sub) {
  return `
    <div class="kpi-card">
      <div class="kpi-glow" style="background:${color}"></div>
      <div class="kpi-icon" style="background:${color}22">${icon}</div>
      <div class="kpi-value" style="color:${color}">${val}</div>
      <div class="kpi-label">${label}</div>
      <div class="small faint">${sub}</div>
    </div>
  `;
}


