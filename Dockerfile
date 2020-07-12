FROM scratch
COPY script/ca-certificates.crt /etc/ssl/certs/
COPY dist/traefik /
COPY script/default.key /etc/default.key
COPY script/default.cert /etc/default.cert
COPY script/dynamictls.yaml /etc/dynamictls.yaml
EXPOSE 80
VOLUME ["/tmp"]
ENTRYPOINT ["/traefik"]
