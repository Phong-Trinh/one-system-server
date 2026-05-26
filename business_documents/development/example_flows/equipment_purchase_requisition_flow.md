#####################

 Ví dụ: Quy trình mua sắm thiết bị tài sản (CapEx Procurement Flow) cho lò nướng Pizza mới (từ PR tới Machine)

Bối cảnh:
- Store S1 muốn bổ sung một lò nướng Pizza Công nghiệp (Industrial Pizza Oven) để bán Pizza cuối tuần.
- Đây là thiết bị mới chưa từng đăng ký trong hệ thống catalog của chuỗi.
- Lò nướng này là tài sản dài hạn (CapEx) nên bắt buộc phải tạo Purchase Requisition (PR) gửi lên HQ phê duyệt chứ không qua hệ thống RO châm hàng tự động.

Quy trình các bước (List System Steps):

  [1] Store Manager gửi yêu cầu PR cho lò nướng Pizza mới.
        - Tên thiết bị đề xuất: "Industrial Pizza Oven"
        - Expected capacity: 2.0 (capacity_unit là "tray")
        - Estimated price: 3,000 USD
        - UX Optimization: Store Manager nhập tên thiết bị, hệ thống tự động sinh ID kỹ thuật `eq_pizza_oven` ở background.

  [2] Hệ thống tạo danh mục DRAFT cho thiết bị mới.
        - Vì `eq_pizza_oven` chưa có trong catalog hệ thống, hệ thống tự động thêm mới EquipmentType này với trạng thái `DRAFT`.
        - PR được gửi lên HQ với trạng thái `PENDING_HQ_APPROVAL`.

  [3] HQ xem xét PR và phê duyệt (Approve PR).
        - Admin HQ bấm phê duyệt PR.
        - Trạng thái PR chuyển thành `APPROVED`.
        - Hệ thống tự động kích hoạt EquipmentType `eq_pizza_oven` từ trạng thái `DRAFT` chuyển sang `ACTIVE` trên toàn chuỗi.

  [4] HQ đặt hàng lần 1 từ Supplier A.
        - HQ chọn Supplier A (Lousy Logistics) và tạo đơn mua hàng **`PurchaseOrder` (PurO #1)** liên kết với PR của S1.
        - Giá thương lượng: 2,900 USD.
        - Trạng thái PurO #1 tự động là `CONFIRMED`.

  [5] Supplier A giao hàng nhưng bị hỏng hoàn toàn tại Store.
        - Supplier A vận chuyển hàng (PurO chuyển trạng thái `SHIPPED`).
        - Xe giao tới S1. Store Manager mở thùng kiểm tra phát hiện lò bị hỏng hoàn toàn.
        - Store Manager tạo phiếu nhận hàng **`GoodsReceipt` (GR #1)** ghi nhận số lượng thực tế nhận được là **0**.
        - Hệ thống chuyển trạng thái GR #1 sang `DISCREPANCY`, đồng thời tự động sinh một **`DiscrepancyTicket`** trạng thái `OPEN` để báo cáo HQ. Tài sản bị khóa không được đưa vào bếp.

  [6] HQ hủy đơn Supplier A và Reset PR để pivot sang Supplier B.
        - Nhận thấy lỗi từ Supplier A, HQ chuyển trạng thái PurO #1 thành `CANCELLED`.
        - Hệ thống tự động khôi phục trạng thái PR ban đầu của S1 về lại `APPROVED` để tiếp tục chuyển đổi nhà cung cấp.

  [7] HQ đặt hàng lần 2 từ Supplier B.
        - HQ tạo đơn mua hàng mới **`PurchaseOrder` (PurO #2)** liên kết với PR đó gửi đến Supplier B (Premium Logistics).
        - Giá mua: 2,900 USD.

  [8] Supplier B giao hàng thành công.
        - Supplier B giao lò nướng Pizza nguyên vẹn tới S1.
        - Store Manager kiểm nhận, tạo phiếu **`GoodsReceipt` (GR #2)** với số lượng nhận thực tế là **1**.
        - Trạng thái GR #2 chuyển thành `CONFIRMED`.

  [9] HQ đối soát tài chính 3 bên (3-Way Matching).
        - Supplier B gửi hóa đơn (Supplier Invoice) trị giá 2,900 USD cho HQ.
        - Kế toán HQ kích hoạt đối soát 3 bên: PurO #2 (2,900 USD) = GR #2 (1 chiếc lò) = Invoice (2,900 USD).
        - Đối soát khớp 100%, trạng thái Invoice chuyển thành `MATCHED`, PurO #2 chuyển thành `COMPLETED`.

  [10] Hệ thống ghi nhận Sổ cái và sinh tài sản.
        - Hệ thống tự động ghi nhận một giao dịch chi phí **`Expense Transaction`** trị giá 2,900 USD tại chi nhánh S1 trên General Ledger.
        - Hệ thống tự động tạo bản ghi tài sản **`Asset`** tương ứng ở trạng thái `PENDING_REGISTRATION`.

  [11] Đăng ký lò Pizza đưa vào bếp vận hành.
        - Store Manager tiến hành đăng ký lò nướng Pizza vào bếp với mã thiết bị **`Machine ID: M_PIZZA_OVEN_01`** (max_capacity = 2.0).
        - Trạng thái bản ghi `Asset` chuyển thành `IN_USE`.
        - Lò nướng Pizza trên Kitchen KDS chuyển sang trạng thái **`IDLE`** (sẵn sàng phân phối order làm bánh).

  [12] Đồng bộ trạng thái khi bảo trì (Maintenance Sync).
        - Thiết bị gặp sự cố, Quản lý Store set trạng thái Asset thành `UNDER_MAINTENANCE`.
        - Hệ thống bếp tự động đồng bộ trạng thái của lò Pizza `M_PIZZA_OVEN_01` sang **`UNDER_MAINTENANCE`** (ngưng nhận order làm bánh).
        - Sau khi sửa chữa xong, Quản lý set Asset thành `IN_USE`, trạng thái lò Pizza tự động đồng bộ về lại **`IDLE`** (tiếp tục nhận order làm bánh).