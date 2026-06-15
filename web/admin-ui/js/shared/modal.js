/* ─── Modal System ───────────────────────────────────────────────────────────
   Generic modal open/close plus all modal form submit handlers.
   Depends on: mock-data.js, toast.js, core/router.js (renderPage, state).
──────────────────────────────────────────────────────────────────────────── */

/* ── Modal Templates ─────────────────────────────────────── */
const MODAL_TEMPLATES = {

  newB2B: () => `
    <div class="modal-header">
      <h3>New B2B Sales Order</h3>
      <button class="modal-close" onclick="closeModal()">✕</button>
    </div>
    <div class="flex col gap-16">
      <div class="field">
        <label>Customer Name</label>
        <input id="m-cust" type="text" placeholder="e.g. TrueFood Hotel Chain" />
      </div>
      <div class="field">
        <label>Item</label>
        <select id="m-item">
          ${ITEMS.map(i => `<option>${i.name}</option>`).join('')}
        </select>
      </div>
      <div class="grid-2 gap-16">
        <div class="field"><label>Quantity</label><input id="m-qty" type="number" value="100" /></div>
        <div class="field"><label>Unit Price (₫)</label><input id="m-price" type="number" value="150000" /></div>
      </div>
      <button class="btn btn-primary" onclick="submitB2B()">Create B2B Order</button>
    </div>
  `,

  newPO: () => `
    <div class="modal-header">
      <h3>New Production Order</h3>
      <button class="modal-close" onclick="closeModal()">✕</button>
    </div>
    <div class="flex col gap-16">
      <div class="field">
        <label>Item to Produce</label>
        <select id="m-po-item">
          ${ITEMS.filter(i => i.type !== 'RAW_MATERIAL').map(i => `<option>${i.name}</option>`).join('')}
        </select>
      </div>
      <div class="grid-2 gap-16">
        <div class="field"><label>Target Qty</label><input id="m-po-qty" type="number" value="100" /></div>
        <div class="field"><label>Scheduled Start</label><input id="m-po-start" type="time" value="08:00" /></div>
      </div>
      <button class="btn btn-primary" onclick="submitNewPO()">Create Production Order</button>
    </div>
  `,

  newITO: () => `
    <div class="modal-header">
      <h3>Manual ITO Request</h3>
      <button class="modal-close" onclick="closeModal()">✕</button>
    </div>
    <div class="flex col gap-16">
      <div class="field">
        <label>Item Needed</label>
        <select id="m-ito-item">
          ${ITEMS.filter(i => i.type !== 'RAW_MATERIAL').map(i => `<option>${i.name}</option>`).join('')}
        </select>
      </div>
      <div class="field">
        <label>Quantity Requested</label>
        <input id="m-ito-qty" type="number" value="100" />
      </div>
      <div class="field">
        <label>Reason</label>
        <textarea id="m-ito-note" placeholder="Reason for manual request..."></textarea>
      </div>
      <button class="btn btn-primary" onclick="submitManualITO()">Submit ITO Request</button>
    </div>
  `,

  newOrder: () => `
    <div class="modal-header">
      <h3>New POS Order</h3>
      <button class="modal-close" onclick="closeModal()">✕</button>
    </div>
    <div class="flex col gap-16">
      <div class="field">
        <label>Items (free text)</label>
        <input id="m-ord-items" type="text" placeholder="e.g. Fried Chicken Leg ×2" />
      </div>
      <div class="field">
        <label>Source</label>
        <select id="m-ord-src">
          <option>POS</option>
          <option>GrabFood</option>
          <option>ShopeeFood</option>
        </select>
      </div>
      <button class="btn btn-primary" onclick="submitNewOrder()">Create Order</button>
    </div>
  `,

  recordGR: (poId) => {
    const po = PURCHASE_ORDERS.find(p => p.id === poId);
    if (!po) return '';
    return `
      <div class="modal-header">
        <h3>Record Goods Receipt</h3>
        <button class="modal-close" onclick="closeModal()">✕</button>
      </div>
      <div class="flex col gap-16">
        <p class="dim">Receiving items for <code>${po.id}</code> at ${po.delivery_to}</p>
        <div class="grid-2 gap-16">
          <div class="field"><label>Item</label><input type="text" value="${po.item}" disabled /></div>
          <div class="field"><label>Expected Qty</label><input type="number" value="${po.qty}" disabled /></div>
        </div>
        <div class="field">
          <label>Actual Quantity Received</label>
          <input id="m-gr-qty" type="number" value="${po.qty}" max="${po.qty}" />
          <p class="small faint" style="margin-top:4px">If received quantity is less than expected, a Discrepancy Ticket will be automatically created.</p>
        </div>
        <button class="btn btn-primary" onclick="submitGR('${po.id}')">Confirm Receipt</button>
      </div>
    `;
  },

  recordInvoice: () => `
    <div class="modal-header">
      <h3>Record Supplier Invoice</h3>
      <button class="modal-close" onclick="closeModal()">✕</button>
    </div>
    <div class="flex col gap-16">
      <div class="field">
        <label>Select Purchase Order</label>
        <select id="m-inv-po">
          <option value="">-- Select PO --</option>
          ${PURCHASE_ORDERS.filter(p => p.status === 'ON_WAY_DELIVERY' || p.status === 'CONFIRMED').map(p => `<option value="${p.id}">${p.id} - ${p.item}</option>`).join('')}
        </select>
      </div>
      <div class="field">
        <label>Supplier Invoice Number</label>
        <input id="m-inv-num" type="text" placeholder="e.g. INV-2026-001" />
      </div>
      <div class="field">
        <label>Total Amount</label>
        <input id="m-inv-amount" type="number" placeholder="e.g. 5000000" />
      </div>
      <button class="btn btn-primary" onclick="submitInvoice()">Record Invoice</button>
    </div>
  `,

  registerMachine: (assetId) => {
    const ast = ASSETS.find(a => a.id === assetId);
    if (!ast) return '';
    const eq = EQUIPMENT_TYPES.find(e => e.id === ast.equipment_type_id);
    return `
      <div class="modal-header">
        <h3>Register Asset as Machine</h3>
        <button class="modal-close" onclick="closeModal()">✕</button>
      </div>
      <div class="flex col gap-16">
        <p class="dim">Registering <code>${assetId}</code> (${eq?eq.name:ast.equipment_type_id}) in the Production System.</p>
        <div class="field">
          <label>Machine ID</label>
          <input id="m-mach-id" type="text" placeholder="e.g. M_OVEN_02" />
        </div>
        <div class="field">
          <label>Max Capacity (${eq?eq.capacity_unit:'units'})</label>
          <input id="m-mach-cap" type="number" value="4.0" step="0.1" />
        </div>
        <button class="btn btn-primary" onclick="submitMachineRegistration('${assetId}')">Register & Active Machine</button>
      </div>
    `;
  },

  convertPR: (prId) => {
    const pr = PURCHASE_REQS.find(p => p.id === prId);
    if (!pr) return '';
    return `
      <div class="modal-header">
        <h3>Convert PR to Purchase Order</h3>
        <button class="modal-close" onclick="closeModal()">✕</button>
      </div>
      <div class="flex col gap-16">
        <p class="dim">Convert <code>${pr.id}</code> to a confirmed PO.</p>
        <div class="field">
          <label>Supplier</label>
          <select id="m-conv-supplier" onchange="toggleNewSupplier()">
            <option value="">-- Select Supplier --</option>
            ${SUPPLIERS.map(s => `<option value="${s.id}">${s.name}</option>`).join('')}
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

        <button class="btn btn-primary" onclick="submitPRConversion('${pr.id}')">Create Purchase Order</button>
      </div>
    `;
  },

  newSupplier: () => `
    <div class="modal-header">
      <h3>Add New Supplier</h3>
      <button class="modal-close" onclick="closeModal()">✕</button>
    </div>
    <div class="flex col gap-16">
      <div class="field">
        <label>Supplier Name</label>
        <input id="m-supp-name-only" type="text" placeholder="e.g., Premium Logistics">
      </div>
      <div class="field">
        <label>Contact Info</label>
        <input id="m-supp-contact-only" type="text" placeholder="e.g., sales@supplier.com">
      </div>
      <button class="btn btn-primary" onclick="submitNewSupplier()">Save Supplier</button>
    </div>
  `
};

