/* ─── HQ / Stock Initialization ──────────────────────────────────────────────
   Stock-take tool to initialize NodeStock records during onboarding.
──────────────────────────────────────────────────────────────────────────── */

async function renderHQStockInit() {
  let nodes = [], items = [];
  let error = null;

  try {
    [nodes, items] = await Promise.all([
      api.getNodes(state.orgId),
      api.getItems(state.orgId)
    ]);
  } catch(e) { error = e.message; }

  const selectedNodeId = window._stockInitNode || (nodes[0] && nodes[0].id) || '';

  let stocks = [];
  if (selectedNodeId) {
    try {
      stocks = await api.getInventory(selectedNodeId) || [];
    } catch(e) { stocks = []; }
  }

  const itemMap = {};
  (items || []).forEach(it => itemMap[it.id] = it);

  const nodeOptions = (nodes || []).map(n =>
    `<option value="${n.id}" ${n.id === selectedNodeId ? 'selected' : ''}>${n.name} (${n.type})</option>`
  ).join('');

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
        <td>
          <button class="btn btn-ghost btn-sm" onclick="openAdjustStockModal('${selectedNodeId}', '${s.item_id}', '${item.name}', ${s.qty_on_hand})">Adjust</button>
        </td>
      </tr>
    `;
  }).join('');

  const itemOptions = items.map(it =>
    `<option value="${it.id}">${it.name} (${it.type})</option>`
  ).join('');

  return `
    ${pageHeader('Stock Initialization', 'Initialize or correct stock levels via manual stock-take')}
    ${error ? `<div class="empty-state" style="color:var(--red)">${error}</div>` : ''}

    <div class="flex row gap-12 align-center" style="margin-bottom:20px">
      <div class="field" style="margin:0; min-width:280px">
        <label class="small dim">Select Node</label>
        <select onchange="window._stockInitNode = this.value; navigate('hq-stock-init', 'Stock Initialization')">
          ${nodeOptions}
        </select>
      </div>
      <button class="btn btn-primary btn-sm" onclick="openInitStockModal('${selectedNodeId}', \`${itemOptions.replace(/`/g, '\\`')}\`)">+ Initialize Stock</button>
    </div>

    ${stocks.length === 0 ? `
      <div class="empty-state">
        <div style="font-size:32px;margin-bottom:16px">🔢</div>
        <h3>No stock records for this node</h3>
        <p class="dim">Use "Initialize Stock" to set opening quantities for items.</p>
      </div>
    ` : `
      <div class="card" style="overflow:auto">
        <table class="data-table">
          <thead><tr><th>Item</th><th>Item ID</th><th>Qty on Hand</th><th>Last Updated</th><th>Action</th></tr></thead>
          <tbody>${rows}</tbody>
        </table>
      </div>
    `}
  `;
}

function openInitStockModal(nodeId, itemOptionsHtml) {
  showModal('Initialize Stock', `
    <div class="flex col gap-12">
      <div class="info-box" style="background:rgba(99,102,241,0.08);border:1px solid rgba(99,102,241,0.2);border-radius:8px;padding:12px;font-size:13px;color:var(--text-dim)">
        ⚠️ This will SET the stock to the specified quantity (not add to it). Use this for onboarding stock-take corrections.
      </div>
      <div class="field"><label>Item *</label><select id="init-item">${itemOptionsHtml}</select></div>
      <div class="field"><label>Quantity (Base Units) *</label><input id="init-qty" type="number" step="0.01" min="0" value="0" /></div>
    </div>
  `, [
    { label: 'Set Stock', primary: true, action: async () => {
      const itemId = document.getElementById('init-item').value;
      const qty = parseFloat(document.getElementById('init-qty').value);
      if (isNaN(qty) || qty < 0) { toast('Enter a valid quantity', 'error'); return; }
      try {
        await api.initStock(nodeId, itemId, qty);
        toast('Stock initialized', 'success');
        closeModal();
        navigate('hq-stock-init', 'Stock Initialization');
      } catch(e) { toast(e.message, 'error'); }
    }},
    { label: 'Cancel', action: closeModal }
  ]);
}

function openAdjustStockModal(nodeId, itemId, itemName, currentQty) {
  showModal(`Adjust Stock — ${itemName}`, `
    <div class="flex col gap-12">
      <div class="info-box" style="background:rgba(99,102,241,0.08);border:1px solid rgba(99,102,241,0.2);border-radius:8px;padding:12px;font-size:13px;color:var(--text-dim)">
        Current quantity: <strong>${currentQty}</strong>
      </div>
      <div class="field"><label>New Quantity (Base Units)</label><input id="adj-qty" type="number" step="0.01" min="0" value="${currentQty}" /></div>
    </div>
  `, [
    { label: 'Update Stock', primary: true, action: async () => {
      const qty = parseFloat(document.getElementById('adj-qty').value);
      if (isNaN(qty) || qty < 0) { toast('Enter a valid quantity', 'error'); return; }
      try {
        await api.initStock(nodeId, itemId, qty);
        toast('Stock adjusted', 'success');
        closeModal();
        navigate('hq-stock-init', 'Stock Initialization');
      } catch(e) { toast(e.message, 'error'); }
    }},
    { label: 'Cancel', action: closeModal }
  ]);
}
