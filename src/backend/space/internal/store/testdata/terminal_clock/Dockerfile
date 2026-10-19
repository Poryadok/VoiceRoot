FROM postgres:18-alpine
RUN apk add --no-cache libfaketime=0.9.13-r0 \
 && printf '+0\n' > /tmp/native-terminal-clock \
 && chown postgres:postgres /tmp/native-terminal-clock \
 && chmod 666 /tmp/native-terminal-clock
ENV LD_PRELOAD=/usr/lib/faketime/libfaketime.so.1 \
    FAKETIME_TIMESTAMP_FILE=/tmp/native-terminal-clock \
    FAKETIME_NO_CACHE=1 \
    FAKETIME_DONT_FAKE_MONOTONIC=1
