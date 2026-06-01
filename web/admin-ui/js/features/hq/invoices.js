/* ─── HQ / Invoices ──────────────────────────────────────────────────────────
   Supplier Invoices and 3-Way Matching.
   Depends on: helpers.js, mock-data.js, toast.js, router.js, modal.js.
──────────────────────────────────────────────────────────────────────────── */

async function renderHQInvoices() {
  return `
    ${pageHeader(
      'Supplier Invoices',
      'Finance overview: 3-Way Match (PO + GR + Invoice) and Prepayment',
      `<button class="btn btn-primary btn-sm" onclick="alert('Record Invoice not fully implemented')">🧾 Record Invoice</button>`
    )}

    <div class="card p-0">
      <div class="table-wrap">
        <div class="p-4 dim text-center">
          Fetching all invoices is not fully implemented in the admin UI.
        </div>
      </div>
    </div>
  `;
}

/* ── Actions ─────────────────────────────────────────────── */

function performThreeWayMatch(invId) {
  alert('Not implemented in UI');
}

function performPrepaymentMatch(invId) {
  alert('Not implemented in UI');
}

function markInvoicePaid(invId) {
  alert('Not implemented in UI');
}
