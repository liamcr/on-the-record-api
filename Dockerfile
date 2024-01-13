FROM golang:latest
WORKDIR /app
COPY . .

ENV DB_HOST="ep-twilight-butterfly-21715046.us-east-2.aws.neon.tech"
ENV DB_USER="liamcrocket"

RUN go build -o main ./cmd/
EXPOSE 8080

CMD ["./main"]
