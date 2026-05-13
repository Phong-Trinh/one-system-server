# Tài liệu Quy trình Điều phối Sản xuất (Allocation Process) - OneSystem

Hệ thống điều phối của OneSystem đóng vai trò là "bộ não" quản lý vận hành bếp, tự động hóa việc phân chia công việc và tối ưu hóa hiệu suất sử dụng máy móc.

## 1. Luồng xử lý tổng quát
Khi có một Đơn hàng sản xuất (Production Order - PO) đến, hệ thống sẽ thực hiện các bước sau:

1.  **Phân rã (Decomposition)**: PO được tách thành các bước nhỏ (SOP Steps).
2.  **Xếp hàng (Queuing)**: Các bước không có phụ thuộc (hoặc đã hoàn thành phụ thuộc) sẽ được đưa vào hàng đợi `QUEUED`.
3.  **Điều phối (Allocation)**: Thuật toán tìm máy phù hợp và gán Batch vào máy (`ALLOCATED`).
4.  **Thực thi (Execution)**: Bếp xác nhận đặt đồ vào máy (`IN_PROGRESS`).
5.  **Hoàn tất (Completion)**: Máy giải phóng, kích hoạt các bước phụ thuộc tiếp theo.

---

## 2. Cơ chế Điều phối thông minh (Allocation Engine)

Hệ thống tự động nhận diện 2 chiến lược vận hành khác nhau của máy móc để tối ưu hóa:

### A. Chiến lược Đồng bộ (BATCH_SYNC) - Ví dụ: Nồi chiên, Lò nướng
Dành cho các máy mà các vật phẩm bên trong phải bắt đầu và kết thúc cùng lúc trong một môi trường (như dầu chiên hoặc buồng nhiệt).

*   **Cơ chế Gom lô (Consolidated Batching)**: 
    *   Nếu máy đang ở trạng thái chờ (`ALLOCATED`) và chưa bắt đầu nấu, hệ thống cho phép **gộp thêm** các Batch cùng loại Item từ các Order khác vào cùng một mẻ.
    *   *Ví dụ*: Order 1 có chiên hành, Order 2 cũng có chiên hành. Cả hai sẽ được hiện lên cùng lúc để đầu bếp bỏ vào chiên một lần.
*   **Khóa máy (Locking)**: Khi máy đã chuyển sang `IN_PROGRESS`, máy sẽ bị khóa hoàn toàn cho đến khi kết thúc để đảm bảo chất lượng món ăn.

### B. Chiến lược Bất đồng bộ (SLOT_ASYNC) - Ví dụ: Bếp nướng phẳng (Grill)
Dành cho các bề mặt nấu nướng rộng, nơi có thể đặt thêm đồ ăn vào các vị trí trống bất kỳ lúc nào.

*   **Cơ chế Bin-packing**: Hệ thống tính toán dựa trên diện tích chiếm dụng (`SlotsUsed`) và sức chứa tối đa (`MaxCapacity`).
*   **Xử lý song song độc lập**: Các Batch có thể bắt đầu vào các thời điểm khác nhau. 
    *   *Ví dụ*: Thịt của Order 1 đang nướng được 3 phút, Order 2 đến có thể đặt ngay vào vùng trống bên cạnh. Mỗi miếng thịt sẽ có một đồng hồ đếm ngược riêng biệt trên màn hình bếp.
*   **Không chặn máy (Non-blocking)**: Máy chỉ bị từ chối nhận thêm việc khi tổng số Slot đang dùng vượt quá ngưỡng vận hành (`OperationalThreshold`).

---

## 3. Trạng thái và Tương tác của Bếp

| Trạng thái | Hiển thị trên KDS | Hành động của Bếp |
| :--- | :--- | :--- |
| **QUEUED** | Nằm trong danh sách chờ | Chờ hệ thống tìm máy |
| **ALLOCATED** | Hiện lên máy cụ thể (Nháy đèn/Thông báo) | Bỏ nguyên liệu vào và nhấn **"Bắt đầu"** |
| **IN_PROGRESS** | Đang đếm ngược thời gian nấu | Chờ món chín |
| **COMPLETED** | Thông báo lấy món ra | Lấy món và nhấn **"Hoàn tất"** |

---

## 4. Lợi ích của hệ thống
- **Giảm sai sót**: Bếp không cần nhớ SOP, chỉ cần làm theo các bước hiện trên màn hình.
- **Tối ưu năng suất**: Tự động gom các món giống nhau để nấu một lần.
- **Tính toán chính xác**: Đảm bảo không bao giờ xếp quá tải cho một máy hoặc một nhân viên bếp.

