/* ─── Factory / Submit PR ──────────────────────────────────────────────────
   CapEx purchase requisition form for Factory.
   Depends on: api.js, toast.js, router.js, shared/helpers.js.
──────────────────────────────────────────────────────────────────────────── */

async function renderFacPR() {
  let eqTypes = [];
  try {
    const res = await api.getEquipmentTypes();
    eqTypes = res || [];
  } catch (e) { }

  let prs = [];
  try {
    const res = await api.getPRs(state.node);
    prs = res || [];
  } catch (e) { }

  return `
    ${pageHeader(
    'Submit PR to HQ',
    'Request CapEx equipment or exceptional purchases'
  )}
    
    <div class="grid-2 gap-24" style="margin-bottom: 24px;">
      <!-- Actual PR Form -->
      <div class="card" style="width: 100%;">
      <div class="field">
        <label>Equipment Type</label>
        <select id="pr-eq-type" onchange="toggleNewEquipmentType()">
          <option value="">-- Select Equipment Type --</option>
          ${eqTypes.filter(e => e.status === 'ACTIVE').map(eq => `<option value="${eq.id}" data-unit="${eq.capacity_unit || ''}">${eq.name}</option>`).join('')}
          <option value="NEW">➕ Request New Equipment Type...</option>
        </select>
      </div>

      <div id="new-eq-fields" class="hidden" style="margin-top: 16px; padding: 16px; border: 1px solid var(--border); border-radius: 6px;">
        <div class="field">
          <label>Proposed Name</label>
          <input type="text" id="pr-prop-name" placeholder="e.g., Industrial Pizza Oven">
        </div>
        <div class="field">
          <label>Capacity Unit</label>
          <input type="text" id="pr-prop-cap-unit" placeholder="e.g., trays, liters, slots">
        </div>
      </div>

      <div class="grid-2" style="margin-top:16px">
        <div class="field">
          <label>Expected Capacity <span id="pr-exp-cap-unit-label" class="dim" style="font-weight: normal;"></span></label>
          <input type="number" id="pr-exp-cap" placeholder="e.g., 4.0">
        </div>
        <div class="field">
          <label>Quantity</label>
          <input type="number" id="pr-qty" value="1" min="1">
        </div>
      </div>

      <div class="grid-2" style="margin-top:16px">
        <div class="field">
          <label>Unit of Measure</label>
          <input type="text" id="pr-uom" value="unit">
        </div>
        <div class="field">
          <label>Est. Unit Price (VND)</label>
          <input type="number" id="pr-price" placeholder="Estimated cost">
        </div>
      </div>

      <div class="field" style="margin-top:16px">
        <label>Detailed Description (Optional)</label>
        <textarea id="pr-desc" rows="4" placeholder="Describe the item specifications..."></textarea>
        <div class="dim small" style="margin-top:4px">Provide as much detail as possible to help HQ.</div>
      </div>

      <div class="field" style="margin-top:16px">
        <label>Justification</label>
        <textarea id="pr-note" rows="2" placeholder="Why is this needed?"></textarea>
      </div>
      
      <div style="margin-top:24px">
        <button class="btn btn-primary fw" onclick="submitFacPR()">Submit Requisition</button>
      </div>
    </div>

    <!-- Example Template Form -->
    <div class="card" style="width: 100%; border: 2px dashed var(--border); background: var(--bg-hover); opacity: 0.85;">
      <div style="display:flex; align-items:center; gap:8px; margin-bottom: 16px;">
        <span style="font-size:20px">💡</span>
        <h3 style="margin:0; color:var(--text)">Example: How to fill a PR</h3>
      </div>
      
      <div class="field">
        <label>Equipment Type</label>
        <select disabled style="background: var(--bg);"><option>➕ Request New Equipment Type...</option></select>
      </div>

      <div style="margin-top: 16px; padding: 16px; border: 1px solid var(--border); border-radius: 6px; background: var(--bg);">
        <div class="field">
          <label>Proposed Name</label>
          <input type="text" disabled value="Industrial Pizza Oven">
        </div>
        <div class="field">
          <label>Capacity Unit</label>
          <input type="text" disabled value="trays">
        </div>
      </div>

      <div class="grid-2" style="margin-top:16px">
        <div class="field">
          <label>Expected Capacity</label>
          <input type="number" disabled value="4">
        </div>
        <div class="field">
          <label>Quantity</label>
          <input type="number" disabled value="1">
        </div>
      </div>

      <div class="grid-2" style="margin-top:16px">
        <div class="field">
          <label>Unit of Measure</label>
          <input type="text" disabled value="unit">
        </div>
        <div class="field">
          <label>Est. Unit Price (VND)</label>
          <input type="text" disabled value="15000000">
        </div>
      </div>

      <div class="field" style="margin-top:16px">
        <label>Detailed Description</label>
        <textarea disabled rows="3" style="font-family:monospace; color:var(--text-dim); background: var(--bg);">- Kích thước tối đa: 1.2m x 0.8m
- Nguồn điện: 220V / 50Hz
- Chất liệu: Inox 304 chống rỉ
- Yêu cầu khác: Có bánh xe di chuyển</textarea>
      </div>

      <div class="field" style="margin-top:16px">
        <label>Justification</label>
        <textarea disabled rows="2" style="background: var(--bg);">Cần máy nướng bánh mới để mở rộng menu cho ca sáng, máy cũ đã quá tải và hay hỏng.</textarea>
      </div>
    </div>
  </div>

    <h3>My Purchase Requisitions</h3>
    <div class="card p-0" style="margin-top: 12px">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>ID</th><th>From</th><th>Justification</th><th>Status</th>
            </tr>
          </thead>
          <tbody>
            ${prs.length === 0 ? '<tr><td colspan="4" class="text-center dim py-4">No requisitions found.</td></tr>' : ''}
            ${prs.map(r => `
              <tr>
                <td><code>${r.id.split('-')[0]}</code></td>
                <td><span class="badge badge-fac">${r.requester_node_id}</span></td>
                <td>${r.justification || '-'}</td>
                <td>${statusBadge(r.status)}</td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    </div>
  `;
}

function toggleNewEquipmentType() {
  const sel = document.getElementById('pr-eq-type');
  const f = document.getElementById('new-eq-fields');
  const unitLabel = document.getElementById('pr-exp-cap-unit-label');

  if (sel.value === 'NEW') {
    f.classList.remove('hidden');
    if (unitLabel) unitLabel.textContent = '';
  } else {
    f.classList.add('hidden');
    const opt = sel.options[sel.selectedIndex];
    const unit = opt ? opt.getAttribute('data-unit') : '';
    if (unitLabel) {
      unitLabel.textContent = unit ? `(${unit})` : '';
    }
  }
}

async function submitFacPR() {
  const eqType = document.getElementById('pr-eq-type').value;
  const propName = document.getElementById('pr-prop-name').value;
  const propCapUnit = document.getElementById('pr-prop-cap-unit').value;
  const expCap = parseFloat(document.getElementById('pr-exp-cap').value);
  const qty = parseInt(document.getElementById('pr-qty').value);
  const uom = document.getElementById('pr-uom').value;
  const price = parseFloat(document.getElementById('pr-price').value);
  const note = document.getElementById('pr-note').value;
  const desc = document.getElementById('pr-desc').value;

  if (!eqType || !qty || !price || !note) {
    toast('Please fill all required fields.', 'error');
    return;
  }

  let finalEqTypeId = eqType;
  if (eqType === 'NEW') {
    if (!propName || !propCapUnit) {
      toast('Please provide a name and capacity unit for the new equipment type.', 'error');
      return;
    }
    finalEqTypeId = 'eq_' + propName.toLowerCase().replace(/\s+/g, '_');
  }

  const payload = {
    org_id: state.currentUser.orgId,
    requester_node_id: state.node,
    staff_id: state.currentUser.staffId,
    justification: note,
    lines: [
      {
        equipment_type_id: finalEqTypeId,
        proposed_equipment_name: eqType === 'NEW' ? propName : undefined,
        proposed_capacity_unit: eqType === 'NEW' ? propCapUnit : undefined,
        expected_capacity: expCap || undefined,
        qty: qty,
        unit_of_measure: uom,
        estimated_unit_price: price,
        justification: note,
        description: desc
      }
    ]
  };

  try {
    await api.submitPR(payload);
    toast('Purchase Requisition submitted to HQ.', 'success');
    renderPage(state.page);
  } catch (e) {
    toast(`Failed to submit PR: ${e.message}`, 'error');
  }
}
