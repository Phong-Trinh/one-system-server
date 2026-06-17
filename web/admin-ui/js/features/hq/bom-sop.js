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
  } catch (e) { error = e.message; }

  const itemMap = {};
  items.forEach(it => itemMap[it.id] = it);

  const cards = boms.map(bom => {
    const outputItem = itemMap[bom.output_item_id] || { name: bom.output_item_id };
    return `
      <div class="card" style="padding:20px">
        <div class="flex row justify-between align-start">
          <div>
            <div style="font-weight:700;font-size:15px">${outputItem.name}</div>
            <div class="dim small" style="margin-top:4px">BOM v${bom.version} • <code style="font-size:11px">${bom.id.slice(0, 8)}…</code></div>
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
  let items = [], boms = [];
  try {
    [items, boms] = await Promise.all([
      api.getItems(state.orgId).catch(() => []),
      api.getBOMs().catch(() => [])
    ]);
    items = items || [];
    boms = boms || [];
  } catch (e) { }

  // Items that already have a BOM are ineligible as output
  const bookedOutputIds = new Set(boms.map(b => b.output_item_id));

  const outputItems = items.filter(it =>
    (it.type === 'PRODUCT' || it.type === 'SEMI_PRODUCT') && !bookedOutputIds.has(it.id)
  );

  if (outputItems.length === 0) {
    toast('All PRODUCT/SEMI_PRODUCT items already have a BOM. Create a new item first.', 'warning');
    return;
  }

  const outputOptions = outputItems
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
        <small class="dim">Only items without an existing BOM are shown.</small>
      </div>
      <div style="font-weight:600;margin-top:8px">Ingredients (BOM Lines)</div>
      <div id="bom-lines" class="flex col gap-8">
        <div class="bom-line flex row gap-8 align-center">
          <select class="bom-line-item" style="flex:2" onchange="updateBOMLineUnit(this)">${allItemOptions}</select>
          <input class="bom-line-qty" type="number" step="0.01" min="0.01" value="1" placeholder="Qty" style="flex:1;min-width:80px" />
          <span class="bom-line-unit dim small" style="min-width:40px;white-space:nowrap"></span>
          <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="this.closest('.bom-line').remove()">✕</button>
        </div>
      </div>
      <div class="flex row gap-8">
        <button class="btn btn-outline btn-sm" onclick="addBOMLine()">+ Add Ingredient</button>
        <button class="btn btn-ghost btn-sm" onclick="toggleQuickAddItem()" style="font-size:12px">⚡ Quick add new item</button>
      </div>

      <!-- Quick Add New Item Panel -->
      <div id="quick-add-item-panel" style="display:none;border:1px dashed var(--border);border-radius:8px;padding:16px;background:var(--bg-hover)">
        <div style="font-weight:600;font-size:13px;margin-bottom:12px">⚡ Quick Add New Item</div>
        <div class="grid-2 gap-12">
          <div class="field">
            <label>Name *</label>
            <input id="qa-item-name" placeholder="e.g. Burger Bun" />
          </div>
          <div class="field">
            <label>SKU</label>
            <input id="qa-item-sku" placeholder="e.g. BUN-001" />
          </div>
          <div class="field">
            <label>Type *</label>
            <select id="qa-item-type">
              <option value="RAW_MATERIAL">Raw Material</option>
              <option value="SEMI_PRODUCT">Semi Product</option>
              <option value="PRODUCT">Product</option>
            </select>
          </div>
          <div class="field">
            <label>Base Unit *</label>
            <input id="qa-item-unit" placeholder="e.g. gram, piece, ml" value="gram" />
          </div>
        </div>
        <div class="flex row gap-8" style="margin-top:12px">
          <button class="btn btn-primary btn-sm" onclick="quickCreateAndAddItem()">Create &amp; Add as Ingredient</button>
          <button class="btn btn-ghost btn-sm" onclick="toggleQuickAddItem()">Cancel</button>
        </div>
      </div>
    </div>
  `, [
    {
      label: 'Create BOM', primary: true, action: async () => {
        const outputItemId = document.getElementById('bom-output').value;
        const lineEls = document.querySelectorAll('#modal-body .bom-line');
        const lines = [];
        let selfRef = false;
        lineEls.forEach(el => {
          const itemId = el.querySelector('.bom-line-item').value;
          const qty = parseFloat(el.querySelector('.bom-line-qty').value);
          if (itemId === outputItemId) { selfRef = true; return; }
          if (itemId && qty > 0) lines.push({ item_id: itemId, qty });
        });
        if (selfRef) { toast('An ingredient cannot be the same as the output item', 'error'); return; }
        if (lines.length === 0) { toast('Add at least one ingredient', 'error'); return; }
        try {
          await api.createBOM({ output_item_id: outputItemId, lines });
          toast('BOM created!', 'success');
          closeModal();
          navigate('hq-bom', 'BOM & SOP');
        } catch (e) { toast(e.message, 'error'); }
      }
    },
    { label: 'Cancel', action: closeModal }
  ]);

  // Initialise unit badges for the first pre-rendered line
  document.querySelectorAll('#modal-body .bom-line-item').forEach(sel => updateBOMLineUnit(sel));
}

