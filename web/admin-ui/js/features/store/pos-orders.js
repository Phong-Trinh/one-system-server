/* ─── Store / POS Orders ───────────────────────────────────────────────────
   Point of Sale (POS) interface. Creates Sale Orders and marks them COMPLETED.
   Completion triggers Inventory deduction and the automated ROP replenishment flow.
──────────────────────────────────────────────────────────────────────────── */

async function renderStoPOS() {
  const nodeId = state.node;
  let orders = [], items = [];
  let error = null;

  try {
    [orders, items] = await Promise.all([
      api.getSaleOrders(nodeId),
      api.getItems(state.orgId)
    ]);
    orders = orders || [];
    items = (items || []).filter(it => it.type === 'PRODUCT'); // Only sell finished goods
  } catch(e) { error = e.message; }

  const itemMap = {};
  items.forEach(it => itemMap[it.id] = it);

  const statusColor = {
    'PENDING': 'badge-dim',
    'PROCESSING': 'badge-primary',
    'COMPLETED': 'badge-green',
    'CANCELLED': 'badge-red',
  };

  const rows = orders.map(o => `
    <tr>
      <td><code style="font-size:11px">${o.id.slice(0,8)}…</code></td>
      <td>${o.source === 'PLATFORM' ? `<span class="badge badge-purple">${o.platform || 'Platform'}</span>` : `<span class="badge badge-dim">Direct POS</span>`}</td>
      <td>$${o.total_amount.toFixed(2)}</td>
      <td>${o.items.length} items</td>
      <td><span class="badge ${statusColor[o.status] || 'badge-dim'}">${o.status}</span></td>
      <td><span class="dim small">${new Date(o.created_at).toLocaleString()}</span></td>
      <td>
        ${o.status === 'PENDING' ? `
          ${o.production_status === 'COOKING' ? `
            <button class="btn btn-primary btn-sm" disabled style="opacity:0.6;cursor:not-allowed">Cooking...</button>
          ` : `
            <button class="btn btn-primary btn-sm" onclick="completeOrder('${o.id}')">Complete (Deduct Stock)</button>
          `}
          <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="cancelOrder('${o.id}')">Cancel</button>
        ` : ''}
      </td>
    </tr>
  `).join('');

  const itemOptionsHtml = items.map(it => `<option value="${it.id}" data-price="5.99">${it.name}</option>`).join('');
  window._posItemOptionsHtml = itemOptionsHtml;

  return `
    ${pageHeader('POS Orders', 'Create customer orders. Completing an order deducts stock and may trigger automatic replenishment.')}
    ${error ? `<div class="empty-state" style="color:var(--red)">${error}</div>` : ''}

    <div class="flex row justify-between align-center" style="margin-bottom:20px">
      <div class="dim small">${orders.length} order(s) today</div>
      <button class="btn btn-primary btn-sm" onclick="openNewOrderModal('${nodeId}')">🛒 New Order</button>
    </div>

    ${orders.length === 0 ? `
      <div class="empty-state">
        <div style="font-size:32px;margin-bottom:16px">🛒</div>
        <h3>No orders yet</h3>
        <p class="dim">Click "New Order" to simulate a customer purchase.</p>
      </div>
    ` : `
      <div class="card" style="overflow:auto">
        <table class="data-table">
          <thead><tr><th>Order ID</th><th>Source</th><th>Total</th><th>Items</th><th>Status</th><th>Created</th><th>Actions</th></tr></thead>
          <tbody>${rows}</tbody>
        </table>
      </div>
    `}
  `;
}

function openNewOrderModal(nodeId) {
  const itemOptionsHtml = window._posItemOptionsHtml || '';
  showModal('New POS Order', `
    <div class="flex col gap-12">
      <div class="info-box" style="background:rgba(99,102,241,0.08);border:1px solid rgba(99,102,241,0.2);border-radius:8px;padding:12px;font-size:13px;color:var(--text-dim)">
        Simulate a customer order. The order will be created as <strong>PENDING</strong>. When marked <strong>COMPLETED</strong>, it will deduct stock and trigger automated ROP checks.
      </div>
      <div class="field">
        <label>Order Source</label>
        <select id="pos-source" onchange="document.getElementById('pos-platform-row').style.display=this.value==='PLATFORM'?'flex':'none'">
          <option value="DIRECT_POS">Direct POS (In-Store)</option>
          <option value="PLATFORM">Delivery Platform (Grab/Shopee)</option>
        </select>
      </div>
      <div id="pos-platform-row" class="field" style="display:none">
        <label>Platform Name</label>
        <input id="pos-platform" placeholder="e.g. GRAB" />
      </div>
      <div style="font-weight:600;margin-top:8px">Order Items</div>
      <div id="pos-lines" class="flex col gap-8">
        <div class="pos-line flex row gap-8 align-center">
          <select class="pos-item" style="flex:2">${itemOptionsHtml}</select>
          <input class="pos-qty" type="number" value="1" min="1" placeholder="Qty" style="flex:1;min-width:60px" />
          <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="this.closest('.pos-line').remove()">✕</button>
        </div>
      </div>
      <button class="btn btn-outline btn-sm" onclick="addPOSLine()">+ Add Item</button>
    </div>
  `, [
    { label: 'Place Order', primary: true, action: async () => {
      const source = document.getElementById('pos-source').value;
      const platform = source === 'PLATFORM' ? document.getElementById('pos-platform').value : null;

      const lineEls = document.querySelectorAll('#modal-body .pos-line');
      const items = [];
      lineEls.forEach(el => {
        const itemSelect = el.querySelector('.pos-item');
        const qty = parseInt(el.querySelector('.pos-qty').value);
        if (itemSelect.value && qty > 0) {
          items.push({
            item_id: itemSelect.value,
            quantity: qty,
            price: 5.99 // Mock price for demo
          });
        }
      });

      if (items.length === 0) { toast('Add at least one item', 'error'); return; }

      try {
        await api.createSaleOrder({
          node_id: nodeId,
          source: source,
          platform: platform,
          items: items
        });
        toast('Order placed successfully', 'success');
        closeModal();
        navigate('sto-pos', 'POS Orders');
      } catch(e) { toast(e.message, 'error'); }
    }},
    { label: 'Cancel', action: closeModal }
  ]);
}

function addPOSLine() {
  const itemOptionsHtml = window._posItemOptionsHtml || '';
  const container = document.getElementById('pos-lines');
  const div = document.createElement('div');
  div.className = 'pos-line flex row gap-8 align-center';
  div.innerHTML = `
    <select class="pos-item" style="flex:2">${itemOptionsHtml}</select>
    <input class="pos-qty" type="number" value="1" min="1" placeholder="Qty" style="flex:1;min-width:60px" />
    <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="this.closest('.pos-line').remove()">✕</button>
  `;
  container.appendChild(div);
}

async function completeOrder(id) {
  try {
    await api.completeSaleOrder(id);
    toast('Order completed! Stock deducted, ROP triggers evaluated.', 'success');
    navigate('sto-pos', 'POS Orders');
  } catch(e) { toast(e.message, 'error'); }
}

async function cancelOrder(id) {
  if (!confirm('Cancel this order?')) return;
  try {
    await api.cancelSaleOrder(id);
    toast('Order cancelled', 'success');
    navigate('sto-pos', 'POS Orders');
  } catch(e) { toast(e.message, 'error'); }
}
