FROM golang:1.22.6

WORKDIR /

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./

COPY .env ./

RUN CGO_ENABLED=0 GOOS=linux go build -o main