FROM alpine AS certs
RUN apk update && apk add ca-certificates

FROM busybox:stable-musl

ARG TARGETOS
ARG TARGETARCH

COPY --from=certs /etc/ssl/certs /etc/ssl/certs
COPY ./script/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /opt/santaizi/dashboard
COPY dist/dashboard-${TARGETOS}-${TARGETARCH} ./app

RUN mkdir -p /etc/santaizi /var/lib/santaizi-dashboard
VOLUME ["/var/lib/santaizi-dashboard"]
EXPOSE 80 5555
ARG TZ=Asia/Shanghai
ENV TZ=$TZ
ENTRYPOINT ["/entrypoint.sh"]
