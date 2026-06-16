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
        ? `<button class="btn btn-primary btn-sm" onclick="openConfirmDraftModal('${po.id}')">Confirm</button>`
        : ''}
                    ${po.status === 'CONFIRMED'
        ? `<button class="btn btn-outline btn-sm" onclick="markOnWayDelivery('${po.id}')">On Way Delivery</button>`
        : ''}
                    ${po.status === 'DELIVERED'
        ? `<button class="btn btn-primary btn-sm" onclick="openRecordInvoiceModal('${po.id}')">🧾 Record Invoice</button>`
        : ''}
                    ${(po.status === 'CONFIRMED' || po.status === 'ON_WAY_DELIVERY') && po.trigger_type === 'PR_TRIGGERED'
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

// ─── Quote Matrix State ───────────────────────────────────────────────────────
// quoteMatrix[supplierId][lineIdx] = price (number)
let quoteMatrix = {};
let quoteSuppliers = [];
let quoteLines = [];
let selectedQuoteSupplier = null;

async function openConvertPRModal(prId) {
  try {
    const prDetails = await api.getPR(prId);
    if (!prDetails || !prDetails.pr) {
      toast('Failed to load PR details', 'error');
      return;
    }
    currentConvertingPR = prDetails;
    quoteLines = prDetails.lines || [];

    const [suppliersRes, eqTypesRes] = await Promise.all([
      api.getSuppliers(state.currentUser.orgId),
      api.getEquipmentTypes()
    ]);
    quoteSuppliers = suppliersRes || [];
    const eqTypes = eqTypesRes || [];

    // Initialize quoteMatrix with historical prices from system
    quoteMatrix = {};
    
    // Prepare the list of lines to query
    const queryLines = quoteLines.map(l => ({
      item_id: l.item_id,
      equipment_type_id: l.equipment_type_id
    }));

    // Fetch historical quotes in parallel for all suppliers
    await Promise.all(quoteSuppliers.map(async (s) => {
      try {
        const res = await api.getSupplierHistoricalPrices(s.id, queryLines);
        quoteMatrix[s.id] = res.prices || quoteLines.map(() => 0);
      } catch (err) {
        console.warn(`Failed to fetch history for supplier ${s.id}`, err);
        quoteMatrix[s.id] = quoteLines.map(() => 0);
      }
    }));
    selectedQuoteSupplier = null;

    const modalHtml = `
      <div class="modal-header">
        <h3>Convert PR to Purchase Order</h3>
        <button class="modal-close" onclick="closeModal()">✕</button>
      </div>
      <div class="flex col gap-16">
        <p class="dim">PR <code>${prId.split('-')[0]}</code> — select the best supplier offer below, then confirm.</p>

        <!-- ── Supplier Price Comparison Panel ── -->
        <div>
          <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:8px">
            <div style="font-size:12px;font-weight:700;color:var(--text-dim);text-transform:uppercase;letter-spacing:.5px">
              🏷️ Supplier Quote Comparison
            </div>
            <div style="font-size:11px;color:var(--text-faint)">Enter quoted prices · Click a row to select</div>
          </div>
          <div class="quote-panel" id="quote-panel-root">
            ${renderQuotePanel()}
          </div>
        </div>

        <!-- ── Divider ── -->
        <div style="display:flex;align-items:center;gap:12px">
          <hr style="flex:1"/>
          <span style="font-size:11px;color:var(--text-faint)">SELECTED ORDER</span>
          <hr style="flex:1"/>
        </div>

        <!-- ── Supplier selector (synced with comparison panel) ── -->
        <div class="field">
          <label>Selected Supplier</label>
          <select id="m-conv-supplier" onchange="onConvSupplierChange()">
            <option value="">-- Choose from comparison above or pick here --</option>
            ${quoteSuppliers.map(s => `<option value="${s.id}">${s.name}</option>`).join('')}
            <option value="NEW">➕ Create New Supplier...</option>
          </select>
        </div>

        <div id="new-supp-fields" class="hidden" style="padding: 16px; border: 1px solid var(--border); border-radius: 6px;">
          <div class="field">
            <label>New Supplier Name</label>
            <input type="text" id="m-supp-name" placeholder="e.g., Premium Logistics">
          </div>
          <div class="field">
            <label>Contact Info</label>
            <input type="text" id="m-supp-contact" placeholder="e.g., sales@supplier.com">
          </div>
        </div>

        <!-- ── Line items (prices auto-filled from comparison) ── -->
        <div class="field">
          <label>Line Items &amp; Final Prices</label>
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
                ${quoteLines.map((l, idx) => `
                  <tr>
                    <td>
                      ${l.proposed_equipment_name || l.equipment_type_id || l.item_id || 'Unknown'}
                      ${l.expected_capacity !== undefined ? `<br><span class="dim small" style="display:flex; align-items:center; gap:4px; margin-top:4px;">Capacity: <input type="number" id="po-cap-${idx}" value="${l.expected_capacity}" style="width:60px; font-size:11px; padding:2px;" step="0.1" min="0" /> ${(eqTypes.find(e => e.id === l.equipment_type_id)?.capacity_unit) || l.proposed_capacity_unit || 'unit'}</span>` : ''}
                    </td>
                    <td><input type="number" id="po-qty-${idx}" value="${l.qty}" style="width:70px" /></td>
                    <td><input type="text"   id="po-unit-${idx}" value="${l.unit_of_measure}" style="width:70px" /></td>
                    <td><input type="number" id="po-conv-${idx}" value="1" style="width:60px" /></td>
                    <td><input type="number" id="po-price-${idx}" value="${l.estimated_unit_price}" style="width:110px" /></td>
                  </tr>
                `).join('')}
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
        <div class="modal" style="max-width: 800px">${modalHtml}</div>
      </div>
    `;

  } catch (e) {
    toast('Error loading PR: ' + e.message, 'error');
  }
}

/* ── Quote Panel Renderer ──────────────────────────────────────────────────── */

function renderQuotePanel() {
  const numLines = quoteLines.length;
  const colCount = numLines;

  // Column headers
  const itemHeaders = quoteLines.map((l, i) => {
    const name = l.proposed_equipment_name || l.equipment_type_id || l.item_id || `Item ${i+1}`;
    return `<div class="qch-cell" title="${name}">${name.length > 14 ? name.slice(0, 13) + '…' : name}</div>`;
  }).join('');

  const headerHtml = `
    <div class="quote-col-header" style="--col-count:${colCount}">
      <div class="qch-cell">Supplier</div>
      ${itemHeaders}
      <div class="qch-cell" style="text-align:center">Total</div>
    </div>
  `;

  // Compute totals for ranking
  const totals = quoteSuppliers.map(s => {
    const prices = quoteMatrix[s.id] || [];
    const total = quoteLines.reduce((sum, l, idx) => sum + (l.qty || 1) * (prices[idx] || 0), 0);
    return { id: s.id, name: s.name, prices, total };
  });

  const hasAnyQuote = totals.some(t => t.prices.some(p => p > 0));
  if (!hasAnyQuote) {
    return `
      <div style="padding: 32px 16px; text-align: center; color: var(--text-faint); border: 1px dashed var(--border); border-radius: var(--radius); background: var(--bg-alt);">
        <div style="font-size: 24px; margin-bottom: 8px;">📭</div>
        <div style="font-weight: 500; color: var(--text-dim); margin-bottom: 4px;">No Historical Data</div>
        <div style="font-size: 13px;">There are no previous quotes from any suppliers for these items.<br>Please select your preferred supplier directly below.</div>
      </div>
    `;
  }

  // Sort by total ascending to get ranks
  const sorted = [...totals].sort((a, b) => a.total - b.total);
  const rankMap = {};
  sorted.forEach((s, i) => { rankMap[s.id] = i + 1; });

  const bestTotal = sorted[0]?.total || 0;

  // Per-line best price per column
  const bestPricePerLine = quoteLines.map((_, idx) => {
    const prices = quoteSuppliers.map(s => (quoteMatrix[s.id] || [])[idx] || 0).filter(p => p > 0);
    return prices.length ? Math.min(...prices) : 0;
  });

  const rowsHtml = totals.map(s => {
    const rank = rankMap[s.id];
    const isSelected = selectedQuoteSupplier === s.id;
    const savings = rank === 1 ? '' :
      (s.total > bestTotal ? `<span class="quote-savings">+${fmt(s.total - bestTotal)}</span>` : '');

    const priceCells = quoteLines.map((l, idx) => {
      const price = (quoteMatrix[s.id] || [])[idx] || 0;
      const isBest = bestPricePerLine[idx] > 0 && price === bestPricePerLine[idx];
      return `
        <div class="qsr-price-cell" onclick="event.stopPropagation()">
          <input
            type="number"
            class="${isBest ? 'best-price' : ''}"
            value="${price}"
            min="0"
            oninput="onQuotePriceInput('${s.id}', ${idx}, this.value)"
            onclick="event.stopPropagation()"
          />
        </div>
      `;
    }).join('');

    const totalDisplay = s.total > 0
      ? `<span style="font-size:12px;font-weight:700;color:${rank === 1 ? 'var(--green)' : 'var(--text)'}">${fmt(s.total)}</span>${savings}`
      : `<span class="faint" style="font-size:11px">—</span>`;

    return `
      <div class="quote-supplier-row ${isSelected ? 'selected' : ''}"
           style="--col-count:${colCount}"
           onclick="selectQuoteSupplier('${s.id}')"
           id="qrow-${s.id}">
        <div class="qsr-name">
          <span class="badge-rank rank-${rank <= 3 ? rank : ''}">${rank}</span>
          ${s.name}
          ${rank === 1 && s.total > 0 ? '<span class="badge-best">✓ BEST</span>' : ''}
        </div>
        ${priceCells}
        <div class="qsr-action">
          ${totalDisplay}
        </div>
      </div>
    `;
  }).join('');

  return headerHtml + rowsHtml;
}

function refreshQuotePanel() {
  const root = document.getElementById('quote-panel-root');
  if (root) root.innerHTML = renderQuotePanel();
}

function onQuotePriceInput(supplierId, lineIdx, rawValue) {
  if (!quoteMatrix[supplierId]) quoteMatrix[supplierId] = [];
  quoteMatrix[supplierId][lineIdx] = parseFloat(rawValue) || 0;
  // Refresh ranking without losing focus — defer to next tick
  setTimeout(() => {
    refreshQuotePanel();
    // If this supplier is currently selected, also update the po-price fields
    if (selectedQuoteSupplier === supplierId) {
      syncPriceFieldsFromQuote(supplierId);
    }
  }, 0);
}

function selectQuoteSupplier(supplierId) {
  selectedQuoteSupplier = supplierId;

  // Update the dropdown
  const sel = document.getElementById('m-conv-supplier');
  if (sel) sel.value = supplierId;

  // Hide new supplier fields
  const f = document.getElementById('new-supp-fields');
  if (f) f.classList.add('hidden');

  // Auto-fill price fields from the quote matrix
  syncPriceFieldsFromQuote(supplierId);

  // Re-render panel to show selection highlight
  refreshQuotePanel();

  toast(`Selected: ${quoteSuppliers.find(s => s.id === supplierId)?.name || supplierId}`, 'info');
}

function syncPriceFieldsFromQuote(supplierId) {
  const prices = quoteMatrix[supplierId] || [];
  quoteLines.forEach((_, idx) => {
    const el = document.getElementById(`po-price-${idx}`);
    if (el && prices[idx] !== undefined) {
      el.value = prices[idx];
      // Flash highlight
      el.style.borderColor = 'var(--green)';
      setTimeout(() => { el.style.borderColor = ''; }, 800);
    }
  });
}

function onConvSupplierChange() {
  const sel = document.getElementById('m-conv-supplier').value;
  const f = document.getElementById('new-supp-fields');
  if (sel === 'NEW') {
    f.classList.remove('hidden');
    selectedQuoteSupplier = null;
  } else if (sel) {
    f.classList.add('hidden');
    selectedQuoteSupplier = sel;
    refreshQuotePanel();
  } else {
    f.classList.add('hidden');
    selectedQuoteSupplier = null;
    refreshQuotePanel();
  }
}

// Legacy alias for old inline onchange
function toggleNewSupplier() { onConvSupplierChange(); }

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
    const capInput = document.getElementById(`po-cap-${idx}`);
    return {
      equipment_type_id: l.equipment_type_id || undefined,
      item_id: l.item_id || undefined,
      expected_capacity: capInput ? parseFloat(capInput.value) : l.expected_capacity,
      qty_ordered: parseFloat(document.getElementById(`po-qty-${idx}`).value),
      pkg_unit: document.getElementById(`po-unit-${idx}`).value,
      conversion: parseFloat(document.getElementById(`po-conv-${idx}`).value),
      unit_price: parseFloat(document.getElementById(`po-price-${idx}`).value)
    };
  });

  const payload = {
    pr_id: pr.id,
    supplier_id: supplierId,
    hq_node_id: state.node,
    confirmed_by_staff_id: state.currentUser.staffId,
    lines: payloadLines
  };

  try {
    await api.createPO(payload);
    toast('Purchase Order created successfully!', 'success');
    closeModal();
    renderPage(state.page);
  } catch (err) {
    toast('Failed to create PO: ' + err.message, 'error');
  }
}

