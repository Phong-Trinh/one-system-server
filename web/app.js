const API_BASE = 'http://localhost:8080/api/v1';

let globalItems = [];
let currentQuickAddSelect = null;
let currentQuickStationSelect = null;
let editingItemId = null; // ID của item đang được chỉnh sửa (form cơ bản)
let globalStationTypes = [];
let editingBOMID = null;
let editingSOPID = null;
let editingWizardItemID = null;
let globalNodes = [];
const activeNodeId = 'CUA_HANG_01'; // Hardcoded for this Store-local version
let activeOrgId = null; // Will be fetched from node info


// Load Initial Data
async function loadInitialData() {
    await Promise.all([
        loadItems(),
        loadStationTypes(),
        loadMachines(),
        loadNodes()
    ]);
    loadProductionOrders(); // Initial load of active orders
    startPolling();         // Start background fetching of pool and KDS tasks
}

let globalMachines = [];

async function loadMachines() {
    try {
        const url = activeNodeId ? `${API_BASE}/machines?node_id=${activeNodeId}` : `${API_BASE}/machines`;
        const res = await fetch(url);
        if (res.ok) {
            globalMachines = await res.json();
            renderMachinesGrid();
        }
    } catch (err) {
        console.error("Failed to load machines", err);
    }
}

function renderMachinesGrid() {
    const grid = document.getElementById('machineGrid');
    if (!grid) return;
    const safeMachines = Array.isArray(globalMachines) ? globalMachines : [];
    if (safeMachines.length === 0) {
        grid.innerHTML = '<div class="empty-state">Chưa có thiết bị nào được đăng ký tại chi nhánh này.</div>';
        return;
    }

    safeMachines.forEach(m => {
        const st = globalStationTypes.find(s => s.id === m.station_type_id);
        const card = document.createElement('div');
        card.className = `machine-card ${m.status.toLowerCase() === 'idle' ? 'active' : 'busy'}`;

        // Calculate capacity percentage (mock for now, should come from API)
        const capacityUsed = m.status.toLowerCase() === 'busy' ? 75 : 0; // Example
        const dashOffset = 251.2 - (251.2 * capacityUsed / 100);

        card.innerHTML = `
            <div class="machine-card-header">
                <div>
                    <h4>${m.id}</h4>
                    <span class="machine-type-tag">${st ? st.name : m.station_type_id}</span>
                </div>
                <span class="strategy-badge">${m.allocation_strategy}</span>
            </div>
            
            <div class="capacity-indicator">
                <svg class="capacity-svg" viewBox="0 0 100 100">
                    <circle class="capacity-circle-bg" cx="50" cy="50" r="40"></circle>
                    <circle class="capacity-circle-progress" cx="50" cy="50" r="40" 
                        style="stroke-dasharray: 251.2; stroke-dashoffset: ${dashOffset}; stroke: ${capacityUsed > 80 ? 'var(--accent)' : 'var(--success)'}">
                    </circle>
                </svg>
                <div class="capacity-text">${capacityUsed}%</div>
            </div>

            <div class="machine-meta">
                <div class="meta-item">
                    <span class="meta-val">${m.max_capacity} ${st ? st.capacity_unit : ''}</span>
                    Sức chứa
                </div>
            </div>
        `;
        grid.appendChild(card);
    });
}

async function loadStationTypes() {
    try {
        const res = await fetch(`${API_BASE}/station-types`);
        if (res.ok) {
            globalStationTypes = await res.json();
            updateWizStationSelects();
        }
    } catch (err) {
        console.error("Failed to load station types", err);
    }
}

async function loadNodes() {
    try {
        const res = await fetch(`${API_BASE}/nodes`);
        if (res.ok) {
            globalNodes = await res.json();

            // Auto-detect OrgID for our hardcoded store
            const node = globalNodes.find(n => n.id === activeNodeId);
            if (node) {
                activeOrgId = node.org_id;
            }

            updatePOSelects();
        }
    } catch (err) {
        console.error("Failed to load nodes", err);
    }
}

// Dynamic switching disabled for Store-local version
function switchNodeContext(nodeId) {
    console.log("Node context fixed to:", activeNodeId);
}

function updatePOSelects() {
    const itemSelect = document.getElementById('poItemSelect');
    if (!itemSelect) return;
    itemSelect.innerHTML = '<option value="" disabled selected>Chọn món cần sản xuất...</option>';

    const safeItems = Array.isArray(globalItems) ? globalItems : [];
    safeItems.filter(i => i.type !== 'RAW_MATERIAL').forEach(item => {
        const opt = document.createElement('option');
        opt.value = item.id;
        opt.textContent = `${item.name} (${item.type})`;
        itemSelect.appendChild(opt);
    });
}

function updateWizStationSelects() {
    const selects = document.querySelectorAll('.wiz-station-input');
    selects.forEach(select => {
        const val = select.value;
        select.innerHTML = '<option value="" disabled selected>Chọn loại thiết bị...</option>';

        const safeTypes = Array.isArray(globalStationTypes) ? globalStationTypes : [];
        safeTypes.forEach(st => {
            const opt = document.createElement('option');
            opt.value = st.id;
            opt.textContent = st.name;
            select.appendChild(opt);
        });
        select.value = val;
    });
}

function updateWizItemSelects() {
    const selects = document.querySelectorAll('.wiz-item-id-input, .wiz-step-ingredients-select');
    selects.forEach(select => {
        const val = select.value;
        const placeholder = select.classList.contains('wiz-item-id-input') ? 'Chọn nguyên liệu...' : '+ Thêm nguyên liệu từ BOM...';
        select.innerHTML = `<option value="" disabled selected>${placeholder}</option>`;

        const safeItems = Array.isArray(globalItems) ? globalItems : [];
        safeItems.forEach(item => {
            const opt = document.createElement('option');
            opt.value = item.id;
            opt.textContent = `${item.name} (${item.type})`;
            select.appendChild(opt);
        });
        select.value = val;
    });
}

// Load Items
async function loadItems() {
    try {
        const url = activeOrgId ? `${API_BASE}/items?org_id=${activeOrgId}` : `${API_BASE}/items`;
        const res = await fetch(url);
        if (res.ok) {
            const data = await res.json();
            globalItems = Array.isArray(data) ? data : [];

            updateWizItemSelects();
            updatePOSelects();
            renderItemGrid();
        }
    } catch (err) {
        console.error("Failed to load items", err);
    }
}

let currentItemTypeFilter = 'ALL';

function filterItemType(type) {
    currentItemTypeFilter = type;
    document.querySelectorAll('.filter-btn').forEach(btn => {
        btn.classList.toggle('active', btn.getAttribute('onclick').includes(type));
    });
    renderItemGrid();
}

function renderItemGrid(searchTerm = "") {
    const grid = document.getElementById('itemGrid');
    if (!grid) return;
    grid.innerHTML = "";

    const safeItems = Array.isArray(globalItems) ? globalItems : [];
    const filtered = safeItems.filter(item => {
        if (!item) return false;
        const matchesSearch = item.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
            (item.sku && item.sku.toLowerCase().includes(searchTerm.toLowerCase()));
        const matchesType = currentItemTypeFilter === 'ALL' || item.type === currentItemTypeFilter;
        return matchesSearch && matchesType;
    });

    if (filtered.length === 0) {
        grid.innerHTML = '<div class="empty-state">Không tìm thấy kết quả phù hợp.</div>';
        return;
    }

    filtered.forEach(item => {
        const card = document.createElement('div');
        card.className = `item-card ${item.type.toLowerCase().replace('_', '-')}`;

        const hasProcess = item.type === 'PRODUCT' || item.type === 'SEMI_PRODUCT';

        card.innerHTML = `
            <div class="item-card-header">
                <div>
                    <h4>${item.name}</h4>
                    <span class="item-sku">${item.sku || 'Không có SKU'}</span>
                </div>
                <span class="badge" style="font-size: 0.6rem;">${item.type}</span>
            </div>
            <div class="item-card-footer">
                <span class="item-unit">📦 ${item.base_unit}</span>
                <div class="card-actions">
                    <button class="btn-edit" onclick="event.stopPropagation(); editItem('${item.id}')" title="Sửa thông tin cơ bản">✏️</button>
                    ${hasProcess ? `<button class="btn-primary" onclick="event.stopPropagation(); openItemDetail('${item.id}')" style="font-size: 0.7rem; padding: 0.3rem 0.6rem;">👁️ Xem Quy trình</button>` : ''}
                </div>
            </div>
        `;

        card.onclick = () => {
            if (hasProcess) openItemDetail(item.id);
            else editItem(item.id);
        };

        grid.appendChild(card);
    });
}

function openQuickAddRawMaterial() {
    editingItemId = null;
    document.getElementById('itemForm').reset();
    document.getElementById('itemType').value = 'RAW_MATERIAL';
    document.getElementById('itemSubmitBtn').textContent = "Tạo mới";
    document.getElementById('editItemOverlay').classList.add('active');
}

function closeEditItem() {
    document.getElementById('editItemOverlay').classList.remove('active');
}