/* ── Open / Close ────────────────────────────────────────── */
function openModal(type, contextId = null) {
  const templateFn = MODAL_TEMPLATES[type];
  if (!templateFn) return;

  const mc = document.getElementById('modal-container');
  mc.classList.remove('hidden');
  mc.innerHTML = `
    <div class="modal-overlay" onclick="handleOverlayClick(event)">
      <div class="modal">${templateFn(contextId)}</div>
    </div>
  `;
}

function closeModal() {
  document.getElementById('modal-container').classList.add('hidden');
}

function handleOverlayClick(e) {
  if (e.target.classList.contains('modal-overlay')) closeModal();
}

/**
 * showModal - dynamic modal generator used by feature modules
 * @param {string} title - The modal title
 * @param {string} bodyHtml - The HTML content for the modal body
 * @param {Array} buttons - Array of button config objects: { label, primary, action }
 */
function showModal(title, bodyHtml, buttons = []) {
  const mc = document.getElementById('modal-container');
  mc.classList.remove('hidden');
  
  // Render structure
  mc.innerHTML = `
    <div class="modal-overlay">
      <div class="modal">
        <div class="modal-header">
          <h3>${title}</h3>
          <button class="modal-close" id="dynamic-modal-close">✕</button>
        </div>
        <div class="flex col gap-16" id="modal-body">
          ${bodyHtml}
        </div>
        ${buttons && buttons.length > 0 ? `<div class="modal-actions" id="dynamic-modal-actions" style="margin-top: 24px; display: flex; gap: 12px; justify-content: flex-end;"></div>` : ''}
      </div>
    </div>
  `;

  // Attach overlay and close button listeners
  mc.querySelector('.modal-overlay').addEventListener('click', (e) => {
    if (e.target.classList.contains('modal-overlay')) closeModal();
  });
  mc.querySelector('#dynamic-modal-close').addEventListener('click', closeModal);

  // Attach button actions safely
  if (buttons && buttons.length > 0) {
    const actionsContainer = mc.querySelector('#dynamic-modal-actions');
    buttons.forEach(btn => {
      const buttonEl = document.createElement('button');
      buttonEl.className = btn.primary ? 'btn btn-primary' : 'btn btn-outline';
      buttonEl.textContent = btn.label;
      if (btn.action) {
        buttonEl.addEventListener('click', btn.action);
      }
      actionsContainer.appendChild(buttonEl);
    });
  }
}

