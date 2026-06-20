/* ─── Router ───────────────────────────────────────────────────────────────
   Manages sidebar navigation and main content rendering based on active node.
──────────────────────────────────────────────────────────────────────────── */

/* ── Navigation Config (per node) ───────────────────────── */
const NAV_CONFIG = {
  HQ: [
    { section: 'Overview', pages: [
      { id: 'hq-dashboard',   label: 'Dashboard',             icon: '◈' },
    ]},
    { section: 'Procurement', pages: [
      { id: 'hq-prs',         label: 'Purchase Requisitions', icon: '📋' },
      { id: 'hq-puros',       label: 'Purchase Orders',       icon: '🛒' },
      { id: 'hq-grs',         label: 'Goods Receipts',        icon: '📦' },
      { id: 'hq-invoices',    label: 'Supplier Invoices',     icon: '🧾' },
    ]},
    { section: 'Master Data', pages: [
      { id: 'hq-assets',      label: 'Asset Registry',        icon: '🏗️' },
      { id: 'hq-eqtypes',     label: 'Equipment Types',       icon: '🛠️' },
      { id: 'hq-suppliers',   label: 'Suppliers',             icon: '🏢' },
      { id: 'hq-items',       label: 'Items Catalog',         icon: '📦' },
      { id: 'hq-bom',         label: 'BOM & SOP',             icon: '⚙️' },
      { id: 'hq-rop-config',  label: 'ROP Configuration',     icon: '📊' },
      { id: 'hq-stock-init',  label: 'Stock Initialization',  icon: '🔢' },
    ]},
    { section: 'Sales', pages: [
      { id: 'hq-b2b',         label: 'B2B Sales Orders',      icon: '🤝' },
    ]},
    { section: 'Finance', pages: [
      { id: 'hq-discrepancy', label: 'Discrepancy Tickets',   icon: '⚠️' },
      { id: 'hq-financials',  label: 'Financials',            icon: '💰' },
    ]},
  ],

  FACTORY: [
    { section: 'Overview', pages: [
      { id: 'fac-dashboard',  label: 'Dashboard',             icon: '◈' },
    ]},
    { section: 'Production', pages: [
      { id: 'fac-orders',     label: 'Production Orders',     icon: '🏭' },
      { id: 'fac-kds',        label: 'KDS (Machine View)',    icon: '🖥️' },
    ]},
    { section: 'Supply Chain', pages: [
      { id: 'fac-ito',        label: 'Internal Transfers',    icon: '⇄' },
      { id: 'fac-inventory',  label: 'Inventory',             icon: '📦' },
      { id: 'fac-pr',         label: 'Submit PR to HQ',       icon: '📝' },
    ]},
  ],

  STORE: [
    { section: 'Overview', pages: [
      { id: 'sto-dashboard',  label: 'Dashboard',             icon: '◈' },
    ]},
    { section: 'Operations', pages: [
      { id: 'sto-pos',        label: 'POS Orders',            icon: '🛒' },
      { id: 'sto-kds',        label: 'KDS (Kitchen View)',    icon: '🖥️' },
      { id: 'sto-inventory',  label: 'Inventory',             icon: '📦' },
    ]},
    { section: 'Supply Chain', pages: [
      { id: 'sto-ito',        label: 'Internal Transfers',    icon: '⇄' },
      { id: 'sto-pr',         label: 'Submit PR to HQ',       icon: '📝' },
    ]},
  ]
};

/* ── Build Sidebar ──────────────────────────────────────── */
function buildSidebar() {
  const container = document.getElementById('nav-menu');
  container.innerHTML = '';

  const config = NAV_CONFIG[state.nodeType] || [];
  
  config.forEach(group => {
    // Section Header
    const sh = document.createElement('div');
    sh.className = 'nav-group-title';
    sh.textContent = group.section;
    container.appendChild(sh);

    // Links
    group.pages.forEach(page => {
      const a = document.createElement('a');
      a.className = 'nav-item';
      a.id = `nav-${page.id}`;
      a.innerHTML = `<span class="nav-icon">${page.icon}</span> ${page.label}`;
      a.onclick = (e) => {
        e.preventDefault();
        navigate(page.id, page.label);
      };
      container.appendChild(a);
    });
  });

  // Set default page if not set
  if (!state.page || !config.some(g => g.pages.some(p => p.id === state.page))) {
    const defaultPage = config[0].pages[0];
    navigate(defaultPage.id, defaultPage.label);
  } else {
    // Find label for current page
    let currentLabel = 'Page';
    for (const g of config) {
      const p = g.pages.find(x => x.id === state.page);
      if (p) { currentLabel = p.label; break; }
    }
    navigate(state.page, currentLabel);
  }
}

