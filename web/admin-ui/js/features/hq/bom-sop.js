/* ─── HQ / BOM & SOP ─────────────────────────────────────────────────────────
   Bill of Materials and Standard Operating Procedure management.
   Fully wired to real API — no mock data.
──────────────────────────────────────────────────────────────────────────── */

async function renderHQBOM() {
  let boms = [], items = [];
  let error = null;
  try {
    [boms, items] = await Promise.all([
      api.getBOMs(),
      api.getItems(state.orgId)
    ]);
    boms = boms || [];
    items = items || [];
  } catch(e) { error = e.message; }

  const itemMap = {};
  items.forEach(it => itemMap[it.id] = it);

  const cards = boms.map(bom => {
    const outputItem = itemMap[bom.output_item_id] || { name: bom.output_item_id };
    return `
      <div class="card" style="padding:20px">
        <div class="flex row justify-between align-start">
          <div>
            <div style="font-weight:700;font-size:15px">${outputItem.name}</div>
            <div class="dim small" style="margin-top:4px">BOM v${bom.version} • <code style="font-size:11px">${bom.id.slice(0,8)}…</code></div>
          </div>
          <div class="flex row gap-8">
            <button class="btn btn-ghost btn-sm" onclick="openBOMDetail('${bom.id}')">View Detail</button>
            <button class="btn btn-ghost btn-sm" onclick="openSOPModal('${bom.id}', '${outputItem.name}')">SOP</button>
          </div>
        </div>
      </div>
    `;
  }).join('');

  const itemOptionsForCreate = items
    .filter(it => it.type === 'PRODUCT' || it.type === 'SEMI_PRODUCT')
    .map(it => `<option value="${it.id}">${it.name} (${it.type})</option>`)
    .join('');

  return `
    ${pageHeader('BOM & SOP', 'Bill of Materials and Standard Operating Procedures')}
    ${error ? `<div class="empty-state" style="color:var(--red)">${error}</div>` : ''}

    <div class="flex row justify-between align-center" style="margin-bottom:20px">
      <div class="dim small">${boms.length} BOM(s) defined</div>
      <button class="btn btn-primary btn-sm" onclick="openCreateBOMModal()">+ Create BOM</button>
    </div>

    ${boms.length === 0 ? `
      <div class="empty-state">
        <div style="font-size:32px;margin-bottom:16px">⚙️</div>
        <h3>No BOMs defined</h3>
        <p class="dim">Create items first, then define BOMs for products and semi-products.</p>
      </div>
    ` : `<div class="flex col gap-12">${cards}</div>`}
  `;
}

/* ── Create BOM ──────────────────────────────────────────────────────────── */

async function openCreateBOMModal() {
  let items = [];
  try { items = await api.getItems(state.orgId) || []; } catch(e) {}

  const outputOptions = items
    .filter(it => it.type === 'PRODUCT' || it.type === 'SEMI_PRODUCT')
    .map(it => `<option value="${it.id}">${it.name} (${it.type})</option>`)
    .join('');

  const allItemOptions = items.map(it =>
    `<option value="${it.id}" data-unit="${it.base_unit || 'unit'}">${it.name} (${it.type}) [${it.base_unit || 'unit'}]</option>`
  ).join('');

  showModal('Create BOM', `
    <div class="flex col gap-12">
      <div class="field">
        <label>Output Item (what this BOM produces) *</label>
        <select id="bom-output">${outputOptions}</select>
      </div>
      <div style="font-weight:600;margin-top:8px">Ingredients (BOM Lines)</div>
      <div id="bom-lines" class="flex col gap-8">
        <div class="bom-line flex row gap-8 align-center">
          <select class="bom-line-item" style="flex:2">${allItemOptions}</select>
          <input class="bom-line-qty" type="number" step="0.01" min="0.01" value="1" placeholder="Qty" style="flex:1;min-width:80px" />
          <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="this.closest('.bom-line').remove()">✕</button>
        </div>
      </div>
      <button class="btn btn-outline btn-sm" onclick="addBOMLine(\`${allItemOptions.replace(/`/g, '\\`')}\`)">+ Add Ingredient</button>
    </div>
  `, [
    { label: 'Create BOM', primary: true, action: async () => {
      const outputItemId = document.getElementById('bom-output').value;
      const lineEls = document.querySelectorAll('#modal-body .bom-line');
      const lines = [];
      lineEls.forEach(el => {
        const itemId = el.querySelector('.bom-line-item').value;
        const qty = parseFloat(el.querySelector('.bom-line-qty').value);
        if (itemId && qty > 0) lines.push({ input_item_id: itemId, qty_required: qty });
      });
      if (lines.length === 0) { toast('Add at least one ingredient', 'error'); return; }
      try {
        await api.createBOM({ output_item_id: outputItemId, lines });
        toast('BOM created!', 'success');
        closeModal();
        navigate('hq-bom', 'BOM & SOP');
      } catch(e) { toast(e.message, 'error'); }
    }},
    { label: 'Cancel', action: closeModal }
  ]);
}

function addBOMLine(allItemOptions) {
  const container = document.getElementById('bom-lines');
  const div = document.createElement('div');
  div.className = 'bom-line flex row gap-8 align-center';
  div.innerHTML = `
    <select class="bom-line-item" style="flex:2">${allItemOptions}</select>
    <input class="bom-line-qty" type="number" step="0.01" min="0.01" value="1" placeholder="Qty" style="flex:1;min-width:80px" />
    <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="this.closest('.bom-line').remove()">✕</button>
  `;
  container.appendChild(div);
}

/* ── BOM Detail ──────────────────────────────────────────────────────────── */