/* ── Dynamic Form Togglers ───────────────────────────────── */
function toggleNewSupplier() {
  const sel = document.getElementById('m-conv-supplier').value;
  const f = document.getElementById('new-supp-fields');
  if (sel === 'NEW') f.classList.remove('hidden');
  else f.classList.add('hidden');
}

/* ── Modal Submit Handlers ───────────────────────────────── */
function submitB2B() {
  const cust  = document.getElementById('m-cust')?.value?.trim();
  const item  = document.getElementById('m-item')?.value;
  const qty   = parseInt(document.getElementById('m-qty')?.value || 0);
  const price = parseInt(document.getElementById('m-price')?.value || 0);

  if (!cust || !qty) { toast('Fill all fields.', 'error'); return; }

  B2B_ORDERS.push({
    id:       `B2B-00${B2B_ORDERS.length + 1}`,
    customer: cust,
    item,
    qty,
    price:    qty * price,
    status:   'PENDING',
    factory:  null,
  });

  closeModal();
  toast('B2B Sales Order created!', 'success');
  renderPage(state.page);
}

function submitNewPO() {
  const item  = document.getElementById('m-po-item')?.value;
  const qty   = parseInt(document.getElementById('m-po-qty')?.value || 0);
  const start = document.getElementById('m-po-start')?.value;

  if (!item || !qty) { toast('Fill all fields.', 'error'); return; }

  PRODUCTION_ORDERS.push({
    id:     `PO-00${PRODUCTION_ORDERS.length + 1}`,
    item,
    qty,
    status: 'PENDING',
    node:   'FACTORY',
    start,
    end:    '—',
  });

  closeModal();
  toast('Production Order created!', 'success');
  renderPage(state.page);
}

