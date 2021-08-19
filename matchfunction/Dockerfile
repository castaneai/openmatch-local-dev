FROM golang:1.16 as builder
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/matchfunction main.go

FROM gcr.io/distroless/static-debian10
COPY --from=builder /bin/matchfunction /bin/matchfunction
ENTRYPOINT ["/bin/matchfunction"]
