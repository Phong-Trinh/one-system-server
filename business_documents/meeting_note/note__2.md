
Supply chain workflow:
store.SO (system batch allocation) > store.PO (trigger) > store.RO [hoặc PR] > factory.SO > factory.PO > factory.RO [hoặc PR] > HQ.PurO > suppliers.

[CORRECTION: "PurO" chỉ dùng cho luồng HQ → Supplier. Tất cả nghiệp vụ internal logistics (S↔F, F→HQ xin nguyên vật liệu) dùng RO hoặc PR thay thế.]

HQ.PurO mang tính duyệt đề xuất, đàm phán với suppliers để mua hàng theo quy mô đơn hàng (giao hàng nhiều điểm về F).

S.RO/PR ở S: chủ yếu mua hàng từ F hoặc S khác (internal order). Trạng thái hiện tại: chỉ được mua nội bộ, tất cả items.
- S.RO: triggered by stock level — hệ thống tự sinh khi tồn kho xuống ngưỡng.
- S.PR: ngoại lệ (mua đột xuất, CapEx equipment gửi lên HQ xét duyệt).

Factory: items, inventory, purchase (RO/PR), inbound logistics, production, outbound logistics, sales order (S hoặc khách sỉ (low priority)).

    * Giả định F1-SO sẽ tiếp nhận S1-RO sản xuất 100 bánh burger vào T3, T6.
        -> F.I: Tối ưu vòng quay tồn kho nguyên vật liệu.
        -> F.RO (OpEx) goals: Tối ưu chiết khấu mua nguyên vật liệu — lựa ncc giá rẻ nhất, có ncc thay thế. Triggered by stock threshold tại Factory.
        -> F.PR (CapEx) goals: Setup F1 — mua máy trộn bột, tủ, đi lại dây điện,... => luôn gửi PR lên HQ, HQ xét duyệt, không có RO cho loại này.
        -> F.OL (out logistics): Tối ưu chi phí ship materials về F, tối ưu chi phí ship products đến S. Phần lớn đặt shopee, tạo lệnh mua trứng,... ở cửa hàng tạp hoá sỉ gần đây cũng đc tính vào cost.
        -> F.IL: Tối ưu bảo quản, lưu trữ materials.
        -> F.PO: Tối ưu sản xuất để giảm tỷ lệ hàng hư/huỷ/dư.
        -> F.SO: Nhận đơn đặt hàng từ cửa hàng (S.RO).


Vấn đề hiện tại của khách hàng:
    - Chưa thống kê được giá thành các nguyên vật liệu nhập vào -> do chưa có supplier cố định, giá cả thay đổi theo từng đợt mua, chưa tối ưu được vấn đề ghép batch để tối ưu chi phí.
        -> Giải quyết vấn đề này sẽ tính ra được chi phí sản xuất của từng sản phẩm => giá thành.




 #####################

 PurO chỉ dùng cho giữa HQ và Sup, tất cả nghiệp vụ inbound/outbound logictics internal sẽ thay bằng RO + PR

"Purchase requisition" -> PR là khi một bộ phận/cửa hàng (F & S) nhận ra mình cần mua thứ gì đó và gửi đề nghị lên bộ phận 
mua hàng (HQ) để xem xét và thực hiện.

"Replishment order" -> RO là lệnh bổ sung hàng — khi tồn kho của một item xuống thấp đến ngưỡng nhất định, hệ thống hoặc người quản lý chủ động ra lệnh nhập thêm để đảm bảo không bị stockout. (châm hàng)

phân biệt 2 loại resource của hệ thống: OpEx — bạn tiêu tiền để vận hành hôm nay.
CapEx — bạn đầu tư tiền để tạo ra năng lực cho tương lai.


        | CapEx | OpEx |
        | --- | --- |
| Là gì | Mua tài sản dài hạn | Chi phí vận hành hàng ngày |
| Ví dụ | Mua máy pha cà phê, sửa chữa lớn | Mua nguyên liệu, trả lương, điện nước |
| Hạch toán | Ghi nhận tài sản → khấu hao dần | Ghi nhận chi phí ngay trong kỳ |
| Vòng đời | Nhiều năm | Ngắn hạn (ngày/tháng) |


- Đối với equipment và tools, thì S hoặc F sẽ gửi PR lên HQ để mua. Vd: Đi lại đường dây điện, mua lò this lò that => HQ sẽ xét duyệt dựa trên CapEx
Sau khi được duyệt, HQ sẽ đi tìm Sup, deal, rồi tạo PurchaseOrder tới họ
Sup giao hàng đến S, F hoặc kho trung tâm
cần có invoice + goods receipt + PurchaseOrder (3 ways matching) thì HQ mới thanh toán cho Sup



Items (OpEx)
├── RO  ← triggered by stock level     (thường xuyên)
└── PR  ← mua đột xuất, supplier mới,  (ngoại lệ)
          số lượng lớn bất thường,...

Equipment (CapEx)
└── PR  ← luôn luôn                    (không có RO)



Lead Time (LT)
Khoảng thời gian từ lúc phát sinh nhu cầu đến lúc hàng sẵn sàng sử dụng.

Lead time ảnh hưởng trực tiếp đến các tham số replenishment:

Reorder Poin(ROP) = Daily consumption × Lead time + Safety stock
Lead time càng dài → ROP càng cao → phải đặt hàng sớm hơn
Lead time không ổn định → cần safety stock lớn hơn để bù

Lead time thường có hai nguồn:
- Supplier cam kết: Lý thuyết, ghi trên hợp đồng
- Lịch sử thực tế: Đo từ dữ liệu các lần nhập trước
Hệ thống tốt nên dùng actual lead time từ lịch sử để tính toán, không nên chỉ tin vào số supplier cung cấp.



#####
equipment/tools/machine

có nên gộp S.RO và F.SO trong case internal purchase? -> nếu dữ liệu ko khác nhau thì nên gộp (hệ thống auto approved)


F chỉ tiếp nhận đơn hàng mà nó xử lý, nó ko tham gia việc bán hàng
HQ sẽ đảm nhận việc sale, và điều phối về cho F sản xuất

transfer product internal: 
F sẽ tạo Goods Issue để xuất kho
    - sẽ có video record và image, tài xế, biển số xe
    - chụp ảnh hàng hóa
    - phí ship 
S sẽ tạo Goods Receipt để nhận hàng -> base trên Goods Issue của F
-> nếu có chênh lệch -> hiện tại sẽ chuyển về HQ để xử lý sau đó, và cho phép nhập kho bth
-> phải có cách để kiểm soát hàng hư hỏng trong quá trình vận chuyển -> cần giải quyết nghiệp vụ phụ chỗ này