function submitManualITO() {
  const item = document.getElementById('m-ito-item')?.value;
  const qty  = parseInt(document.getElementById('m-ito-qty')?.value || 0);

  if (!item || !qty) { toast('Fill all fields.', 'error'); return; }

  ITORDERS.push({
    id:        `ITO-00${ITORDERS.length + 1}`,
    from:      'FACTORY',
    to:        'STORE',
    item,
    qty,
    status:    'AUTO_APPROVED',
    trigger:   'MANUAL',
    same_site: true,
  });

  closeModal();
  toast(`Manual ITO created for ${qty}× ${item}.`, 'success');
  renderPage(state.page);
}

function submitNewOrder() {
  const items = document.getElementById('m-ord-items')?.value?.trim();
  const src   = document.getElementById('m-ord-src')?.value;

  if (!items) { toast('Enter order items.', 'error'); return; }

  const now = new Date();
  const t   = `${now.getHours()}:${String(now.getMinutes()).padStart(2, '0')}`;

  POS_ORDERS.push({
    id:     `ORD-00${POS_ORDERS.length + 1}`,
    items,
    source: src,
    status: 'PENDING',
    time:   t,
  });

  closeModal();
  toast('Order created!', 'success');
  renderPage(state.page);
}

/* ── CapEx Procurement Actions ───────────────────────────── */

function submitGR(poId) {
  const po = PURCHASE_ORDERS.find(p => p.id === poId);
  if (!po) return;
  
  const expectedQty = parseFloat(po.qty);
  const receivedQty = parseFloat(document.getElementById('m-gr-qty').value);
  
  if (isNaN(receivedQty)) {
    toast('Invalid quantity.', 'error');
    return;
  }
  
  let status = 'CONFIRMED';
  let tstMsg = `Goods Receipt confirmed for PO ${po.id}.`;
  
  const grId = 'GR-' + Math.floor(Math.random() * 10000);
  
  if (receivedQty < expectedQty) {
    status = 'DISCREPANCY';
    // Auto-create discrepancy ticket
    DISC_TICKETS.push({
      id: 'DT-' + Math.floor(Math.random() * 10000),
      gr: grId,
      item: po.item,
      missing: expectedQty - receivedQty,
      damaged: 0,
      status: 'OPEN',
      date: new Date().toISOString().split('T')[0]
    });
    tstMsg = `Discrepancy detected! Ticket auto-created for missing items.`;
    toast(tstMsg, 'warning');
  } else {
    toast(tstMsg, 'success');
  }
  
  GOODS_RECEIPTS.push({
    id: grId,
    ref_type: 'PURO',
    ref_id: po.id,
    receiving_node: po.delivery_to,
    status: status,
    lines: [{ item: po.item, qty_expected: expectedQty, qty_received: receivedQty }]
  });
  
  // Update PO Status if all looks well (or even if discrepancy, technically GR is done)
  // For simplicity, we just mark it COMPLETED or leave it for finance to 3-way match.
  // Actually, PO stays SHIPPED until 3-way match, so we don't change PO status here!
  
  closeModal();
  renderPage(state.page);
}

