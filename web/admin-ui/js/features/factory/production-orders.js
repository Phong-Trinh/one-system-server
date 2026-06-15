/* ─── Factory / Production Orders ────────────────────────────────────────────
   Wired to real API. Shows auto-created POs and allows manual PO creation.
──────────────────────────────────────────────────────────────────────────── */

async function renderFacOrders() {
  const nodeId = state.node;
  let orders = [], items = [], boms = [];
  let error = null;
  try {
    [orders, items, boms] = await Promise.all([
      api.getProductionOrders(nodeId),
      api.getItems(),
      api.getBOMs()
    ]);
    orders = orders || [];
    items = items || [];
    boms = boms || [];
  } catch(e) { error = e.message; }

  const itemMap = {};
  items.forEach(it => itemMap[it.id] = it);
  const bomMap = {};
  boms.forEach(b => bomMap[b.id] = b);

  const statusColor = {
    'PENDING': 'badge-dim',
    'IN_PROGRESS': 'badge-primary',
    'COMPLETED': 'badge-green',
    'CANCELLED': 'badge-red',
    'BLOCKED': 'badge-red',
  };

  const rows = orders.map(po => {
    const bom = bomMap[po.bom_id] || {};
    const item = itemMap[bom.output_item_id] || { name: bom.output_item_id || '—' };
    const isAutoCreated = po.source === 'AUTO_ITO' || !po.created_by_staff_id;
    return `
      <tr>
        <td>
          <code style="font-size:11px">${po.id.slice(0,8)}…</code>
          ${isAutoCreated ? `<span class="badge badge-primary" style="font-size:10px;margin-left:4px">AUTO</span>` : ''}
        </td>
        <td style="font-weight:600">${item.name}</td>
        <td>${po.target_qty}</td>
        <td><span class="badge ${statusColor[po.status] || 'badge-dim'}">${po.status}</span></td>
        <td><span class="dim small">${new Date(po.created_at).toLocaleString()}</span></td>
        <td>
          ${po.status === 'PENDING' ? `
            <button class="btn btn-primary btn-sm" onclick="startPO('${po.id}')">Start</button>
          ` : ''}
          ${po.status === 'IN_PROGRESS' ? `
            <button class="btn btn-ghost btn-sm" onclick="completePO('${po.id}', ${po.target_qty})">Complete</button>
          ` : ''}
        </td>
      </tr>
    `;
  }).join('');

  return `
    ${pageHeader('Production Orders', 'Factory production queue — auto-created from Store demand and manual orders')}
    ${error ? `<div class="empty-state" style="color:var(--red)">${error}</div>` : ''}

    <div class="flex row justify-between align-center" style="margin-bottom:16px">
      <div class="dim small">${orders.length} order(s) • <span class="badge badge-primary" style="font-size:10px">AUTO</span> = system-generated from ITO demand</div>
      <button class="btn btn-primary btn-sm" onclick="openCreatePOModal('${nodeId}')">+ Manual PO</button>
    </div>

    ${orders.length === 0 ? `
      <div class="empty-state">
        <div style="font-size:32px;margin-bottom:16px">🏭</div>
        <h3>No production orders</h3>
        <p class="dim">Orders appear automatically when Store stock hits ROP, or create them manually.</p>
      </div>
    ` : `
      <div class="card" style="overflow:auto">
        <table class="data-table">
          <thead><tr><th>Order ID</th><th>Product</th><th>Target Qty</th><th>Status</th><th>Created</th><th>Actions</th></tr></thead>
          <tbody>${rows}</tbody>
        </table>
      </div>
    `}
  `;
}

async function openCreatePOModal(nodeId) {
  let boms = [], items = [];
  try {
    [boms, items] = await Promise.all([api.getBOMs(), api.getItems()]);
    boms = boms || [];
    items = items || [];
  } catch(e) { toast(e.message, 'error'); return; }

  const itemMap = {};
  items.forEach(it => itemMap[it.id] = it);

  const bomOptions = boms.map(b => {
    const item = itemMap[b.output_item_id] || { name: b.output_item_id };
    return `<option value="${b.id}">${item.name} (v${b.version})</option>`;
  }).join('');

  showModal('Create Production Order', `
    <div class="flex col gap-12">
      <div class="field"><label>Product BOM *</label><select id="po-bom">${bomOptions}</select></div>
      <div class="field"><label>Target Quantity (units)</label><input id="po-qty" type="number" min="1" value="100" /></div>
    </div>
  `, [
    { label: 'Create Order', primary: true, action: async () => {
      const bomId = document.getElementById('po-bom').value;
      const qty = parseFloat(document.getElementById('po-qty').value);
      if (!bomId || qty <= 0) { toast('BOM and quantity required', 'error'); return; }
      try {
        await api.createProductionOrder({ bom_id: bomId, node_id: nodeId, target_qty: qty });
        toast('Production order created and queued!', 'success');
        closeModal();
        navigate('fac-orders', 'Production Orders');
      } catch(e) { toast(e.message, 'error'); }
    }},
    { label: 'Cancel', action: closeModal }
  ]);
}

async function startPO(id) {
  try {
    await api.updateProductionOrderStatus(id, 'IN_PROGRESS', null);
    toast('Production started!', 'success');
    navigate('fac-orders', 'Production Orders');
  } catch(e) { toast(e.message, 'error'); }
}

async function completePO(id, targetQty) {
  try {
    await api.updateProductionOrderStatus(id, 'COMPLETED', targetQty);
    toast('Production order completed!', 'success');
    navigate('fac-orders', 'Production Orders');
  } catch(e) { toast(e.message, 'error'); }
}
