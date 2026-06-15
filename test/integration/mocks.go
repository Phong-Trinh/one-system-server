package integration

import (
	"context"
	"fmt"
	"time"

	"one-system-server/internal/domain/models"
	"one-system-server/internal/domain/services"
	"one-system-server/internal/usecase"
)

// ─── MOCK REPOSITORIES ────────────────────────────────────────────────────────

type mockPRRepo struct {
	prs map[string]*models.PurchaseRequisition
}

func (m *mockPRRepo) Create(ctx context.Context, pr *models.PurchaseRequisition) error {
	m.prs[pr.ID] = pr
	return nil
}
func (m *mockPRRepo) FindByID(ctx context.Context, id string) (*models.PurchaseRequisition, error) {
	return m.prs[id], nil
}
func (m *mockPRRepo) FindByNode(ctx context.Context, nodeID string) ([]*models.PurchaseRequisition, error) {
	var res []*models.PurchaseRequisition
	for _, pr := range m.prs {
		if pr.RequesterNodeID == nodeID {
			res = append(res, pr)
		}
	}
	return res, nil
}
func (m *mockPRRepo) FindPendingByOrg(ctx context.Context, orgID string) ([]*models.PurchaseRequisition, error) {
	var res []*models.PurchaseRequisition
	for _, pr := range m.prs {
		if pr.OrgID == orgID && pr.Status == models.PRPendingHQApproval {
			res = append(res, pr)
		}
	}
	return res, nil
}
func (m *mockPRRepo) Update(ctx context.Context, pr *models.PurchaseRequisition) error {
	m.prs[pr.ID] = pr
	return nil
}

type mockPRLineRepo struct {
	lines map[string][]*models.PRLine
}

func (m *mockPRLineRepo) AddLine(ctx context.Context, line *models.PRLine) error {
	m.lines[line.PRID] = append(m.lines[line.PRID], line)
	return nil
}
func (m *mockPRLineRepo) ListByPR(ctx context.Context, prID string) ([]*models.PRLine, error) {
	return m.lines[prID], nil
}
func (m *mockPRLineRepo) UpdateLine(ctx context.Context, line *models.PRLine) error {
	for _, lines := range m.lines {
		for i, l := range lines {
			if l.ID == line.ID {
				lines[i] = line
				return nil
			}
		}
	}
	return fmt.Errorf("mockPRLineRepo.UpdateLine: line %s not found", line.ID)
}


type mockPurORepo struct {
	puros map[string]*models.PurchaseOrder
}

func (m *mockPurORepo) Create(ctx context.Context, po *models.PurchaseOrder) error {
	m.puros[po.ID] = po
	return nil
}
func (m *mockPurORepo) FindByID(ctx context.Context, id string) (*models.PurchaseOrder, error) {
	return m.puros[id], nil
}
func (m *mockPurORepo) FindByStatus(ctx context.Context, orgID string, status models.PurchaseOrderStatus) ([]*models.PurchaseOrder, error) {
	var res []*models.PurchaseOrder
	for _, po := range m.puros {
		if po.OrgID == orgID && po.Status == status {
			res = append(res, po)
		}
	}
	return res, nil
}
func (m *mockPurORepo) FindByDeliveryNode(ctx context.Context, nodeID string) ([]*models.PurchaseOrder, error) {
	var res []*models.PurchaseOrder
	for _, po := range m.puros {
		if po.DeliveryToNodeID == nodeID {
			res = append(res, po)
		}
	}
	return res, nil
}
func (m *mockPurORepo) Update(ctx context.Context, po *models.PurchaseOrder) error {
	m.puros[po.ID] = po
	return nil
}

type mockPurOLineRepo struct {
	lines map[string][]*models.PurchaseOrderLine
}

func (m *mockPurOLineRepo) AddLine(ctx context.Context, line *models.PurchaseOrderLine) error {
	m.lines[line.PurOID] = append(m.lines[line.PurOID], line)
	return nil
}
func (m *mockPurOLineRepo) ListByPurO(ctx context.Context, purOID string) ([]*models.PurchaseOrderLine, error) {
	return m.lines[purOID], nil
}
func (m *mockPurOLineRepo) DeleteLine(ctx context.Context, id string) error {
	for purID, lines := range m.lines {
		for i, l := range lines {
			if l.ID == id {
				m.lines[purID] = append(lines[:i], lines[i+1:]...)
				return nil
			}
		}
	}
	return nil
}
func (m *mockPurOLineRepo) GetHistoricalPrice(ctx context.Context, supplierID string, itemID *string, equipmentTypeID *string) (float64, error) {
	// Mock returns 0
	return 0, nil
}