function submitInvoice() {
  const poId = document.getElementById('m-inv-po').value;
  const num = document.getElementById('m-inv-num').value;
  const amount = parseFloat(document.getElementById('m-inv-amount').value);
  
  if (!poId || !num || isNaN(amount)) {
    toast('Fill all required fields.', 'error');
    return;
  }
  
  const po = PURCHASE_ORDERS.find(p => p.id === poId);
  if (!po) return;
  
  // Find associated GR if any
  const gr = GOODS_RECEIPTS.find(g => g.ref_id === poId);
  
  INVOICES.push({
    id: 'INV-' + Math.floor(Math.random() * 10000),
    puro_id: poId,
    supplier_id: po.supplier_id,
    invoice_number: num,
    total_amount: amount,
    gr_id: gr ? gr.id : null,
    status: 'PENDING',
    matched_by: null
  });
  
  toast(`Invoice ${num} recorded successfully.`, 'success');
  closeModal();
  renderPage(state.page);
}

function submitMachineRegistration(assetId) {
  const mId = document.getElementById('m-mach-id').value;
  const cap = parseFloat(document.getElementById('m-mach-cap').value);
  
  if (!mId || isNaN(cap)) {
    toast('Fill all fields.', 'error');
    return;
  }
  
  const ast = ASSETS.find(a => a.id === assetId);
  if (!ast) return;
  
  ast.status = 'IN_USE';
  ast.linked_machine_id = mId;
  
  MACHINES.push({
    id: mId,
    equipment_type_id: ast.equipment_type_id,
    node: ast.node_id,
    max_cap: cap,
    status: 'IDLE',
    batch: null,
    linked_asset_id: ast.id
  });
  
  toast(`Machine ${mId} registered successfully!`, 'success');
  closeModal();
  renderPage(state.page);
}

function submitPRConversion(prId) {
  const pr = PURCHASE_REQS.find(p => p.id === prId);
  if (!pr) return;
  
  let suppId = document.getElementById('m-conv-supplier').value;
  
  if (suppId === 'NEW') {
    const name = document.getElementById('m-supp-name').value;
    const contact = document.getElementById('m-supp-contact').value;
    if (!name) { toast('Please provide supplier name.', 'error'); return; }
    suppId = 'supp_' + Math.floor(Math.random() * 10000);
    SUPPLIERS.push({ id: suppId, name, contact });
    toast(`Supplier ${name} created.`, 'info');
  } else if (!suppId) {
    toast('Select a supplier.', 'error');
    return;
  }
  
  pr.status = 'CONVERTED_TO_PURO';
  
  // Extract info from PR lines
  const line = pr.lines && pr.lines.length > 0 ? pr.lines[0] : null;
  const eqType = line && line.equipment_type_id ? line.equipment_type_id : 'Unknown';
  let itemDesc = eqType;
  if (line && line.proposed_name) itemDesc = line.proposed_name;
  else if (line && line.equipment_type_id) {
    const eq = EQUIPMENT_TYPES.find(e => e.id === line.equipment_type_id);
    if (eq) itemDesc = eq.name;
  }
  
  const poId = 'PurO-' + Math.floor(Math.random() * 10000);
  
  PURCHASE_ORDERS.push({
    id: poId,
    trigger_type: 'PR_TRIGGERED',
    pr_id: pr.id,
    supplier_id: suppId,
    item: itemDesc,
    qty: line ? line.qty : 1,
    unit: line ? line.uom : 'unit',
    price: line ? line.estimated_unit_price * (line.qty||1) : 0,
    status: 'CONFIRMED', // Test step 2.2: PR converted to PurO has status CONFIRMED
    delivery_to: pr.from
  });
  
  toast(`PR converted to Purchase Order ${poId}.`, 'success');
  closeModal();
  renderPage(state.page);
}

function submitNewSupplier() {
  const name = document.getElementById('m-supp-name-only').value;
  const contact = document.getElementById('m-supp-contact-only').value;
  if (!name) { toast('Please provide supplier name.', 'error'); return; }
  
  const suppId = 'supp_' + Math.floor(Math.random() * 10000);
  SUPPLIERS.push({ id: suppId, name, contact });
  
  toast(`Supplier ${name} created.`, 'success');
  closeModal();
  renderPage(state.page);
}