async function openBOMDetail(bomId) {
  let result, items = [];
  try {
    [result, items] = await Promise.all([api.getBOMByID(bomId), api.getItems(state.orgId)]);
  } catch(e) { toast(e.message, 'error'); return; }

  const itemMap = {};
  (items || []).forEach(it => itemMap[it.id] = it);

  const lines = (result.lines || []).map(l => {
    const item = itemMap[l.item_id] || { name: l.item_id, base_unit: '' };
    return `<tr><td>${item.name}</td><td>${l.qty} ${item.base_unit}</td></tr>`;
  }).join('');

  const outputItem = itemMap[result.bom.output_item_id] || { name: result.bom.output_item_id };

  showModal(`BOM — ${outputItem.name}`, `
    <div class="flex col gap-12">
      <div class="dim small">Version ${result.bom.version} • ID: ${result.bom.id}</div>
      <table class="data-table">
        <thead><tr><th>Ingredient</th><th>Qty Required</th></tr></thead>
        <tbody>${lines}</tbody>
      </table>
    </div>
  `, [{ label: 'Close', action: closeModal }]);
}

/* ── SOP Modal ───────────────────────────────────────────────────────────── */

async function openSOPModal(bomId, productName) {
  let result = null;
  try { result = await api.getSOPByBOM(bomId); } catch(e) {}

  const steps = result ? (result.steps || []) : [];

  const stepRows = steps.map((s, i) => `
    <div class="flex row gap-8 align-start" style="padding:8px 0;border-bottom:1px solid var(--border)">
      <div style="width:24px;height:24px;border-radius:50%;background:var(--primary);color:#fff;display:flex;align-items:center;justify-content:center;font-size:12px;font-weight:700;flex-shrink:0">${s.seq_no}</div>
      <div style="flex:1">
        <div style="font-weight:600">${s.description || 'Step ' + s.seq_no}</div>
        <div class="dim small">Equip: ${s.equipment_type_id || 'None'}</div>
        <div class="small" style="margin-top:4px">Duration: ${s.duration ? (s.duration / 60).toFixed(1) : 0} min</div>
      </div>
    </div>
  `).join('');

  showModal(`SOP — ${productName}`, `
    <div class="flex col gap-12">
      ${result ? `<div class="dim small">SOP ID: ${result.sop.id}</div>` : '<div class="dim small">No SOP created yet for this BOM.</div>'}
      ${stepRows || '<div class="dim" style="padding:16px 0">No steps defined.</div>'}
      <button class="btn btn-outline btn-sm" onclick="openCreateSOPStepsModal('${bomId}', '${productName}')">
        ${result ? 'Edit Steps' : 'Create SOP'}
      </button>
    </div>
  `, [{ label: 'Close', action: closeModal }]);
}

async function openCreateSOPStepsModal(bomId, productName) {
  closeModal();
  showModal(`Create SOP — ${productName}`, `
    <div class="flex col gap-12">
      <div id="sop-steps" class="flex col gap-8">
        <div class="sop-step flex col gap-8" style="border:1px solid var(--border);border-radius:8px;padding:12px">
          <div class="flex row gap-8">
            <div class="field" style="flex:1"><label>Step Name</label><input class="sop-name" value="Mix Ingredients" /></div>
            <div class="field" style="width:80px"><label>Minutes</label><input class="sop-duration" type="number" value="10" min="1" /></div>
          </div>
          <div class="field"><label>Description</label><input class="sop-desc" placeholder="Optional description" /></div>
          <div class="field"><label>Station Type ID (optional)</label><input class="sop-station" placeholder="equipment type ID" /></div>
        </div>
      </div>
      <button class="btn btn-outline btn-sm" onclick="addSOPStep()">+ Add Step</button>
    </div>
  `, [
    { label: 'Create SOP', primary: true, action: async () => {
      const stepEls = document.querySelectorAll('#modal-body .sop-step');
      const steps = [];
      stepEls.forEach((el, i) => {
        steps.push({
          step_order: i + 1,
          name: el.querySelector('.sop-name').value || `Step ${i+1}`,
          description: el.querySelector('.sop-desc').value,
          duration_minutes: parseInt(el.querySelector('.sop-duration').value) || 10,
          station_type_id: el.querySelector('.sop-station').value || null,
        });
      });
      if (steps.length === 0) { toast('Add at least one step', 'error'); return; }
      try {
        await api.createSOP({ bom_id: bomId, steps });
        toast('SOP created!', 'success');
        closeModal();
        navigate('hq-bom', 'BOM & SOP');
      } catch(e) { toast(e.message, 'error'); }
    }},
    { label: 'Cancel', action: closeModal }
  ]);
}

function addSOPStep() {
  const container = document.getElementById('sop-steps');
  const stepCount = container.querySelectorAll('.sop-step').length + 1;
  const div = document.createElement('div');
  div.className = 'sop-step flex col gap-8';
  div.style.cssText = 'border:1px solid var(--border);border-radius:8px;padding:12px;position:relative';
  div.innerHTML = `
    <button class="btn btn-ghost btn-sm" style="position:absolute;top:8px;right:8px;color:var(--red)" onclick="this.closest('.sop-step').remove()">✕</button>
    <div class="flex row gap-8">
      <div class="field" style="flex:1"><label>Step Name</label><input class="sop-name" value="Step ${stepCount}" /></div>
      <div class="field" style="width:80px"><label>Minutes</label><input class="sop-duration" type="number" value="10" min="1" /></div>
    </div>
    <div class="field"><label>Description</label><input class="sop-desc" placeholder="Optional" /></div>
    <div class="field"><label>Station Type ID (optional)</label><input class="sop-station" placeholder="equipment type ID" /></div>
  `;
  container.appendChild(div);
}