async function openItemDetail(itemId) {
    const item = globalItems.find(i => i.id === itemId);
    if (!item) return;

    document.getElementById('detailItemName').textContent = item.name;
    document.getElementById('detailItemBadge').textContent = item.type;
    document.getElementById('detailItemBadge').className = `badge ${item.type.toLowerCase().replace('_', '-')}`;

    // Clear lists
    document.getElementById('detailBomList').innerHTML = '<li>Đang tải...</li>';
    document.getElementById('detailMermaid').innerHTML = '<div class="empty-state">Đang tải...</div>';

    document.getElementById('itemDetailModal').classList.add('active');

    try {
        // 1. Fetch BOM
        const bomRes = await fetch(`${API_BASE}/production/boms/by-item/${itemId}`);
        if (bomRes.ok) {
            const bomData = await bomRes.json();
            const bomId = bomData.bom.id;

            // Render BOM Lines
            document.getElementById('detailBomList').innerHTML = bomData.lines.map(line => {
                const ing = globalItems.find(i => i.id === line.item_id);
                return `<li><span>${ing ? ing.name : 'Unknown'}</span> <strong>${line.qty} ${ing ? ing.base_unit : ''}</strong></li>`;
            }).join('');

            // 2. Fetch SOP
            const sopRes = await fetch(`${API_BASE}/production/sops/by-bom/${bomId}`);
            if (sopRes.ok) {
                const sopData = await sopRes.json();
                renderDetailSOP(sopData.steps);
            } else {
                document.getElementById('detailMermaid').innerHTML = '<div class="empty-state">Chưa có quy trình (SOP).</div>';
            }
        } else {
            document.getElementById('detailBomList').innerHTML = '<li>Chưa có định mức (BOM).</li>';
            document.getElementById('detailMermaid').innerHTML = '<div class="empty-state">Chưa có dữ liệu sản xuất.</div>';
        }
    } catch (err) {
        console.error("Detail load error", err);
    }

    document.getElementById('detailEditBtn').onclick = () => {
        closeItemDetail();
        editBOMSOP(itemId);
    };
}

function renderDetailSOP(steps) {
    if (!steps || steps.length === 0) {
        document.getElementById('detailMermaid').innerHTML = '<div class="empty-state">Chưa có các bước thực hiện.</div>';
        return;
    }

    let graph = 'graph TD;\n';
    steps.forEach((step, idx) => {
        const num = idx + 1;
        const station = globalStationTypes.find(s => s.id === step.station_type_id);
        const nodeLabel = `"${num}. ${step.description.replace(/"/g, "'")}\\n[${station ? station.name : 'Manual'}]"`;
        graph += `  S${num}[${nodeLabel}]\n`;

        step.depends_on.forEach(depId => {
            const depIdx = steps.findIndex(s => s.id === depId);
            if (depIdx !== -1) {
                graph += `  S${depIdx + 1} --> S${num}\n`;
            }
        });
    });

    try {
        mermaid.render('detailGraph', graph).then(({ svg }) => {
            document.getElementById('detailMermaid').innerHTML = svg;
        });
    } catch (err) {
        console.error("Mermaid detail error", err);
    }
}

function closeItemDetail() {
    document.getElementById('itemDetailModal').classList.remove('active');
}

function prepareProductionOrder(itemId) {
    const item = globalItems.find(i => i.id === itemId);
    if (!item) return;

    // Chuyển sang tab Sản xuất
    document.querySelector('[data-target="production-section"]').click();

    // Tự động chọn item
    const itemSelect = document.getElementById('poItemSelect');
    itemSelect.value = itemId;
    document.getElementById('poUnitLabel').textContent = item.base_unit;

    // Kiểm tra BOM
    checkBOMForItem(itemId);
}

async function checkBOMForItem(itemId) {
    const info = document.getElementById('poBomInfo');
    info.style.display = 'none';
    try {
        const res = await fetch(`${API_BASE}/production/boms/by-item/${itemId}`);
        if (res.ok) {
            const data = await res.json();
            info.style.display = 'block';
            info.setAttribute('data-bom-id', data.bom.id);
        } else {
            alert("Món này chưa có công thức (BOM). Vui lòng tạo BOM trước!");
        }
    } catch (err) {
        console.error("Check BOM error", err);
    }
}

// Lắng nghe thay đổi trên select món ăn ở tab PO
document.getElementById('poItemSelect')?.addEventListener('change', (e) => {
    const item = globalItems.find(i => i.id === e.target.value);
    if (item) {
        document.getElementById('poUnitLabel').textContent = item.base_unit;
        checkBOMForItem(item.id);
    }
});

function editItem(id) {
    const item = globalItems.find(i => i.id === id);
    if (!item) return;

    editingItemId = id;
    document.getElementById('itemName').value = item.name;
    document.getElementById('itemSku').value = item.sku || "";
    document.getElementById('itemType').value = item.type;
    document.getElementById('itemBaseUnit').value = item.base_unit;

    const submitBtn = document.getElementById('itemSubmitBtn');
    submitBtn.textContent = "Cập nhật thay đổi";
    submitBtn.classList.replace('btn-primary', 'btn-accent');

    document.getElementById('editItemOverlay').classList.add('active');
}

// Xử lý tìm kiếm
document.getElementById('itemSearchInput')?.addEventListener('input', (e) => {
    renderItemGrid(e.target.value);
});

function updateAllItemSelects() {
    // Cập nhật select cho Thành phẩm (chỉ chọn Product / Semi-product)
    const outputSelect = document.getElementById('outputItemId');
    const currentOutputVal = outputSelect.value;
    outputSelect.innerHTML = '<option value="" disabled selected>Chọn thành phẩm...</option>';

    // Cập nhật select cho thành phần (nguyên liệu)
    const ingredientSelects = document.querySelectorAll('.item-id-input');

    globalItems.forEach(item => {
        // Cho select Thành phẩm
        if (item.type === 'PRODUCT' || item.type === 'SEMI_PRODUCT') {
            const opt = document.createElement('option');
            opt.value = item.id;
            opt.textContent = `${item.name} (${item.type})`;
            outputSelect.appendChild(opt);
        }
    });
    outputSelect.value = currentOutputVal || "";

    ingredientSelects.forEach(select => {
        const currentVal = select.value;
        select.innerHTML = '<option value="" disabled selected>Chọn nguyên liệu...</option>';
        globalItems.forEach(item => {
            const opt = document.createElement('option');
            opt.value = item.id;
            opt.textContent = `${item.name} (${item.type})`;
            select.appendChild(opt);
        });
        select.value = currentVal || "";
    });
}

// Navigation Logic
document.querySelectorAll('.nav-item').forEach(button => {
    button.addEventListener('click', () => {
        // Remove active class from all
        document.querySelectorAll('.nav-item').forEach(btn => btn.classList.remove('active'));
        document.querySelectorAll('.section-card').forEach(sec => sec.classList.remove('active'));

        // Add active class to clicked
        button.classList.add('active');
        const targetId = button.getAttribute('data-target');
        document.getElementById(targetId).classList.add('active');

        // Special logic per tab
        if (targetId === 'wizard-section') {
            resetWizard();
        }
        if (targetId === 'org-section') {
            renderNodesList();
        }
    });
});

function resetWizard() {
    editingWizardItemID = null;
    editingBOMID = null;
    editingSOPID = null;
    document.getElementById('wizItemName').value = "";
    document.getElementById('wizItemSku').value = "";
    document.getElementById('wizItemBaseUnit').value = "gram";
    document.getElementById('wizIngredientsList').innerHTML = "";
    document.getElementById('wizStepsList').innerHTML = "";
    addWizIngredientRow();
    addWizStepRow();
    document.getElementById('wizard-btn-finish').textContent = "✨ Hoàn tất & Lưu dữ liệu";
    goToStep(1);
}

