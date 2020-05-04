FROM alpine:latest

RUN apk add --no-cache ffmpeg

COPY ./camtester-prober /usr/bin/camtester-prober

ENTRYPOINT ["/usr/bin/camtester-prober"]
