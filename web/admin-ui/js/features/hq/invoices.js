/* ─── HQ / Invoices ──────────────────────────────────────────────────────────
   Supplier Invoices and 3-Way Matching.
   Depends on: helpers.js, toast.js, router.js, modal.js, api.js.
──────────────────────────────────────────────────────────────────────────── */

async function renderHQInvoices() {
  return `
    ${pageHeader(
      'Supplier Invoices',
      'Finance overview: 3-Way Match (PO + GR + Invoice) and Prepayment'
    )}
    <div class="card p-0">
      <div class="table-wrap">
        <div class="p-4 dim text-center">
          To record an invoice, go to <strong>Purchase Orders</strong> and click
          <strong>🧾 Record Invoice</strong> on any <span class="badge badge-purple">DELIVERED</span> PO.
        </div>
      </div>
    </div>
  `;
}

/* ── Record Invoice Modal ─────────────────────────────────────────────────── */

let _invoiceTargetPO = null;  // { po, lines, suppliers }

async function openRecordInvoiceModal(poId) {
  try {
    const [poRes, suppliersRes] = await Promise.all([
      api.getPO(poId),
      api.getSuppliers(state.currentUser.orgId),
    ]);

    _invoiceTargetPO = { po: poRes.po, lines: poRes.lines || [], suppliers: suppliersRes || [] };
    const po = _invoiceTargetPO.po;
    const supplier = _invoiceTargetPO.suppliers.find(s => s.id === po.supplier_id);

    const linesHtml = _invoiceTargetPO.lines.map((l, idx) => `
      <tr>
        <td>${l.item_id || l.equipment_type_id || '—'}</td>
        <td>${l.qty_ordered} ${l.pkg_unit}</td>
        <td><input type="number" id="inv-unit-price-${idx}" value="${l.unit_price}" style="width:110px" /></td>
        <td><input type="number" id="inv-qty-${idx}" value="${l.qty_ordered}" style="width:80px" /></td>
        <td class="faint" id="inv-line-total-${idx}">${(l.qty_ordered * l.unit_price).toLocaleString()}</td>
      </tr>
    `).join('');

    const mc = document.getElementById('modal-container');
    mc.classList.remove('hidden');
    mc.innerHTML = `
      <div class="modal-overlay" onclick="handleOverlayClick(event)">
        <div class="modal" style="max-width:680px">
          <div class="modal-header">
            <h3>🧾 Record Supplier Invoice</h3>
            <button class="modal-close" onclick="closeModal()">✕</button>
          </div>
          <div class="flex col gap-16">
            <p class="dim">PO <code>${poId.split('-')[0]}</code> · Supplier: <strong>${supplier ? supplier.name : po.supplier_id}</strong></p>

            <div class="flex gap-16">
              <div class="field" style="flex:1">
                <label>Invoice Number *</label>
                <input type="text" id="inv-number" placeholder="e.g., INV-2026-001" />
              </div>
              <div class="field" style="flex:1">
                <label>Tax Amount (₫)</label>
                <input type="number" id="inv-tax" value="0" />
              </div>
            </div>

            <div class="field">
              <label>Invoice Image URL (optional)</label>
              <input type="text" id="inv-image-url" placeholder="https://..." />
            </div>

            ${linesHtml.length > 0 ? `
            <div class="field">
              <label>Line Items</label>
              <div class="table-wrap">
                <table style="font-size:13px">
                  <thead>
                    <tr>
                      <th>Item</th><th>PO Qty</th><th>Invoice Unit Price (₫)</th><th>Invoice Qty</th><th>Line Total</th>
                    </tr>
                  </thead>
                  <tbody>${linesHtml}</tbody>
                </table>
              </div>
            </div>
            ` : ''}

            <div class="flex gap-16 mt-8">
              <button class="btn btn-primary" onclick="submitRecordInvoice()">Submit Invoice</button>
              <button class="btn btn-outline" onclick="closeModal()">Cancel</button>
            </div>
          </div>
        </div>
      </div>
    `;
  } catch (e) {
    toast('Failed to load PO details: ' + e.message, 'error');
  }
}

