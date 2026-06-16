/* ─── HQ / Purchase Requisitions ───────────────────────────────────────────────
   Review and approve/reject PRs from Factory and Store.
   Depends on: helpers.js, api.js, toast.js, router.js.

   Flow:
     1. renderHQPRs()         — list PRs with "Review & Approve" / "Reject" buttons
     2. openReviewPRModal()   — two-column modal: Factory preference (read-only) │ HQ corrections
     3. submitPRReview()      — calls api.approvePR with corrections → backend writes back
     4. openRejectPRModal()   — inline reason field → api.rejectPR
──────────────────────────────────────────────────────────────────────────── */

async function renderHQPRs() {
  let prs = [];
  try {
    const res = await api.getPendingPRs(state.currentUser.orgId);
    prs = res || [];
  } catch (err) {
    return `<div class="error">Failed to load PRs: ${err.message}</div>`;
  }

  let eqTypes = [];
  try {
    eqTypes = await api.getEquipmentTypes() || [];
  } catch (err) { }

  return `
    ${pageHeader(
    'Purchase Requisitions',
    'Review and verify requests from Factory and Store nodes'
  )}
    
    <div class="card p-0">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>PR ID</th><th>From</th><th>Justification</th><th>Items Requested</th><th>Est. Total</th><th>Status</th><th>Actions</th>
            </tr>
          <tbody>
            ${prs.length === 0 ? '<tr><td colspan="7" class="text-center dim py-4">No PRs found.</td></tr>' : ''}
            ${(await Promise.all(prs.map(async (pr) => {
    let prDetails;
    try {
      prDetails = await api.getPR(pr.id);
    } catch (e) {
      return `<tr><td colspan="7">Error loading details for PR ${pr.id}</td></tr>`;
    }
    const lines = prDetails.lines || [];

    let total = 0;
    let lineHtml = '';
    lines.forEach(l => {
      total += (l.qty * l.estimated_unit_price);
      let name = l.proposed_equipment_name || l.equipment_type_id || l.item_id;
      const eq = eqTypes.find(e => e.id === l.equipment_type_id);
      if (eq) name = eq.name;
      lineHtml += `<div class="small">${name} &times; ${l.qty} ${l.unit_of_measure}</div>`;
    });

    const isPending = pr.status === 'PENDING_HQ_APPROVAL';

    return `
                <tr>
                  <td><code>${pr.id.split('-')[0]}</code></td>
                  <td><span class="badge badge-sto">${pr.requester_node_id}</span></td>
                  <td><div class="truncate" style="max-width:200px" title="${pr.justification}">${pr.justification}</div></td>
                  <td>${lineHtml}</td>
                  <td class="num">${fmt(total)}</td>
                  <td>${statusBadge(pr.status)}</td>
                  <td>
                    ${isPending ? `
                      <button class="btn btn-primary btn-sm" onclick="openReviewPRModal('${pr.id}')">Review &amp; Approve</button>
                      <button class="btn btn-ghost btn-sm" style="color:var(--red)" onclick="openRejectPRModal('${pr.id}')">Reject</button>
                    ` : ''}
                  </td>
                </tr>
              `;
  }))).join('')}
          </tbody>
        </table>
      </div>
    </div>
  `;
}

/* ── Review & Approve Modal ─────────────────────────────────────────────────── */

let _reviewingPR = null;
let _reviewEqTypes = [];

