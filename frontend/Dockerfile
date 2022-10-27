#### BUILDER IMAGE  ####
FROM docker.io/node:15.6 as builder
LABEL description="burn-in frontend builder image"

WORKDIR /opt/builder

COPY . .
RUN yarn install && \
    yarn build && \
    yarn cache clean

#### OUTPUT IMAGE ####
FROM docker.io/nginx:stable-alpine
LABEL description="burn-in frontend"

WORKDIR /usr/share/nginx/html

COPY --from=builder /opt/builder/build /usr/share/nginx/html

EXPOSE 80