async function submitRecordInvoice() {
  const po = _invoiceTargetPO?.po;
  if (!po) { toast('No PO loaded', 'error'); return; }

  const invoiceNumber = document.getElementById('inv-number').value.trim();
  if (!invoiceNumber) { toast('Invoice number is required', 'error'); return; }

  const taxAmount = parseFloat(document.getElementById('inv-tax').value) || 0;
  const imageURL = document.getElementById('inv-image-url').value.trim();

  const lines = _invoiceTargetPO.lines.map((l, idx) => {
    const unitPrice = parseFloat(document.getElementById(`inv-unit-price-${idx}`)?.value) || l.unit_price;
    const qty       = parseFloat(document.getElementById(`inv-qty-${idx}`)?.value) || l.qty_ordered;
    return {
      item_id: l.item_id || undefined,
      raw_line_text: l.item_id || l.equipment_type_id || `Line ${idx + 1}`,
      qty: qty,
      unit_price: unitPrice,
    };
  });

  const totalAmount = lines.reduce((sum, l) => sum + l.qty * l.unit_price, 0) + taxAmount;

  const payload = {
    org_id: state.currentUser.orgId,
    purchase_order_id: po.id,
    supplier_id: po.supplier_id,
    invoice_number: invoiceNumber,
    total_amount: totalAmount,
    tax_amount: taxAmount,
    image_url: imageURL,
    lines: lines,
  };

  try {
    const res = await api.recordInvoice(payload);
    toast('Invoice recorded! Now perform 3-Way Match.', 'success');
    closeModal();
    // Open 3-way match modal immediately with the new invoice
    await openThreeWayMatchModal(res.id, po.id);
  } catch (e) {
    toast('Failed to record invoice: ' + e.message, 'error');
  }
}

/* ── 3-Way Match Modal ────────────────────────────────────────────────────── */

async function openThreeWayMatchModal(invoiceId, poId) {
  try {
    // We need the GR linked to this PO — fetch all GRs via the PO's delivery node
    // The GR ref_id equals the poId, so we load the PO to get the gr detail
    const poRes = await api.getPO(poId);
    const po = poRes.po;

    const mc = document.getElementById('modal-container');
    mc.classList.remove('hidden');
    mc.innerHTML = `
      <div class="modal-overlay" onclick="handleOverlayClick(event)">
        <div class="modal" style="max-width:520px">
          <div class="modal-header">
            <h3>✅ 3-Way Match</h3>
            <button class="modal-close" onclick="closeModal()">✕</button>
          </div>
          <div class="flex col gap-16">
            <p class="dim">Match PO <code>${poId.split('-')[0]}</code> · Invoice <code>${invoiceId.split('-')[0]}</code></p>

            <div class="field">
              <label>Goods Receipt ID *</label>
              <input type="text" id="match-gr-id" placeholder="Paste the GR ID from the Factory receipt" style="font-family:monospace" />
              <small class="dim">The GR was recorded by the Factory when goods arrived.</small>
            </div>

            <div class="flex gap-16 mt-8">
              <button class="btn btn-primary" onclick="submitThreeWayMatch('${invoiceId}')">Perform 3-Way Match</button>
              <button class="btn btn-outline" onclick="closeModal()">Later</button>
            </div>
          </div>
        </div>
      </div>
    `;
  } catch (e) {
    toast('Error: ' + e.message, 'error');
  }
}

async function submitThreeWayMatch(invoiceId) {
  const grId = document.getElementById('match-gr-id').value.trim();
  if (!grId) { toast('GR ID is required', 'error'); return; }

  try {
    await api.match3Way(invoiceId, grId, state.currentUser.staffId);
    toast('3-Way Match successful! PO is now COMPLETED.', 'success');
    closeModal();
    renderPage(state.page);
  } catch (e) {
    toast('3-Way Match failed: ' + e.message, 'error');
  }
}

/* ── Stubs kept for backward compat ─────────────────────────────────────── */

function performThreeWayMatch(invId) { openThreeWayMatchModal(invId, null); }
function performPrepaymentMatch(invId) { toast('Prepayment match not implemented in UI', 'warning'); }
function markInvoicePaid(invId) { toast('Mark Paid not implemented in UI', 'warning'); }
