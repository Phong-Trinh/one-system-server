/* ─── Store / Dashboard ──────────────────────────────────────────────────────
   Depends on: helpers.js, api.js.
──────────────────────────────────────────────────────────────────────────── */

async function renderStoDashboard() {
  const belowROP = 0; // NODE_STOCK.STORE

  let activeITO = 0; // Incoming from Factory
  let allPuros = [];
  try {
    const puros = await api.getPOsByNode(state.node);
    allPuros = puros || [];
  } catch (e) { }

  const incomingDeliveries = allPuros.filter(p => p.status === 'SHIPPED');

  const activeOrds = 0; // POS_ORDERS

  return `
    ${pageHeader('Store Dashboard', 'Nobi Fried Chicken — POS, inventory, incoming transfers')}

    <div class="grid-3 gap-16">
      ${kpi('⚠️', 'Below ROP', belowROP, 'var(--amber)', 'Items need replenishment')}
      ${kpi('📦', 'Active ITOs', activeITO, 'var(--blue)', 'Incoming from Factory')}
      ${kpi('🧾', 'Active Orders', activeOrds, 'var(--green)', 'POS + platform orders')}
    </div>

    <div class="mt-24 grid-2 gap-16" style="margin-top:24px">
      <div>
        <h3 style="margin-bottom:12px">Inventory Alerts</h3>
        <div class="card p-0">
          <div class="table-wrap">
            <table>
              <thead>
                <tr><th>Item</th><th>On Hand</th><th>ROP</th><th>Unit</th><th>Level</th></tr>
              </thead>
              <tbody>
                <tr><td colspan="5" class="text-center dim py-4">Inventory module not connected.</td></tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
      <div>
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
    </div>
  `;
}

/* ── Actions ─────────────────────────────────────────────── */

let currentReceivingPO = null;

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
          <input type="number" id="gr-actual-${idx}" value="${l.qty_ordered * l.conversion}" style="width:70px" />
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
          <label>Received Items (Base Units)</label>
          <div class="table-wrap">
            <table style="font-size: 13px;">
              <thead>
                <tr>
                  <th>Item</th>
                  <th>Expected Qty</th>
                  <th>Actual Received</th>
                </tr>
              </thead>
              <tbody>
                ${linesHtml}
              </tbody>
            </table>
          </div>
          <p class="small faint" style="margin-top:4px">If received quantity is less than expected, a Discrepancy Ticket will be auto-created.</p>
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
      item_id: l.item_id || "", // CapEx uses empty string
      qty_expected: parseFloat(document.getElementById(`gr-expected-${idx}`).value),
      qty_received: parseFloat(document.getElementById(`gr-actual-${idx}`).value)
    };
  });

  const payload = {
    puro_id: po.id,
    receiving_node_id: state.node, // Factory/Store
    staff_id: state.currentUser.staffId,
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
