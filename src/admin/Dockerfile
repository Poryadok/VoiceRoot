# syntax=docker/dockerfile:1

FROM node:22-alpine AS build

WORKDIR /app
COPY package.json ./
RUN npm install
COPY . .

# Baked into the bundle; browser on the host calls the published gateway port.
ARG VITE_VOICE_API_BASE=http://127.0.0.1:8080
ARG VITE_STAFF_TOKEN=compose-staff-token
ENV VITE_VOICE_API_BASE=${VITE_VOICE_API_BASE}
ENV VITE_STAFF_TOKEN=${VITE_STAFF_TOKEN}
RUN npm run build

FROM nginx:1.27-alpine
COPY docker/nginx-default.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
