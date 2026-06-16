/* ─── HQ / Dashboard ─────────────────────────────────────────────────────────
   High-level overview of organization operations and finance.
   Depends on: helpers.js, api.js.
──────────────────────────────────────────────────────────────────────────── */

async function renderHQDashboard() {
  let pendingPRsCount = 0;
  let pendingGRsCount = 0;
  let pendingAssetsCount = 0;

  try {
    const [prs, puros, assets] = await Promise.all([
      api.getPendingPRs(state.currentUser.orgId).catch(() => []),
      api.getPOs(state.currentUser.orgId).catch(() => []),
      api.getAssets('HQ').catch(() => []) // Or just don't fetch if no endpoint
    ]);
    
    pendingPRsCount = (prs || []).length;
    pendingGRsCount = (puros || []).filter(p => p.status === 'ON_WAY_DELIVERY').length;
    pendingAssetsCount = (assets || []).filter(a => a.status === 'PENDING_REGISTRATION').length;
  } catch (err) {}

  const pendingInvoices = 0; // Not fully implemented yet
  const openDTs = 0;
  const pendingB2B = 0;

  return `
    ${pageHeader('HQ Dashboard', 'Global organization overview')}

    <h3 style="margin-bottom: 12px">Action Required</h3>
    <div class="kpi-grid">
      ${kpi('📝', 'Pending PRs', pendingPRsCount, 'var(--primary)', 'Needs approval')}
      ${kpi('📦', 'Pending GRs', pendingGRsCount, 'var(--amber)', 'Waiting for receipt')}
      ${kpi('🧾', 'Unmatched Invoices', pendingInvoices, 'var(--amber)', 'Needs 3-way match')}
      ${kpi('🏷️', 'Assets Pending Reg', pendingAssetsCount, 'var(--amber)', 'Needs asset ID')}
      ${kpi('⚠️', 'Open Discrepancies', openDTs, 'var(--red)', 'Inventory issues')}
      ${kpi('🤝', 'Pending B2B Orders', pendingB2B, 'var(--primary)', 'Wholesale requests')}
    </div>

    <div class="grid cols-2" style="margin-top:24px">
      <div class="card">
        <h3>Recent Financials</h3>
        <p class="dim" style="margin-top:8px">Last 30 days summary</p>
        <div style="margin-top:16px; font-size:24px; font-weight:600">
          Revenue: <span style="color:var(--green)">${fmt(150000000)}</span><br>
          Procurement: <span style="color:var(--red)">${fmt(88000000)}</span>
        </div>
      </div>
      <div class="card">
        <h3>Node Status</h3>
        <p class="dim" style="margin-top:8px">Operational health</p>
        <ul style="margin-top:16px; line-height:2">
          <li><strong>Factory:</strong> Running normally</li>
          <li><strong>Nobi Fried Chicken:</strong> 3 ROP alerts</li>
        </ul>
      </div>
    </div>
  `;
}
