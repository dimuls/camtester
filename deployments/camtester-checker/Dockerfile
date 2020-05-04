FROM alpine:latest

RUN apk add --no-cache ffmpeg

COPY ./camtester-checker /usr/bin/camtester-checker

ENTRYPOINT ["/usr/bin/camtester-checker"]
