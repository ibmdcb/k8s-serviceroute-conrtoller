  
FROM golang:alpine as builder
RUN mkdir /app 
ADD . /app/ 
WORKDIR /app 
RUN go build -o routecontroller . 

FROM alpine:edge
RUN apk add --no-cache ca-certificates
RUN mkdir /app
WORKDIR /app
COPY --from=builder /app/routecontroller .

#EXPOSE 8080
ENV DEBUG=""
CMD ["/app/routecontroller"]
