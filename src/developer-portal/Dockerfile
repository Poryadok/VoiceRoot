
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .

ARG VITE_VOICE_API_BASE=http://127.0.0.1:18080
ARG VITE_OAUTH_CLIENT_ID=voice-developer-portal
ARG VITE_OAUTH_DISABLED=false
ENV VITE_VOICE_API_BASE=${VITE_VOICE_API_BASE}
ENV VITE_OAUTH_CLIENT_ID=${VITE_OAUTH_CLIENT_ID}
ENV VITE_OAUTH_DISABLED=${VITE_OAUTH_DISABLED}
RUN npm run build

FROM nginx:1.27-alpine
COPY docker/nginx-default.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK CMD wget -qO- http://127.0.0.1/ || exit 1