type mockSupplierRepo struct {
	suppliers map[string]*models.Supplier
}

func (m *mockSupplierRepo) Create(ctx context.Context, supplier *models.Supplier) error {
	m.suppliers[supplier.ID] = supplier
	return nil
}

func (m *mockSupplierRepo) FindByID(ctx context.Context, id string) (*models.Supplier, error) {
	return m.suppliers[id], nil
}

func (m *mockSupplierRepo) FindByOrg(ctx context.Context, orgID string) ([]*models.Supplier, error) {
	var res []*models.Supplier
	for _, s := range m.suppliers {
		if s.OrgID == orgID {
			res = append(res, s)
		}
	}
	return res, nil
}

// Goods Issue/Receipt stubs
type mockGIRepo struct {
	gis map[string]*models.GoodsIssue
}

func (m *mockGIRepo) Create(ctx context.Context, gi *models.GoodsIssue) error {
	m.gis[gi.ID] = gi
	return nil
}

func (m *mockGIRepo) FindByID(ctx context.Context, id string) (*models.GoodsIssue, error) {
	return m.gis[id], nil
}

func (m *mockGIRepo) UpdateStatus(ctx context.Context, id string, status models.GoodsIssueStatus) error {
	if gi, ok := m.gis[id]; ok {
		gi.Status = status
	}
	return nil
}

type mockGRRepo struct {
	grs map[string]*models.GoodsReceipt
}

func (m *mockGRRepo) Create(ctx context.Context, gr *models.GoodsReceipt) error {
	m.grs[gr.ID] = gr
	return nil
}
func (m *mockGRRepo) FindByID(ctx context.Context, id string) (*models.GoodsReceipt, error) {
	return m.grs[id], nil
}
func (m *mockGRRepo) UpdateStatus(ctx context.Context, id string, status models.GoodsReceiptStatus) error {
	if gr, ok := m.grs[id]; ok {
		gr.Status = status
		return nil
	}
	return fmt.Errorf("gr not found")
}
func (m *mockGRRepo) FindByRef(ctx context.Context, refType models.GoodsReceiptRefType, refID string) ([]*models.GoodsReceipt, error) {
	var res []*models.GoodsReceipt
	for _, gr := range m.grs {
		if gr.RefType == refType && gr.RefID == refID {
			res = append(res, gr)
		}
	}
	return res, nil
}

type mockGRLineRepo struct {
	lines map[string][]*models.GoodsReceiptLine
}

func (m *mockGRLineRepo) AddLine(ctx context.Context, line *models.GoodsReceiptLine) error {
	m.lines[line.GRID] = append(m.lines[line.GRID], line)
	return nil
}
func (m *mockGRLineRepo) ListByGR(ctx context.Context, grID string) ([]*models.GoodsReceiptLine, error) {
	return m.lines[grID], nil
}

type mockDiscrepancyRepo struct {
	tickets map[string]*models.DiscrepancyTicket
}

func (m *mockDiscrepancyRepo) Create(ctx context.Context, dt *models.DiscrepancyTicket) error {
	m.tickets[dt.ID] = dt
	return nil
}
func (m *mockDiscrepancyRepo) FindByID(ctx context.Context, id string) (*models.DiscrepancyTicket, error) {
	return m.tickets[id], nil
}
func (m *mockDiscrepancyRepo) FindByGR(ctx context.Context, grID string) ([]*models.DiscrepancyTicket, error) {
	var res []*models.DiscrepancyTicket
	for _, dt := range m.tickets {
		if dt.GRID == grID {
			res = append(res, dt)
		}
	}
	return res, nil
}
func (m *mockDiscrepancyRepo) UpdateStatus(ctx context.Context, id string, status models.DiscrepancyTicketStatus, resolution *string, resolvedBy *string) error {
	if dt, ok := m.tickets[id]; ok {
		dt.Status = status
		dt.Resolution = resolution
		dt.ResolvedBy = resolvedBy
		now := time.Now()
		dt.ResolvedAt = &now
		return nil
	}
	return fmt.Errorf("ticket not found")
}