async function editBOMSOP(itemId) {
    const item = globalItems.find(i => i.id === itemId);
    if (!item) return;

    resetWizard();
    editingWizardItemID = itemId;

    // Switch to Wizard tab
    document.querySelectorAll('.nav-item').forEach(btn => btn.classList.remove('active'));
    document.querySelectorAll('.section-card').forEach(sec => sec.classList.remove('active'));
    document.querySelector('[data-target="wizard-section"]').classList.add('active');
    document.getElementById('wizard-section').classList.add('active');

    // Step 1: Info
    document.getElementById('wizItemName').value = item.name;
    document.getElementById('wizItemSku').value = item.sku || "";
    document.getElementById('wizItemType').value = item.type;
    document.getElementById('wizItemBaseUnit').value = item.base_unit;

    // Load BOM
    try {
        const bomRes = await fetch(`${API_BASE}/production/boms/by-item/${itemId}`);
        if (bomRes.ok) {
            const bomData = await bomRes.json();
            editingBOMID = bomData.bom.id;

            // Clear default row
            document.getElementById('wizIngredientsList').innerHTML = "";
            bomData.lines.forEach(line => {
                const template = document.getElementById('ingredientTemplate');
                const clone = template.content.cloneNode(true);
                const row = clone.querySelector('.list-row');
                row.setAttribute('data-line-id', line.id);

                row.querySelector('.wiz-item-id-input').value = line.item_id;
                row.querySelector('.wiz-qty-input').value = line.qty;

                row.querySelector('.btn-remove').addEventListener('click', () => { row.remove(); updateSOPPreview(); });
                row.querySelector('.wiz-item-id-input').addEventListener('change', updateSOPPreview);
                row.querySelector('.wiz-qty-input').addEventListener('input', updateSOPPreview);

                document.getElementById('wizIngredientsList').appendChild(clone);
            });
            updateWizIngredientSelects(item.type === 'PRODUCT');

            // Load SOP
            const sopRes = await fetch(`${API_BASE}/production/sops/by-bom/${editingBOMID}`);
            if (sopRes.ok) {
                const sopData = await sopRes.json();
                editingSOPID = sopData.sop.id;

                document.getElementById('wizStepsList').innerHTML = "";
                sopData.steps.forEach(step => {
                    const template = document.getElementById('stepTemplate');
                    const clone = template.content.cloneNode(true);
                    const row = clone.querySelector('.list-row');
                    row.setAttribute('data-step-id', step.id);

                    row.querySelector('.wiz-desc-input').value = step.description;
                    row.querySelector('.wiz-station-input').value = step.station_type_id || "";
                    row.querySelector('.wiz-duration-input').value = step.duration;

                    // Add dependencies badges
                    const depContainer = row.querySelector('.depends-container');
                    step.depends_on.forEach(depId => {
                        const badge = document.createElement('span');
                        badge.className = 'dep-badge';
                        badge.setAttribute('data-dep-id', depId);
                        badge.innerHTML = `Bước ? <button type="button">×</button>`;
                        badge.querySelector('button').addEventListener('click', () => badge.remove());
                        depContainer.appendChild(badge);
                    });

                    // Add ingredient badges
                    const ingContainer = row.querySelector('.step-ingredients-container');
                    step.ingredient_bom_line_ids.forEach(lineId => {
                        const badge = document.createElement('span');
                        badge.className = 'dep-badge';
                        badge.style.background = '#065f46';
                        badge.style.border = '1px solid #10b981';
                        badge.setAttribute('data-line-id', lineId);
                        badge.innerHTML = `NL ? <button type="button">×</button>`;
                        badge.querySelector('button').addEventListener('click', () => badge.remove());
                        ingContainer.appendChild(badge);
                    });

                    document.getElementById('wizStepsList').appendChild(clone);
                });
                updateWizStationSelects();
                updateSOPPreview(); // This will refresh the badge texts with proper numbers/names
            }
        }
    } catch (err) {
        console.error("Error loading BOM/SOP", err);
    }

    document.getElementById('wizard-btn-finish').textContent = "💾 Cập nhật Dữ liệu";
    goToStep(1);
}

function goToStep(step) {
    // Hide all steps
    document.querySelectorAll('.wizard-step').forEach(s => s.style.display = 'none');
    // Show target step
    const target = document.getElementById(`wizard-step-${step}`);
    if (target) target.style.display = 'block';

    // Update indicators
    document.querySelectorAll('.step-indicator').forEach((ind, index) => {
        if (index < step) {
            ind.classList.add('active');
        } else {
            ind.classList.remove('active');
        }
    });
}

// WIZARD LOGIC
function nextWizardStep(step) {
    // Validate Step 1 before moving to Step 2
    if (step === 2) {
        const name = document.getElementById('wizItemName').value.trim();
        if (!name) return alert("Vui lòng nhập tên món!");

        // Kiểm tra trùng tên
        const safeItems = Array.isArray(globalItems) ? globalItems : [];
        const isDuplicate = safeItems.some(i => i && i.name && i.name.toLowerCase() === name.toLowerCase() && i.id !== editingWizardItemID);
        if (isDuplicate) {
            const confirmDup = confirm(`Cảnh báo: Nguyên liệu/Món ăn mang tên "${name}" đã tồn tại trong hệ thống! Bạn có chắc chắn muốn tạo thêm một bản ghi trùng tên không?`);
            if (!confirmDup) return;
        }

        // Cập nhật cảnh báo và danh sách thả xuống ở Step 2
        const typeSelect = document.getElementById('wizItemType');
        const type = typeSelect ? typeSelect.value : 'PRODUCT';
        const warning = document.getElementById('bomWarning');
        if (warning) warning.style.display = type === 'PRODUCT' ? 'block' : 'none';

        // Populate current ingredients with filtered options
        updateWizIngredientSelects(type === 'PRODUCT');
    }

    // Step 3: Populate Station Types
    if (step === 3) {
        updateWizStationSelects();
    }

    // Step 4: Show Summary
    if (step === 4) {
        const name = document.getElementById('wizItemName').value;
        const type = document.getElementById('wizItemType').value;

        const bomRows = document.getElementById('wizIngredientsList').querySelectorAll('.list-row').length;
        const sopRows = document.getElementById('wizStepsList').querySelectorAll('.list-row').length;

        const summary = document.getElementById('wizardSummary');
        if (summary) {
            summary.innerHTML = `
                <p><strong>Tên món:</strong> ${name}</p>
                <p><strong>Loại:</strong> ${type}</p>
                <p><strong>Thành phần (BOM):</strong> ${bomRows} nguyên liệu</p>
                <p><strong>Công đoạn (SOP):</strong> ${sopRows} bước</p>
            `;
        }
    }

    goToStep(step);
}

function prevWizardStep(step) {
    goToStep(step);
}

function addWizIngredientRow() {
    const template = document.getElementById('ingredientTemplate');
    const clone = template.content.cloneNode(true);
    const row = clone.querySelector('.list-row');

    // Generate Temp ID for BOM Line
    const lineId = 'L_' + Math.random().toString(36).substr(2, 9);
    row.setAttribute('data-line-id', lineId);

    row.querySelector('.btn-remove').addEventListener('click', () => {
        row.remove();
        updateSOPPreview();
    });

    const select = clone.querySelector('.wiz-item-id-input');
    const type = document.getElementById('wizItemType').value;

    // Nút tạo nhanh
    clone.querySelector('.btn-quick-add').addEventListener('click', () => {
        currentQuickAddSelect = select;
        document.getElementById('quickCreateModal').classList.add('active');
    });

    select.addEventListener('change', updateSOPPreview);
    row.querySelector('.wiz-qty-input').addEventListener('input', updateSOPPreview);

    document.getElementById('wizIngredientsList').appendChild(clone);
    updateWizIngredientSelects(type === 'PRODUCT');
}

function updateWizIngredientSelects(onlyRawAndSemi) {
    const selects = document.querySelectorAll('.wiz-item-id-input');
    const safeItems = Array.isArray(globalItems) ? globalItems : [];

    selects.forEach(select => {
        const currentVal = select.value;
        select.innerHTML = '<option value="" disabled selected>Chọn nguyên liệu...</option>';
        safeItems.forEach(item => {
            if (!item) return;
            if (onlyRawAndSemi && item.type === 'PRODUCT') return; // Bỏ qua Product nếu món đang tạo là Product

            const opt = document.createElement('option');
            opt.value = item.id;
            opt.textContent = `${item.name} (${item.type})`;
            select.appendChild(opt);
        });
        select.value = currentVal || "";
    });
}

// Initialize Mermaid
mermaid.initialize({ startOnLoad: false, theme: 'dark' });

function addWizStepRow() {
    const template = document.getElementById('stepTemplate');
    const clone = template.content.cloneNode(true);
    const row = clone.querySelector('.list-row');

    // Generate Temp ID
    const stepId = 'S_' + Math.random().toString(36).substr(2, 9);
    row.setAttribute('data-step-id', stepId);

    // Remove button
    clone.querySelector('.btn-remove').addEventListener('click', () => {
        row.remove();
        updateSOPPreview();
    });

    // Add Dependency logic
    const selectDep = clone.querySelector('.wiz-depends-select');
    const containerDep = clone.querySelector('.depends-container');
    const selectIng = clone.querySelector('.wiz-step-ingredients-select');
    const containerIng = clone.querySelector('.step-ingredients-container');
    const selectStation = clone.querySelector('.wiz-station-input');

    // Nút tạo nhanh Station
    clone.querySelector('.wiz-station-quick-add').addEventListener('click', () => {
        currentQuickStationSelect = selectStation;
        document.getElementById('quickStationModal').classList.add('active');
    });

    selectDep.addEventListener('change', (e) => {
        const depId = e.target.value;
        const depText = e.target.options[e.target.selectedIndex].text;
        if (depId) {
            const badge = document.createElement('div');
            badge.className = 'dep-badge';
            badge.setAttribute('data-dep-id', depId);
            badge.innerHTML = `${depText} <button type="button">×</button>`;

            badge.querySelector('button').addEventListener('click', () => {
                badge.remove();
                updateSOPPreview();
            });

            containerDep.appendChild(badge);
            e.target.value = ""; // Reset
            updateSOPPreview();
        }
    });

    selectIng.addEventListener('change', (e) => {
        const lineId = e.target.value;
        const ingText = e.target.options[e.target.selectedIndex].text;
        if (lineId) {
            const badge = document.createElement('div');
            badge.className = 'dep-badge'; // Reuse styling
            badge.style.background = 'rgba(16, 185, 129, 0.2)'; // Greenish for ingredients
            badge.style.border = '1px solid #10b981';
            badge.setAttribute('data-line-id', lineId);
            badge.innerHTML = `${ingText} <button type="button">×</button>`;

            badge.querySelector('button').addEventListener('click', () => {
                badge.remove();
            });

            containerIng.appendChild(badge);
            e.target.value = ""; // Reset
        }
    });

    // Listen to changes to update graph and UI labels
    selectStation.addEventListener('change', (e) => {
        const stId = e.target.value;
        const st = globalStationTypes.find(s => s.id === stId);
        const badge = row.querySelector('.wiz-strategy-badge');
        const unitLabel = row.querySelector('.wiz-unit-label');

        if (st) {
            // Update Unit
            unitLabel.textContent = st.capacity_unit;

            // Update Strategy Badge
            badge.style.display = 'flex';
            const isBatch = st.default_strategy === 'BATCH_SYNC';
            badge.querySelector('.strategy-icon').textContent = isBatch ? "🔒" : "⏲️";
            badge.querySelector('.strategy-name').textContent = isBatch ? "Batch" : "Async";
            badge.title = isBatch ? "Khóa theo mẻ (Sync)" : "Nấu tự do (Async)";
        }
        updateSOPPreview();
    });

    clone.querySelector('.wiz-desc-input').addEventListener('input', updateSOPPreview);

    document.getElementById('wizStepsList').appendChild(clone);
    updateWizStationSelects();
    updateSOPPreview();
}

