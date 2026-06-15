/* ─── Store / Internal Transfers ─────────────────────────────────────────────
   Store view of ITOs. Shows auto-created ITOs from ROP and manual ITOs.
   Allows manual creation and Goods Receipt confirmation.
──────────────────────────────────────────────────────────────────────────── */

async function renderStoITO() {
  const nodeId = state.node;
  let itos = [], items = [], nodes = [];
  let error = null;

  try {
    [itos, items, nodes] = await Promise.all([
      api.getITOs(nodeId),
      api.getItems(state.orgId),
      api.getNodes(state.orgId)
    ]);
    itos = itos || [];
    items = items || [];
    nodes = nodes || [];
  } catch(e) { error = e.message; }

  const itemMap = {};
  items.forEach(it => itemMap[it.id] = it);
  const nodeMap = {};
  nodes.forEach(n => nodeMap[n.id] = n);

  const statusColor = {
    'DRAFT': 'badge-dim',
    'APPROVED': 'badge-primary',
    'REJECTED': 'badge-red',
    'IN_TRANSIT': 'badge-orange',
    'COMPLETED': 'badge-green',
  };

  const rows = itos.map(ito => {
    // Determine direction: Store -> Inbound, or Store -> Outbound
    const isInbound = ito.requester_node_id === nodeId;
    const otherNodeId = isInbound ? ito.provider_node_id : ito.requester_node_id;
    const otherNode = nodeMap[otherNodeId] || { name: otherNodeId };

    const firstLineItem = ito.lines && ito.lines.length > 0 ? (itemMap[ito.lines[0].item_id] || {name: ito.lines[0].item_id}).name : '—';
    const totalLines = ito.lines ? ito.lines.length : 0;
    const isAuto = ito.created_by_staff_id === 'SYSTEM';

    return `
      <tr>
        <td>
          <code style="font-size:11px">${ito.id.slice(0,8)}…</code>
          ${isAuto ? `<span class="badge badge-primary" style="font-size:10px;margin-left:4px">AUTO</span>` : ''}
        </td>
        <td>
          <span class="badge ${isInbound ? 'badge-green' : 'badge-orange'}">${isInbound ? 'INBOUND' : 'OUTBOUND'}</span>
        </td>
        <td style="font-weight:600">${otherNode.name}</td>
        <td>${totalLines} items (e.g. ${firstLineItem})</td>
        <td><span class="badge ${statusColor[ito.status] || 'badge-dim'}">${ito.status}</span></td>
        <td><span class="dim small">${new Date(ito.created_at).toLocaleString()}</span></td>
        <td>
          <button class="btn btn-ghost btn-sm" onclick="openITODetail('${ito.id}')">View</button>
          ${(isInbound && ito.status === 'IN_TRANSIT') ? `
            <button class="btn btn-primary btn-sm" onclick="openGoodsReceiptModal('${ito.id}', '${ito.goods_issue_id || ''}')">Receive Goods</button>
          ` : ''}
        </td>
      </tr>
    `;
  }).join('');

  const providerOptions = nodes.filter(n => n.id !== nodeId).map(n => `<option value="${n.id}">${n.name} (${n.type})</option>`).join('');
  const itemOptions = items.map(it => `<option value="${it.id}">${it.name} (${it.type})</option>`).join('');

  return `
    ${pageHeader('Internal Transfers', 'Track stock replenishments from Factory and inter-store transfers')}
    ${error ? `<div class="empty-state" style="color:var(--red)">${error}</div>` : ''}

    <div class="flex row justify-between align-center" style="margin-bottom:20px">
      <div class="dim small">${itos.length} transfer(s) • <span class="badge badge-primary" style="font-size:10px">AUTO</span> = triggered by ROP</div>
      <button class="btn btn-primary btn-sm" onclick="openCreateITOModal('${nodeId}', \`${providerOptions.replace(/`/g, '\\`')}\`, \`${itemOptions.replace(/`/g, '\\`')}\`)">+ Request Transfer</button>
    </div>

    ${itos.length === 0 ? `
      <div class="empty-state">
        <div style="font-size:32px;margin-bottom:16px">⇄</div>
        <h3>No internal transfers</h3>
        <p class="dim">Transfers are created automatically when stock drops below reorder point.</p>
      </div>
    ` : `
      <div class="card" style="overflow:auto">
        <table class="data-table">
          <thead><tr><th>ITO ID</th><th>Type</th><th>Partner Node</th><th>Items</th><th>Status</th><th>Date</th><th>Actions</th></tr></thead>
          <tbody>${rows}</tbody>
        </table>
      </div>
    `}
  `;
}