type mockInvoiceRepo struct {
	invoices map[string]*models.SupplierInvoice
}

func (m *mockInvoiceRepo) Create(ctx context.Context, inv *models.SupplierInvoice) error {
	m.invoices[inv.ID] = inv
	return nil
}
func (m *mockInvoiceRepo) FindByID(ctx context.Context, id string) (*models.SupplierInvoice, error) {
	return m.invoices[id], nil
}
func (m *mockInvoiceRepo) FindByPurO(ctx context.Context, purOID string) ([]*models.SupplierInvoice, error) {
	var res []*models.SupplierInvoice
	for _, inv := range m.invoices {
		if inv.PurchaseOrderID == purOID {
			res = append(res, inv)
		}
	}
	return res, nil
}
func (m *mockInvoiceRepo) Update(ctx context.Context, inv *models.SupplierInvoice) error {
	m.invoices[inv.ID] = inv
	return nil
}

type mockInvoiceLineRepo struct {
	lines map[string][]*models.SupplierInvoiceLine
}

func (m *mockInvoiceLineRepo) AddLine(ctx context.Context, line *models.SupplierInvoiceLine) error {
	m.lines[line.InvoiceID] = append(m.lines[line.InvoiceID], line)
	return nil
}
func (m *mockInvoiceLineRepo) ListByInvoice(ctx context.Context, invoiceID string) ([]*models.SupplierInvoiceLine, error) {
	return m.lines[invoiceID], nil
}

type mockTxRepo struct {
	txs map[string]*models.Transaction
}

func (m *mockTxRepo) Create(ctx context.Context, tx *models.Transaction) error {
	m.txs[tx.ID] = tx
	return nil
}
func (m *mockTxRepo) FindByID(ctx context.Context, id string) (*models.Transaction, error) {
	return m.txs[id], nil
}
func (m *mockTxRepo) ListByNode(ctx context.Context, nodeID string, txType *models.TransactionType) ([]*models.Transaction, error) {
	var res []*models.Transaction
	for _, tx := range m.txs {
		if tx.NodeID == nodeID && (txType == nil || tx.Type == *txType) {
			res = append(res, tx)
		}
	}
	return res, nil
}
func (m *mockTxRepo) ListByRef(ctx context.Context, refType models.TransactionRefType, refID string) ([]*models.Transaction, error) {
	var res []*models.Transaction
	for _, tx := range m.txs {
		if tx.RefType == refType && tx.ReferenceID == refID {
			res = append(res, tx)
		}
	}
	return res, nil
}

type mockAssetRepo struct {
	assets map[string]*models.Asset
}

func (m *mockAssetRepo) Create(ctx context.Context, asset *models.Asset) error {
	m.assets[asset.ID] = asset
	return nil
}
func (m *mockAssetRepo) FindByID(ctx context.Context, id string) (*models.Asset, error) {
	return m.assets[id], nil
}
func (m *mockAssetRepo) FindByNode(ctx context.Context, nodeID string) ([]*models.Asset, error) {
	var res []*models.Asset
	for _, a := range m.assets {
		if a.NodeID == nodeID {
			res = append(res, a)
		}
	}
	return res, nil
}
func (m *mockAssetRepo) FindByPurO(ctx context.Context, purOID string) (*models.Asset, error) {
	for _, a := range m.assets {
		if a.LinkedPurOID == purOID {
			return a, nil
		}
	}
	return nil, nil
}
func (m *mockAssetRepo) Update(ctx context.Context, asset *models.Asset) error {
	m.assets[asset.ID] = asset
	return nil
}

type mockMachineRepo struct {
	machines map[string]*models.Machine
}

