/* ─── HQ / Purchase Requisitions ───────────────────────────────────────────────
   Review and approve/reject PRs from Factory and Store.
   Depends on: helpers.js, mock-data.js, toast.js, router.js.
──────────────────────────────────────────────────────────────────────────── */

async function renderHQPRs() {
  let prs = [];
  try {
    const res = await api.getPendingPRs(state.currentUser.orgId);
    prs = res || [];
  } catch (err) {
    return `<div class="error">Failed to load PRs: ${err.message}</div>`;
  }

  // Fetch all equipment types to map names
  let eqTypes = [];
  try {
    eqTypes = await api.getEquipmentTypes();
  } catch (err) {}

  return `
    ${pageHeader(
      'Purchase Requisitions',
      'Review CapEx and exceptional requests from nodes (Pending Approval)'
    )}
    
    <div class="card p-0">
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>PR ID</th><th>From</th><th>Justification</th><th>Items Requested</th><th>Est. Total</th><th>Status</th><th>Actions</th>
            </tr>
          </thead>
          <tbody>
            ${prs.length === 0 ? '<tr><td colspan="7" class="text-center dim py-4">No pending PRs found.</td></tr>' : ''}
            ${await Promise.all(prs.map(async (pr) => {
              // We need to fetch the PR lines
              let prDetails;
              try {
                prDetails = await api.getPR(pr.id);
              } catch (e) {
                return `<tr><td colspan="7">Error loading details for PR ${pr.id}</td></tr>`;
              }
              const lines = prDetails.lines || [];

              // Calculate total and format lines
              let total = 0;
              let lineHtml = '';
              lines.forEach(l => {
                total += (l.qty * l.estimated_unit_price);
                let name = l.proposed_equipment_name || l.equipment_type_id || l.item_id;
                if (l.equipment_type_id && !l.proposed_equipment_name) {
                  const eq = eqTypes.find(e => e.id === l.equipment_type_id);
                  if (eq) name = eq.name;
                }
                lineHtml += `<div class="small">${name} &times; ${l.qty} ${l.unit_of_measure}</div>`;
              });

              return `
                <tr>
                  <td><code>${pr.id.split('-')[0]}</code></td>
                  <td><span class="badge badge-sto">${pr.requester_node_id}</span></td>
                  <td><div class="truncate" style="max-width:200px" title="${pr.justification}">${pr.justification}</div></td>
                  <td>${lineHtml}</td>
                  <td class="num">${fmt(total)}</td>
                  <td>${statusBadge(pr.status)}</td>
                  <td>
                    ${pr.status === 'PENDING_HQ_APPROVAL' ? `
                      <button class="btn btn-primary btn-sm" onclick="approvePR('${pr.id}')">Approve</button>
                      <button class="btn btn-outline btn-sm" onclick="rejectPR('${pr.id}')">Reject</button>
                    ` : ''}
                  </td>
                </tr>
              `;
            })).then(htmls => htmls.join(''))}
          </tbody>
        </table>
      </div>
    </div>
  `;
}

/* ── Actions ─────────────────────────────────────────────── */

async function approvePR(id) {
  try {
    await api.approvePR(id, state.currentUser.staffId, 'Approved from HQ Dashboard');
    toast(`PR ${id.split('-')[0]} approved. Ready for PO conversion.`, 'success');
    renderPage(state.page);
  } catch (err) {
    toast(`Failed to approve PR: ${err.message}`, 'error');
  }
}

async function rejectPR(id) {
  const reason = prompt("Enter rejection reason:");
  if (!reason) return;

  try {
    await api.rejectPR(id, state.currentUser.staffId, reason);
    toast(`PR ${id.split('-')[0]} rejected.`, 'error');
    renderPage(state.page);
  } catch (err) {
    toast(`Failed to reject PR: ${err.message}`, 'error');
  }
}

