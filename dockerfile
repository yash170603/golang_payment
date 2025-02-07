# FROM golang:1.21 as Builder
# WORKDIR /APP
# COPY go.mod go.sum ./

# RUN go mod download 

# COPY . .

# RUN     CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /APP/main .
FROM golang:1.22 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .



RUN go build -o main .

FROM gcr.io/distroless/base-debian12

WORKDIR /root/

COPY --from=builder /app/main .

COPY .env  ./.env


EXPOSE 8080

CMD ["./main"]