function updateSOPPreview() {
    const stepRows = document.querySelectorAll('#wizStepsList .sop-step-row');
    const ingRows = document.querySelectorAll('#wizIngredientsList .ingredient-row');
    const stepsData = [];

    // Get ingredients from Step 2
    const currentIngredients = Array.from(ingRows).map(row => {
        const itemId = row.querySelector('.wiz-item-id-input').value;
        const lineId = row.getAttribute('data-line-id');
        const item = globalItems.find(i => i.id === itemId);
        return { lineId, name: item ? item.name : 'Unknown' };
    }).filter(i => i.lineId && i.name !== 'Unknown');

    // 1. Gather current state
    stepRows.forEach((row, index) => {
        const num = index + 1;
        row.querySelector('.step-number').textContent = num;

        const id = row.getAttribute('data-step-id');
        const desc = row.querySelector('.wiz-desc-input').value.trim() || `Bước ${num}`;
        const stationSelect = row.querySelector('.wiz-station-input');
        const station = stationSelect.options[stationSelect.selectedIndex]?.text || '';
        const deps = Array.from(row.querySelectorAll('.dep-badge[data-dep-id]')).map(b => b.getAttribute('data-dep-id'));
        const usedIngs = Array.from(row.querySelectorAll('.dep-badge[data-line-id]')).map(b => b.getAttribute('data-line-id'));

        stepsData.push({ id, num, desc, station, deps, usedIngs, row });
    });

    // 2. Refresh Dropdowns (Dependencies & Ingredients)
    stepsData.forEach(step => {
        // Refresh Dependencies
        const selectDep = step.row.querySelector('.wiz-depends-select');
        selectDep.innerHTML = '<option value="">+ Thêm bước cần chờ...</option>';
        stepsData.forEach(other => {
            if (other.id !== step.id && !step.deps.includes(other.id)) {
                const opt = document.createElement('option');
                opt.value = other.id;
                opt.textContent = `B${other.num}. ${other.desc}`;
                selectDep.appendChild(opt);
            }
        });

        // Refresh Ingredients
        const selectIng = step.row.querySelector('.wiz-step-ingredients-select');
        selectIng.innerHTML = '<option value="">+ Thêm nguyên liệu từ BOM...</option>';
        currentIngredients.forEach(ing => {
            if (!step.usedIngs.includes(ing.lineId)) {
                const opt = document.createElement('option');
                opt.value = ing.lineId;
                opt.textContent = ing.name;
                selectIng.appendChild(opt);
            }
        });
    });

    // 3. Render Mermaid
    const preview = document.getElementById('mermaidPreview');
    if (stepsData.length === 0) {
        preview.innerHTML = '<div class="empty-state">Thêm bước để xem sơ đồ...</div>';
        return;
    }

    let graphDef = 'graph TD;\n';

    stepsData.forEach(step => {
        const safeDesc = step.desc.replace(/["\\]/g, '');
        const st = globalStationTypes.find(s => s.name === step.station);
        const isManual = !st;

        const safeStation = step.station && step.station !== 'Chọn loại thiết bị...' ? `\\n[${step.station.replace(/["\\]/g, '')}]` : '';
        const nodeLabel = `"${step.num}. ${safeDesc}${safeStation}"`;

        // Use different shapes for machine vs manual steps
        const nodeShape = isManual ? `([${nodeLabel}])` : `[${nodeLabel}]`;
        graphDef += `  ${step.id}${nodeShape}\n`;
        if (!isManual) graphDef += `  style ${step.id} fill:#6366f1,stroke:#4f46e5,stroke-width:2px,color:#fff\n`;

        // Remove invalid badges and add valid edges
        step.deps.forEach(depId => {
            const depStep = stepsData.find(s => s.id === depId);
            const badge = step.row.querySelector(`.dep-badge[data-dep-id="${depId}"]`);
            if (depStep) {
                graphDef += `  ${depId} --> ${step.id}\n`;
                if (badge) badge.childNodes[0].textContent = `B${depStep.num}. ${depStep.desc} `;
            } else {
                if (badge) badge.remove();
            }
        });

        step.usedIngs.forEach(lineId => {
            const ing = currentIngredients.find(i => i.lineId === lineId);
            const badge = step.row.querySelector(`.dep-badge[data-line-id="${lineId}"]`);
            if (ing && badge) {
                badge.childNodes[0].textContent = `${ing.name} `;
            } else if (badge) {
                badge.remove();
            }
        });
    });

    try {
        mermaid.render('sopGraphRendered', graphDef).then(({ svg }) => {
            preview.innerHTML = svg;
        }).catch(err => console.warn(err));
    } catch (err) {
        console.warn("Mermaid render error", err);
    }
}

function updateWizStationSelects() {
    const selects = document.querySelectorAll('.wiz-station-input');
    const machineSelect = document.getElementById('machineStationType');
    const safeStationTypes = Array.isArray(globalStationTypes) ? globalStationTypes : [];

    selects.forEach(select => {
        const currentVal = select.value;
        select.innerHTML = '<option value="" disabled selected>Chọn trạm...</option>';
        safeStationTypes.forEach(st => {
            const opt = document.createElement('option');
            opt.value = st.id;
            opt.textContent = st.name;
            select.appendChild(opt);
        });
        select.value = currentVal || "";
    });

    if (machineSelect) {
        const currentVal = machineSelect.value;
        machineSelect.innerHTML = '<option value="" disabled selected>Chọn loại trạm...</option>';
        safeStationTypes.forEach(st => {
            const opt = document.createElement('option');
            opt.value = st.id;
            opt.textContent = st.name;
            machineSelect.appendChild(opt);
        });
        machineSelect.value = currentVal || "";
    }
}

document.getElementById('wizAddIngredientBtn').addEventListener('click', addWizIngredientRow);
document.getElementById('wizAddStepBtn').addEventListener('click', addWizStepRow);

async function submitWizard() {
    const btn = document.getElementById('wizard-btn-finish');
    if (!btn) return;
    btn.disabled = true;
    btn.textContent = "Đang xử lý...";

    try {
        // 1. Item
        let itemId = editingWizardItemID;
        const name = document.getElementById('wizItemName').value;
        const sku = document.getElementById('wizItemSku').value;
        const type = document.getElementById('wizItemType').value;
        const baseUnit = document.getElementById('wizItemBaseUnit').value;

        if (!itemId) {
            const itemRes = await fetch(`${API_BASE}/items`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    org_id: activeOrgId, // <-- Thêm OrgID
                    name, sku, type, base_unit: baseUnit
                })
            });
            const itemData = await itemRes.json();
            if (!itemRes.ok) throw new Error("Lỗi tạo Item: " + (itemData.error || itemData));
            itemId = itemData.id;
        }

        // 2. BOM
        const bomLines = [];
        document.getElementById('wizIngredientsList').querySelectorAll('.ingredient-row').forEach(row => {
            const ingItemId = row.querySelector('.wiz-item-id-input').value;
            const qty = parseFloat(row.querySelector('.wiz-qty-input').value);
            if (ingItemId && qty) {
                bomLines.push({ item_id: ingItemId, qty: qty });
            }
        });

        let bomId = editingBOMID;
        if (bomLines.length > 0) {
            const url = bomId ? `${API_BASE}/production/boms/${bomId}` : `${API_BASE}/production/boms`;
            const method = bomId ? 'PUT' : 'POST';
            const body = bomId ? { lines: bomLines } : { output_item_id: itemId, lines: bomLines };

            const bomRes = await fetch(url, {
                method: method,
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            });

            if (bomRes.ok) {
                if (!bomId) {
                    const bomData = await bomRes.json();
                    bomId = bomData.id;
                }
            } else {
                const bomData = await bomRes.json();
                throw new Error("Lỗi lưu BOM: " + (bomData.error || JSON.stringify(bomData)));
            }
        }

        // 3. SOP
        if (bomId) {
            const steps = [];
            document.getElementById('wizStepsList').querySelectorAll('.sop-step-row').forEach(row => {
                const id = row.getAttribute('data-step-id');
                const stationId = row.querySelector('.wiz-station-input').value;
                const duration = parseInt(row.querySelector('.wiz-duration-input').value);
                const desc = row.querySelector('.wiz-desc-input').value;
                const dependsOn = Array.from(row.querySelectorAll('.dep-badge[data-dep-id]')).map(b => b.getAttribute('data-dep-id'));
                const ingredients = Array.from(row.querySelectorAll('.dep-badge[data-line-id]')).map(b => b.getAttribute('data-line-id'));
                const slots = parseFloat(row.querySelector('.wiz-slots-input').value) || 1;
                const allowMix = row.querySelector('.wiz-mix-input').checked;

                if (duration) {
                    steps.push({
                        id: id,
                        depends_on: dependsOn,
                        station_type_id: stationId || "",
                        ingredient_bom_line_ids: ingredients,
                        duration: duration,
                        description: desc,
                        slot_consumption: slots,
                        allow_mix: allowMix
                    });
                }
            });

            if (steps.length > 0) {
                const sopId = editingSOPID;
                const url = sopId ? `${API_BASE}/production/sops/${sopId}` : `${API_BASE}/production/sops`;
                const method = sopId ? 'PUT' : 'POST';
                const body = sopId ? { steps } : { bom_id: bomId, steps };

                const sopRes = await fetch(url, {
                    method: method,
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });
                if (!sopRes.ok) {
                    const sopData = await sopRes.json();
                    throw new Error("Lỗi lưu SOP: " + (sopData.error || JSON.stringify(sopData)));
                }
            }
        }

        alert("✨ Đã lưu quy trình sản xuất thành công!");
        resetWizard();
        loadItems();
    } catch (err) {
        alert("Lỗi: " + err.message);
    } finally {
        btn.disabled = false;
        btn.textContent = "✨ Hoàn tất & Lưu dữ liệu";
    }
}

