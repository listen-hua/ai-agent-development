FROM node:22-alpine AS build
WORKDIR /app
ARG VITE_IMAGE_AGENT_ENABLED=false
ENV VITE_IMAGE_AGENT_ENABLED=$VITE_IMAGE_AGENT_ENABLED
COPY package.json package-lock.json* .npmrc ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:1.29-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
