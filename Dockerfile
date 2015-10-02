FROM centurylink/ca-certs

COPY json_api /
COPY certs /

EXPOSE 80

ENTRYPOINT ["/json_api"]
