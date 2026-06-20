/* ─── Factory / Dashboard ────────────────────────────────────────────────────
   Depends on: helpers.js, api.js.
──────────────────────────────────────────────────────────────────────────── */

async function renderFacDashboard() {
  const inProgress = 0; // PRODUCTION_ORDERS

  let pending = 0; // Pending POs
  let allPuros = [];
  let pendingITOs = 0; // ITOs needing dispatch
  try {
    const [puros, itos] = await Promise.all([
      api.getPOsByNode(state.node).catch(() => []),
      api.getITOs(state.node).catch(() => [])
    ]);
    allPuros = puros || [];
    pending = allPuros.filter(p => p.status !== 'COMPLETED').length;
    
    // Count ITOs where Factory is the provider and it's approved
    const activeItos = itos || [];
    pendingITOs = activeItos.filter(ito => 
      ito.provider_node_id === state.node && 
      (ito.status === 'APPROVED' || ito.status === 'AUTO_APPROVED')
    ).length;
  } catch (e) { }

  const incomingDeliveries = allPuros.filter(p => p.status === 'ON_WAY_DELIVERY');

  const busyMach = 0; // MACHINES
  const lowStock = 0; // NODE_STOCK.FACTORY
  const MACHINES = [];
  const BATCHES = [];

  return `
    ${pageHeader('Factory Dashboard', 'Factory — production, machines, stock')}

    <div class="grid-5 gap-16">
      ${kpi('🏭', 'In Production', inProgress, 'var(--blue)', 'Active production orders')}
      ${kpi('🚚', 'Pending Dispatch', pendingITOs, 'var(--orange)', 'ITOs needing dispatch')}
      ${kpi('⏳', 'Active POs', pending, 'var(--amber)', 'Awaiting deliveries')}
      ${kpi('⚙️', 'Machines Busy', busyMach, 'var(--amber)', 'of ${MACHINES.length} total')}
      ${kpi('⚠️', 'Low Stock', lowStock, 'var(--red)', 'Items below ROP')}
    </div>

    <div class="mt-24 grid-2 gap-16" style="margin-top:24px">

      <div>
        <h3 style="margin-bottom:12px">Machine Status</h3>
        <div class="flex col gap-8">
          ${MACHINES.length ? MACHINES.map(m => `
            <div class="card"
                 style="padding:14px 18px; display:flex; align-items:center;
                        justify-content:space-between; gap:12px">
              <div class="flex ai-c gap-8">
                <span class="machine-status-dot dot-${m.status.toLowerCase().replace(/_/g, '-')}"></span>
                <span style="font-weight:600">${m.id}</span>
                <span class="badge badge-dim">${m.type}</span>
              </div>
              ${statusBadge(m.status)}
            </div>
          `).join('') : '<div class="card" style="padding:14px 18px;color:var(--text-faint)">No machines found.</div>'}
        </div>
      </div>

      <div>
        <h3 style="margin-bottom:12px">Active Batches</h3>
        <div class="flex col gap-8">
          ${BATCHES.length
      ? BATCHES.map(b => `
              <div class="card" style="padding:14px 18px">
                <div class="flex ai-c jc-sb">
                  <span style="font-weight:600">${b.id}</span>
                  ${statusBadge(b.status)}
                </div>
                <div class="small dim mt-4">Machine: ${b.machine} · ${b.item} · ${b.qty} units</div>
                <div class="small dim">ETA: ${b.eta}</div>
                <div class="progress-bar" style="margin-top:8px">
                  <div class="progress-fill ${b.status === 'IN_PROGRESS' ? 'amber' : 'green'}"
                       style="width:${b.status === 'IN_PROGRESS' ? 65 : 20}%"></div>
                </div>
              </div>
            `).join('')
      : '<div class="card" style="padding:14px 18px;color:var(--text-faint)">No active batches.</div>'}
        </div>
      </div>

    </div>

    <div class="mt-24" style="margin-top:24px">
      <h3 style="margin-bottom:12px">Incoming Deliveries</h3>
      <div class="card p-0">
        <div class="table-wrap">
          <table>
            <thead>
              <tr><th>PO ID</th><th>Trigger</th><th>Supplier</th><th>Status</th><th>Actions</th></tr>
            </thead>
            <tbody>
              ${incomingDeliveries.length === 0 ? '<tr><td colspan="5" class="text-center dim py-4">No incoming deliveries.</td></tr>' : ''}
              ${incomingDeliveries.map(po => `
                <tr>
                  <td><code>${po.id.split('-')[0]}</code></td>
                  <td><span class="badge badge-dim">${po.trigger_type}</span></td>
                  <td>${po.supplier_id}</td>
                  <td>${statusBadge(po.status)}</td>
                  <td>
                    <button class="btn btn-primary btn-sm" onclick="openRecordGRModal('${po.id}')">Record Receipt</button>
                  </td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  `;
}

/* ── Actions ─────────────────────────────────────────────── */

var currentReceivingPO = null;

async function openRecordGRModal(poId) {
  try {
    const poDetails = await api.getPO(poId);
    if (!poDetails || !poDetails.po) {
      toast('Failed to load PO details', 'error');
      return;
    }
    currentReceivingPO = poDetails;

    const linesHtml = (poDetails.lines || []).map((l, idx) => `
      <tr>
        <td>${l.equipment_type_id || l.item_id || 'Unknown'}</td>
        <td><input type="number" id="gr-expected-${idx}" value="${l.qty_ordered * l.conversion}" disabled style="width:70px" /></td>
        <td>
          <input type="number" id="gr-usable-${idx}" value="${l.qty_ordered * l.conversion}" style="width:70px" oninput="toggleDiscrepancyPanel(${idx})" />
        </td>
      </tr>
      <tr id="discrepancy-panel-${idx}" class="hidden" style="background:var(--bg-card);border-left:3px solid var(--amber)">
        <td colspan="3" style="padding:12px;">
          <div style="font-weight:600;font-size:12px;color:var(--amber);margin-bottom:8px">Discrepancy Details</div>
          <div class="grid-2 gap-8">
            <div class="field">
              <label>Damaged Qty</label>
              <input type="number" id="gr-damaged-${idx}" value="0" min="0" oninput="updateMissingQty(${idx})" style="width:100%">
            </div>
            <div class="field">
              <label>Missing Qty</label>
              <input type="number" id="gr-missing-${idx}" value="0" disabled style="width:100%">
            </div>
          </div>
          <div class="field mt-8">
            <label>Reason for Discrepancy</label>
            <input type="text" id="gr-reason-${idx}" placeholder="e.g. Box was crushed">
          </div>
          <div class="field mt-8">
            <label>Photo Evidence URL</label>
            <input type="text" id="gr-evidence-${idx}" placeholder="e.g. https://.../damage.jpg">
          </div>
        </td>
      </tr>
    `).join('');

    const modalHtml = `
      <div class="modal-header">
        <h3>Record Goods Receipt</h3>
        <button class="modal-close" onclick="closeModal()">✕</button>
      </div>
      <div class="flex col gap-16">
        <p class="dim">Receiving delivery for PO <code>${poId.split('-')[0]}</code></p>
        
        <div class="field">
          <label>Received Items (Usable Base Units)</label>
          <div class="table-wrap">
            <table style="font-size: 13px;">
              <thead>
                <tr>
                  <th>Item</th>
                  <th>Expected Qty</th>
                  <th>Usable Received</th>
                </tr>
              </thead>
              <tbody>
                ${linesHtml}
              </tbody>
            </table>
          </div>
          <p class="small faint" style="margin-top:4px">If usable quantity is less than expected, a Discrepancy Ticket will be auto-created.</p>
        </div>

        <div class="field mt-8">
          <label>Delivery Note Photo URL</label>
          <input type="text" id="gr-delivery-note" placeholder="Paste link to physical note photo">
        </div>
        <div class="field mt-8">
          <label>General Notes</label>
          <textarea id="gr-notes" rows="2" placeholder="Any general comments on the delivery..."></textarea>
        </div>

        <div class="flex gap-16 mt-8">
          <button class="btn btn-primary" onclick="submitGR()">Confirm Receipt</button>
          <button class="btn btn-outline" onclick="closeModal()">Cancel</button>
        </div>
      </div>
    `;

    const mc = document.getElementById('modal-container');
    mc.classList.remove('hidden');
    mc.innerHTML = `
      <div class="modal-overlay" onclick="handleOverlayClick(event)">
        <div class="modal" style="max-width: 600px">${modalHtml}</div>
      </div>
    `;

  } catch (e) {
    toast('Error loading PO: ' + e.message, 'error');
  }
}

async function submitGR() {
  if (!currentReceivingPO) return;
  const po = currentReceivingPO.po;
  const lines = currentReceivingPO.lines || [];

  const payloadLines = lines.map((l, idx) => {
    return {
      item_id: l.item_id || l.equipment_type_id || "", // Fix for CapEx items
      qty_expected: parseFloat(document.getElementById(`gr-expected-${idx}`).value),
      qty_received: parseFloat(document.getElementById(`gr-usable-${idx}`).value),
      qty_damaged: parseFloat(document.getElementById(`gr-damaged-${idx}`)?.value || "0"),
      reason: document.getElementById(`gr-reason-${idx}`)?.value || "",
      evidence_url: document.getElementById(`gr-evidence-${idx}`)?.value || ""
    };
  });

  const payload = {
    puro_id: po.id,
    receiving_node_id: state.node, // Factory/Store
    staff_id: state.currentUser.staffId,
    notes: document.getElementById('gr-notes').value || "",
    delivery_note_url: document.getElementById('gr-delivery-note').value || "",
    lines: payloadLines
  };

  try {
    await api.confirmGR(payload);

    // Check if there was a discrepancy to show the right toast
    const hasDiscrepancy = payloadLines.some(l => l.qty_received < l.qty_expected);
    if (hasDiscrepancy) {
      toast('Discrepancy detected! Ticket auto-created for missing items.', 'warning');
    } else {
      toast('Goods Receipt confirmed successfully!', 'success');
    }

    closeModal();
    renderPage(state.page); // Refresh dashboard
  } catch (err) {
    toast('Failed to record GR: ' + err.message, 'error');
  }
}

function toggleDiscrepancyPanel(idx) {
  const expected = parseFloat(document.getElementById(`gr-expected-${idx}`).value);
  const usable = parseFloat(document.getElementById(`gr-usable-${idx}`).value);
  const panel = document.getElementById(`discrepancy-panel-${idx}`);
  
  if (usable < expected) {
    panel.classList.remove('hidden');
    updateMissingQty(idx);
  } else {
    panel.classList.add('hidden');
    // Reset values when hidden
    const damagedEl = document.getElementById(`gr-damaged-${idx}`);
    if (damagedEl) damagedEl.value = 0;
    const missingEl = document.getElementById(`gr-missing-${idx}`);
    if (missingEl) missingEl.value = 0;
    const reasonEl = document.getElementById(`gr-reason-${idx}`);
    if (reasonEl) reasonEl.value = "";
    const evidenceEl = document.getElementById(`gr-evidence-${idx}`);
    if (evidenceEl) evidenceEl.value = "";
  }
}

function updateMissingQty(idx) {
  const expected = parseFloat(document.getElementById(`gr-expected-${idx}`).value) || 0;
  const usable = parseFloat(document.getElementById(`gr-usable-${idx}`).value) || 0;
  const damaged = parseFloat(document.getElementById(`gr-damaged-${idx}`).value) || 0;
  
  let missing = expected - usable - damaged;
  if (missing < 0) missing = 0;
  document.getElementById(`gr-missing-${idx}`).value = missing;
}
