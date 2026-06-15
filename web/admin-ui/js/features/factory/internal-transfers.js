/* ─── Factory / Internal Transfers ───────────────────────────────────────────
   Factory view of ITOs. Usually Factory acts as the Provider.
   Allows Approving, Rejecting, and Dispatching Goods (Goods Issue).
──────────────────────────────────────────────────────────────────────────── */

async function renderFacITO() {
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
    'AUTO_APPROVED': 'badge-primary',
    'REJECTED': 'badge-red',
    'IN_TRANSIT': 'badge-orange',
    'COMPLETED': 'badge-green',
  };

  const rows = itos.map(ito => {
    const isOutbound = ito.provider_node_id === nodeId;
    const otherNodeId = isOutbound ? ito.requester_node_id : ito.provider_node_id;
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
          <span class="badge ${isOutbound ? 'badge-orange' : 'badge-green'}">${isOutbound ? 'OUTBOUND' : 'INBOUND'}</span>
        </td>
        <td style="font-weight:600">${otherNode.name}</td>
        <td>${totalLines} items (e.g. ${firstLineItem})</td>
        <td><span class="badge ${statusColor[ito.status] || 'badge-dim'}">${ito.status}</span></td>
        <td><span class="dim small">${new Date(ito.created_at).toLocaleString()}</span></td>
        <td>
          <button class="btn btn-ghost btn-sm" onclick="openFacITODetail('${ito.id}')">View</button>
          ${(isOutbound && ito.status === 'DRAFT') ? `
            <button class="btn btn-primary btn-sm" onclick="approveITO('${ito.id}')">Approve</button>
            <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="rejectITO('${ito.id}')">Reject</button>
          ` : ''}
          ${(isOutbound && (ito.status === 'APPROVED' || ito.status === 'AUTO_APPROVED')) ? `
            <button class="btn btn-primary btn-sm" onclick="openGoodsIssueModal('${ito.id}')">Dispatch Goods</button>
          ` : ''}
        </td>
      </tr>
    `;
  }).join('');

  return `
    ${pageHeader('Internal Transfers', 'Manage stock requests from stores and dispatch goods')}
    ${error ? `<div class="empty-state" style="color:var(--red)">${error}</div>` : ''}

    <div class="flex row justify-between align-center" style="margin-bottom:20px">
      <div class="dim small">${itos.length} transfer(s) • <span class="badge badge-primary" style="font-size:10px">AUTO</span> = system-generated store replenishment</div>
    </div>

    ${itos.length === 0 ? `
      <div class="empty-state">
        <div style="font-size:32px;margin-bottom:16px">⇄</div>
        <h3>No transfer requests</h3>
        <p class="dim">Requests from stores will appear here.</p>
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

async function approveITO(id) {
  try {
    await api.approveITO(id);
    toast('Transfer request approved', 'success');
    navigate('fac-ito', 'Internal Transfers');
  } catch(e) { toast(e.message, 'error'); }
}

async function rejectITO(id) {
  const reason = prompt('Reason for rejection:');
  if (reason === null) return;
  try {
    await api.rejectITO(id, reason);
    toast('Transfer request rejected', 'success');
    navigate('fac-ito', 'Internal Transfers');
  } catch(e) { toast(e.message, 'error'); }
}

async function openFacITODetail(id) {
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

async function openGoodsIssueModal(itoId) {
  let res, items;
  try {
    [res, items] = await Promise.all([api.getITO(itoId), api.getItems(state.orgId)]);
  } catch(e) { toast(e.message, 'error'); return; }

  const itemMap = {}; (items || []).forEach(it => itemMap[it.id] = it);
  const lines = res.lines || [];

  const lineInputs = lines.map((l, i) => {
    const it = itemMap[l.item_id] || { name: l.item_id };
    return `
      <div class="gi-line flex row gap-8 align-center" data-item="${l.item_id}" data-exp="${l.qty_ordered}">
        <div style="flex:2;font-weight:600">${it.name}</div>
        <div class="dim small" style="width:80px">Requested: ${l.qty_ordered}</div>
        <input class="gi-qty" type="number" step="0.01" value="${l.qty_ordered}" style="flex:1;min-width:80px" />
      </div>
    `;
  }).join('');

  showModal('Dispatch Goods (Goods Issue)', `
    <div class="flex col gap-12">
      <div class="info-box" style="background:rgba(245,158,11,0.08);border:1px solid rgba(245,158,11,0.2);border-radius:8px;padding:12px;font-size:13px;color:var(--text-dim)">
        Dispatching goods will immediately deduct these items from the Factory inventory. The ITO status will change to IN_TRANSIT.
      </div>
      <div class="flex row gap-12">
        <div class="field" style="flex:1"><label>Driver Name</label><input id="gi-driver" placeholder="e.g. John Doe" /></div>
        <div class="field" style="flex:1"><label>Vehicle Plate</label><input id="gi-plate" placeholder="e.g. 29A-12345" /></div>
      </div>
      <div style="font-weight:600;margin-top:8px">Quantities to Dispatch</div>
      ${lineInputs}
    </div>
  `, [
    { label: 'Dispatch Goods', primary: true, action: async () => {
      const els = document.querySelectorAll('#modal-body .gi-line');
      const linesData = [];
      els.forEach(el => {
        linesData.push({
          item_id: el.dataset.item,
          qty_issued: parseFloat(el.querySelector('.gi-qty').value) || 0
        });
      });
      try {
        await api.itoGoodsIssue(itoId, {
          driver_name: document.getElementById('gi-driver').value,
          vehicle_plate: document.getElementById('gi-plate').value,
          lines: linesData
        });
        toast('Goods dispatched successfully! Factory stock deducted.', 'success');
        closeModal();
        navigate('fac-ito', 'Internal Transfers');
      } catch(e) { toast(e.message, 'error'); }
    }},
    { label: 'Cancel', action: closeModal }
  ]);
}