async function openReviewPRModal(prId) {
  try {
    const [prDetails, eqTypesRes] = await Promise.all([
      api.getPR(prId),
      api.getEquipmentTypes()
    ]);
    if (!prDetails || !prDetails.pr) { toast('Failed to load PR', 'error'); return; }

    _reviewingPR = prDetails;
    _reviewEqTypes = eqTypesRes || [];

    const linesHtml = (prDetails.lines || []).map((l, idx) => {
      const factoryEq = _reviewEqTypes.find(e => e.id === l.equipment_type_id);
      const factoryName = l.proposed_equipment_name || (factoryEq ? factoryEq.name : l.equipment_type_id) || l.item_id || 'Unknown';
      const isProposed = !!l.proposed_equipment_name;

      const eqOptions = _reviewEqTypes
        .filter(e => e.status === 'ACTIVE' || e.status === 'DRAFT')
        .map(e => `<option value="${e.id}" ${e.id === l.equipment_type_id ? 'selected' : ''}>${e.name}${e.status === 'DRAFT' ? ' (DRAFT)' : ''}</option>`)
        .join('');

      const capUnit = l.proposed_capacity_unit || (factoryEq ? factoryEq.capacity_unit : 'unit');
      const capacityHtml = l.expected_capacity
        ? `<div class="rl-field-row">
             <div class="rl-label">Capacity</div>
             <div class="rl-value">${l.expected_capacity} ${capUnit}</div>
           </div>`
        : '';

      return `
        <div class="review-line-card" id="rl-${idx}">
          <!-- Factory preference column (read-only) -->
          <div class="rl-factory">
            <div class="rl-section-label">Factory Request</div>
            <div class="rl-field-row">
              <div class="rl-label">Equipment</div>
              <div class="rl-value">
                ${factoryName}
                ${isProposed ? '<span class="badge badge-amber" style="font-size:9px;margin-left:4px">PROPOSED NEW</span>' : ''}
              </div>
            </div>
            ${capacityHtml}
            <div class="rl-field-row">
              <div class="rl-label">Qty</div>
              <div class="rl-value">${l.qty} ${l.unit_of_measure}</div>
            </div>
            <div class="rl-field-row">
              <div class="rl-label">Est. Price</div>
              <div class="rl-value">${fmt(l.estimated_unit_price)}</div>
            </div>

            ${l.justification ? `
            <div class="rl-field-row">
              <div class="rl-label">Reason</div>
              <div class="rl-value dim small">${l.justification}</div>
            </div>` : ''}
            ${l.description ? `
            <div class="rl-field-row">
              <div class="rl-label">Details</div>
              <div class="rl-value dim small" style="white-space: pre-wrap;">${l.description}</div>
            </div>` : ''}
          </div>

          <!-- Arrow separator -->
          <div class="rl-arrow">&#8594;</div>

          <!-- HQ correction column (editable) -->
          <div class="rl-hq">
            <div class="rl-section-label hq">HQ Verified</div>
            <div class="field" style="margin-bottom:10px">
              <label>Equipment Type</label>
              <div style="display:flex; align-items:flex-start; gap:8px;">
                <div style="flex:1">
                  <select id="rev-eq-${idx}" style="font-size:13px; margin-bottom: 6px; width: 100%" onchange="updateCapacityUnit(${idx}, this.value)">
                    <option value="">-- Select equipment type --</option>
                    ${eqOptions}
                  </select>
                  ${(isProposed && !l.equipment_type_id) ? `
                    <button type="button" class="btn btn-ghost" style="font-size:11px; padding: 2px 6px;" onclick="createDraftEquipmentFromPR(${idx}, '${l.proposed_equipment_name}', '${l.proposed_capacity_unit || 'unit'}')">
                      [+] Create Draft Type
                    </button>
                  ` : ''}
                </div>
                <button type="button" id="rev-eq-edit-${idx}" class="btn btn-ghost ${factoryEq && factoryEq.status === 'DRAFT' ? '' : 'hidden'}" style="padding: 4px 8px; font-size: 11px; height: 32px;" onclick="editDraftEquipment(${idx})">✎ Edit</button>
              </div>

              <div id="inline-eq-form-${idx}" class="hidden" style="margin-top: 8px; padding: 8px; background: var(--bg-alt); border-radius: 4px; border: 1px dashed var(--border);">
                <div style="font-size: 11px; font-weight: 600; color: var(--text-dim); margin-bottom: 6px;">Draft Equipment Type Settings</div>
                <div class="field" style="margin-bottom: 6px;">
                  <label style="font-size: 10px;">Name</label>
                  <input type="text" id="inline-eq-name-${idx}" style="font-size: 12px; padding: 4px 8px; min-height: 28px;">
                </div>
                <div class="field" style="margin-bottom: 6px;">
                  <label style="font-size: 10px;">Capacity Unit</label>
                  <input type="text" id="inline-eq-unit-${idx}" style="font-size: 12px; padding: 4px 8px; min-height: 28px;">
                </div>
                <div style="display:flex; gap:8px; margin-top: 8px;">
                  <button type="button" class="btn btn-primary btn-sm" style="font-size: 11px; padding: 2px 8px; min-height: 24px;" onclick="saveInlineEq(${idx})">Save to Database</button>
                  <button type="button" class="btn btn-ghost btn-sm" style="font-size: 11px; padding: 2px 8px; min-height: 24px;" onclick="hideInlineEq(${idx})">Cancel</button>
                </div>
              </div>
            </div>
            <div class="field" style="margin-top:8px; margin-bottom:10px">
              <label>Verified Capacity</label>
              <div style="display:flex; align-items:center; gap:8px;">
                <input type="number" id="rev-cap-${idx}" value="${l.expected_capacity || ''}" min="0" step="0.1" style="flex:1" placeholder="e.g. 5.0" />
                <span id="rev-cap-unit-${idx}" class="dim small" style="width: 50px;">${capUnit}</span>
              </div>
            </div>
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:8px">
              <div class="field">
                <label>Qty</label>
                <input type="number" id="rev-qty-${idx}" value="${l.qty}" min="0.01" step="0.01" />
              </div>
              <div class="field">
                <label>Unit</label>
                <input type="text" id="rev-uom-${idx}" value="${l.unit_of_measure}" />
              </div>
            </div>
            <div class="field" style="margin-top:8px">
              <label>HQ Price Estimate (&#8363;)</label>
              <input type="number" id="rev-price-${idx}" value="${l.estimated_unit_price}" min="0" />
            </div>
          </div>
        </div>
      `;
    }).join('');

    const modalHtml = `
      <div class="modal-header">
        <div>
          <h3>Review PR &mdash; <code>${prId.split('-')[0]}</code></h3>
          <p class="dim small mt-4">Verify and correct Factory's request before approving. HQ corrections are saved to the PR.</p>
        </div>
        <button class="modal-close" onclick="closeModal()">&#x2715;</button>
      </div>
      <div class="flex col gap-16">

        <div style="padding:12px 16px;background:hsl(38,40%,14%);border:1px solid hsl(38,55%,25%);border-radius:var(--radius);font-size:12px;color:var(--amber)">
          &#9888; Factory data is <strong>not trusted</strong>. For each line, select the verified equipment type from the catalog and confirm qty &amp; price before approving.
        </div>

        <div style="font-size:12px;font-weight:700;color:var(--text-dim);text-transform:uppercase;letter-spacing:.5px">
          Line Items Review
        </div>

        <div class="flex col gap-12">
          ${linesHtml}
        </div>

        <div class="field">
          <label>Approval Note (optional)</label>
          <textarea id="rev-note" rows="2" placeholder="e.g., Verified with Factory manager on 15/6. Price adjusted to market rate."></textarea>
        </div>

        <div class="flex gap-12 mt-4">
          <button class="btn btn-primary" onclick="submitPRReview()">Approve &amp; Save Corrections</button>
          <button class="btn btn-ghost" onclick="closeModal()">Cancel</button>
        </div>
      </div>
    `;

    const mc = document.getElementById('modal-container');
    mc.classList.remove('hidden');
    mc.innerHTML = `
      <div class="modal-overlay" onclick="handleOverlayClick(event)">
        <div class="modal" style="max-width:860px">${modalHtml}</div>
      </div>
    `;

  } catch (e) {
    toast('Error loading PR: ' + e.message, 'error');
  }
}

