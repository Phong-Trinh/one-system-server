/* ─── HQ / ROP Configuration ─────────────────────────────────────────────────
   Per-node, per-item reorder point (ROP) and sourcing strategy configuration.
──────────────────────────────────────────────────────────────────────────── */

async function renderHQROPConfig() {
  let nodes = [], items = [], configs = [];
  let error = null;

  try {
    [nodes, items] = await Promise.all([
      api.getNodes(state.orgId),
      api.getItems(state.orgId)
    ]);
  } catch(e) { error = e.message; }

  const selectedNodeId = window._ropSelectedNode || (nodes[0] && nodes[0].id) || '';

  // Load configs for selected node
  if (selectedNodeId) {
    try {
      configs = await api.getNodeItemConfigs(selectedNodeId) || [];
    } catch(e) { configs = []; }
  }

  const itemMap = {};
  (items || []).forEach(it => itemMap[it.id] = it);

  const nodeOptions = (nodes || []).map(n =>
    `<option value="${n.id}" ${n.id === selectedNodeId ? 'selected' : ''}>${n.name} (${n.type})</option>`
  ).join('');

  const strategyBadge = s => s === 'INTERNAL_TRANSFER'
    ? `<span class="badge badge-primary">Internal Transfer</span>`
    : `<span class="badge badge-dim">External Procurement</span>`;

  const rows = configs.map(cfg => {
    const item = itemMap[cfg.item_id] || { name: cfg.item_id };
    return `
      <tr>
        <td style="font-weight:600">${item.name}</td>
        <td>${strategyBadge(cfg.sourcing_strategy)}</td>
        <td>${cfg.reorder_point}</td>
        <td>${cfg.safety_stock}</td>
        <td>${cfg.supplier_lead_time_days}d</td>
        <td>${cfg.provider_node_id || cfg.supplier_id || '—'}</td>
        <td>
          <button class="btn btn-ghost btn-sm" onclick="openEditROPModal('${selectedNodeId}', '${cfg.item_id}', ${JSON.stringify(cfg).replace(/"/g, '&quot;')})">Edit</button>
        </td>
      </tr>
    `;
  }).join('');

  return `
    ${pageHeader('ROP Configuration', 'Set reorder points, safety stock, and sourcing strategy per node and item')}
    ${error ? `<div class="empty-state" style="color:var(--red)">${error}</div>` : ''}

    <div class="flex row gap-12 align-center" style="margin-bottom:20px">
      <div class="field" style="margin:0; min-width:280px">
        <label class="small dim">Select Node</label>
        <select onchange="window._ropSelectedNode = this.value; navigate('hq-rop-config', 'ROP Configuration')">
          ${nodeOptions}
        </select>
      </div>
      <button class="btn btn-primary btn-sm" onclick="openCreateROPModal('${selectedNodeId}')">+ Add Config</button>
    </div>

    ${configs.length === 0 ? `
      <div class="empty-state">
        <div style="font-size:32px;margin-bottom:16px">📊</div>
        <h3>No ROP configs for this node</h3>
        <p class="dim">Add an item configuration to enable automatic replenishment.</p>
      </div>
    ` : `
      <div class="card" style="overflow:auto">
        <table class="data-table">
          <thead><tr><th>Item</th><th>Strategy</th><th>ROP</th><th>Safety Stock</th><th>Lead Time</th><th>Provider / Supplier</th><th>Action</th></tr></thead>
          <tbody>${rows}</tbody>
        </table>
      </div>
    `}
  `;
}

/* ── Modals ──────────────────────────────────────────────────────────────── */

