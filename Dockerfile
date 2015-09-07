FROM centurylink/ca-certs

COPY json_api /

EXPOSE 80

ENTRYPOINT ["/json_api"]