function updateCapacityUnit(idx, eqId) {
  const eq = _reviewEqTypes.find(e => e.id === eqId);
  const unitSpan = document.getElementById(`rev-cap-unit-${idx}`);
  if (unitSpan) {
    unitSpan.textContent = eq ? eq.capacity_unit : 'unit';
  }
  const editBtn = document.getElementById(`rev-eq-edit-${idx}`);
  if (editBtn) {
    if (eq && eq.status === 'DRAFT') {
      editBtn.classList.remove('hidden');
    } else {
      editBtn.classList.add('hidden');
    }
  }
}

async function editDraftEquipment(idx) {
  const eqId = document.getElementById(`rev-eq-${idx}`).value;
  const eq = _reviewEqTypes.find(e => e.id === eqId);
  if (!eq || eq.status !== 'DRAFT') return;

  document.getElementById(`inline-eq-name-${idx}`).value = eq.name;
  document.getElementById(`inline-eq-unit-${idx}`).value = eq.capacity_unit;
  document.getElementById(`inline-eq-form-${idx}`).classList.remove('hidden');
}

function createDraftEquipmentFromPR(idx, proposedName, capacityUnit) {
  const slug = "eq_" + proposedName.toLowerCase().replace(/\s+/g, '_');
  document.getElementById(`inline-eq-name-${idx}`).value = proposedName;
  document.getElementById(`inline-eq-unit-${idx}`).value = capacityUnit;
  document.getElementById(`inline-eq-form-${idx}`).dataset.newSlug = slug;
  document.getElementById(`inline-eq-form-${idx}`).classList.remove('hidden');
}

function hideInlineEq(idx) {
  const form = document.getElementById(`inline-eq-form-${idx}`);
  if (form) {
    form.classList.add('hidden');
    delete form.dataset.newSlug;
  }
}