function toggleQuickAddItem() {
  const panel = document.getElementById('quick-add-item-panel');
  if (panel) panel.style.display = panel.style.display === 'none' ? 'block' : 'none';
}

async function quickCreateAndAddItem() {
  const name = (document.getElementById('qa-item-name')?.value || '').trim();
  const sku = (document.getElementById('qa-item-sku')?.value || '').trim();
  const type = document.getElementById('qa-item-type')?.value || 'RAW_MATERIAL';
  const unit = (document.getElementById('qa-item-unit')?.value || 'gram').trim();
  if (!name) { toast('Item name is required', 'error'); return; }
  try {
    const newItem = await api.createItem({ org_id: state.orgId || 'SNAPBITE_ORG', name, sku, type, base_unit: unit });
    toast(`Item "${name}" created`, 'success');

    // Append the new option to every existing ingredient dropdown so they stay in sync
    const newOpt = `<option value="${newItem.id}" data-unit="${unit}">${name} (${type}) [${unit}]</option>`;
    document.querySelectorAll('#modal-body .bom-line-item').forEach(sel => {
      sel.insertAdjacentHTML('beforeend', newOpt);
    });

    // Add a new BOM line pre-selected to the new item
    const container = document.getElementById('bom-lines');
    const div = document.createElement('div');
    div.className = 'bom-line flex row gap-8 align-center';
    div.innerHTML = `
      <select class="bom-line-item" style="flex:2" onchange="updateBOMLineUnit(this)">
        <option value="${newItem.id}" data-unit="${unit}" selected>${name} (${type}) [${unit}]</option>
      </select>
      <input class="bom-line-qty" type="number" step="0.01" min="0.01" value="1" placeholder="Qty" style="flex:1;min-width:80px" />
      <span class="bom-line-unit dim small" style="min-width:40px;white-space:nowrap">${unit}</span>
      <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="this.closest('.bom-line').remove()">✕</button>
    `;
    container.appendChild(div);

    // Reset and hide the quick-add panel
    document.getElementById('qa-item-name').value = '';
    document.getElementById('qa-item-sku').value = '';
    document.getElementById('qa-item-unit').value = 'gram';
    toggleQuickAddItem();
  } catch (e) { toast(e.message, 'error'); }
}

function updateBOMLineUnit(selectEl) {
  const selected = selectEl.options[selectEl.selectedIndex];
  const unit = selected ? (selected.dataset.unit || '') : '';
  const badge = selectEl.closest('.bom-line').querySelector('.bom-line-unit');
  if (badge) badge.textContent = unit;
}

function addBOMLine() {
  // Clone options from an existing ingredient select — always reflects the latest items
  const existingSelect = document.querySelector('#modal-body .bom-line-item');
  const allItemOptions = existingSelect ? existingSelect.innerHTML : '';

  const container = document.getElementById('bom-lines');
  const div = document.createElement('div');
  div.className = 'bom-line flex row gap-8 align-center';
  div.innerHTML = `
    <select class="bom-line-item" style="flex:2" onchange="updateBOMLineUnit(this)">${allItemOptions}</select>
    <input class="bom-line-qty" type="number" step="0.01" min="0.01" value="1" placeholder="Qty" style="flex:1;min-width:80px" />
    <span class="bom-line-unit dim small" style="min-width:40px;white-space:nowrap"></span>
    <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="this.closest('.bom-line').remove()">✕</button>
  `;
  container.appendChild(div);
  // Initialise unit badge for the newly added line
  updateBOMLineUnit(div.querySelector('.bom-line-item'));
}

/* ── BOM Detail ──────────────────────────────────────────────────────────── */

