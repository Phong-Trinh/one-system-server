/* ─── HQ / Items Catalog ──────────────────────────────────────────────────────
   CRUD management for the Item master data catalog.
   HQ only — Factory/Store see items in dropdowns only.
──────────────────────────────────────────────────────────────────────────── */

async function renderHQItems() {
  let items = [];
  let error = null;
  try {
    items = await api.getItems(state.orgId) || [];
  } catch(e) { error = e.message; }

  const typeBadge = t => ({
    'RAW_MATERIAL': `<span class="badge badge-dim">Raw Material</span>`,
    'SEMI_PRODUCT': `<span class="badge badge-primary">Semi Product</span>`,
    'PRODUCT':      `<span class="badge badge-green">Product</span>`,
  }[t] || `<span class="badge">${t}</span>`);

  const rows = items.map(it => `
    <tr>
      <td><code style="font-size:11px">${it.id.slice(0,8)}…</code></td>
      <td style="font-weight:600">${it.name}</td>
      <td><code>${it.sku || '—'}</code></td>
      <td>${typeBadge(it.type)}</td>
      <td><code>${it.base_unit || 'unit'}</code></td>
      <td>
        <button class="btn btn-ghost btn-sm" onclick="openEditItemModal('${it.id}', '${it.name}', '${it.sku || ''}', '${it.type}', '${it.base_unit || 'unit'}')">Edit</button>
        <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="deleteItem('${it.id}', '${it.name}')">Delete</button>
      </td>
    </tr>
  `).join('');

  return `
    ${pageHeader('Items Catalog', 'Master item definitions — raw materials, semi-products, and finished products')}
    ${error ? `<div class="empty-state" style="color:var(--red)">${error}</div>` : ''}

    <div class="flex row justify-between align-center" style="margin-bottom:16px">
      <div class="dim small">${items.length} items total</div>
      <button class="btn btn-primary btn-sm" onclick="openCreateItemModal()">+ New Item</button>
    </div>

    ${items.length === 0 ? `<div class="empty-state"><div style="font-size:32px;margin-bottom:16px">📦</div><h3>No items yet</h3><p class="dim">Create items to start building BOMs and production orders.</p></div>` : `
    <div class="card" style="overflow:auto">
      <table class="data-table">
        <thead>
          <tr><th>ID</th><th>Name</th><th>SKU</th><th>Type</th><th>Base Unit</th><th>Actions</th></tr>
        </thead>
        <tbody>${rows}</tbody>
      </table>
    </div>`}
  `;
}

/* ── Modals ──────────────────────────────────────────────────────────────── */

function openCreateItemModal() {
  const orgId = state.orgId || 'SNAPBITE_ORG';
  showModal('New Item', `
    <div class="flex col gap-12">
      <div class="field">
        <label>Name *</label>
        <input id="item-name" placeholder="e.g. Burger Bun" />
      </div>
      <div class="field">
        <label>SKU</label>
        <input id="item-sku" placeholder="e.g. BUN-001" />
      </div>
      <div class="field">
        <label>Type *</label>
        <select id="item-type">
          <option value="RAW_MATERIAL">Raw Material</option>
          <option value="SEMI_PRODUCT">Semi Product</option>
          <option value="PRODUCT">Product (Finished Good)</option>
        </select>
      </div>
      <div class="field">
        <label>Base Unit *</label>
        <input id="item-unit" placeholder="e.g. g, piece, kg, ml" value="piece" />
      </div>
    </div>
  `, [
    { label: 'Create Item', primary: true, action: async () => {
      const name = document.getElementById('item-name').value.trim();
      const sku  = document.getElementById('item-sku').value.trim();
      const type = document.getElementById('item-type').value;
      const unit = document.getElementById('item-unit').value.trim() || 'piece';
      if (!name) { toast('Name is required', 'error'); return; }
      try {
        await api.createItem({ org_id: orgId, name, sku, type, base_unit: unit });
        toast(`Item "${name}" created`, 'success');
        closeModal();
        navigate('hq-items', 'Items Catalog');
      } catch(e) { toast(e.message, 'error'); }
    }},
    { label: 'Cancel', action: closeModal }
  ]);
}

function openEditItemModal(id, name, sku, type, unit) {
  showModal('Edit Item', `
    <div class="flex col gap-12">
      <div class="field"><label>Name *</label><input id="edit-item-name" value="${name}" /></div>
      <div class="field"><label>SKU</label><input id="edit-item-sku" value="${sku}" /></div>
      <div class="field">
        <label>Type *</label>
        <select id="edit-item-type">
          <option value="RAW_MATERIAL" ${type==='RAW_MATERIAL'?'selected':''}>Raw Material</option>
          <option value="SEMI_PRODUCT" ${type==='SEMI_PRODUCT'?'selected':''}>Semi Product</option>
          <option value="PRODUCT" ${type==='PRODUCT'?'selected':''}>Product (Finished Good)</option>
        </select>
      </div>
      <div class="field"><label>Base Unit *</label><input id="edit-item-unit" value="${unit}" /></div>
    </div>
  `, [
    { label: 'Save Changes', primary: true, action: async () => {
      const nameV = document.getElementById('edit-item-name').value.trim();
      const skuV  = document.getElementById('edit-item-sku').value.trim();
      const typeV = document.getElementById('edit-item-type').value;
      const unitV = document.getElementById('edit-item-unit').value.trim() || 'piece';
      if (!nameV) { toast('Name is required', 'error'); return; }
      try {
        await api.updateItem(id, { name: nameV, sku: skuV, type: typeV, base_unit: unitV });
        toast(`Item updated`, 'success');
        closeModal();
        navigate('hq-items', 'Items Catalog');
      } catch(e) { toast(e.message, 'error'); }
    }},
    { label: 'Cancel', action: closeModal }
  ]);
}

async function deleteItem(id, name) {
  if (!confirm(`Delete item "${name}"? This cannot be undone.`)) return;
  try {
    await api.deleteItem(id);
    toast(`Item "${name}" deleted`, 'success');
    navigate('hq-items', 'Items Catalog');
  } catch(e) { toast(e.message, 'error'); }
}
