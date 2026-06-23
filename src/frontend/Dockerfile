
# Match toolchain in repo README / CI Flutter job.
ARG FLUTTER_IMAGE=ghcr.io/cirruslabs/flutter:3.41.7
FROM ${FLUTTER_IMAGE} AS build

WORKDIR /app
COPY pubspec.yaml pubspec.lock ./
RUN flutter pub get
COPY . .

# Baked into the web bundle; browser on the host calls the published gateway port.
ARG VOICE_API_BASE_URL=http://127.0.0.1:8080
ARG VOICE_LIVEKIT_URL=ws://127.0.0.1:7880
RUN flutter build web --release --dart-define=VOICE_API_BASE_URL=${VOICE_API_BASE_URL} --dart-define=VOICE_LIVEKIT_URL=${VOICE_LIVEKIT_URL}

FROM nginx:1.27-alpine
COPY docker/nginx-default.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/build/web /usr/share/nginx/html
EXPOSE 80