/* ── Navigation ─────────────────────────────────────────── */
function navigate(pageId, pageLabel) {
  state.page = pageId;

  // Update active state in sidebar
  document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
  const activeLink = document.getElementById(`nav-${pageId}`);
  if (activeLink) activeLink.classList.add('active');

  // Update breadcrumb
  document.getElementById('hdr-page-label').textContent = pageLabel;

  renderPage(pageId);
}

/* ── Render Main Content ────────────────────────────────── */
/**
 * Maps a pageId to its render function.
 * All render functions must be defined globally before router.js executes.
 */
function renderPage(id) {
  const renderers = {
    /* HQ */
    'hq-dashboard':   typeof renderHQDashboard === 'function' ? renderHQDashboard : () => 'Loading...',
    'hq-prs':         typeof renderHQPRs === 'function' ? renderHQPRs : () => 'Loading...',
    'hq-puros':       typeof renderHQPurchaseOrders === 'function' ? renderHQPurchaseOrders : () => 'Loading...',
    'hq-grs':         typeof renderHQGoodsReceipts === 'function' ? renderHQGoodsReceipts : () => 'Loading...',
    'hq-invoices':    typeof renderHQInvoices === 'function' ? renderHQInvoices : () => 'Loading...',
    'hq-assets':      typeof renderHQAssets === 'function' ? renderHQAssets : () => 'Loading...',
    'hq-eqtypes':     typeof renderHQEquipmentTypes === 'function' ? renderHQEquipmentTypes : () => 'Loading...',
    'hq-suppliers':   typeof renderHQSuppliers === 'function' ? renderHQSuppliers : () => 'Loading...',
    'hq-b2b':         typeof renderHQB2B === 'function' ? renderHQB2B : () => 'Loading...',
    'hq-bom':         typeof renderHQBOM === 'function' ? renderHQBOM : () => 'Loading...',
    'hq-items':       typeof renderHQItems === 'function' ? renderHQItems : () => 'Loading...',
    'hq-rop-config':  typeof renderHQROPConfig === 'function' ? renderHQROPConfig : () => 'Loading...',
    'hq-stock-init':  typeof renderHQStockInit === 'function' ? renderHQStockInit : () => 'Loading...',
    'hq-discrepancy': typeof renderHQDiscrepancy === 'function' ? renderHQDiscrepancy : () => 'Loading...',
    'hq-financials':  typeof renderHQFinancials === 'function' ? renderHQFinancials : () => 'Loading...',
    /* Factory */
    'fac-dashboard':  typeof renderFacDashboard === 'function' ? renderFacDashboard : () => 'Loading...',
    'fac-orders':     typeof renderFacOrders === 'function' ? renderFacOrders : () => 'Loading...',
    'fac-kds':        typeof renderFacKDS === 'function' ? renderFacKDS : () => 'Loading...',
    'fac-ito':        typeof renderFacITO === 'function' ? renderFacITO : () => 'Loading...',
    'fac-inventory':  typeof renderFacInventory === 'function' ? renderFacInventory : () => 'Loading...',
    'fac-pr':         typeof renderFacPR === 'function' ? renderFacPR : () => 'Loading...',
    /* Store */
    'sto-dashboard':  typeof renderStoDashboard === 'function' ? renderStoDashboard : () => 'Loading...',
    'sto-pos':        typeof renderStoPOS === 'function' ? renderStoPOS : () => 'Loading...',
    'sto-kds':        typeof renderFacKDS === 'function' ? renderFacKDS : () => 'Loading...',
    'sto-inventory':  typeof renderStoInventory === 'function' ? renderStoInventory : () => 'Loading...',
    'sto-ito':        typeof renderStoITO === 'function' ? renderStoITO : () => 'Loading...',
    'sto-pr':         typeof renderStoPR === 'function' ? renderStoPR : () => 'Loading...',
  };

  const renderer = renderers[id];
  const content = document.getElementById('content');
  
  if (renderer) {
    const result = renderer();
    if (result instanceof Promise) {
      content.innerHTML = '<div class="empty-state">Loading...</div>';
      result.then(html => content.innerHTML = html).catch(err => {
        content.innerHTML = `<div class="empty-state" style="color:red">Error: ${err.message}</div>`;
      });
    } else {
      content.innerHTML = result;
    }
  } else {
    content.innerHTML = `
      <div class="empty-state">
        <div style="font-size:32px; margin-bottom:16px">🚧</div>
        <h3>Page Under Construction</h3>
        <p class="dim">The view for <code>${id}</code> has not been implemented yet.</p>
      </div>
    `;
  }
}