function resetWizard() {
    editingWizardItemID = null;
    editingBOMID = null;
    editingSOPID = null;

    document.getElementById('wizItemName').value = "";
    document.getElementById('wizItemSku').value = "";
    document.getElementById('wizItemType').value = "PRODUCT";
    document.getElementById('wizItemBaseUnit').value = "";

    document.getElementById('wizIngredientsList').innerHTML = "";
    document.getElementById('wizStepsList').innerHTML = "";
    document.getElementById('mermaidPreview').innerHTML = '<div class="empty-state">Thêm bước để xem sơ đồ...</div>';

    goToStep(1);
}

// ---------------------------------------------------------
// EVENT LISTENERS
// ---------------------------------------------------------

// Sidebar Navigation
document.querySelectorAll('.nav-item').forEach(item => {
    item.addEventListener('click', (e) => {
        const targetId = item.getAttribute('data-target');
        console.log("Navigating to:", targetId);

        // Special case for Wizard: Open as Modal
        if (targetId === 'wizard-section') {
            openWizardModal();
            return;
        }

        // 1. Update Sidebar UI
        document.querySelectorAll('.nav-item').forEach(nav => nav.classList.remove('active'));
        item.classList.add('active');

        // 2. Switch Sections
        const allSections = document.querySelectorAll('.section-card');
        console.log("Found sections:", allSections.length);
        allSections.forEach(sec => {
            // Keep sections inside modals as active if they are (to not break modal internal layout)
            if (sec.closest('.modal')) return;
            sec.classList.remove('active');
            console.log("Deactivating:", sec.id);
        });

        const targetSec = document.getElementById(targetId);
        if (targetSec) {
            targetSec.classList.add('active');
            console.log("Section activated:", targetId);
        } else {
            console.error("Target section not found:", targetId);
        }
    });
});

// Item Form (Simple Add)
document.getElementById('itemForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const name = document.getElementById('itemName').value;
    const sku = document.getElementById('itemSku').value;
    const type = document.getElementById('itemType').value;
    const baseUnit = document.getElementById('itemBaseUnit').value;

    const url = editingItemId ? `${API_BASE}/items/${editingItemId}` : `${API_BASE}/items`;
    const method = editingItemId ? 'PUT' : 'POST';

    try {
        const res = await fetch(url, {
            method: method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                org_id: activeOrgId,
                name, sku, type, base_unit: baseUnit
            })
        });
        const data = await res.json();
        if (res.ok) {
            alert(editingItemId ? "✅ Đã cập nhật thành công!" : "✅ Đã tạo mới thành công!");
            document.getElementById('itemForm').reset();
            editingItemId = null;
            const submitBtn = document.getElementById('itemSubmitBtn');
            submitBtn.textContent = "Tạo mới";
            submitBtn.classList.replace('btn-accent', 'btn-primary');
            closeEditItem();
            loadItems();
        } else {
            alert("Lỗi: " + (data.error || JSON.stringify(data)));
        }
    } catch (err) {
        alert("Lỗi kết nối: " + err.message);
    }
});

document.getElementById('itemSearchInput')?.addEventListener('input', (e) => {
    renderItemGrid(e.target.value);
});

// Quick Add Item Modal
// Quick Add Item Modal
document.getElementById('quickCreateForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const name = document.getElementById('quickItemName').value;
    const type = document.getElementById('quickItemType').value;
    const unit = "cái"; // Default unit for quick add

    try {
        const res = await fetch(`${API_BASE}/items`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ org_id: activeOrgId, name, type, base_unit: unit })
        });
        const data = await res.json();
        if (res.ok) {
            await loadItems();
            if (currentQuickAddSelect) {
                currentQuickAddSelect.value = data.id;
                updateSOPPreview();
            }
            document.getElementById('quickCreateModal').classList.remove('active');
            document.getElementById('quickCreateForm').reset();
        } else {
            alert("Lỗi: " + (data.error || JSON.stringify(data)));
        }
    } catch (err) {
        alert("Lỗi kết nối: " + err.message);
    }
});

document.getElementById('cancelQuickCreate')?.addEventListener('click', () => {
    document.getElementById('quickCreateModal').classList.remove('active');
});

// Quick Add Station Modal
document.getElementById('stationTemplateSelect')?.addEventListener('change', (e) => {
    const val = e.target.value;
    const nameInput = document.getElementById('quickStationName');
    const unitInput = document.getElementById('quickStationUnit');
    const idInput = document.getElementById('quickStationId');
    const strategyInput = document.getElementById('quickStationStrategy');

    const templates = {
        'GRILL': { name: 'Bếp nướng / Áp chảo', unit: 'phần', id: 'bep_nuong', strategy: 'SLOT_ASYNC' },
        'FRYER': { name: 'Nồi chiên / Lò quay', unit: 'lít', id: 'noi_chien', strategy: 'BATCH_SYNC' },
        'OVEN': { name: 'Lò nướng khay', unit: 'khay', id: 'lo_nuong', strategy: 'BATCH_SYNC' },
        'MANUAL': { name: 'Sơ chế / Thao tác tay', unit: 'phần', id: 'thao_tac_tay', strategy: 'SLOT_ASYNC' },
        'MIXER': { name: 'Máy trộn bột', unit: 'mẻ', id: 'may_tron_bot', strategy: 'BATCH_SYNC' },
        'PROOFER': { name: 'Tủ ủ bột', unit: 'khay', id: 'tu_u_bot', strategy: 'SLOT_ASYNC' },
        'BAKE_OVEN': { name: 'Lò nướng bánh', unit: 'khay', id: 'lo_nuong_banh', strategy: 'BATCH_SYNC' }
    };

    if (templates[val]) {
        nameInput.value = templates[val].name;
        unitInput.value = templates[val].unit;
        idInput.value = templates[val].id;
        strategyInput.value = templates[val].strategy;
    }
});

document.getElementById('quickStationForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const id = document.getElementById('quickStationId').value || `st_${Date.now()}`;
    const name = document.getElementById('quickStationName').value;
    const unit = document.getElementById('quickStationUnit').value;
    const strategy = document.getElementById('quickStationStrategy').value;

    try {
        const res = await fetch(`${API_BASE}/station-types`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                id,
                name,
                capacity_unit: unit,
                default_strategy: strategy
            })
        });
        const data = await res.json();
        if (res.ok) {
            await loadStationTypes();
            if (currentQuickStationSelect) {
                currentQuickStationSelect.value = data.id;
                updateSOPPreview();
            }
            document.getElementById('quickStationModal').classList.remove('active');
            document.getElementById('quickStationForm').reset();
        } else {
            alert("Lỗi: " + (data.error || JSON.stringify(data)));
        }
    } catch (err) {
        alert("Lỗi kết nối: " + err.message);
    }
});

document.getElementById('cancelQuickStation')?.addEventListener('click', () => {
    document.getElementById('quickStationModal').classList.remove('active');
});

// Machine Registration
document.getElementById('machineForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const id = document.getElementById('machineId').value;
    const stationTypeId = document.getElementById('machineStationType').value;

    // Get strategy from the hidden input (inherited from Station Type)
    const strategy = document.getElementById('machineStrategy').value;
    const maxSlots = parseFloat(document.getElementById('machineCapacity').value);

    try {
        const res = await fetch(`${API_BASE}/machines`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                id,
                station_type_id: stationTypeId,
                node_id: activeNodeId,
                max_capacity: maxSlots,
                allocation_strategy: strategy
            })
        });
        if (res.ok) {
            alert("✨ Thiết bị đã được kích hoạt và sẵn sàng vận hành!");
            document.getElementById('machineForm').reset();
            // Reset capacity label
            document.getElementById('capacityUnitLabel').textContent = "(theo loại máy)";
            document.getElementById('machineCapacity').disabled = true;
            loadMachines();
        } else {
            const data = await res.json();
            alert("Lỗi: " + (data.error || JSON.stringify(data)));
        }
    } catch (err) {
        alert("Lỗi kết nối: " + err.message);
    }
});