async function openBOMDetail(bomId) {
  let result, items = [];
  try {
    [result, items] = await Promise.all([api.getBOMByID(bomId), api.getItems(state.orgId)]);
  } catch (e) { toast(e.message, 'error'); return; }

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
  try { result = await api.getSOPByBOM(bomId); } catch (e) { }

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

let currentEquipTypes = [];
let currentSOPSteps = [];

async function openCreateSOPStepsModal(bomId, productName) {
  closeModal();
  let existingSOP = null;
  let steps = [];
  try {
    const result = await api.getSOPByBOM(bomId);
    if (result && result.sop) {
      existingSOP = result.sop;
      steps = result.steps || [];
    }
  } catch (e) { }

  try {
    currentEquipTypes = await api.getEquipmentTypes() || [];
  } catch (e) {
    currentEquipTypes = [];
  }

  currentSOPSteps = steps;

  const isUpdate = !!existingSOP;

  showModal(`${isUpdate ? 'Edit' : 'Create'} SOP — ${productName}`, `
    <div class="flex col gap-12">
      <div id="sop-steps" class="flex col gap-16"></div>
      <button class="btn btn-outline btn-sm" onclick="addSOPStep()">+ Add Step</button>
    </div>
  `, [
    {
      label: isUpdate ? 'Save Changes' : 'Create SOP', primary: true, action: async () => {
        const stepEls = document.querySelectorAll('#modal-body .sop-step');
        const newSteps = [];
        stepEls.forEach((el, i) => {
          const dependsOnChks = el.querySelectorAll('.sop-depends-on-chk:checked');
          const dependsOn = Array.from(dependsOnChks).map(chk => chk.value);
          const equipTypeId = el.querySelector('.sop-station').value || null;

          const step = {
            id: el.dataset.id || '',
            seq_no: i + 1,
            description: el.querySelector('.sop-desc').value || `Step ${i + 1}`,
            duration: parseInt(el.querySelector('.sop-duration').value) || 10,
            equipment_type_id: equipTypeId,
            depends_on: dependsOn,
            ingredient_bom_line_ids: [],
            slot_consumption: parseFloat(el.querySelector('.sop-slots').value) || 0,
            allow_mix: el.querySelector('.sop-allow-mix').checked
          };
          newSteps.push(step);
        });
        if (newSteps.length === 0) { toast('Add at least one step', 'error'); return; }
        try {
          if (isUpdate) {
            await api.updateSOP(existingSOP.id, { steps: newSteps });
            toast('SOP updated!', 'success');
          } else {
            await api.createSOP({ bom_id: bomId, steps: newSteps });
            toast('SOP created!', 'success');
          }
          closeModal();
          navigate('hq-bom', 'BOM & SOP');
        } catch (e) { toast(e.message, 'error'); }
      }
    },
    { label: 'Cancel', action: closeModal }
  ]);

  const container = document.getElementById('sop-steps');
  if (steps.length > 0) {
    steps.forEach((s, i) => {
      container.appendChild(createStepElement(s, i));
    });
  } else {
    addSOPStep();
  }

  // Initialize dependency lists
  setTimeout(updateDependsOnLists, 0);
}

function updateDependsOnLists() {
  const stepEls = Array.from(document.querySelectorAll('#modal-body .sop-step'));

  stepEls.forEach((el, i) => {
    if (!el.dataset.id) {
      el.dataset.id = 'temp_' + Math.random().toString(36).substr(2, 9);
    }
  });

  stepEls.forEach((el, i) => {
    const container = el.querySelector('.sop-depends-container');
    if (!container) return;

    let checked = [];
    const chks = container.querySelectorAll('.sop-depends-on-chk');
    if (chks.length > 0) {
      checked = Array.from(container.querySelectorAll('.sop-depends-on-chk:checked')).map(c => c.value);
    } else {
      try { checked = JSON.parse(container.dataset.depends || '[]'); } catch (e) { }
    }

    let html = '';
    let hasDepends = false;

    for (let j = 0; j < i; j++) {
      hasDepends = true;
      const prevEl = stepEls[j];
      const prevId = prevEl.dataset.id;
      const prevDesc = prevEl.querySelector('.sop-desc').value || `Step ${j + 1}`;
      const isChecked = checked.includes(prevId) ? 'checked' : '';

      html += `
        <label class="flex row gap-8 align-center" style="cursor:pointer; padding: 4px 8px; border-radius: 4px; background: var(--bg-hover);">
          <input type="checkbox" class="sop-depends-on-chk" value="${prevId}" ${isChecked} style="margin:0" />
          <span style="font-weight:normal;font-size:13px">${prevDesc} <span class="dim small">(${prevId.startsWith('temp_') ? 'New' : prevId})</span></span>
        </label>
      `;
    }

    if (!hasDepends) {
      html = '<div class="dim small" style="padding-top:8px">No previous steps available to depend on.</div>';
    }

    container.innerHTML = html;
  });
}

function createStepElement(step = {}, index = 0) {
  const div = document.createElement('div');
  div.className = 'sop-step flex col gap-12';
  div.style.cssText = 'border:1px solid var(--border);border-radius:8px;padding:16px;position:relative';
  div.dataset.id = step.id || '';

  const stepIdDisplay = step.id ? `<div class="small dim mb-8">Step ID: ${step.id}</div>` : '';

  const equipOptions = '<option value="" data-capacity-unit="">None (Manual)</option>' + currentEquipTypes.map(et =>
    `<option value="${et.id}" data-capacity-unit="${et.capacity_unit || ''}" ${step.equipment_type_id === et.id ? 'selected' : ''}>${et.name} (${et.capacity_unit || 'unit'})</option>`
  ).join('');

  // Determine initial capacity unit label from the step's selected equipment type
  const selectedEqType = step.equipment_type_id
    ? currentEquipTypes.find(et => et.id === step.equipment_type_id)
    : null;
  const initialCapacityUnit = selectedEqType ? selectedEqType.capacity_unit : '';

  div.innerHTML = `
    <button class="btn btn-ghost btn-sm" style="position:absolute;top:8px;right:8px;color:var(--red)" onclick="this.closest('.sop-step').remove(); updateDependsOnLists();">✕</button>
    ${stepIdDisplay}
    <div class="grid-2 gap-16">
      <div class="field"><label>Description *</label><input class="sop-desc" value="${step.description || ''}" placeholder="E.g., Nướng Bò" oninput="updateDependsOnLists()" /></div>
      <div class="field"><label>Duration (Seconds) *</label><input class="sop-duration" type="number" value="${step.duration || 60}" min="1" /></div>
    </div>
    
    <div class="grid-2 gap-16">
      <div class="field">
        <label>Equipment Type</label>
        <select class="sop-station" onchange="updateSOPStepSlotUnit(this)">${equipOptions}</select>
      </div>
      <div class="field">
        <label>Depends On (Prerequisites)</label>
        <div class="sop-depends-container flex col gap-4" style="max-height: 120px; overflow-y: auto; padding: 4px 0;" data-depends='${JSON.stringify(step.depends_on || [])}'>
        </div>
      </div>
    </div>
    
    <div class="grid-2 gap-16" style="padding-top:12px; border-top:1px dashed var(--border)">
      <div class="field">
        <label>Slot Consumption</label>
        <div class="flex row gap-8 align-center">
          <input class="sop-slots" type="number" step="0.1" value="${step.slot_consumption !== undefined ? step.slot_consumption : 1}" style="flex:1" />
          <span class="sop-slot-cap-unit dim small" style="white-space:nowrap;min-width:32px">${initialCapacityUnit}</span>
        </div>
        <small class="dim sop-slot-unit-hint">${initialCapacityUnit ? `Capacity consumed per unit of output [${initialCapacityUnit}]` : 'Select an equipment type to see capacity unit'}</small>
      </div>
      <div class="field" style="display:flex;align-items:center;padding-top:24px;">
        <label class="flex row gap-8" style="cursor:pointer; font-weight:normal; user-select:none;">
          <input type="checkbox" class="sop-allow-mix" ${step.allow_mix ? 'checked' : ''} style="width:auto;margin:0" />
          Allow Mix (can share machine with other items)
        </label>
      </div>
    </div>
  `;
  return div;
}

/**
 * updateSOPStepSlotUnit — called when Equipment Type select changes in a SOP step.
 * Reads data-capacity-unit from the selected option and updates the slot consumption
 * unit badge and hint text within the same step card.
 */
function updateSOPStepSlotUnit(selectEl) {
  const selected = selectEl.options[selectEl.selectedIndex];
  const unit = selected ? (selected.dataset.capacityUnit || '') : '';
  const step = selectEl.closest('.sop-step');
  if (!step) return;

  const capUnitBadge = step.querySelector('.sop-slot-cap-unit');
  const hint = step.querySelector('.sop-slot-unit-hint');
  if (capUnitBadge) capUnitBadge.textContent = unit;
  if (hint) hint.textContent = unit
    ? `Capacity consumed per unit of output [${unit}]`
    : 'Select an equipment type to see capacity unit';
}

function addSOPStep() {
  const container = document.getElementById('sop-steps');
  const count = container.querySelectorAll('.sop-step').length;
  container.appendChild(createStepElement({}, count));
  updateDependsOnLists();
}
