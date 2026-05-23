#####################
Ví dụ: Quy trình chuyển hàng nội bộ giữa hai Node cùng địa điểm (Cùng `site_id`)

Bối cảnh Doanh nghiệp:
- Chuỗi "Gà Rán Nobi" có một mô hình kết hợp (Hybrid Location) tại địa chỉ 123 Nguyễn Văn Linh.
- Tại địa điểm này, trên hệ thống OneSystem tạo 2 virtual nodes dùng chung một `site_id` (VD: `site_123_nvl`):
  - Bếp trung tâm (tầng 2): Factory F1
  - Quầy bán hàng (tầng 1): Store S1
- Mục tiêu: Giúp nhân viên luân chuyển hàng hóa từ tầng 2 xuống tầng 1 một cách nhanh chóng nhất, bỏ qua các bước xác nhận thông tin vận chuyển không cần thiết (không có tài xế, không phí ship).

Dưới đây là vòng đời vận hành khi chuyển hàng nội bộ tại cùng một địa điểm (In-Site Transfer):

---

### Giai đoạn 1: Quầy bán hàng (Store) Cần châm hàng (Internal Transfer Order)
  [1] Tồn kho "Gà ướp sẵn" tại quầy S1 rớt xuống dưới mức ROP (hoặc Quản lý S1 chủ động tạo lệnh do khách đông đột xuất).
  [2] Hệ thống (hoặc Quản lý) tạo **`InternalTransferOrder`**.
        - Nguồn (Provider): F1
        - Đích (Requester): S1
        - Số lượng: 500 miếng Gà ướp sẵn.
        - Hệ thống nhận diện `F1.site_id == S1.site_id`.
  [3] Lệnh được tự động duyệt (`AUTO_APPROVED`).

---

### Giai đoạn 2: Bếp (Factory) Xuất hàng (Simplified Goods Issue)
*Vì hai node nằm chung một tòa nhà, UI/UX trên ứng dụng sẽ được tối giản.*

  [4] Nhân viên bếp F1 soạn 500 miếng gà vào khay và xách đi thang bộ/thang máy xuống tầng 1.
  [5] Trên thiết bị của nhân viên F1, giao diện phiếu xuất kho (`GoodsIssue`) thay vì bắt nhập thông tin vận chuyển, hệ thống chỉ hiển thị một nút đơn giản: **"Move to Store"** (hoặc "Chuyển tới Quầy").
  [6] Nhân viên F1 bấm **"Move to Store"**.
  [7] Hệ thống thực thi ở backend:
        - Tạo ra phiếu **`GoodsIssue` (GI)** với thông tin: Không tài xế (driver=null), Không media proof, Không phí ship (fee=0).
        - Thực hiện trừ tồn kho (Stock Out) 500 miếng gà tại F1.

---

### Giai đoạn 3: Tự động Nhận hàng (Auto Goods Receipt)
*Để quy trình liền mạch, không bắt nhân sự trong cùng một nhà phải xác nhận qua lại trên app, hệ thống sẽ tự động hoàn tất khâu nhận hàng (GR).*

  [8] Ngay khi F1 bấm xác nhận "Move to Store", hệ thống **tự động sinh ra phiếu `GoodsReceipt` (GR)** cho đích đến S1.
  [9] Số lượng nhận trên GR được điền tự động bằng số lượng GI (500 miếng).
  [10] Hệ thống tự động Confirm phiếu GR này -> Cộng tồn kho (Stock In) 500 miếng gà cho S1.
  [11] Lệnh `InternalTransferOrder` lập tức chuyển sang trạng thái `COMPLETED`. Không có độ trễ `IN_TRANSIT` và không cần thao tác nào từ phía S1.

---
**Tổng kết giá trị mô hình:**
- **Chặt chẽ về kế toán & tồn kho:** Mọi dịch chuyển tài sản giữa 2 sổ kho tách biệt (Bếp F1 và Quầy S1) vẫn được ghi nhận rõ ràng thông qua hai bút toán xuất (GI) và nhập (GR).
- **Thực dụng về vận hành (UX):** Nhanh, gọn, lẹ. Việc xách hàng đi vài bước chân giờ đây chỉ cần đúng **1 chạm** (1-click) trên app từ người giao hàng, giảm thiểu tối đa thao tác rườm rà.
