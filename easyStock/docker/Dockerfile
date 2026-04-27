# Build context: easyStock/
FROM node:20-alpine AS build
WORKDIR /build
COPY package.json package-lock.json* ./
RUN npm install --no-fund --no-audit
COPY tsconfig.json vite.config.ts index.html ./
COPY shared ./shared
COPY src ./src
RUN npm run build

FROM nginx:alpine
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /build/dist /usr/share/nginx/html
EXPOSE 80
