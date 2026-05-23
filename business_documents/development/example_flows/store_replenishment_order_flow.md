#####################

Ví dụ: Hệ thống tự động tạo RO châm hàng (OpEx) cho S1 khi tồn kho xuống thấp và quy trình điều chuyển nội bộ (Internal Logistics)

Bối cảnh:
- S1 đang bán hàng, tiêu hao "Vỏ bánh Burger" (thuộc nhóm Items - OpEx).
- Reorder Point (ROP) của vỏ bánh tại S1 được cấu hình là 500 cái.
- Trong ngày, sau khi trừ kho từ các lệnh sản xuất tại store (S.PO), tồn kho vỏ bánh của S1 rớt xuống còn 480 cái (< ROP).
- Hệ thống phát hiện và kích hoạt luồng châm hàng tự động.

Flow:

  [1] Hệ thống tự động tạo và Duyệt lệnh điều chuyển nội bộ (Internal Transfer Order)
        - Do đây là nghiệp vụ mua bán nội bộ (Internal Purchase), hệ thống gộp S.RO (nhu cầu của Store) và F.SO (yêu cầu xử lý của Factory) thành một đối tượng duy nhất để tránh trùng lặp dữ liệu.
        - Hệ thống tự động duyệt (`AUTO_APPROVED`) lệnh điều chuyển nội bộ này.
        - Thông tin lệnh:
          * Nguồn cung ứng: Factory
          * Nơi nhận: S1
          * Item: Vỏ bánh Burger
          * Số lượng: 1000 cái (định mức châm)

  [2] Factory thực hiện xuất kho (Goods Issue - GI)
        - Nhân viên kho Factory chuẩn bị đóng gói 1000 vỏ bánh burger.
        - Nhân viên tạo phiếu **Goods Issue (GI)** trên hệ thống, bắt buộc phải upload các thông tin đối chứng:
          * Thông tin tài xế & Biển số xe vận chuyển.
          * Video ghi lại quá trình bốc xếp hàng lên xe và ảnh chụp cận cảnh tình trạng hàng hóa trước khi dispatch (để chứng minh hàng đi từ kho không bị lỗi).
          * Phí ship thực tế (ví dụ: 50,000 VND).
        - Xác nhận xuất kho -> Trạng thái chuyển thành `IN_TRANSIT`. Hệ thống tự động trừ tồn kho (Stock Out) 1000 vỏ bánh tại Factory.

  [3] S1 thực hiện nhận hàng (Goods Receipt - GR) và Xử lý chênh lệch vận chuyển
        - Tài xế giao hàng tới S1. Nhân viên S1 kiểm đếm thực tế đối chiếu với phiếu Goods Issue (GI) từ Factory.
        - Nhân viên S1 tạo phiếu **Goods Receipt (GR)** dựa trên GI của Factory.
        
        *Xử lý 2 kịch bản nhận hàng:*
        
        - Kịch bản A (Khớp hàng): Nhận đủ 1000 cái nguyên vẹn.
          * S1 xác nhận GR = 1000 cái.
          * Hệ thống cộng tồn kho (Stock In) 1000 cái tại S1. Hoàn thành luồng.

        - Kịch bản B (Lệch hàng / Hao hụt do vận chuyển): 
          * Thực tế kiểm đếm chỉ có 980 cái dùng được, 20 cái bị bẹp/nát trong quá trình vận chuyển.
          * Nhân viên S1 nhập số lượng thực nhận: **980 cái**.
          * Hệ thống ghi nhận nhận kho 980 cái (Stock In tại S1 để đảm bảo số liệu tồn kho thật chính xác để bán hàng).
          * 20 cái hao hụt được hệ thống tự động ghi nhận vào trạng thái **"Transit Discrepancy"** và tự động tạo một **Discrepancy Ticket** gửi về HQ.
          * S1 vẫn được phép hoàn thành thủ tục nhận hàng bình thường để đưa 980 cái bánh vào vận hành ngay, không bị nghẽn quy trình.

  [4] HQ xử lý chênh lệch (Discrepancy Handling)
        - Bộ phận kiểm soát tại HQ tiếp nhận Discrepancy Ticket (hao hụt 20 cái vỏ bánh tại S1).
        - HQ đối chiếu hình ảnh/video lúc xuất kho của Factory (GI) và hình ảnh hàng lỗi lúc nhận của Store (GR):
          * Nếu lỗi do khâu đóng gói của Factory: ghi nhận hao hụt vào chi phí hao hụt của Factory.
          * Nếu lỗi do tài xế vận chuyển: tiến hành làm việc với đơn vị vận chuyển để đền bù/trừ tiền ship.
        - Kế toán HQ duyệt xử lý lệch hàng trên hệ thống -> Đóng ticket.

  [5] Hạch toán Chi phí (OpEx)
        - Giá trị của số vỏ bánh thực tế nhập kho (980 cái) + phí ship 50,000 VND được hạch toán vào Chi phí vận hành (OpEx) của S1.
        - Giá trị 20 cái vỏ bánh bị hỏng được hạch toán vào tài khoản Chi phí hao hụt vận chuyển (Transit Loss Cost).

Tóm tắt luồng:
  Tồn kho S1 < ROP ➔ Hệ thống tự tạo & duyệt lệnh chuyển nội bộ ➔ Factory xuất kho tạo GI (kèm video/ảnh/tài xế) ➔ Ship hàng ➔ S1 nhận hàng tạo GR (nếu lệch: S1 nhận phần chuẩn, phần lệch tạo Discrepancy gửi HQ xử lý) ➔ HQ đóng ticket lệch ➔ Hạch toán OpEx.