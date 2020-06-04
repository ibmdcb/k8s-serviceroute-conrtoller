  
FROM golang:alpine as builder
RUN mkdir /app 
ADD . /app/ 
WORKDIR /app 
RUN go build -o routecontroller . 

FROM alpine:edge
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/routecontroller .
EXPOSE 8080
ENV NSXTHOST="nsxt-api-host"
ENV NSXTUSER="user"
ENV NSXTPASS="pass"
ENV DEBUG=""
CMD ["/routecontroller"]