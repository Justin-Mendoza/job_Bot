FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /jobwatch .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /jobwatch /app/jobwatch
COPY companies.json /app/companies.json
CMD ["/app/jobwatch"]