---

## 5. Ví dụ thực tế: Cửa hàng Hamburger

### 5.1. Tài nguyên hiện có của Cửa hàng:
*   **Fryer 1 (BATCH_SYNC)**: Nồi chiên lớn (Dung lượng 4).
*   **Fryer 2 (BATCH_SYNC)**: Nồi chiên nhỏ (Dung lượng 2).
*   **Flat Grill (SLOT_ASYNC)**: Bếp nướng phẳng (Dung lượng 8 slots).

### 5.2. Input Đơn hàng (Vào cùng lúc 09:00):
*   **Order A**: 1 Cheese Burger + 1 Khoai tây chiên.
*   **Order B**: 1 Chicken Burger (Burger Gà).

### 5.3. Áp dụng quy trình OneSystem vào các Order này:

#### Bước 1: Phân rã (Decomposition)
Hệ thống tạo ra các Batch (mẻ việc) trong hàng đợi `QUEUED`:
- **Batch A1**: Chiên khoai (Order A) - Cần 2 slots FRYER.
- **Batch A2**: Nướng thịt bò (Order A) - Cần 1 slot GRILL.
- **Batch A3**: Chiên hành tây (Order A) - Cần 1 slot FRYER.
- **Batch B1**: Chiên gà (Order B) - Cần 2 slots FRYER.
- **Batch B2**: Chiên hành tây (Order B) - Cần 1 slot FRYER.
- **Batch B3**: Nướng bánh (Order B) - Cần 1 slot GRILL.

#### Bước 2: Điều phối (Allocation) - Cách hệ thống ghép mẻ:

| Thiết bị | Batch được gán | Trạng thái | Giải thích logic |
| :--- | :--- | :--- | :--- |
| **Flat Grill** | **A2** (Thịt bò) + **B3** (Bánh) | **IN_PROGRESS** | Vì là `SLOT_ASYNC`, hệ thống cho nướng cả thịt bò và bánh burger cùng lúc trên mặt bếp. |
| **Fryer 1** | **A3** (Hành) + **B2** (Hành) | **ALLOCATED** | **Consolidated Batching**: Hệ thống thấy cả 2 đơn đều cần chiên hành. Nó gộp 2 mẻ này lại để đầu bếp chiên 1 lần cho tiết kiệm. |
| **Fryer 2** | **A1** (Khoai tây) | **IN_PROGRESS** | Khoai tây cần chiên riêng, được gán vào nồi chiên số 2. |

*Hàng đợi còn lại: **B1** (Chiên gà) vẫn ở trạng thái `QUEUED` vì các nồi chiên đang bận hoặc không cùng loại item.*

#### Bước 3: Thực thi (Execution) - 3 phút sau (09:03)
*   **Order C** đến: Thêm 1 Cheese Burger.
*   Lúc này, **Flat Grill** đang nướng bò (A2). Vì là `SLOT_ASYNC`, hệ thống lập tức gán thêm mẻ nướng bò của Order C vào slot trống còn lại trên Grill.
*   **Kết quả**: Trên màn hình Grill sẽ thấy 3 đồng hồ đếm ngược khác nhau cho 3 mẻ (Thịt bò A2 còn 2p, Bánh B3 còn 1p, Thịt bò C còn 5p).

#### Bước 4: Hoàn tất (Completion)
*   Khi mẻ **A3+B2** (Hành tây) hoàn thành, **Fryer 1** trống.
*   Hệ thống lập tức "nhặt" **B1** (Chiên gà) đang đợi trong hàng đợi và đẩy lên **Fryer 1**.

### 5.4. Tóm tắt lợi ích vận hành:
1.  **Tiết kiệm năng lượng**: Thay vì bật nồi chiên 2 lần cho hành tây, bạn chỉ làm 1 lần.
2.  **Tối ưu mặt bếp**: Bếp nướng không bị bỏ trống, nướng gối đầu liên tục nhiều đơn hàng.
3.  **Giảm áp lực cho nhân viên**: Hệ thống tự cộng dồn nguyên liệu ("Hãy bỏ 2 phần hành vào Fryer 1"), nhân viên không cần đọc order giấy.
4.  **Kiểm soát thời gian**: Mỗi đơn hàng nhỏ lẻ đều có bộ đếm giờ riêng, đảm bảo chất lượng món ăn đồng nhất.