func (m *mockMachineRepo) Create(ctx context.Context, mch *models.Machine) error {
	m.machines[mch.ID] = mch
	return nil
}
func (m *mockMachineRepo) FindByID(ctx context.Context, id string) (*models.Machine, error) {
	return m.machines[id], nil
}
func (m *mockMachineRepo) FindByNodeID(ctx context.Context, nodeID string) ([]*models.Machine, error) {
	var res []*models.Machine
	for _, mch := range m.machines {
		if mch.NodeID == nodeID {
			res = append(res, mch)
		}
	}
	return res, nil
}
func (m *mockMachineRepo) FindAll(ctx context.Context) ([]*models.Machine, error) {
	var res []*models.Machine
	for _, mch := range m.machines {
		res = append(res, mch)
	}
	return res, nil
}
func (m *mockMachineRepo) FindIdleByStationType(ctx context.Context, nodeID, stationTypeID string) ([]*models.Machine, error) {
	var res []*models.Machine
	for _, mch := range m.machines {
		if mch.NodeID == nodeID && mch.EquipmentTypeID == stationTypeID && mch.Status == models.MachineIdle {
			res = append(res, mch)
		}
	}
	return res, nil
}
func (m *mockMachineRepo) UpdateStatus(ctx context.Context, id string, status models.MachineStatus, batchID *string) error {
	if mch, ok := m.machines[id]; ok {
		mch.Status = status
		mch.CurrentBatchID = batchID
		return nil
	}
	return fmt.Errorf("machine not found")
}
func (m *mockMachineRepo) Update(ctx context.Context, mch *models.Machine) error {
	m.machines[mch.ID] = mch
	return nil
}
func (m *mockMachineRepo) Delete(ctx context.Context, id string) error {
	delete(m.machines, id)
	return nil
}

type mockEquipmentTypeRepo struct {
	types map[string]*models.EquipmentType
}

func (m *mockEquipmentTypeRepo) Create(ctx context.Context, st *models.EquipmentType) error {
	m.types[st.ID] = st
	return nil
}
func (m *mockEquipmentTypeRepo) FindByID(ctx context.Context, id string) (*models.EquipmentType, error) {
	return m.types[id], nil
}
func (m *mockEquipmentTypeRepo) FindAll(ctx context.Context) ([]*models.EquipmentType, error) {
	var res []*models.EquipmentType
	for _, t := range m.types {
		res = append(res, t)
	}
	return res, nil
}
func (m *mockEquipmentTypeRepo) Update(ctx context.Context, st *models.EquipmentType) error {
	m.types[st.ID] = st
	return nil
}
func (m *mockEquipmentTypeRepo) Delete(ctx context.Context, id string) error {
	delete(m.types, id)
	return nil
}

type mockNodeRepo struct {
	nodes map[string]*models.Node
}

func (m *mockNodeRepo) Create(ctx context.Context, node *models.Node) error {
	m.nodes[node.ID] = node
	return nil
}
func (m *mockNodeRepo) FindByID(ctx context.Context, id string) (*models.Node, error) {
	return m.nodes[id], nil
}
func (m *mockNodeRepo) FindByOrgID(ctx context.Context, orgID string) ([]*models.Node, error) {
	var res []*models.Node
	for _, n := range m.nodes {
		if n.OrgID == orgID {
			res = append(res, n)
		}
	}
	return res, nil
}
func (m *mockNodeRepo) FindAll(ctx context.Context) ([]*models.Node, error) {
	var res []*models.Node
	for _, n := range m.nodes {
		res = append(res, n)
	}
	return res, nil
}
func (m *mockNodeRepo) Update(ctx context.Context, node *models.Node) error {
	m.nodes[node.ID] = node
	return nil
}
func (m *mockNodeRepo) Delete(ctx context.Context, id string) error {
	delete(m.nodes, id)
	return nil
}

type mockNodeStockRepo struct {
	stock map[string]*models.NodeStock
}
func (m *mockNodeStockRepo) Get(ctx context.Context, nodeID, itemID string) (*models.NodeStock, error) {
	return m.stock[nodeID+"_"+itemID], nil
}
func (m *mockNodeStockRepo) ListByNode(ctx context.Context, nodeID string) ([]*models.NodeStock, error) {
	var res []*models.NodeStock
	for _, s := range m.stock {
		if s.NodeID == nodeID {
			res = append(res, s)
		}
	}
	return res, nil
}
func (m *mockNodeStockRepo) Upsert(ctx context.Context, stock *models.NodeStock) error {
	m.stock[stock.NodeID+"_"+stock.ItemID] = stock
	return nil
}

type mockNodeItemConfigRepo struct {
	configs map[string]*models.NodeItemConfig
}
func (m *mockNodeItemConfigRepo) Get(ctx context.Context, nodeID, itemID string) (*models.NodeItemConfig, error) {
	return m.configs[nodeID+"_"+itemID], nil
}
func (m *mockNodeItemConfigRepo) ListByNode(ctx context.Context, nodeID string) ([]*models.NodeItemConfig, error) {
	var res []*models.NodeItemConfig
	for _, c := range m.configs {
		if c.NodeID == nodeID {
			res = append(res, c)
		}
	}
	return res, nil
}
func (m *mockNodeItemConfigRepo) Upsert(ctx context.Context, cfg *models.NodeItemConfig) error {
	m.configs[cfg.NodeID+"_"+cfg.ItemID] = cfg
	return nil
}