async function saveInlineEq(idx) {
  const form = document.getElementById(`inline-eq-form-${idx}`);
  const newName = document.getElementById(`inline-eq-name-${idx}`).value.trim();
  const newUnit = document.getElementById(`inline-eq-unit-${idx}`).value.trim();

  if (!newName || !newUnit) {
    toast('Name and Capacity Unit are required.', 'error');
    return;
  }

  const select = document.getElementById(`rev-eq-${idx}`);
  const existingEqId = select.value;
  const newSlug = form.dataset.newSlug;

  try {
    if (newSlug) {
      const newEq = await api.createEquipmentType({
        id: newSlug,
        name: newName,
        capacity_unit: newUnit,
        status: 'DRAFT'
      });
      _reviewEqTypes.push(newEq);
      const option = document.createElement('option');
      option.value = newEq.id;
      option.text = `${newEq.name} (DRAFT)`;
      select.add(option);
      select.value = newEq.id;
      updateCapacityUnit(idx, newEq.id);
      toast('Draft Equipment Type created successfully!', 'success');
    } else {
      const updated = await api.updateEquipmentType(existingEqId, { name: newName, capacity_unit: newUnit });
      const eq = _reviewEqTypes.find(e => e.id === existingEqId);
      if (eq) {
        eq.name = updated.name;
        eq.capacity_unit = updated.capacity_unit;
      }
      const option = Array.from(select.options).find(o => o.value === existingEqId);
      if (option) option.text = `${updated.name} (DRAFT)`;
      updateCapacityUnit(idx, existingEqId);
      toast('Draft Equipment Type updated successfully!', 'success');
    }
    hideInlineEq(idx);
  } catch (err) {
    toast('Failed to save Draft Equipment Type: ' + err.message, 'error');
  }
}

async function submitPRReview() {
  const pr = _reviewingPR.pr;
  const lines = _reviewingPR.lines || [];

  const corrections = [];
  for (let idx = 0; idx < lines.length; idx++) {
    const l = lines[idx];
    const eqTypeId = document.getElementById(`rev-eq-${idx}`)?.value || '';
    const capStr = document.getElementById(`rev-cap-${idx}`)?.value;
    const expectedCap = capStr ? parseFloat(capStr) : undefined;
    const qty = parseFloat(document.getElementById(`rev-qty-${idx}`)?.value);
    const uom = document.getElementById(`rev-uom-${idx}`)?.value || '';
    const price = parseFloat(document.getElementById(`rev-price-${idx}`)?.value);

    if (!eqTypeId && !l.item_id) {
      toast(`Line ${idx + 1}: Please select a verified equipment type.`, 'error');
      return;
    }
    if (!qty || qty <= 0) {
      toast(`Line ${idx + 1}: Qty must be greater than 0.`, 'error');
      return;
    }

    corrections.push({
      line_id: l.id,
      equipment_type_id: eqTypeId,
      expected_capacity: expectedCap,
      qty: qty,
      unit_of_measure: uom,
      estimated_unit_price: price || 0
    });
  }

  const noteEl = document.getElementById('rev-note');
  const note = noteEl ? noteEl.value.trim() : '';

  try {
    await api.approvePR(pr.id, state.currentUser.staffId, note || null, corrections);
    toast(`PR ${pr.id.split('-')[0]} approved. Corrections saved.`, 'success');
    closeModal();
    renderPage(state.page);
  } catch (err) {
    toast(err.message, 'error');
  }
}



// ── Reject PR ─────────────────────────────────────────────────────────────────

function openRejectPRModal(prId) {
  const modalHtml = `
    <div class="modal-header">
      <h3>Reject PR — <code>${prId.split('-')[0]}</code></h3>
      <button class="modal-close" onclick="closeModal()">✕</button>
    </div>
    <div class="flex col gap-16">
      <div class="field">
        <label>Reason for Rejection <span class="required">*</span></label>
        <textarea id="rej-reason" rows="3" placeholder="Explain why this request is denied..."></textarea>
      </div>
      <div class="flex gap-12 mt-4">
        <button class="btn btn-danger" onclick="submitPRReject('${prId}')">Reject PR</button>
        <button class="btn btn-ghost" onclick="closeModal()">Cancel</button>
      </div>
    </div>
  `;
  openModal(modalHtml, { maxWidth: '500px' });
}

async function submitPRReject(prId) {
  const reason = document.getElementById('rej-reason')?.value?.trim();
  if (!reason) {
    toast('Rejection reason is required.', 'error');
    return;
  }
  try {
    await api.rejectPR(prId, state.currentUser.staffId, reason);
    toast(`PR ${prId.split('-')[0]} rejected.`, 'error');
    closeModal();
    renderPage(state.page);
  } catch (err) {
    toast(`Failed to reject PR: ${err.message}`, 'error');
  }
}

// Keep legacy function names in case they're referenced elsewhere
async function approvePR(id) { openReviewPRModal(id); }
async function rejectPR(id) { openRejectPRModal(id); }
