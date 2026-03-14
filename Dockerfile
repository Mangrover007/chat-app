FROM golang:1.26.1-alpine
WORKDIR /app
COPY ./app .
RUN go build -o /bin/app main.go

FROM alpine:3.23.3
COPY --from=0 /bin/app /bin/app
CMD ["./bin/app"]
EXPOSE 4200