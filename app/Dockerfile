FROM golang:1.20-alpine as build
RUN mkdir /app
COPY go.mod go.sum main.go /app/
RUN cd /app/ && go build -o main main.go
FROM alpine
COPY --from=build /app/main /main
ENTRYPOINT [ "/main" ]
