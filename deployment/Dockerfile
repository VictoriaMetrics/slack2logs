ARG certs_image
ARG root_image
FROM $certs_image as certs
RUN apk --update --no-cache add ca-certificates

FROM $root_image
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
EXPOSE 8420
ENTRYPOINT ["/slack2logs-amd64-prod"]
COPY slack2logs-amd64-prod ./slack2logs-amd64-prod
