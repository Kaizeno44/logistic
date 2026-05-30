FROM nginx:alpine
# Xóa cấu hình mặc định của Nginx
RUN rm /etc/nginx/conf.d/default.conf
# Copy file cấu hình custom vào
COPY nginx.conf /etc/nginx/nginx.conf
EXPOSE 8080
