FROM golang:1.23.0-alpine3.20

WORKDIR '/app'

COPY ./Process.go .

RUN go build Process.go

ENTRYPOINT [ "./Process" ]