FROM centurylink/ca-certs

COPY dest /
COPY certs /

EXPOSE 80

ENTRYPOINT ["/http_frontend"]
