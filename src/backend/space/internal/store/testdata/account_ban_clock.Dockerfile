# Test-only PG wall-clock fixture; parent image is the locally verified target18.
FROM postgres@sha256:77f585114c32fbca283dc835b0596f4e52b51b4c6662d7810b2f4084f60a1873
COPY account_ban_wall_clock.c /tmp/account_ban_wall_clock.c
RUN apk add --no-cache --virtual .native7-clock-build gcc musl-dev \
 && gcc --version \
 && cc -std=c11 -Wall -Wextra -Werror -fPIC -shared /tmp/account_ban_wall_clock.c -ldl -o /usr/local/lib/account_ban_wall_clock.so \
 && sha256sum /usr/local/lib/account_ban_wall_clock.so \
 && apk del .native7-clock-build
ENV LD_PRELOAD=/usr/local/lib/account_ban_wall_clock.so
