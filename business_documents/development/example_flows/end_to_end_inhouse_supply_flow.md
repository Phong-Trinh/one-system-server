#####################
Ví dụ: Quy trình chuỗi cung ứng toàn diện (End-to-End) cho một Doanh nghiệp có đủ cả HQ, Factory và Store In-house.

Bối cảnh Doanh nghiệp:
- Chuỗi "Gà Rán Nobi" sử dụng OneSystem.
- Bao gồm: 1 Trụ sở chính (HQ), 1 Bếp trung tâm (Factory F1), và 1 Cửa hàng (Store S1).
- Sản phẩm bán: Gà Rán. Nguyên liệu gồm: Gà tươi (Raw Material) và Gà ướp sẵn (Semi-Product).

Dưới đây là một vòng đời vận hành hoàn chỉnh minh hoạ cách 3 hệ thống này tương tác với nhau:

---

### Giai đoạn 1: HQ nhập nguyên liệu đầu vào cho Factory (External Procurement - PurO)
*Vì F1 không được phép tự mua ngoài, HQ sẽ lo việc đàm phán và đặt hàng.*

  [1] HQ nhận thấy F1 sắp hết Gà tươi (dựa vào cảnh báo tồn kho của F1).
  [2] HQ tạo lệnh **`PurchaseOrder` (HQ.PurO)** đặt mua 1000kg Gà tươi từ nhà cung cấp CP Foods.
  [3] Địa chỉ giao hàng trên PurO được chỉ định thẳng đến Factory F1.
  [4] Xe của CP Foods chở 1000kg gà đến F1. Nhân viên kho F1 kiểm đếm và tạo phiếu **`GoodsReceipt` (GR)** trực tiếp link với mã `HQ.PurO` đó. -> *F1 tăng tồn kho 1000kg Gà tươi.*
  [5] Kế toán HQ thực hiện **3-Way Matching** (Hóa đơn CP Foods + GR của F1 + HQ.PurO). Trùng khớp 100% -> HQ giải ngân thanh toán cho CP Foods.

---

### Giai đoạn 2: Factory Sản xuất (Production)
*Factory tiến hành sơ chế nguyên liệu thô thành bán thành phẩm để sẵn sàng cung cấp cho Store.*

  [6] Quản lý F1 lên lệnh Sản xuất (Production Order) để sơ chế gà.
  [7] F1 xuất kho 1000kg Gà tươi + các loại gia vị (tiêu hao nguyên liệu theo định mức BOM).
  [8] Sau quy trình tẩm ướp, F1 thu được 5000 miếng "Gà ướp sẵn" (Semi-product).
  [9] Xác nhận hoàn thành sản xuất -> *F1 giảm tồn kho Gà tươi, tăng tồn kho 5000 miếng Gà ướp sẵn.* Chi phí nguyên liệu cấu thành nên Giá vốn (Cost) của miếng Gà ướp.

---

### Giai đoạn 3: Store bán hàng và Tự động Châm hàng (Internal Transfer Order)
*Store bán hàng và được tự động replenish hàng từ Factory.*

  [10] Store S1 chiên "Gà ướp sẵn" bán cho khách hàng. Hệ thống liên tục trừ tồn kho.
  [11] Cuối ngày, tồn kho Gà ướp sẵn tại S1 rớt xuống dưới ROP (Ví dụ: còn 300 miếng, trong khi ROP là 500).
  [12] Hệ thống tự động kích hoạt tạo **`InternalTransferOrder`** (S.RO / F.SO).
        - Nguồn: F1
        - Đích: S1
        - Số lượng: 1000 miếng (Định mức châm hàng tiêu chuẩn).
        - Trạng thái: `AUTO_APPROVED`.

---

### Giai đoạn 4: Factory Xuất hàng nội bộ (Goods Issue)
  [13] Quản lý F1 mở app, thấy lệnh `InternalTransferOrder` yêu cầu giao 1000 miếng gà cho S1.
  [14] Nhân viên F1 soạn đủ 1000 miếng gà, đóng thùng đá.
  [15] Nhập thông tin lên hệ thống tạo phiếu **`GoodsIssue` (GI)**:
        - Chọn tài xế Lalamove (Biển số: 29A-12345).
        - Phí ship: 100,000 VND.
        - Chụp ảnh thùng hàng đã đóng seal đính kèm phiếu GI.
  [16] Bấm Dispatch -> *Tồn kho F1 bị trừ (Stock Out) 1000 miếng Gà ướp sẵn. Trạng thái lệnh thành `IN_TRANSIT`.*

---

### Giai đoạn 5: Store Nhận hàng và Xử lý Lệch hàng (Goods Receipt)
  [17] Tài xế Lalamove giao hàng đến S1.
  [18] Nhân viên S1 mở thùng kiểm tra. Phát hiện do lỗi vận chuyển, thùng đá bị rách thủng khiến 20 miếng gà bị rơi ra ngoài hỏng hoàn toàn. Chỉ còn 980 miếng xài được.
  [19] Nhân viên S1 tạo phiếu **`GoodsReceipt` (GR)** trên hệ thống, điền số lượng nhận thực tế là **980 miếng**.
  [20] Bấm Confirm -> *Tồn kho S1 được cộng (Stock In) 980 miếng Gà ướp sẵn.*
  [21] Nhận thấy số lượng GR (980) < GI (1000), hệ thống tự động sinh ra một **`DiscrepancyTicket`** (Phiếu lệch hàng 20 miếng) đẩy thẳng về màn hình HQ. Trạng thái lệnh luân chuyển thành `DISCREPANCY`.

---

### Giai đoạn 6: HQ Kế toán xử lý cuối (Cost Allocation)
  [22] Cửa hàng S1 vẫn tiếp tục lấy 980 miếng gà ra chiên bán bình thường, không bị gián đoạn.
  [23] Tại HQ, Kiểm soát viên mở `DiscrepancyTicket`. Họ check ảnh lúc xuất kho (GI) của F1 thấy thùng nguyên vẹn -> Lỗi do tài xế Lalamove.
  [24] HQ khiếu nại Lalamove để đòi bồi thường 20 miếng gà.
  [25] HQ đóng Ticket. Hệ thống hạch toán:
        - Chi phí của 980 miếng gà thực nhận + phí ship 100k -> Tính vào OpEx của S1.
        - Chi phí của 20 miếng gà hỏng -> Tính vào "Transit Loss Cost" của chuỗi, chờ tiền bồi thường từ Lalamove bù vào.

---
**Tổng kết giá trị mô hình:**
Nhờ cấu trúc này, khách hàng F&B của bạn sẽ vận hành cực kỳ trơn tru. S1 không bao giờ bị "đứt gãy" dữ liệu bán hàng. F1 chỉ tập trung làm đúng nhiệm vụ soạn hàng - xuất kho có bằng chứng. HQ nắm trong tay toàn bộ quyền sinh sát về tài chính và kiểm soát rủi ro thất thoát.