function openCreateITOModal(nodeId, providerOptionsHtml, itemOptionsHtml) {
  showModal('Request Internal Transfer', `
    <div class="flex col gap-12">
      <div class="field">
        <label>Source Node (Provider) *</label>
        <select id="ito-provider">${providerOptionsHtml}</select>
      </div>
      <div style="font-weight:600;margin-top:8px">Items to Request</div>
      <div id="ito-lines" class="flex col gap-8">
        <div class="ito-line flex row gap-8 align-center">
          <select class="ito-item" style="flex:2">${itemOptionsHtml}</select>
          <input class="ito-qty" type="number" value="1" min="1" placeholder="Qty" style="flex:1;min-width:60px" />
          <input class="ito-unit" value="pieces" placeholder="Unit" style="flex:1;min-width:60px" />
          <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="this.closest('.ito-line').remove()">✕</button>
        </div>
      </div>
      <button class="btn btn-outline btn-sm" onclick="addITOLine(\`${itemOptionsHtml.replace(/`/g, '\\`')}\`)">+ Add Item</button>
    </div>
  `, [
    { label: 'Submit Request', primary: true, action: async () => {
      const providerId = document.getElementById('ito-provider').value;
      const lineEls = document.querySelectorAll('#modal-body .ito-line');
      const lines = [];
      lineEls.forEach(el => {
        const itemId = el.querySelector('.ito-item').value;
        const qty = parseFloat(el.querySelector('.ito-qty').value);
        const unit = el.querySelector('.ito-unit').value || 'unit';
        if (itemId && qty > 0) {
          lines.push({ item_id: itemId, qty_ordered: qty, pkg_unit: unit, conversion: 1 });
        }
      });
      if (lines.length === 0) { toast('Add at least one item', 'error'); return; }

      try {
        await api.createITO({
          org_id: state.orgId || 'SNAPBITE_ORG',
          requester_node_id: nodeId,
          provider_node_id: providerId,
          staff_id: 'STAFF_001',
          lines
        });
        toast('Transfer request submitted', 'success');
        closeModal();
        navigate('sto-ito', 'Internal Transfers');
      } catch(e) { toast(e.message, 'error'); }
    }},
    { label: 'Cancel', action: closeModal }
  ]);
}

function addITOLine(itemOptionsHtml) {
  const container = document.getElementById('ito-lines');
  const div = document.createElement('div');
  div.className = 'ito-line flex row gap-8 align-center';
  div.innerHTML = `
    <select class="ito-item" style="flex:2">${itemOptionsHtml}</select>
    <input class="ito-qty" type="number" value="1" min="1" placeholder="Qty" style="flex:1;min-width:60px" />
    <input class="ito-unit" value="pieces" placeholder="Unit" style="flex:1;min-width:60px" />
    <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="this.closest('.ito-line').remove()">✕</button>
  `;
  container.appendChild(div);
}

async function openITODetail(id) {
  let res, items, nodes;
  try {
    [res, items, nodes] = await Promise.all([
      api.getITO(id), api.getItems(state.orgId), api.getNodes(state.orgId)
    ]);
  } catch(e) { toast(e.message, 'error'); return; }

  const itemMap = {}; (items || []).forEach(it => itemMap[it.id] = it);
  const nodeMap = {}; (nodes || []).forEach(n => nodeMap[n.id] = n);

  const ito = res.ito;
  const lines = res.lines || [];
  const reqNode = nodeMap[ito.requester_node_id] || { name: ito.requester_node_id };
  const provNode = nodeMap[ito.provider_node_id] || { name: ito.provider_node_id };

  const lineRows = lines.map(l => {
    const it = itemMap[l.item_id] || { name: l.item_id };
    return `<tr><td>${it.name}</td><td>${l.qty_ordered} ${l.pkg_unit}</td></tr>`;
  }).join('');

  showModal(`Transfer Details — ${ito.id.slice(0,8)}`, `
    <div class="flex col gap-12">
      <div class="flex row justify-between">
        <div><div class="dim small">Requester</div><div style="font-weight:600">${reqNode.name}</div></div>
        <div><div class="dim small">Provider</div><div style="font-weight:600">${provNode.name}</div></div>
        <div><div class="dim small">Status</div><div><span class="badge badge-dim">${ito.status}</span></div></div>
      </div>
      <table class="data-table">
        <thead><tr><th>Item</th><th>Requested Qty</th></tr></thead>
        <tbody>${lineRows}</tbody>
      </table>
    </div>
  `, [{ label: 'Close', action: closeModal }]);
}

async function openGoodsReceiptModal(itoId, goodsIssueId) {
  let res, items;
  try {
    [res, items] = await Promise.all([api.getITO(itoId), api.getItems(state.orgId)]);
  } catch(e) { toast(e.message, 'error'); return; }

  const itemMap = {}; (items || []).forEach(it => itemMap[it.id] = it);
  const lines = res.lines || [];

  const lineInputs = lines.map((l, i) => {
    const it = itemMap[l.item_id] || { name: l.item_id };
    return `
      <div class="gr-line flex row gap-8 align-center" data-item="${l.item_id}" data-exp="${l.qty_ordered}">
        <div style="flex:2;font-weight:600">${it.name}</div>
        <div class="dim small" style="width:80px">Expected: ${l.qty_ordered}</div>
        <input class="gr-qty" type="number" step="0.01" value="${l.qty_ordered}" style="flex:1;min-width:80px" />
      </div>
    `;
  }).join('');

  showModal('Receive Goods', `
    <div class="flex col gap-12">
      <div class="info-box" style="background:rgba(16,185,129,0.08);border:1px solid rgba(16,185,129,0.2);border-radius:8px;padding:12px;font-size:13px;color:var(--text-dim)">
        Confirming receipt will automatically increase your store's inventory and complete the transfer process.
      </div>
      <div style="font-weight:600">Received Quantities</div>
      ${lineInputs}
    </div>
  `, [
    { label: 'Confirm Receipt', primary: true, action: async () => {
      const els = document.querySelectorAll('#modal-body .gr-line');
      const linesData = [];
      els.forEach(el => {
        linesData.push({
          item_id: el.dataset.item,
          qty_expected: parseFloat(el.dataset.exp),
          qty_received: parseFloat(el.querySelector('.gr-qty').value) || 0
        });
      });
      try {
        await api.itoGoodsReceipt(itoId, {
          goods_issue_id: goodsIssueId || 'MISSING_GI_REF',
          received_by_staff_id: 'STAFF_001',
          lines: linesData
        });
        toast('Goods receipt confirmed and inventory updated', 'success');
        closeModal();
        navigate('sto-ito', 'Internal Transfers');
      } catch(e) { toast(e.message, 'error'); }
    }},
    { label: 'Cancel', action: closeModal }
  ]);
}
