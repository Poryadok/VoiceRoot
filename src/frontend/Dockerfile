
# Match toolchain in repo README / CI Flutter job.
ARG FLUTTER_IMAGE=ghcr.io/cirruslabs/flutter:3.41.7
FROM ${FLUTTER_IMAGE} AS build

WORKDIR /app
COPY pubspec.yaml pubspec.lock ./
RUN flutter pub get
COPY . .

# Release artifacts have no environment endpoints. Compose may explicitly opt
# into its existing local compile-time configuration; that is not promotable.
ARG VOICE_RUNTIME_CONFIG_REQUIRED=true
ARG VOICE_API_BASE_URL=
ARG VOICE_LIVEKIT_URL=
RUN if [ "$VOICE_RUNTIME_CONFIG_REQUIRED" = true ]; then \
      test -z "$VOICE_API_BASE_URL" && test -z "$VOICE_LIVEKIT_URL" && \
      flutter build web --release --dart-define=VOICE_RUNTIME_CONFIG_REQUIRED=true; \
    elif [ "$VOICE_RUNTIME_CONFIG_REQUIRED" = false ]; then \
      test -n "$VOICE_API_BASE_URL" && test -n "$VOICE_LIVEKIT_URL" && \
      flutter build web --release --dart-define=VOICE_API_BASE_URL="$VOICE_API_BASE_URL" --dart-define=VOICE_LIVEKIT_URL="$VOICE_LIVEKIT_URL"; \
    else exit 1; fi
RUN python3 docker/version_web_assets.py build/web

FROM nginx:1.27-alpine
ARG SOURCE_REVISION=unqualified
LABEL org.opencontainers.image.revision="${SOURCE_REVISION}"
COPY docker/nginx-default.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/build/web /usr/share/nginx/html
EXPOSE 80
