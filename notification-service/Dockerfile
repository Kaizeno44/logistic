# Giai đoạn 1: Builder (Dùng image Golang đầy đủ để biên dịch code)
FROM golang:1.26-alpine AS builder
# Thiết lập thư mục làm việc trong container
WORKDIR /app
# Thêm dòng này để đổi proxy, giúp tải thư viện Go nhanh gấp 10 lần
ENV GOPROXY=https://goproxy.io,direct

RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download

# Copy toàn bộ mã nguồn vào container
COPY . .

# Biên dịch ứng dụng thành file chạy độc lập (binary) tên là 'main'
# CGO_ENABLED=0 giúp file chạy được trên môi trường Alpine siêu nhẹ
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/main.go

# Giai đoạn 2: Production Image (Chỉ lấy file binary, bỏ lại toàn bộ source code)
FROM alpine:latest

# Cài đặt chứng chỉ SSL và múi giờ (cần thiết nếu gọi API bên thứ 3)
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy file binary từ giai đoạn builder sang
COPY --from=builder /app/main .

# Mở cổng 8003 cho Notification Service
EXPOSE 8003
# Lệnh chạy ứng dụng khi container khởi động
CMD ["./main"]