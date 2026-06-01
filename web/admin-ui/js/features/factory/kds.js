/* ─── Factory / KDS ──────────────────────────────────────────────────────────
   Kitchen Display System. View machine status, batches, report breakdown.
   Depends on: helpers.js, mock-data.js, toast.js, router.js.
──────────────────────────────────────────────────────────────────────────── */

function renderFacKDS() {
  return `
    ${pageHeader(
      'Kitchen Display System',
      'Real-time machine status and batch execution'
    )}
    
    <div class="grid cols-3" style="gap:24px">
      ${MACHINES.map(m => {
        const eqType = EQUIPMENT_TYPES.find(e => e.id === m.equipment_type_id);
        const eqName = eqType ? eqType.name : m.equipment_type_id;
        let bHtml = `<div class="faint">No active batch</div>`;
        if (m.batch) {
          const b = BATCHES.find(x => x.id === m.batch);
          if (b) {
            bHtml = `
              <div style="font-weight:600">${b.item}</div>
              <div class="small">Qty: ${b.qty} • ETA: ${b.eta}</div>
              <div style="margin-top:12px">
                <button class="btn btn-primary fw btn-sm" onclick="completeBatch('${b.id}')">Complete Batch</button>
              </div>
            `;
          }
        }
        
        let headerColor = m.status === 'UNDER_MAINTENANCE' ? 'var(--red)' : 
                          m.status === 'BUSY' ? 'var(--primary)' : 'var(--border-heavy)';
        
        return `
          <div class="machine-card" style="border-top-color: ${headerColor}">
            <div class="mc-header">
              <div>
                <div class="mc-id">${m.id}</div>
                <div class="mc-type">${eqName}</div>
              </div>
              <div class="mc-status badge ${
                m.status === 'BUSY' ? 'badge-primary' :
                m.status === 'IDLE' ? 'badge-dim' : 'badge-red'
              }">${m.status}</div>
            </div>
            <div class="mc-body">
              ${m.status === 'UNDER_MAINTENANCE' 
                ? `<div class="faint" style="color:var(--red)">Machine is out of service.</div>` 
                : bHtml}
            </div>
            <div class="mc-footer flex row justify-between" style="border-top: 1px solid var(--border); padding-top: 12px; margin-top: 12px">
               <div class="small dim">Asset: ${m.linked_asset_id ? `<code>${m.linked_asset_id}</code>` : 'None'}</div>
               <div>
                 ${m.status !== 'UNDER_MAINTENANCE'
                   ? `<button class="btn btn-ghost btn-sm" style="color:var(--red); padding:4px 8px" onclick="reportBreakdown('${m.id}')">Report Breakdown</button>`
                   : `<button class="btn btn-outline btn-sm" style="padding:4px 8px" onclick="returnToService('${m.id}')">Return to Service</button>`
                 }
               </div>
            </div>
          </div>
        `;
      }).join('')}
    </div>
  `;
}

/* ── Actions ─────────────────────────────────────────────── */

function completeBatch(id) {
  const b = BATCHES.find(x => x.id === id);
  if (!b) return;
  
  b.status = 'COMPLETED';
  
  // Free up machine
  const m = MACHINES.find(x => x.id === b.machine);
  if (m) {
    m.status = 'IDLE';
    m.batch = null;
  }
  
  toast(`Batch ${id} completed.`, 'success');
  renderPage(state.page);
}

function reportBreakdown(id) {
  const m = MACHINES.find(x => x.id === id);
  if (!m) return;
  m.status = 'UNDER_MAINTENANCE';
  if (m.batch) {
    toast(`Machine ${id} breakdown reported! Active batch suspended.`, 'error');
  } else {
    toast(`Machine ${id} reported broken. Status synced to Asset.`, 'warning');
  }
  
  // Sync to Asset
  if (m.linked_asset_id) {
    const ast = ASSETS.find(a => a.id === m.linked_asset_id);
    if (ast) ast.status = 'UNDER_MAINTENANCE';
  }

  renderPage(state.page);
}

function returnToService(id) {
  const m = MACHINES.find(x => x.id === id);
  if (!m) return;
  m.status = 'IDLE';
  
  // Sync to Asset
  if (m.linked_asset_id) {
    const ast = ASSETS.find(a => a.id === m.linked_asset_id);
    if (ast) ast.status = 'IN_USE';
  }

  toast(`Machine ${id} returned to service.`, 'success');
  renderPage(state.page);
}
