FROM scratch
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ADD --chmod=0755 --link https://github.com/middle-management/ace/releases/download/v0.6.0/ace-$TARGETOS-$TARGETARCH /bin/ace
COPY ./env /bin/aceenv
ENTRYPOINT ["/bin/ace","env","-e=/run/secrets/env","-i=/run/secrets/identity","--"]
CMD ["/bin/aceenv"]
