/* ─── Factory / Inventory ──────────────────────────────────────────────────
   View-only interface for Factory stock levels.
   To initialize or correct stock, use the HQ Stock Initialization tool.
──────────────────────────────────────────────────────────────────────────── */

async function renderFacInventory() {
  const nodeId = state.node;
  let stocks = [], items = [];
  let error = null;

  try {
    [stocks, items] = await Promise.all([
      api.getInventory(nodeId),
      api.getItems(state.orgId)
    ]);
    stocks = stocks || [];
    items = items || [];
  } catch(e) { error = e.message; }

  const itemMap = {};
  items.forEach(it => itemMap[it.id] = it);

  const rows = stocks.map(s => {
    const item = itemMap[s.item_id] || { name: s.item_id, base_unit: '' };
    return `
      <tr>
        <td style="font-weight:600">${item.name}</td>
        <td><code>${s.item_id.slice(0,8)}…</code></td>
        <td>
          <span style="font-size:18px;font-weight:700;color:var(--primary)">${s.qty_on_hand.toFixed(2)}</span>
          <span class="dim small"> ${item.base_unit || 'units'}</span>
        </td>
        <td><span class="dim small">${new Date(s.last_updated_at).toLocaleString()}</span></td>
      </tr>
    `;
  }).join('');

  return `
    ${pageHeader('Inventory Levels', 'Real-time view of stock currently held at the Factory')}
    ${error ? `<div class="empty-state" style="color:var(--red)">${error}</div>` : ''}

    <div class="flex row justify-between align-center" style="margin-bottom:20px">
      <div class="dim small">${stocks.length} item(s) in stock</div>
      <div class="info-box" style="margin:0;padding:8px 12px;font-size:12px;display:flex;align-items:center;gap:8px">
        <span>ℹ️</span> <span>Stock adjustments are handled via HQ Stock Initialization.</span>
      </div>
    </div>

    ${stocks.length === 0 ? `
      <div class="empty-state">
        <div style="font-size:32px;margin-bottom:16px">📦</div>
        <h3>Inventory is empty</h3>
        <p class="dim">Stock will appear here when goods are produced or received.</p>
      </div>
    ` : `
      <div class="card" style="overflow:auto">
        <table class="data-table">
          <thead><tr><th>Item</th><th>Item ID</th><th>Qty on Hand</th><th>Last Updated</th></tr></thead>
          <tbody>${rows}</tbody>
        </table>
      </div>
    `}
  `;
}
