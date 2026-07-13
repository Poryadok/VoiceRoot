
FROM node:24-alpine AS build

WORKDIR /app
COPY package.json package-lock.json* ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install --include=dev; fi
COPY . .

ARG VITE_VOICE_API_BASE=http://127.0.0.1:8080
ARG VITE_OAUTH_CLIENT_ID=voice-admin
ARG VITE_OAUTH_DISABLED=false
ENV VITE_VOICE_API_BASE=${VITE_VOICE_API_BASE}
ENV VITE_OAUTH_CLIENT_ID=${VITE_OAUTH_CLIENT_ID}
ENV VITE_OAUTH_DISABLED=${VITE_OAUTH_DISABLED}
RUN npm run build

FROM nginx:1.27-alpine
COPY docker/nginx-default.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
HEALTHCHECK CMD wget -qO- http://127.0.0.1/health || exit 1
EXPOSE 80