// Sync Capacity Label and Strategy with Station Type selection
document.getElementById('machineStationType')?.addEventListener('change', (e) => {
    const stId = e.target.value;
    const st = globalStationTypes.find(s => s.id === stId);

    const unitLabel = document.getElementById('capacityUnitLabel');
    const capacityInput = document.getElementById('machineCapacity');
    const capacityHint = document.getElementById('capacityHint');

    const strategyInput = document.getElementById('machineStrategy');
    const strategyName = document.getElementById('inheritedStrategyName');
    const strategyIcon = document.getElementById('inheritedStrategyIcon');
    const strategyDesc = document.getElementById('inheritedStrategyDesc');

    if (st) {
        // Update Capacity UI
        unitLabel.textContent = `(${st.capacity_unit})`;
        capacityInput.disabled = false;
        capacityInput.placeholder = `Vd: 8 ${st.capacity_unit}`;
        capacityHint.textContent = `Vui lòng nhập số ${st.capacity_unit} tối đa máy này có thể xử lý.`;

        // Update Strategy UI
        const isBatch = st.default_strategy === 'BATCH_SYNC';
        strategyInput.value = st.default_strategy;
        strategyName.textContent = isBatch ? "Khóa theo mẻ (Batch Mode)" : "Nấu rảnh tay (Free-flow)";
        strategyIcon.textContent = isBatch ? "🔒" : "⏲️";
        strategyDesc.textContent = isBatch
            ? "Phù hợp cho Nồi chiên, Lò quay. Khóa máy khi đang nấu."
            : "Phù hợp cho Bếp nướng, Vỉ áp chảo. Vào/Ra tự do.";
    } else {
        unitLabel.textContent = "(theo loại máy)";
        capacityInput.disabled = true;
        capacityHint.textContent = "Chọn loại máy ở trên để xác định đơn vị đo.";
    }
});

// Production Order
document.getElementById('poForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const itemId = document.getElementById('poItemSelect').value;
    const nodeId = document.getElementById('poNodeSelect').value;
    const qty = parseFloat(document.getElementById('poTargetQty').value);

    const bomId = document.getElementById('poBomInfo').getAttribute('data-bom-id');

    if (!itemId || !nodeId || !qty || !bomId) {
        return alert("Vui lòng nhập đầy đủ thông tin Lệnh sản xuất!");
    }

    try {
        const res = await fetch(`${API_BASE}/production/orders`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                bom_id: bomId,
                node_id: activeNodeId,
                target_qty: qty
            })
        });
        if (res.ok) {
            alert("✨ Đã gửi lệnh sản xuất thành công!");
            document.getElementById('poForm').reset();
            loadProductionOrders();
        } else {
            const data = await res.json();
            alert("Lỗi: " + (data.error || JSON.stringify(data)));
        }
    } catch (err) {
        alert("Lỗi kết nối: " + err.message);
    }
});

// Helper: Show Result (Simple Alert wrapper or UI update)
function showResult(elementId, data, isError = false) {
    const el = document.getElementById(elementId);
    if (!el) {
        if (isError) console.error(data);
        return;
    }
    el.innerHTML = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
    el.className = 'result-box ' + (isError ? 'error' : 'success');
    el.style.display = 'block';
}

async function loadProductionOrders() {
    try {
        const url = activeNodeId ? `${API_BASE}/production/orders?node_id=${activeNodeId}` : `${API_BASE}/production/orders`;
        const res = await fetch(url);
        if (res.ok) {
            const orders = await res.json();
            renderOrdersList(orders);
        }
    } catch (err) {
        console.error("Failed to load orders", err);
    }
}

function renderOrdersList(orders) {
    const list = document.getElementById('activePOList');

    if (!list) return;
    list.innerHTML = "";

    const safeOrders = Array.isArray(orders) ? orders : [];

    if (safeOrders.length === 0) {
        list.innerHTML = '<div class="empty-state">Chưa có lệnh sản xuất nào.</div>';
        return;
    }

    [...safeOrders].sort((a, b) => new Date(b.created_at) - new Date(a.created_at)).forEach(order => {
        const item = globalItems.find(i => i.id === order.item_id);
        const node = globalNodes.find(n => n.id === order.node_id);
        const card = document.createElement('div');
        card.className = 'order-card';
        card.innerHTML = `
            <div class="order-header">
                <span class="order-id">#${order.id.slice(-6)}</span>
                <span class="status-pill status-${order.status.toLowerCase()}">${order.status}</span>
            </div>
            <div class="order-body">
                <h3>${item ? item.name : order.item_id}</h3>
                <p>Số lượng: <strong>${order.planned_qty} ${item ? item.base_unit : ''}</strong></p>
                <p>Bếp: <strong>${node ? node.name : order.node_id}</strong></p>
            </div>
            <div class="order-footer">
                <small>Bắt đầu: ${new Date(order.created_at).toLocaleString('vi-VN')}</small>
            </div>
        `;
        list.appendChild(card);
    });
}

async function setupSimulatedOrg() {
    const name = document.getElementById('orgNameInput').value.trim();
    if (!name) return alert("Vui lòng nhập tên chuỗi!");

    const statusEl = document.getElementById('orgSetupStatus');
    statusEl.innerHTML = "⏳ Đang khởi tạo chuỗi...";

    try {
        // 1. Create Org
        const orgRes = await fetch(`${API_BASE}/orgs`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: name })
        });
        const orgData = await orgRes.json();
        if (!orgRes.ok) throw new Error("Lỗi tạo Org: " + (orgData.error || JSON.stringify(orgData)));
        const orgId = orgData.id;

        statusEl.innerHTML += `<br>✅ Đã tạo Org: ${orgId}`;

        // 2. Create Nodes
        const nodesToCreate = [
            { name: "Trụ sở chính (HQ)", type: "HQ", address: "Quận 1, TP.HCM" },
            { name: "Bếp trung tâm (Factory)", type: "FACTORY", address: "Khu Công Nghiệp Bình Chiểu" },
            { name: "Cửa hàng #1 (Store 1)", type: "STORE", address: "Quận 7, TP.HCM" },
            { name: "Cửa hàng #2 (Store 2)", type: "STORE", address: "Quận Gò Vấp, TP.HCM" }
        ];

        for (const n of nodesToCreate) {
            const nodeRes = await fetch(`${API_BASE}/nodes`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    org_id: orgId,
                    name: n.name,
                    type: n.type,
                    address: n.address
                })
            });
            const nodeData = await nodeRes.json();
            if (nodeRes.ok) {
                statusEl.innerHTML += `<br>✅ Đã tạo ${n.type}: ${nodeData.id}`;
            } else {
                statusEl.innerHTML += `<br>❌ Lỗi tạo ${n.type}: ${JSON.stringify(nodeData)}`;
            }
        }

        statusEl.innerHTML += `<br>✨ **Hoàn tất thiết lập hệ thống!**`;
        await loadNodes(); // Refresh nodes list globally
        renderNodesList(); // Refresh local list
    } catch (err) {
        statusEl.innerHTML += `<br>❌ Lỗi: ${err.message}`;
    }
}

function renderNodesList() {
    const container = document.getElementById('nodesList');
    if (!container) return;

    if (globalNodes.length === 0) {
        container.innerHTML = '<div class="empty-state">Chưa có chi nhánh nào được tạo.</div>';
        return;
    }

    container.innerHTML = "";
    globalNodes.forEach(node => {
        const div = document.createElement('div');
        div.className = "list-row";
        div.style = "padding: 0.75rem; margin-bottom: 0.5rem; border: 1px solid rgba(255,255,255,0.1); border-radius: 8px;";
        div.innerHTML = `
            <div style="display: flex; justify-content: space-between; align-items: center;">
                <div>
                    <div style="font-weight: 600; color: #fff;">${node.name}</div>
                    <div style="font-size: 0.75rem; color: #9ca3af;">📍 ${node.address}</div>
                </div>
                <span class="type-badge" style="background: ${node.type === 'HQ' ? '#3b82f6' : (node.type === 'FACTORY' ? '#f59e0b' : '#10b981')}">
                    ${node.type}
                </span>
            </div>
        `;
        container.appendChild(div);
    });
}

// Window Exports
window.nextWizardStep = nextWizardStep;
window.prevWizardStep = prevWizardStep;
window.submitWizard = submitWizard;
window.addWizIngredientRow = addWizIngredientRow;
window.addWizStepRow = addWizStepRow;
window.editItem = editItem;
window.editBOMSOP = editBOMSOP;
window.prepareProductionOrder = prepareProductionOrder;
window.setupSimulatedOrg = setupSimulatedOrg;
window.switchNodeContext = switchNodeContext;
window.filterItemType = filterItemType;
window.openQuickAddRawMaterial = openQuickAddRawMaterial;
window.closeEditItem = closeEditItem;
window.openItemDetail = openItemDetail;
window.closeItemDetail = closeItemDetail;
window.openWizardModal = openWizardModal;
window.closeWizardModal = closeWizardModal;

function openWizardModal() {
    if (typeof resetWizard === 'function') resetWizard();
    const modal = document.getElementById('wizardModal');
    if (modal) modal.classList.add('active');
}

function closeWizardModal() {
    const modal = document.getElementById('wizardModal');
    if (modal) modal.classList.remove('active');
}

