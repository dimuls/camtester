FROM alpine:latest

RUN apk add --no-cache ffmpeg

ADD ./camtester-prober /usr/bin/camtester-prober

ENTRYPOINT ["/usr/bin/camtester-prober"]
