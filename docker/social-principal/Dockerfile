FROM alpine:3.21
RUN apk add --no-cache openssl
COPY bootstrap.sh /bootstrap.sh
ENTRYPOINT ["sh", "/bootstrap.sh"]
