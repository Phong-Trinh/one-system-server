#####################

 Ví dụ: S1 muốn mua thêm một bếp nướng (CapEx Equipment → luôn dùng PR)

Bối cảnh:
- S1 đang vận hành bình thường, nhưng quản lý cửa hàng nhận ra lượng đơn nướng tăng cao vào cuối tuần,
  bếp nướng hiện tại không đủ công suất → quyết định đề xuất mua thêm 1 bếp nướng điện.
- Đây là CapEx (tài sản dài hạn) → KHÔNG dùng RO, phải dùng PR.

Flow:

  [1] S1 Manager tạo S.PR trên hệ thống
        - Loại: Equipment (CapEx)
        - Item: Bếp nướng điện công nghiệp
        - Số lượng: 1
        - Lý do: Công suất nướng hiện tại không đáp ứng demand cuối tuần
        - Estimated cost: 8,500,000 VND
        - Gửi lên: HQ

  [2] HQ nhận PR → review CapEx
        - HQ xem xét: ngân sách CapEx quý, ROI dự kiến, mức độ ưu tiên
        - Có thể: yêu cầu bổ sung thông tin (báo giá 2-3 nhà cung cấp), hoặc approve/reject
        - Kết quả: HQ APPROVED PR-S1-0042

  [3] HQ tìm supplier, đàm phán, tạo HQ.PurO
        - HQ liên hệ 2-3 nhà cung cấp thiết bị bếp
        - Chọn supplier: Bếp Việt Co. — giá 8,200,000 VND (tốt hơn estimate của S1)
        - HQ tạo HQ.PurO → gửi đến Bếp Việt Co.
        - Delivery point: S1 (giao thẳng đến cửa hàng)

  [4] Supplier giao hàng đến S1
        - Bếp Việt Co. giao bếp nướng đến S1
        - S1 staff xác nhận nhận hàng → tạo Goods Receipt (GR) trên hệ thống
        - GR linked to HQ.PurO

  [5] 3-Way Matching tại HQ
        - HQ kiểm tra: Invoice (từ Bếp Việt Co.) + GR (S1 xác nhận) + HQ.PurO
        - Nếu khớp → HQ approve thanh toán cho supplier
        - Nếu lệch (sai số lượng, giá, item) → hold lại, yêu cầu giải trình

  [6] Ghi nhận tài sản
        - Bếp nướng được ghi nhận vào danh sách tài sản (Equipment Register) của S1
        - Hạch toán: Tài sản cố định → khấu hao dần theo vòng đời thiết bị (ví dụ: 3 năm)
        - KHÔNG ghi nhận toàn bộ vào chi phí ngay trong kỳ (khác với OpEx)

Tóm tắt flow:
  S.PR (S1 tạo) → HQ review & approve → HQ.PurO (tới supplier) → GR tại S1 → 3-way matching → thanh toán → Equipment Register