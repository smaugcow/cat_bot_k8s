FROM golang:alpine AS build
ENV GO111MODULE=on
WORKDIR /app
COPY . .
RUN go build -o cat_bot .

FROM alpine:latest  
WORKDIR /app
COPY --from=build /app/cat_bot /app

CMD ["/app/cat_bot"]
