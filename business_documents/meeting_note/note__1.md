SX thì mọi thứ nên bắt đầu từ item -> gọi item sẽ đúng hơn product

từ item sẽ upgrade thành BOM và SOP -> bill of material

BOM -> một cái burger sẽ được cấu thành từ những vật liệu này,.. semi-product này,.. -> những item để cấu thành những item này

SOP -> quy trình để cấu thành BOM

- > lúc này sẽ có được sản phẩm hàon chỉnh -> sinh ra nghiệp vụ khác như: nhập kho, purchasing,
purchasing sẽ được thực hiện ở đâu? -> purchasing theo thông thường thì có thể thực hiện ở nhiều nơi (store, factory, HQ)
nhưng trong case của SB thì HQ sẽ đảm nhận việc này

sl tồn kho của vỏ bánh burger ở 1 store khi đạt đến số lượng tồn kho tối thiếu sẽ được trigger về factory -> factory sẽ dựa vào BOM -> request lên HQ để cung cấp các raw-material
có quyền để purchasing ở factory only, nhưng tương lai sẽ có thể có các thương hiệu có nhiều factory

HQ nên giữ quyền đặt mua raw-material -> dễ thống kê, deal với nhà cung cấp
extend: HQ purchase order from supplier -> supplier giao hàng đến factory -> factory sẽ có một stock keeper đứng ra nhận hàng -> nhập hàng vào kho (stock in/out)
-> stock in sẽ dựa vào purchase order

factory sẽ có production order (SOP + BOM + (nhân công,...))

item thì sau này sẽ còn extend ra những sản phẩm (không phải để bán) mà cửa hàng sẽ cần -> ví dụ như máy tính, POS,...
-> gọi là item sẽ hợp lý hơn product, basically sẽ có: product, semi-product, material,..

factory bán hàng cho store

production order sẽ tách riêng ở bếp (product) và cả factory (semi-product, product,..)

system của mình ko nên làm phiếu ký tay khi store nhận purchase order từ factory -> quy trình rất dài -> factory tạo lệnh vận chuyển có thể nhập sdt của tài xế, chụp tầm hình, phí ship,.. -> nhấn xuất kho

không làm external purchasing hiện tại -> phức tạp về mặt nghiệp vụ khi triển khai thực tế (khi một nhân viên có thể đem đồ vào kho, thì có rủi ro nv cũng sẽ lấy đồ ra khỏi kho -> mất cắp vật tư)

nhưng lệnh purchase giữa các store có thể triển khai bây giờ (optional)
-> vd: khi store 1 out stock item -> có nghiệp vụ mượn hàng/ mua hàng từ store khác

KHÓ: đơn vị tồn kho -> có nghiệp vụ là bánh burger sẽ được đóng gói vào bao để giao đi -> 1 bao nhiều cái bánh burger
-> packaging cost
-> vậy khi order sẽ tính bằng đơn vị nào?
-> 1 thùng nước ngọt (4 block x 6 lon)

giá có thể khác nhau ở các store

######################
ISSUE HIỆN TẠI CỦA BÊP
quản lý đang nhìn bằng mắt để phỏng đoán số lượng item hiện đang thiếu và đang cần purchase ở cửa hàng -> ko biết chính xác số lượng

khi đơn vào nhiều -> ko nắm được số lượng tồn kho của thịt bò (miếng thịt bò đã được trộn gia vị) -> những đơn cuối cùng sẽ có thể bị thiếu thịt