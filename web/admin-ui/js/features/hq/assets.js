/* ─── HQ / Assets ────────────────────────────────────────────────────────────
   Asset Registry for CapEx equipment.
   Depends on: helpers.js, toast.js, router.js, modal.js, api.js.
──────────────────────────────────────────────────────────────────────────── */

async function renderHQAssets() {
  let assets = [];
  let eqTypes = [];

  try {
    const [facAssets, stoAssets, eq] = await Promise.all([
      api.getAssets('FACTORY'),
      api.getAssets('STORE'),
      api.getEquipmentTypes()
    ]);
    assets = [...(facAssets || []), ...(stoAssets || [])];
    eqTypes = eq || [];
  } catch (err) {
    return `<div class="error">Failed to load assets: ${err.message}</div>`;
  }

  return `
    ${pageHeader(
      'Asset Registry',
      'CapEx financial records and lifecycle'
    )}

    <div class="card p-0">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>Asset ID</th><th>Eq Type</th><th>Node</th><th>Cost</th><th>PR Ref</th><th>PO Ref</th><th>Machine Ref</th><th>Status</th><th>Actions</th>
            </tr>
          </thead>
          <tbody>
            ${assets.length === 0 ? '<tr><td colspan="9" class="text-center dim py-4">No assets found.</td></tr>' : ''}
            ${assets.map(ast => {
              const eqType = eqTypes.find(e => e.id === ast.equipment_type_id);
              const eqName = eqType ? eqType.name : ast.equipment_type_id;
              return `
                <tr>
                  <td><code>${ast.id.split('-')[0]}</code></td>
                  <td>${eqName}</td>
                  <td><span class="badge ${ast.node_id === 'FACTORY' ? 'badge-fac' : 'badge-sto'}">${ast.node_id}</span></td>
                  <td class="num">${ast.acquisition_cost ? ast.acquisition_cost.toLocaleString() + ' ₫' : '<span class="faint">—</span>'}</td>
                  <td>${ast.linked_pr_id ? `<code>${ast.linked_pr_id.split('-')[0]}</code>` : '<span class="faint">—</span>'}</td>
                  <td>${ast.linked_puro_id ? `<code>${ast.linked_puro_id.split('-')[0]}</code>` : '<span class="faint">—</span>'}</td>
                  <td>${ast.linked_machine_id ? `<code>${ast.linked_machine_id}</code>` : '<span class="faint">—</span>'}</td>
                  <td>${statusBadge(ast.status)}</td>
                  <td>
                    ${ast.status === 'IN_USE'
                      ? `<button class="btn btn-outline btn-sm" onclick="syncAssetStatus('${ast.id}', 'UNDER_MAINTENANCE')">Mark Maintenance</button>`
                      : ''}
                    ${ast.status === 'UNDER_MAINTENANCE'
                      ? `<button class="btn btn-outline btn-sm" onclick="syncAssetStatus('${ast.id}', 'IN_USE')">Return to Service</button>`
                      : ''}
                    ${ast.status === 'DECOMMISSIONED'
                      ? `<span class="faint">Decommissioned</span>`
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

/* ── Sync Status ──────────────────────────────────────────────────────────── */

async function syncAssetStatus(assetId, newStatus) {
  try {
    await api.syncAssetStatus(assetId, newStatus);
    toast(`Asset ${assetId.split('-')[0]} status synced to ${newStatus}.`, 'success');
    renderPage(state.page);
  } catch(e) {
    toast(`Error: ${e.message}`, 'error');
  }
}