type mockITORepo struct {
	itos map[string]*models.InternalTransferOrder
}
func (m *mockITORepo) Create(ctx context.Context, ito *models.InternalTransferOrder) error {
	m.itos[ito.ID] = ito
	return nil
}
func (m *mockITORepo) FindByID(ctx context.Context, id string) (*models.InternalTransferOrder, error) {
	return m.itos[id], nil
}
func (m *mockITORepo) FindByNode(ctx context.Context, nodeID string) ([]*models.InternalTransferOrder, error) {
	var res []*models.InternalTransferOrder
	for _, ito := range m.itos {
		if ito.RequesterNodeID == nodeID || ito.ProviderNodeID == nodeID {
			res = append(res, ito)
		}
	}
	return res, nil
}
func (m *mockITORepo) UpdateStatus(ctx context.Context, id string, status models.ITOStatus) error {
	if ito, ok := m.itos[id]; ok {
		ito.Status = status
		return nil
	}
	return fmt.Errorf("ito not found")
}

type mockITOLineRepo struct {
	lines map[string][]*models.ITOLine
}
func (m *mockITOLineRepo) AddLine(ctx context.Context, line *models.ITOLine) error {
	m.lines[line.ITOID] = append(m.lines[line.ITOID], line)
	return nil
}
func (m *mockITOLineRepo) ListByITO(ctx context.Context, itoID string) ([]*models.ITOLine, error) {
	return m.lines[itoID], nil
}


type mockGILineRepo struct {
	lines map[string][]*models.GoodsIssueLine
}
func (m *mockGILineRepo) AddLine(ctx context.Context, line *models.GoodsIssueLine) error {
	m.lines[line.GIID] = append(m.lines[line.GIID], line)
	return nil
}
func (m *mockGILineRepo) ListByGI(ctx context.Context, giID string) ([]*models.GoodsIssueLine, error) {
	return m.lines[giID], nil
}

// B2B stubs
type mockB2BOrderRepo struct {
	orders map[string]*models.B2BSalesOrder
}
func (m *mockB2BOrderRepo) Create(ctx context.Context, order *models.B2BSalesOrder) error { 
	m.orders[order.ID] = order
	return nil
}
func (m *mockB2BOrderRepo) FindByID(ctx context.Context, id string) (*models.B2BSalesOrder, error) { 
	return m.orders[id], nil
}
func (m *mockB2BOrderRepo) FindByFactory(ctx context.Context, factoryNodeID string) ([]*models.B2BSalesOrder, error) { 
	var res []*models.B2BSalesOrder
	for _, o := range m.orders {
		if o.FactoryNodeID == factoryNodeID {
			res = append(res, o)
		}
	}
	return res, nil 
}
func (m *mockB2BOrderRepo) Update(ctx context.Context, order *models.B2BSalesOrder) error { 
	m.orders[order.ID] = order
	return nil 
}

type mockB2BOrderLineRepo struct {
	lines map[string][]*models.B2BSalesOrderLine
}
func (m *mockB2BOrderLineRepo) AddLine(ctx context.Context, line *models.B2BSalesOrderLine) error { 
	m.lines[line.OrderID] = append(m.lines[line.OrderID], line)
	return nil 
}
func (m *mockB2BOrderLineRepo) ListByOrder(ctx context.Context, orderID string) ([]*models.B2BSalesOrderLine, error) { 
	return m.lines[orderID], nil 
}