// ─── KDS State ────────────────────────────────────────────────────────────────
let kdsTasks = [];      // KDSBatchView[] from GET /kds/batches
let frontOrders = [];   // ProductionOrder[] from POST /production/orders
let pooledOrders = [];  // PooledOrderView[] from GET /kds/pool
let kdsMachines = [
    { id: 'M1_BEP_NUONG', name: 'Bếp Nướng #1', capacity: 8 },
    { id: 'M2_MAY_CHIEN_1', name: 'Máy Chiên #1', capacity: 2 },
    { id: 'M3_MAY_CHIEN_2', name: 'Máy Chiên #2', capacity: 2 },
    { id: 'M4_BAN_RAP', name: 'Bàn Ráp', capacity: 10 }
];
let kdsPollingInterval = null;
let poolPollingInterval = null;

// ─── Demo Data (BOM IDs must exist in your DB) ────────────────────────────────
// These match the SOPs defined in hamburger_peak_demo.go
const DEMO_ORDERS = [
    { bom_id: 'BOM_HAMBURGER_BO', _label: 'Hamburger Bò', target_qty: 2, delay: 0 },
    { bom_id: 'BOM_KHOAI_TAY_CHIEN', _label: 'Khoai Tây Chiên', target_qty: 1, delay: 0 },
    { bom_id: 'BOM_HAMBURGER_GA', _label: 'Hamburger Gà', target_qty: 1, delay: 5000 },
    { bom_id: 'BOM_BIT_TET', _label: 'Bò Bít Tết', target_qty: 1, delay: 30000 }
];

// ─── Toast Notifications ─────────────────────────────────────────────────────
function showToast(message, type = 'info') {
    const container = document.getElementById('toastContainer');
    if (!container) return;
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.innerHTML = `<span>${type === 'success' ? '✅' : type === 'warning' ? '⚠️' : '🔔'}</span> <span>${message}</span>`;
    container.appendChild(toast);
    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transform = 'translateX(120%)';
        setTimeout(() => toast.remove(), 300);
    }, 4000);
}

// ─── Peak Demo: Creates real POs via API ─────────────────────────────────────
async function runPeakDemo() {
    if (!activeNodeId) {
        showToast('Vui lòng chọn Node (Chi nhánh) trước!', 'warning');
        return;
    }
    showToast('⚡ Bắt đầu Peak Demo — Đơn hàng sẽ đến lần lượt...', 'info');
    frontOrders = [];
    updateDualView();

    for (const demo of DEMO_ORDERS) {
        await new Promise(r => setTimeout(r, demo.delay));
        try {
            const res = await fetch(`${API_BASE}/production/orders`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ bom_id: demo.bom_id, node_id: activeNodeId, target_qty: demo.target_qty || 1 })
            });
            if (res.ok) {
                const po = await res.json();
                // Attach UI label for display
                po._label = `${demo.target_qty}x ${demo._label}`;
                frontOrders.push(po);
                showToast(`🛒 Đơn mới: ${demo.target_qty}x ${demo._label} (#${po.id.slice(-6)})`, 'info');
                updateDualView();
                // Force an immediate refresh so UI updates instantly
                refreshPool();
                refreshKDSQueue();
            } else {
                const err = await res.json();
                showToast(`Lỗi tạo đơn ${demo._label}: ${err.error}`, 'warning');
            }
        } catch (e) {
            showToast(`Không kết nối được backend: ${e.message}`, 'warning');
        }
    }
}

let frontPollingInterval = null;

// ─── Polling ─────────────────────────────────────────────────────────────────
function startPolling() {
    if (!kdsPollingInterval) {
        kdsPollingInterval = setInterval(refreshKDSQueue, 1000);
    }
    if (!poolPollingInterval) {
        poolPollingInterval = setInterval(refreshPool, 1000);
    }
    if (!frontPollingInterval) {
        frontPollingInterval = setInterval(refreshFrontOrders, 1000);
    }
    refreshKDSQueue();
    refreshPool();
    refreshFrontOrders();
}

function stopPolling() {
    clearInterval(kdsPollingInterval); kdsPollingInterval = null;
    clearInterval(poolPollingInterval); poolPollingInterval = null;
    clearInterval(frontPollingInterval); frontPollingInterval = null;
}

async function refreshKDSQueue() {
    if (!activeNodeId) return;
    try {
        const res = await fetch(
            `${API_BASE}/kds/batches?node_id=${activeNodeId}&status=QUEUED&status=ALLOCATED&status=IN_PROGRESS`
        );
        if (res.ok) {
            kdsTasks = await res.json();
            renderKDSMachines();
            renderKDSTaskQueue();
        }
    } catch (_) { }
}

async function refreshFrontOrders() {
    if (!activeNodeId || frontOrders.length === 0) return;
    try {
        const res = await fetch(`${API_BASE}/production/orders?node_id=${activeNodeId}`);
        if (res.ok) {
            const allOrders = await res.json();
            frontOrders.forEach(fo => {
                const updated = allOrders.find(o => o.id === fo.id);
                if (updated) {
                    fo.status = updated.status;
                }
            });
            renderFrontOrders();
        }
    } catch (_) { }
}

async function refreshPool() {
    if (!activeNodeId) return;
    try {
        const res = await fetch(`${API_BASE}/kds/pool`);
        if (res.ok) {
            const data = await res.json();
            pooledOrders = data[activeNodeId] || [];
            renderPlanningQueue();
        }
    } catch (_) { }
}

// ─── Dual View Render ────────────────────────────────────────────────────────
function updateDualView() {
    renderFrontOrders();
    renderPlanningQueue();
    renderKDSMachines();
    renderKDSTaskQueue();
}

function renderFrontOrders() {
    const list = document.getElementById('frontOrderList');
    if (!list) return;

    if (frontOrders.length === 0) {
        list.innerHTML = '<div class="empty-state">Đang đợi khách...</div>';
        return;
    }

    list.innerHTML = frontOrders.map(order => {
        let progress = 0;
        let statusText = 'POOLED';
        let statusClass = '';

        if (order.status === 'COMPLETED') {
            progress = 100;
            statusText = 'READY';
            statusClass = 'status-allocated';
        } else if (order.status === 'IN_PROGRESS') {
            progress = 50;
            statusText = 'COOKING';
            statusClass = 'status-cooking';
        }

        const itemName = order._label || (globalItems && globalItems.find(i => i.id === order.item_id)?.name) || order.item_id || 'Unknown Item';
        const qty = order.target_qty || 1;

        return `
            <div class="front-order-card">
                <div class="front-order-header">
                    <span class="order-num">#${order.id.slice(-6)}</span>
                    <span class="status-badge ${statusClass}">${statusText}</span>
                </div>
                <div style="font-size:0.9rem;font-weight:600;color:#fff;margin:8px 0;display:flex;justify-content:space-between;align-items:center;">
                    <span>${itemName}</span>
                    <span style="background:var(--bg-dark);padding:2px 6px;border-radius:4px;color:var(--gold);font-size:0.8rem;">x${qty}</span>
                </div>
                <div class="progress-bar-bg" style="height:4px;background:rgba(255,255,255,0.05);">
                    <div class="progress-bar-fill" style="width:${progress}%;background:${progress === 100 ? 'var(--success)' : 'var(--primary)'}"></div>
                </div>
            </div>`;
    }).join('');
}

function renderPlanningQueue() {
    const list = document.getElementById('planningQueue');
    const badge = document.getElementById('poolCountBadge');
    if (!list) return;

    // Update pool count badge in pane header
    if (badge) {
        badge.textContent = pooledOrders.length > 0
            ? `🕐 Pool: ${pooledOrders.length} đơn đang gom`
            : '✅ Pool trống';
    }

    if (pooledOrders.length === 0) {
        list.innerHTML = '<div class="empty-state">Tất cả đã điều phối xuống bếp ✅</div>';
        return;
    }

    list.innerHTML = pooledOrders.map(entry => {
        const waited = entry.waited_seconds || 0;
        const minWin = entry.min_window_seconds || 8;
        const maxWait = entry.max_wait_seconds || 30;

        let urgencyColor, urgencyLabel, barWidth;

        if (waited < minWin) {
            // Phase 1: Batching window
            const secsLeft = minWin - waited;
            urgencyColor = 'var(--primary)';
            urgencyLabel = `⏳ Chờ gom chung (${secsLeft}s)`;
            barWidth = Math.round((waited / minWin) * 100);
        } else {
            // Phase 2: Waiting for a free machine, or force flush if taking too long
            const secsLeft = maxWait - waited;
            if (secsLeft <= 5) {
                urgencyColor = 'var(--accent)'; // Red
                urgencyLabel = `🔴 Ép đẩy Bếp! (${secsLeft}s)`;
                barWidth = 100;
            } else {
                urgencyColor = 'var(--gold)';
                urgencyLabel = `⚠️ Đợi Bếp rảnh (${secsLeft}s)`;
                barWidth = Math.round(((waited - minWin) / (maxWait - minWin)) * 100);
            }
        }

        const shortId = entry.po_id.slice(-6).toUpperCase();
        // Match front-order label by po_id if available
        const matched = frontOrders.find(o => o.id === entry.po_id);
        const itemLabel = matched ? (matched._label || matched.item_id || '') : '';

        return `
            <div class="pool-card">
                <div class="pool-card-header">
                    <div>
                        <span class="pool-order-id">#${shortId}</span>
                        ${itemLabel ? `<span class="pool-item-label">${itemLabel}</span>` : ''}
                    </div>
                    <span class="pool-urgency-badge" style="color:${urgencyColor};">${urgencyLabel}</span>
                </div>
                <div class="pool-countdown-bar-bg">
                    <div class="pool-countdown-bar-fill" style="width:${barWidth}%;background:${urgencyColor};"></div>
                </div>
                <div class="pool-meta">
                    <span style="font-size:0.65rem;color:var(--text-muted);">
                        Vào lúc ${new Date(entry.enqueued_at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                    </span>
                    <span style="font-size:0.65rem;color:var(--text-muted);">AI tự phân rã</span>
                </div>
            </div>`;
    }).join('');
}


