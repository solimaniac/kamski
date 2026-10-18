FROM nginx:stable-alpine
COPY default.conf.template /etc/nginx/templates/default.conf.template
COPY public/ /usr/share/nginx/html/
ENV PORT=8080
EXPOSE 8080
