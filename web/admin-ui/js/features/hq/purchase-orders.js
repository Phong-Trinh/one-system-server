/* ─── HQ / Purchase Orders ───────────────────────────────────────────────────
   Manage external procurement. Convert PRs to POs, manage shipments.
   Depends on: helpers.js, mock-data.js, toast.js, router.js, modal.js.
──────────────────────────────────────────────────────────────────────────── */

async function renderHQPurchaseOrders() {
  let prs = [];
  let puros = [];
  let suppliers = [];

  try {
    const [prsRes, purosRes, suppliersRes] = await Promise.all([
      api.getPendingPRs(state.currentUser.orgId),
      api.getPOs(state.currentUser.orgId),
      api.getSuppliers(state.currentUser.orgId)
    ]);
    prs = prsRes || [];
    puros = purosRes || [];
    suppliers = suppliersRes || [];
  } catch (err) {
    return `<div class="error">Failed to load data: ${err.message}</div>`;
  }

  // Get Approved PRs that haven't been converted yet
  const approvedPRs = prs.filter(pr => pr.status === 'APPROVED');

  return `
    ${pageHeader(
    'Purchase Orders',
    'Manage external procurement and convert PRs'
  )}
    
    ${approvedPRs.length > 0 ? `
    <h3 style="margin-bottom:12px">Pending Conversions (Approved PRs)</h3>
    <div class="card p-0" style="margin-bottom:32px">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>PR ID</th><th>From</th><th>Justification</th><th>Actions</th>
            </tr>
          </thead>
          <tbody>
            ${approvedPRs.map(pr => `
              <tr>
                <td><code>${pr.id.split('-')[0]}</code></td>
                <td><span class="badge badge-sto">${pr.requester_node_id}</span></td>
                <td>${pr.justification}</td>
                <td>
                  <button class="btn btn-primary btn-sm" onclick="openConvertPRModal('${pr.id}')">Convert to PO</button>
                </td>
              </tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    </div>
    ` : ''}

    <h3 style="margin-bottom:12px">Purchase Orders</h3>
    <div class="card p-0">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>PO ID</th><th>Trigger</th><th>PR Ref</th><th>Supplier</th><th>Item</th><th>Status</th><th>Actions</th>
            </tr>
          </thead>
          <tbody>
            ${puros.length === 0 ? '<tr><td colspan="7" class="text-center dim py-4">No Purchase Orders found.</td></tr>' : ''}
            ${puros.map(po => {
    const supplier = suppliers.find(s => s.id === po.supplier_id);
    return `
                <tr>
                  <td><code>${po.id.split('-')[0]}</code></td>
                  <td><span class="badge badge-dim">${po.trigger_type}</span></td>
                  <td>${po.pr_id ? `<code>${po.pr_id.split('-')[0]}</code>` : '<span class="faint">—</span>'}</td>
                  <td>${supplier ? supplier.name : po.supplier_id}</td>
                  <td>-</td>
                  <td>${statusBadge(po.status)}</td>
                  <td>
                    ${po.status === 'DRAFT'
        ? `<button class="btn btn-primary btn-sm" onclick="confirmPO('${po.id}')">Confirm</button>`
        : ''}
                    ${po.status === 'CONFIRMED'
        ? `<button class="btn btn-outline btn-sm" onclick="markShipped('${po.id}')">Mark Shipped</button>`
        : ''}
                    ${(po.status === 'CONFIRMED' || po.status === 'SHIPPED') && po.trigger_type === 'PR_TRIGGERED'
        ? `<button class="btn btn-ghost btn-sm" onclick="cancelAndPivotPO('${po.id}')" style="color:var(--red)">Cancel</button>`
        : ''}
                  </td>
                </tr>
              `;
  }).join('')}
          </tbody>
        </table>
      </div>
    </div>
  `;
}

/* ── Actions ─────────────────────────────────────────────── */

let currentConvertingPR = null;

async function openConvertPRModal(prId) {
  try {
    const prDetails = await api.getPR(prId);
    if (!prDetails || !prDetails.pr) {
      toast('Failed to load PR details', 'error');
      return;
    }
    currentConvertingPR = prDetails;

    // Fetch suppliers if not already fetched in page state
    const suppliersRes = await api.getSuppliers(state.currentUser.orgId);
    const suppliers = suppliersRes || [];

    const linesHtml = (prDetails.lines || []).map((l, idx) => `
      <tr>
        <td>${l.proposed_equipment_name || l.equipment_type_id || l.item_id || 'Unknown'}</td>
        <td><input type="number" id="po-qty-${idx}" value="${l.qty}" style="width:70px" /></td>
        <td><input type="text" id="po-unit-${idx}" value="${l.unit_of_measure}" style="width:70px" /></td>
        <td><input type="number" id="po-conv-${idx}" value="1" style="width:60px" /></td>
        <td><input type="number" id="po-price-${idx}" value="${l.estimated_unit_price}" style="width:100px" /></td>
      </tr>
    `).join('');

    const modalHtml = `
      <div class="modal-header">
        <h3>Convert PR to Purchase Order</h3>
        <button class="modal-close" onclick="closeModal()">✕</button>
      </div>
      <div class="flex col gap-16">
        <p class="dim">Convert PR <code>${prId.split('-')[0]}</code> to a Purchase Order.</p>
        
        <div class="field">
          <label>Select Supplier</label>
          <select id="m-conv-supplier" onchange="toggleNewSupplier()">
            <option value="">-- Select Supplier --</option>
            ${suppliers.map(s => `<option value="${s.id}">${s.name}</option>`).join('')}
            <option value="NEW">➕ Create New Supplier...</option>
          </select>
        </div>

        <div id="new-supp-fields" class="hidden" style="margin-top: 16px; padding: 16px; border: 1px solid var(--border); border-radius: 6px;">
          <div class="field">
            <label>New Supplier Name</label>
            <input type="text" id="m-supp-name" placeholder="e.g., Premium Logistics">
          </div>
          <div class="field">
            <label>Contact Info</label>
            <input type="text" id="m-supp-contact" placeholder="e.g., sales@supplier.com">
          </div>
        </div>

        <div class="field">
          <label>Line Items</label>
          <div class="table-wrap">
            <table style="font-size: 13px;">
              <thead>
                <tr>
                  <th>Item</th>
                  <th>Order Qty</th>
                  <th>Pkg Unit</th>
                  <th>Base Conv</th>
                  <th>Unit Price (₫)</th>
                </tr>
              </thead>
              <tbody>
                ${linesHtml}
              </tbody>
            </table>
          </div>
        </div>

        <div class="flex gap-16 mt-8">
          <button class="btn btn-primary" onclick="submitPOConversion()">Create Purchase Order</button>
          <button class="btn btn-outline" onclick="closeModal()">Cancel</button>
        </div>
      </div>
    `;

    const mc = document.getElementById('modal-container');
    mc.classList.remove('hidden');
    mc.innerHTML = `
      <div class="modal-overlay" onclick="handleOverlayClick(event)">
        <div class="modal" style="max-width: 700px">${modalHtml}</div>
      </div>
    `;

  } catch (e) {
    toast('Error loading PR: ' + e.message, 'error');
  }
}

function toggleNewSupplier() {
  const sel = document.getElementById('m-conv-supplier').value;
  const f = document.getElementById('new-supp-fields');
  if (sel === 'NEW') f.classList.remove('hidden');
  else f.classList.add('hidden');
}

async function submitPOConversion() {
  let supplierId = document.getElementById('m-conv-supplier').value;
  if (!supplierId) {
    toast('Please select a supplier', 'error');
    return;
  }

  if (supplierId === 'NEW') {
    const name = document.getElementById('m-supp-name').value;
    const contact = document.getElementById('m-supp-contact').value;
    if (!name) {
      toast('Please provide a supplier name', 'error');
      return;
    }
    try {
      const newSupp = await api.createSupplier({
        org_id: state.currentUser.orgId,
        name: name,
        contact_info: contact
      });
      supplierId = newSupp.id;
      toast(`Supplier ${name} created.`, 'info');
    } catch (err) {
      toast('Failed to create supplier: ' + err.message, 'error');
      return;
    }
  }

  const pr = currentConvertingPR.pr;
  const lines = currentConvertingPR.lines || [];

  const payloadLines = lines.map((l, idx) => {
    return {
      equipment_type_id: l.equipment_type_id || undefined,
      item_id: l.item_id || undefined,
      qty_ordered: parseFloat(document.getElementById(`po-qty-${idx}`).value),
      pkg_unit: document.getElementById(`po-unit-${idx}`).value,
      conversion: parseFloat(document.getElementById(`po-conv-${idx}`).value),
      unit_price: parseFloat(document.getElementById(`po-price-${idx}`).value)
    };
  });

  const payload = {
    pr_id: pr.id,
    supplier_id: supplierId,
    hq_node_id: state.node, // HQ is creating this
    confirmed_by_staff_id: state.currentUser.staffId,
    lines: payloadLines
  };

  try {
    await api.createPO(payload);
    toast('Purchase Order created successfully!', 'success');
    closeModal();
    renderPage(state.page); // Refresh the list
  } catch (err) {
    toast('Failed to create PO: ' + err.message, 'error');
  }
}

async function confirmPO(id) {
  toast('Confirm PO not fully implemented in API yet.', 'warning');
}

async function markShipped(id) {
  try {
    await api.markPOShipped(id);
    toast(`PO ${id.split('-')[0]} marked as shipped.`, 'success');
    renderPage(state.page);
  } catch (e) {
    toast(`Error: ${e.message}`, 'error');
  }
}

async function cancelAndPivotPO(id) {
  toast('Cancel PO not implemented in API.', 'warning');
}
