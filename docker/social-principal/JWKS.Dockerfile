FROM nginx:stable-alpine
RUN apk add --no-cache curl
COPY jwks.conf /etc/nginx/conf.d/default.conf
