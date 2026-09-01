# --- build stage -------------------------------------------------------------
FROM node:24-alpine AS build

WORKDIR /app

# Install from the lockfile first so the layer is reused across source edits.
COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

# --- runtime stage -----------------------------------------------------------
FROM nginxinc/nginx-unprivileged:1.29-alpine

# The unprivileged image runs as uid 101 and listens on 8080.
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 8080