async function openCreateROPModal(nodeId) {
  let items = [];
  let nodes = [];
  try {
    [items, nodes] = await Promise.all([api.getItems(state.orgId), api.getNodes(state.orgId)]);
  } catch(e) {}

  const itemOptions = items.map(it => `<option value="${it.id}">${it.name} (${it.type})</option>`).join('');
  const nodeOptions = nodes.map(n => `<option value="${n.id}">${n.name} (${n.type})</option>`).join('');

  showModal('New ROP Configuration', `
    <div class="flex col gap-12">
      <div class="field"><label>Item *</label><select id="rop-item">${itemOptions}</select></div>
      <div class="field">
        <label>Sourcing Strategy *</label>
        <select id="rop-strategy" onchange="document.getElementById('rop-provider-row').style.display=this.value==='INTERNAL_TRANSFER'?'flex':'none'; document.getElementById('rop-supplier-row').style.display=this.value==='EXTERNAL_PROCUREMENT'?'flex':'none'">
          <option value="INTERNAL_TRANSFER">Internal Transfer (Factory → Node)</option>
          <option value="EXTERNAL_PROCUREMENT">External Procurement (Supplier)</option>
        </select>
      </div>
      <div id="rop-provider-row" class="field">
        <label>Provider Node *</label>
        <select id="rop-provider">${nodeOptions}</select>
      </div>
      <div id="rop-supplier-row" class="field" style="display:none">
        <label>Supplier ID</label>
        <input id="rop-supplier" placeholder="supplier ID" />
      </div>
      <div class="flex row gap-12">
        <div class="field" style="flex:1"><label>Reorder Point (BU)</label><input id="rop-rop" type="number" value="50" min="0"/></div>
        <div class="field" style="flex:1"><label>Safety Stock (BU)</label><input id="rop-safety" type="number" value="20" min="0"/></div>
      </div>
      <div class="flex row gap-12">
        <div class="field" style="flex:1"><label>Lead Time (days)</label><input id="rop-lead" type="number" value="3" min="0"/></div>
        <div class="field" style="flex:1"><label>Consumption Window (days)</label><input id="rop-window" type="number" value="30" min="1"/></div>
      </div>
    </div>
  `, [
    { label: 'Save Config', primary: true, action: async () => {
      const strategy = document.getElementById('rop-strategy').value;
      const data = {
        node_id: nodeId,
        item_id: document.getElementById('rop-item').value,
        sourcing_strategy: strategy,
        provider_node_id: strategy === 'INTERNAL_TRANSFER' ? document.getElementById('rop-provider').value : null,
        supplier_id: strategy === 'EXTERNAL_PROCUREMENT' ? (document.getElementById('rop-supplier').value || null) : null,
        reorder_point: parseFloat(document.getElementById('rop-rop').value) || 50,
        safety_stock: parseFloat(document.getElementById('rop-safety').value) || 20,
        supplier_lead_time_days: parseInt(document.getElementById('rop-lead').value) || 3,
        consumption_window_days: parseInt(document.getElementById('rop-window').value) || 30,
      };
      try {
        await api.upsertNodeItemConfig(data);
        toast('ROP config saved', 'success');
        closeModal();
        navigate('hq-rop-config', 'ROP Configuration');
      } catch(e) { toast(e.message, 'error'); }
    }},
    { label: 'Cancel', action: closeModal }
  ]);
}

function openEditROPModal(nodeId, itemId, cfg) {
  const config = typeof cfg === 'string' ? JSON.parse(cfg.replace(/&quot;/g, '"')) : cfg;
  showModal('Edit ROP Configuration', `
    <div class="flex col gap-12">
      <div class="flex row gap-12">
        <div class="field" style="flex:1"><label>Reorder Point (BU)</label><input id="erop-rop" type="number" value="${config.reorder_point || 50}" min="0"/></div>
        <div class="field" style="flex:1"><label>Safety Stock (BU)</label><input id="erop-safety" type="number" value="${config.safety_stock || 20}" min="0"/></div>
      </div>
      <div class="flex row gap-12">
        <div class="field" style="flex:1"><label>Lead Time (days)</label><input id="erop-lead" type="number" value="${config.supplier_lead_time_days || 3}" min="0"/></div>
        <div class="field" style="flex:1"><label>Consumption Window (days)</label><input id="erop-window" type="number" value="${config.consumption_window_days || 30}" min="1"/></div>
      </div>
    </div>
  `, [
    { label: 'Save', primary: true, action: async () => {
      try {
        await api.upsertNodeItemConfig({
          ...config,
          node_id: nodeId,
          item_id: itemId,
          reorder_point: parseFloat(document.getElementById('erop-rop').value) || 50,
          safety_stock: parseFloat(document.getElementById('erop-safety').value) || 20,
          supplier_lead_time_days: parseInt(document.getElementById('erop-lead').value) || 3,
          consumption_window_days: parseInt(document.getElementById('erop-window').value) || 30,
        });
        toast('ROP config updated', 'success');
        closeModal();
        navigate('hq-rop-config', 'ROP Configuration');
      } catch(e) { toast(e.message, 'error'); }
    }},
    { label: 'Cancel', action: closeModal }
  ]);
}
