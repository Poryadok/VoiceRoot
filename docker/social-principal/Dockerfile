FROM alpine:3.21
RUN apk add --no-cache openssl python3
COPY bootstrap.sh /bootstrap.sh
COPY export-jwks.py /export-jwks.py
ENTRYPOINT ["sh", "/bootstrap.sh"]