// testContext holds the facade and all repos for easy access during tests
type testContext struct {
	Facade        *usecase.SupplyChainFacade
	PRRepo        services.PurchaseRequisitionRepository
	PRLineRepo    services.PRLineRepository
	PurORepo      services.PurchaseOrderRepository
	PurOLineRepo  services.PurchaseOrderLineRepository
	SupplierRepo  services.SupplierRepository
	GRRepo        services.GoodsReceiptRepository
	GRLineRepo    services.GoodsReceiptLineRepository
	GIRepo        services.GoodsIssueRepository
	GILineRepo    services.GoodsIssueLineRepository
	DTRepo        services.DiscrepancyTicketRepository
	InvoiceRepo   services.SupplierInvoiceRepository
	InvoiceLineRepo services.SupplierInvoiceLineRepository
	TxRepo        services.TransactionRepository
	AssetRepo     services.AssetRepository
	MachineRepo   services.MachineRepository
	EqTypeRepo    services.EquipmentTypeRepository
	NodeRepo      services.NodeRepository
	NodeStockRepo services.NodeStockRepository
	NodeItemConfigRepo services.NodeItemConfigRepository
	ITORepo       services.InternalTransferOrderRepository
	ITOLineRepo   services.ITOLineRepository
}

func setupTestFacade() *testContext {
	tc := &testContext{
		PRRepo:        &mockPRRepo{prs: make(map[string]*models.PurchaseRequisition)},
		PRLineRepo:    &mockPRLineRepo{lines: make(map[string][]*models.PRLine)},
		PurORepo:      &mockPurORepo{puros: make(map[string]*models.PurchaseOrder)},
		PurOLineRepo:  &mockPurOLineRepo{lines: make(map[string][]*models.PurchaseOrderLine)},
		SupplierRepo:  &mockSupplierRepo{suppliers: make(map[string]*models.Supplier)},
		GRRepo:        &mockGRRepo{grs: make(map[string]*models.GoodsReceipt)},
		GRLineRepo:    &mockGRLineRepo{lines: make(map[string][]*models.GoodsReceiptLine)},
		GIRepo:        &mockGIRepo{gis: make(map[string]*models.GoodsIssue)},
		GILineRepo:    &mockGILineRepo{lines: make(map[string][]*models.GoodsIssueLine)},
		DTRepo:        &mockDiscrepancyRepo{tickets: make(map[string]*models.DiscrepancyTicket)},
		InvoiceRepo:   &mockInvoiceRepo{invoices: make(map[string]*models.SupplierInvoice)},
		InvoiceLineRepo: &mockInvoiceLineRepo{lines: make(map[string][]*models.SupplierInvoiceLine)},
		TxRepo:        &mockTxRepo{txs: make(map[string]*models.Transaction)},
		AssetRepo:     &mockAssetRepo{assets: make(map[string]*models.Asset)},
		MachineRepo:   &mockMachineRepo{machines: make(map[string]*models.Machine)},
		EqTypeRepo:    &mockEquipmentTypeRepo{types: make(map[string]*models.EquipmentType)},
		NodeRepo:      &mockNodeRepo{nodes: make(map[string]*models.Node)},
		NodeStockRepo: &mockNodeStockRepo{stock: make(map[string]*models.NodeStock)},
		NodeItemConfigRepo: &mockNodeItemConfigRepo{configs: make(map[string]*models.NodeItemConfig)},
		ITORepo:       &mockITORepo{itos: make(map[string]*models.InternalTransferOrder)},
		ITOLineRepo:   &mockITOLineRepo{lines: make(map[string][]*models.ITOLine)},
	}

	repos := usecase.SupplyChainRepos{
		Stock: tc.NodeStockRepo,
		Config: tc.NodeItemConfigRepo,
		Supplier: tc.SupplierRepo,
		ITO: tc.ITORepo,
		ITOLine: tc.ITOLineRepo,
		PR: tc.PRRepo,
		PRLine: tc.PRLineRepo,
		PurO: tc.PurORepo,
		PurOLine: tc.PurOLineRepo,
		GI: tc.GIRepo,
		GILine: tc.GILineRepo,
		GR: tc.GRRepo,
		GRLine: tc.GRLineRepo,
		DT: tc.DTRepo,
		Invoice: tc.InvoiceRepo,
		InvoiceLine: tc.InvoiceLineRepo,
		Transaction: tc.TxRepo,
		B2BOrder: &mockB2BOrderRepo{orders: make(map[string]*models.B2BSalesOrder)},
		B2BOrderLine: &mockB2BOrderLineRepo{lines: make(map[string][]*models.B2BSalesOrderLine)},
		Asset: tc.AssetRepo,
		Machine: tc.MachineRepo,
		Node: tc.NodeRepo,
		EquipmentType: tc.EqTypeRepo,
	}

	tc.Facade = usecase.NewSupplyChainFacade(repos)
	return tc
}
