/* ─── HQ / Goods Receipts ──────────────────────────────────────────────────────
   View and Record Goods Receipts for on way delivery POs.
   Depends on: helpers.js, mock-data.js, toast.js.
──────────────────────────────────────────────────────────────────────────── */

async function renderHQGoodsReceipts() {
  let puros = [];
  let suppliers = [];

  try {
    const [purosRes, suppliersRes] = await Promise.all([
      api.getPOs(state.currentUser.orgId),
      api.getSuppliers(state.currentUser.orgId)
    ]);
    puros = purosRes || [];
    suppliers = suppliersRes || [];
  } catch (err) {
    return `<div class="error">Failed to load data: ${err.message}</div>`;
  }

  const onWayDeliveryPOs = puros.filter(p => p.status === 'ON_WAY_DELIVERY');

  return `
    ${pageHeader(
      'Goods Receipts',
      'Log deliveries from external suppliers (HQ overlay for Store/Factory GR actions)'
    )}

    <!-- Highlight ON_WAY_DELIVERY POs that need receiving -->
    <h3 style="margin-bottom:12px; color:var(--amber)">Pending Receipts (On Way Delivery)</h3>
    <div class="card p-0" style="margin-bottom:32px">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>PO ID</th><th>Supplier</th><th>Item</th><th>Delivery To</th><th>Actions</th>
            </tr>
          </thead>
          <tbody>
            ${onWayDeliveryPOs.map(p => {
              const supplier = suppliers.find(s => s.id === p.supplier_id);
              return `
                <tr>
                  <td><code>${p.id.split('-')[0]}</code></td>
                  <td>${supplier ? supplier.name : p.supplier_id}</td>
                  <td>-</td>
                  <td><span class="badge ${p.delivery_to_node_id === 'FACTORY' ? 'badge-fac' : 'badge-sto'}">${p.delivery_to_node_id}</span></td>
                  <td>
                    <button class="btn btn-primary btn-sm" onclick="alert('Record GR not fully implemented')">📦 Record GR</button>
                  </td>
                </tr>
              `;
            }).join('')}
            ${onWayDeliveryPOs.length === 0 ? `<tr><td colspan="5" class="faint" style="text-align:center; padding: 24px;">No pending deliveries.</td></tr>` : ''}
          </tbody>
        </table>
      </div>
    </div>

    <h3 style="margin-bottom:12px">Recent Goods Receipts</h3>
    <div class="card p-0">
      <div class="table-wrap">
        <div class="p-4 dim">
          Fetching recent GRs is not yet implemented in the admin UI.
        </div>
      </div>
    </div>
  `;
}
