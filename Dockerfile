FROM centurylink/ca-certs

COPY http_frontend /
COPY certs /

EXPOSE 80

ENTRYPOINT ["/http_frontend"]