function renderKDSMachines() {
    const strip = document.getElementById('kdsMachineStrip');
    const usageLabel = document.getElementById('machineUsage');
    if (!strip) return;

    strip.innerHTML = kdsMachines.map(m => {
        const activeTasks = kdsTasks.filter(t => t.machine_id === m.id && t.status !== 'COMPLETED');
        const slotsUsed = activeTasks.reduce((sum, t) => sum + (t.slots_used || 0), 0);
        return `
            <div class="machine-mini-card ${activeTasks.length > 0 ? 'busy' : ''}">
                <div style="font-size:0.8rem;font-weight:800;">${m.name}</div>
                <div style="font-size:0.6rem;color:var(--text-muted);">${slotsUsed}/${m.capacity} Slots</div>
            </div>`;
    }).join('');

    const busyCount = kdsMachines.filter(m => kdsTasks.some(t => t.machine_id === m.id && t.status === 'IN_PROGRESS')).length;
    if (usageLabel) usageLabel.textContent = `${busyCount}/3 Bận`;
}

function renderKDSTaskQueue() {
    const grid = document.getElementById('kdsTaskQueue');
    if (!grid) return;

    const active = kdsTasks.filter(t => t.status !== 'COMPLETED');
    if (active.length === 0) {
        grid.innerHTML = '<div class="empty-state">Bếp đang trống...</div>';
        return;
    }

    // Helper to get a normalized step name for grouping and display
    function getNormalizedStepName(stepName, itemId) {
        let nameToFormat = stepName;
        // Fallbacks for demo generic IDs if they lack descriptions
        if (stepName === "SAP_XEP_BO") nameToFormat = "Ráp Hamburger Bò";
        if (stepName === "SAP_XEP_GA") nameToFormat = "Ráp Hamburger Gà";
        if (stepName === "SAP_XEP_BIT_TET") nameToFormat = "Chuẩn bị Bít Tết";

        const verbs = ["Chiên", "Nướng", "Ráp", "Luộc", "Hấp", "Chuẩn bị", "Làm", "Bọc", "Sắp", "Xắp"];
        const words = nameToFormat.trim().split(/\s+/);

        let itemLabel = "";
        if (itemId) {
            itemLabel = itemId.replace('ITEM_', '').replace('HAMBURGER_', 'Hamburger ').replace('BIT_TET', 'Bít Tết').replace(/_/g, ' ');
            itemLabel = itemLabel.split(' ').map(w => {
                if (w.toLowerCase() === 'bít' || w.toLowerCase() === 'tết') return w; // keep accents
                return w.charAt(0).toUpperCase() + w.slice(1).toLowerCase();
            }).join(' ');
        }

        if (words.length > 0 && verbs.includes(words[0])) {
            let verb = words[0];
            if (verb === "Sắp" || verb === "Xắp") verb = "Sắp xếp";

            let rest = words.slice(verb === "Sắp xếp" ? 2 : 1).join(" ");

            // Append product name if the step is generic like "món"
            if ((!rest || rest.toLowerCase() === "món") && itemLabel) {
                rest = itemLabel;
            }

            return `${verb} ${rest}`.trim();
        }
        return nameToFormat.trim();
    }

    function formatKDSStepWithQty(normalizedName, qty) {
        const verbs = ["Chiên", "Nướng", "Ráp", "Luộc", "Hấp", "Chuẩn bị", "Làm", "Bọc", "Sắp", "Xắp"];
        const words = normalizedName.split(/\s+/);

        if (words.length > 0) {
            let verb = words[0];
            if (verb === "Sắp" && words[1] === "xếp") verb = "Sắp xếp";

            if (verbs.includes(verb) || verb === "Sắp xếp") {
                const rest = words.slice(verb === "Sắp xếp" ? 2 : 1).join(" ");
                return `${verb} <span class="highlight-qty">${qty} phần</span> ${rest}`.trim();
            }
        }
        return `${normalizedName} (<span class="highlight-qty">${qty} phần</span>)`;
    }

    // Group tasks that have the same machine, normalized_name, and status
    const grouped = [];
    active.forEach(t => {
        const normalizedName = getNormalizedStepName(t.step_name, t.item_id);
        const key = `${t.machine_id}_${normalizedName}_${t.status}`;
        let group = grouped.find(g => g.key === key);
        if (group) {
            group.po_ids.push(t.po_id.slice(-6).toUpperCase());
            group.batch_ids.push(t.id);
            group.total_qty += (t.qty || 1);
            // Sync elapsed/duration to the max across grouped items to avoid jitter
            group.elapsed = Math.max(group.elapsed, t.elapsed);
            group.duration = Math.max(group.duration, t.duration);
        } else {
            grouped.push({
                key,
                machine_id: t.machine_id,
                normalized_name: normalizedName,
                status: t.status,
                elapsed: t.elapsed,
                duration: t.duration,
                po_ids: [t.po_id.slice(-6).toUpperCase()],
                batch_ids: [t.id],
                total_qty: (t.qty || 1)
            });
        }
    });

    grid.innerHTML = grouped.map(group => {
        const progress = group.duration > 0 ? Math.min(100, (group.elapsed / group.duration) * 100) : 0;
        const timeLeft = Math.max(0, group.duration - group.elapsed);
        const isCooking = group.status === 'IN_PROGRESS';
        const isReady = isCooking && timeLeft === 0;
        const isAllocated = group.status === 'ALLOCATED';

        const bell = isAllocated ? '<span class="notif-bell bell-blue shake">🔔</span>'
            : isReady ? '<span class="notif-bell bell-red shake">🔔</span>'
                : '';

        const statusLabel = { QUEUED: 'QUEUED', ALLOCATED: 'ALLOCATED', IN_PROGRESS: 'COOKING' }[group.status] || group.status;
        const batchIdsStr = group.batch_ids.join(',');

        // Resolve human-readable machine name
        const mach = kdsMachines.find(m => m.id === group.machine_id);
        const machineDisplayName = mach ? mach.name : (group.machine_id || 'Chờ máy...');

        const stepDisplayHtml = formatKDSStepWithQty(group.normalized_name, group.total_qty);

        return `
            <div class="task-card priority-medium ${isReady ? 'ready-to-finish' : ''} ${isAllocated ? 'waiting-to-start' : ''}">
                <div class="task-header">
                    <div style="display:flex;align-items:center;gap:8px;">
                        ${bell}
                        <div class="po-pills-container">
                            ${group.po_ids.map(id => `<span class="po-pill">#${id}</span>`).join('')}
                        </div>
                    </div>
                    <span class="status-badge status-${group.status.toLowerCase()}">${statusLabel}</span>
                </div>
                <div class="task-body">
                    <h3 style="margin:0; line-height: 1.4;">${stepDisplayHtml}</h3>
                    <div style="display:flex; align-items:center; gap:6px; margin-top:8px;">
                        <span class="machine-badge">${machineDisplayName}</span>
                    </div>
                    ${isCooking ? `
                        <div class="cooking-progress" style="margin-top:12px;">
                            <div class="cooking-bar">
                                <div class="cooking-fill" style="width:${progress}%"></div>
                                <span class="cooking-time-left">${timeLeft}s</span>
                            </div>
                        </div>` : ''}
                </div>
                <div class="task-footer">
                    ${isAllocated ? `<button class="btn-start" onclick="startCooking('${batchIdsStr}')">▶ START</button>` : ''}
                    ${isCooking ? `<button class="btn-done${isReady ? ' ready' : ''}" onclick="finishTask('${batchIdsStr}')">
                                        ${isReady ? '✅ DONE' : '⏱ Nấu...'}
                                    </button>` : ''}
                    ${group.status === 'QUEUED' ? `<button class="btn-secondary" disabled>⏳ WAIT</button>` : ''}
                </div>
            </div>`;
    }).join('');
}

// ─── KDS Actions ─────────────────────────────────────────────────────────────
async function startCooking(batchIdsStr) {
    const ids = batchIdsStr.split(',');
    try {
        await Promise.all(ids.map(id => fetch(`${API_BASE}/kds/batches/${id}/confirm-placement`, { method: 'POST' })));
        await refreshKDSQueue();
        showToast('🍳 Bắt đầu làm!', 'success');
    } catch (e) {
        showToast('Lỗi kết nối backend', 'warning');
    }
}

async function finishTask(batchIdsStr) {
    const ids = batchIdsStr.split(',');
    try {
        await Promise.all(ids.map(id => fetch(`${API_BASE}/kds/batches/${id}/confirm-completion`, { method: 'POST' })));
        await refreshKDSQueue();
        await refreshPool();
        showToast('✅ Hoàn thành công đoạn!', 'success');
    } catch (e) {
        showToast('Lỗi kết nối backend', 'warning');
    }
}

window.runPeakDemo = runPeakDemo;
window.startCooking = startCooking;
window.finishTask = finishTask;
window.openPOModal = () => alert('Manual PO integration coming soon!');

// Final Initializations
loadInitialData();
