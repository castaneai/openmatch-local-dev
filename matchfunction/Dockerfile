FROM golang:alpine as builder
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go build -o /bin/matchfunction main.go

FROM alpine
COPY --from=builder /bin/matchfunction /bin/matchfunction
ENV PORT 50502
EXPOSE 50502
ENTRYPOINT ["/bin/matchfunction"]
