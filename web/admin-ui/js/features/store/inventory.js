/* ─── Store / Inventory ────────────────────────────────────────────────────
   View-only interface for Store stock levels.
   To initialize or correct stock, use the HQ Stock Initialization tool.
──────────────────────────────────────────────────────────────────────────── */

async function renderStoInventory() {
  const nodeId = state.node;
  let stocks = [], items = [], configs = [];
  let error = null;

  try {
    [stocks, items, configs] = await Promise.all([
      api.getInventory(nodeId),
      api.getItems(state.orgId),
      api.getNodeItemConfigs ? api.getNodeItemConfigs(nodeId) : Promise.resolve([])
    ]);
    stocks = stocks || [];
    items = items || [];
    configs = configs || [];
  } catch(e) { error = e.message; }

  const itemMap = {};
  items.forEach(it => itemMap[it.id] = it);
  
  const configMap = {};
  configs.forEach(c => configMap[c.item_id] = c);

  const rows = stocks.map(s => {
    const item = itemMap[s.item_id] || { name: s.item_id, base_unit: '' };
    const cfg = configMap[s.item_id];
    let statusHtml = '';
    
    if (cfg && s.qty_on_hand <= cfg.reorder_point) {
      const isExternal = cfg.sourcing_strategy === 'EXTERNAL_PROCUREMENT';
      const typeText = isExternal ? 'PO (HQ)' : 'ITO';
      statusHtml = `<div style="margin-top:4px; display:flex; align-items:center; gap:8px">
        <span class="badge badge-orange" style="font-size:10px">⚠️ ROP Triggered (${typeText})</span>
        <button class="btn btn-primary btn-sm" style="padding:2px 6px;font-size:10px" onclick="openTriggerITOModal('${s.item_id}')">Yêu cầu châm hàng</button>
      </div>`;
    }

    return `
      <tr>
        <td>
          <div style="font-weight:600">${item.name}</div>
          ${statusHtml}
        </td>
        <td><code>${s.item_id.slice(0,8)}…</code></td>
        <td>
          <span style="font-size:18px;font-weight:700;color:var(--primary)">${s.qty_on_hand.toFixed(2)}</span>
          <span class="dim small"> ${item.base_unit || 'units'}</span>
        </td>
        <td>
          ${cfg ? `<span class="dim small">${cfg.reorder_point}</span>` : '<span class="dim small">—</span>'}
        </td>
        <td><span class="dim small">${new Date(s.last_updated_at).toLocaleString()}</span></td>
      </tr>
    `;
  }).join('');

  return `
    ${pageHeader('Inventory Levels', 'Real-time view of stock currently held at the Store')}
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
        <p class="dim">Stock will appear here when goods are received via ITOs or initial stock take.</p>
      </div>
    ` : `
      <div class="card" style="overflow:auto">
        <table class="data-table">
          <thead><tr><th>Item</th><th>Item ID</th><th>Qty on Hand</th><th>ROP</th><th>Last Updated</th></tr></thead>
          <tbody>${rows}</tbody>
        </table>
      </div>
    `}
  `;
}

async function openTriggerITOModal(itemId) {
  let nodes, items, configs, stocks;
  try {
    [nodes, items, configs, stocks] = await Promise.all([
      api.getNodes(state.orgId),
      api.getItems(state.orgId),
      api.getNodeItemConfigs(state.node),
      api.getInventory(state.node)
    ]);
  } catch(e) { toast(e.message, 'error'); return; }

  const config = configs.find(c => c.item_id === itemId);
  const item = items.find(i => i.id === itemId);
  const stock = stocks.find(s => s.item_id === itemId);
  
  if (!config || !item || !stock) {
    toast('Missing configuration or stock info for this item.', 'error');
    return;
  }

  const provider = nodes.find(n => n.id === config.provider_node_id) || { name: config.provider_node_id, type: 'HQ' };
  const targetQty = (config.reorder_point - stock.qty_on_hand) + config.safety_stock;

  showModal('Yêu cầu châm hàng khẩn cấp', `
    <div class="flex col gap-12">
      <div class="info-box" style="background:rgba(245,158,11,0.1);color:var(--orange);border:1px solid rgba(245,158,11,0.2)">
        Đây là chức năng khôi phục (fail-safe). Lệnh châm hàng này được tạo ra do hệ thống phát hiện tồn kho của <b>${item.name}</b> đã dưới mức ROP an toàn.
      </div>
      <div class="field">
        <label>Nguồn cung cấp (Hệ thống tự động phân bổ)</label>
        <input type="text" value="${provider.name} (${provider.type || 'HQ'})" disabled style="background:#f9fafb;cursor:not-allowed" />
      </div>
      <div style="font-weight:600;margin-top:8px">Chi tiết yêu cầu</div>
      <div class="flex row gap-8 align-center">
        <input type="text" value="${item.name}" style="flex:2;background:#f9fafb;cursor:not-allowed" disabled />
        <input type="number" value="${targetQty.toFixed(2)}" style="flex:1;background:#f9fafb;cursor:not-allowed;text-align:right" disabled />
        <input type="text" value="${item.base_unit || 'units'}" style="flex:1;background:#f9fafb;cursor:not-allowed" disabled />
      </div>
    </div>
  `, [
    { label: 'Xác nhận & Gửi yêu cầu', primary: true, action: async () => {
      try {
        await api.triggerROP(state.node, itemId);
        toast('Yêu cầu châm hàng đã được hệ thống xử lý', 'success');
        closeModal();
        const html = await renderStoInventory();
        document.getElementById('content').innerHTML = html;
      } catch (e) {
        toast(e.message, 'error');
      }
    }},
    { label: 'Hủy', action: closeModal }
  ]);
}