async function confirmPO(id) {
  try {
    await api.confirmPO(id);
    toast(`PO ${id.split('-')[0]} confirmed.`, 'success');
    renderPage(state.page);
  } catch (e) {
    toast(`Error: ${e.message}`, 'error');
  }
}

async function markOnWayDelivery(id) {
  try {
    await api.markPOOnWayDelivery(id);
    toast(`PO ${id.split('-')[0]} marked as on way delivery.`, 'success');
    renderPage(state.page);
  } catch (e) {
    toast(`Error: ${e.message}`, 'error');
  }
}

async function cancelAndPivotPO(id) {
  toast('Cancel PO not implemented in API.', 'warning');
}

let currentDraftPO = null;

async function openConfirmDraftModal(poId) {
  try {
    const poDetails = await api.getPO(poId);
    if (!poDetails || !poDetails.po) {
      toast('Failed to load PO details', 'error');
      return;
    }
    currentDraftPO = poDetails;

    const suppliersRes = await api.getSuppliers(state.currentUser.orgId);
    const suppliers = suppliersRes || [];

    const linesHtml = (poDetails.lines || []).map((l, idx) => `
      <tr>
        <td>${l.item_id || l.equipment_type_id || 'Unknown'}</td>
        <td><input type="number" id="draft-po-qty-${idx}" value="${l.qty_ordered}" style="width:70px" /></td>
        <td><input type="text" id="draft-po-unit-${idx}" value="${l.pkg_unit || 'box'}" style="width:70px" /></td>
        <td><input type="number" id="draft-po-conv-${idx}" value="${l.conversion || 1}" style="width:60px" /></td>
        <td><input type="number" id="draft-po-price-${idx}" value="${l.unit_price || 0}" style="width:100px" /></td>
      </tr>
    `).join('');

    const modalHtml = `
      <div class="modal-header">
        <h3>Confirm Draft Purchase Order</h3>
        <button class="modal-close" onclick="closeModal()">✕</button>
      </div>
      <div class="flex col gap-16">
        <p class="dim">Review draft PO <code>${poId.split('-')[0]}</code>, select a supplier, and enter quoted prices.</p>
        
        <div class="field">
          <label>Select Supplier</label>
          <select id="m-draft-supplier" onchange="toggleNewDraftSupplier()">
            <option value="">-- Select Supplier --</option>
            ${suppliers.map(s => `<option value="${s.id}" ${s.id === poDetails.po.supplier_id ? 'selected' : ''}>${s.name}</option>`).join('')}
            <option value="NEW">➕ Create New Supplier...</option>
          </select>
        </div>

        <div id="new-draft-supp-fields" class="hidden" style="margin-top: 16px; padding: 16px; border: 1px solid var(--border); border-radius: 6px;">
          <div class="field">
            <label>New Supplier Name</label>
            <input type="text" id="m-draft-supp-name" placeholder="e.g., Premium Logistics">
          </div>
          <div class="field">
            <label>Contact Info</label>
            <input type="text" id="m-draft-supp-contact" placeholder="e.g., sales@supplier.com">
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
          <button class="btn btn-primary" onclick="submitDraftPOConfirmation()">Confirm Order</button>
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
    toast('Error loading PO: ' + e.message, 'error');
  }
}

function toggleNewDraftSupplier() {
  const sel = document.getElementById('m-draft-supplier').value;
  const f = document.getElementById('new-draft-supp-fields');
  if (sel === 'NEW') f.classList.remove('hidden');
  else f.classList.add('hidden');
}

async function submitDraftPOConfirmation() {
  let supplierId = document.getElementById('m-draft-supplier').value;
  if (!supplierId) {
    toast('Please select a supplier', 'error');
    return;
  }

  if (supplierId === 'NEW') {
    const name = document.getElementById('m-draft-supp-name').value;
    const contact = document.getElementById('m-draft-supp-contact').value;
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

  const po = currentDraftPO.po;
  const lines = currentDraftPO.lines || [];

  const payloadLines = lines.map((l, idx) => {
    return {
      equipment_type_id: l.equipment_type_id || undefined,
      item_id: l.item_id || undefined,
      qty_ordered: parseFloat(document.getElementById(`draft-po-qty-${idx}`).value),
      pkg_unit: document.getElementById(`draft-po-unit-${idx}`).value,
      conversion: parseFloat(document.getElementById(`draft-po-conv-${idx}`).value),
      unit_price: parseFloat(document.getElementById(`draft-po-price-${idx}`).value)
    };
  });

  const payload = {
    supplier_id: supplierId,
    lines: payloadLines
  };

  try {
    await api.confirmDraftPO(po.id, payload);
    toast('Draft Purchase Order confirmed successfully', 'success');
    closeModal();
    window.location.hash = '#'; 
    setTimeout(() => { window.location.hash = '#hq/purchase-orders'; }, 50);
  } catch (e) {
    toast('Failed to confirm draft PO: ' + e.message, 'error');
  }
}
