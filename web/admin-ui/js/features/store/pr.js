/* ─── Store / Submit PR ────────────────────────────────────────────────────
   CapEx purchase requisition form for Store.
   Depends on: api.js, toast.js, router.js, shared/helpers.js.
──────────────────────────────────────────────────────────────────────────── */

async function renderStoPR() {
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
    
    <div class="card" style="max-width: 600px; margin-bottom: 24px;">
      <div class="field">
        <label>Equipment Type</label>
        <select id="pr-eq-type" onchange="toggleNewEquipmentTypeSto()">
          <option value="">-- Select Equipment Type --</option>
          ${eqTypes.filter(e => e.status === 'ACTIVE').map(eq => `<option value="${eq.id}">${eq.name}</option>`).join('')}
          <option value="NEW">➕ Request New Equipment Type...</option>
        </select>
      </div>

      <div id="new-eq-fields-sto" class="hidden" style="margin-top: 16px; padding: 16px; border: 1px solid var(--border); border-radius: 6px;">
        <div class="field">
          <label>Proposed Name</label>
          <input type="text" id="pr-prop-name" placeholder="e.g., Industrial Pizza Oven">
        </div>
        <div class="field">
          <label>Capacity Unit</label>
          <input type="text" id="pr-prop-cap-unit" placeholder="e.g., trays, liters, slots">
        </div>
      </div>

      <div class="grid cols-2" style="margin-top:16px">
        <div class="field">
          <label>Expected Capacity</label>
          <input type="number" id="pr-exp-cap" placeholder="e.g., 4.0">
        </div>
        <div class="field">
          <label>Quantity</label>
          <input type="number" id="pr-qty" value="1" min="1">
        </div>
      </div>

      <div class="grid cols-2" style="margin-top:16px">
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
        <label>Justification</label>
        <textarea id="pr-note" rows="3" placeholder="Why is this needed?"></textarea>
      </div>
      
      <div style="margin-top:24px">
        <button class="btn btn-primary fw" onclick="submitStoPR()">Submit Requisition</button>
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
                <td><span class="badge badge-sto">${r.requester_node_id}</span></td>
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

function toggleNewEquipmentTypeSto() {
  const sel = document.getElementById('pr-eq-type').value;
  const f = document.getElementById('new-eq-fields-sto');
  if (sel === 'NEW') f.classList.remove('hidden');
  else f.classList.add('hidden');
}

async function submitStoPR() {
  const eqType = document.getElementById('pr-eq-type').value;
  const propName = document.getElementById('pr-prop-name').value;
  const propCapUnit = document.getElementById('pr-prop-cap-unit').value;
  const expCap = parseFloat(document.getElementById('pr-exp-cap').value);
  const qty = parseInt(document.getElementById('pr-qty').value);
  const uom = document.getElementById('pr-uom').value;
  const price = parseFloat(document.getElementById('pr-price').value);
  const note = document.getElementById('pr-note').value;

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
        justification: note
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
